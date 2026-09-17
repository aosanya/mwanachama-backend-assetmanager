package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/aosanya/mwanachama-backend-assetmanager/routes"
)

// raceTestManager mirrors this package's own newTestManager but pins the
// sqlite connection pool to exactly one physical connection. A pooled
// ":memory:" sqlite database hands each new connection a *different*,
// blank in-memory database unless pinned — see
// mwanachama-backend-git/CLAUDE.md's identical note on its own concurrency
// tests — which would make two concurrent HTTP requests land on two
// separate empty databases instead of actually racing on the same Hold
// row. Pinning to one physical connection does not serialize the two
// requests' business logic — each still does its own separate SELECT then
// UPDATE round trip against that one connection — so the read/write
// interleaving this test targets is still fully possible.
func raceTestManager(t *testing.T) mwanachamaassetmanager.AssetManager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB(): %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	tables := mwanachamaassetmanager.DefaultTableNames("race")
	if err := mwanachamaassetmanager.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	mgr, err := mwanachamaassetmanager.NewAssetManager(db, tables)
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}
	return mgr
}

func raceMux(am mwanachamaassetmanager.AssetManager) *http.ServeMux {
	m := http.NewServeMux()
	for _, rt := range routes.Routes(am) {
		m.Handle(rt.Pattern(""), rt.Handler)
	}
	return m
}

func postJSON(t *testing.T, client *http.Client, url string, body any) (*http.Response, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}
	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

// TestPinsA13_ConcurrentCommitAndReleaseBothSucceedOnSameHold pins board
// row A13: [assetManager.CommitHold] and [assetManager.ReleaseHold] each
// enforce the "reserved -> terminal" transition with a read-then-write
// pair (GetHold, then an unconditional `UPDATE ... WHERE id = ?` with no
// `AND status = 'reserved'` guard) — see hold.go. Two callers racing on the
// same Hold (an operator releasing it by hand while the expiry watchdog's
// sweep — or another operator — commits it, or vice versa) can both read
// status=reserved before either writes, so both writes land and BOTH calls
// return 200/nil-error even though [models.HoldStatus.CanTransitionTo]
// documents every transition out of reserved as terminal — a Hold is meant
// to be committed or released at most once, never both.
//
// This drives the race through the real `routes.Routes(am)` mux behind
// `httptest.NewServer`, with two real *http.Client requests fired
// concurrently — POST {holdID}/commit and POST {holdID}/release on the
// same hold — repeated across many fresh holds so the test does not depend
// on hitting one particular interleaving. Empirically (see this task's
// sweep notes) roughly half of all races land both requests as HTTP 200,
// and — because CommitHold does substantially more work than ReleaseHold's
// single UPDATE (it posts a real Movement first) — CommitHold's write
// almost always lands last, so the *persisted* row ends up "committed"
// even on every occasion where the ReleaseHold response body itself said
// "released". A caller who trusts ReleaseHold's own 200 response is being
// told something the database no longer agrees with a moment later.
//
// Once A13 is fixed (e.g. `UPDATE ... WHERE id = ? AND status =
// 'reserved'`, checking RowsAffected and returning
// ErrInvalidHoldStatusTransition on 0 rows affected, the same
// compare-and-swap shape hold.go's own CommitHold/ReleaseHold currently
// lack but mwanachama-backend-git's `advanceBranchHead` already uses), at
// most one of the two concurrent requests should ever succeed — this test
// should then be rewritten to assert exactly that instead of documenting
// the current double-success behavior.
func TestPinsA13_ConcurrentCommitAndReleaseBothSucceedOnSameHold(t *testing.T) {
	ctx := context.Background()
	const iterations = 40
	bothSucceeded := 0
	releaseResponseContradicted := 0

	for i := 0; i < iterations; i++ {
		am := raceTestManager(t)
		srv := httptest.NewServer(raceMux(am))
		client := srv.Client()

		loc, err := am.CreateLocation(ctx, models.Location{Name: "warehouse"})
		if err != nil {
			t.Fatalf("CreateLocation: %v", err)
		}
		asset, err := am.CreateAsset(ctx, models.Asset{Name: "widget", TrackingMode: models.AssetTrackingModeFungible})
		if err != nil {
			t.Fatalf("CreateAsset: %v", err)
		}
		if _, err := am.PostMovement(ctx, models.Movement{
			AssetID: asset.ID, Kind: models.MovementKindArrived,
			Quantity: 10, ToLocationID: loc.ID, PerformedBy: "tester",
		}); err != nil {
			t.Fatalf("PostMovement: %v", err)
		}
		hold, err := am.CreateHold(ctx, models.Hold{
			AssetID: asset.ID, LocationID: loc.ID, Quantity: 5,
			ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
			PlacedBy:  "tester",
		})
		if err != nil {
			t.Fatalf("CreateHold: %v", err)
		}

		var wg sync.WaitGroup
		var commitStatus, releaseStatus int
		wg.Add(2)
		go func() {
			defer wg.Done()
			resp, _ := postJSON(t, client, fmt.Sprintf("%s/holds/%s/commit", srv.URL, hold.ID), map[string]any{
				"quantity": 5, "kind": "departed", "performed_by": "committer",
			})
			commitStatus = resp.StatusCode
		}()
		go func() {
			defer wg.Done()
			resp, _ := postJSON(t, client, fmt.Sprintf("%s/holds/%s/release", srv.URL, hold.ID), nil)
			releaseStatus = resp.StatusCode
		}()
		wg.Wait()

		if commitStatus == http.StatusOK && releaseStatus == http.StatusOK {
			bothSucceeded++
			final, err := am.GetHold(ctx, hold.ID)
			if err != nil {
				t.Fatalf("GetHold: %v", err)
			}
			if final.Status != models.HoldStatusReleased {
				releaseResponseContradicted++
			}
		}

		srv.Close()
	}

	t.Logf("both commit and release returned HTTP 200 on the same hold: %d/%d iterations", bothSucceeded, iterations)
	t.Logf("of those, the release response's own claim was immediately contradicted by persisted state: %d/%d", releaseResponseContradicted, bothSucceeded)

	if bothSucceeded == 0 {
		t.Fatalf("expected at least one iteration where both commit and release returned 200 on the same hold (race not reproduced across %d iterations — current broken behavior no longer confirmed)", iterations)
	}
}

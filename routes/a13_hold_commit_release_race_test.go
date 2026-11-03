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

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/aosanya/mwanachama-backend-assetmanager/routes"
)

func raceTestManager(t *testing.T) mwanachamaassetmanager.AssetManager {
	t.Helper()
	return managerOn(t, openDB(t, true), testSpec(t))
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

func TestA13_CommitAndReleaseRacingOnOneHoldNeverBothSucceed(t *testing.T) {
	ctx := context.Background()
	const iterations = 40

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
			t.Fatalf("iteration %d: commit and release both answered 200 on one hold", i)
		}
		final, err := am.GetHold(ctx, hold.ID)
		if err != nil {
			t.Fatalf("GetHold: %v", err)
		}
		if commitStatus == http.StatusOK && final.Status != models.HoldStatusCommitted {
			t.Fatalf("iteration %d: commit answered 200 but the hold is %q", i, final.Status)
		}
		if releaseStatus == http.StatusOK && final.Status != models.HoldStatusReleased {
			t.Fatalf("iteration %d: release answered 200 but the hold is %q", i, final.Status)
		}
		srv.Close()
	}
}

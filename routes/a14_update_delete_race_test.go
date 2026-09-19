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

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

// raceTestManagerAndDB is raceTestManager (see a13's own test file) but
// also hands back the underlying *gorm.DB and table names, so a test can
// peek at persisted row state (deleted + a content field together) that no
// AssetManager method exposes directly — GetAsset/GetLocation/ListAssets/
// ListLocations all filter deleted=false, so a deleted row's current
// content is otherwise unobservable through the public API.
func raceTestManagerAndDB(t *testing.T) (mwanachamaassetmanager.AssetManager, *gorm.DB, mwanachamaassetmanager.TableNames) {
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

	tables := mwanachamaassetmanager.DefaultTableNames("a14race")
	if err := mwanachamaassetmanager.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	mgr, err := mwanachamaassetmanager.NewAssetManager(db, tables)
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}
	return mgr, db, tables
}

func patchJSON(t *testing.T, client *http.Client, url string, body any) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		t.Fatalf("encode: %v", err)
	}
	req, err := http.NewRequest(http.MethodPatch, url, &buf)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	return resp
}

func deleteReq(t *testing.T, client *http.Client, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	return resp
}

// TestPinsA14_UpdateAssetCanWriteIntoAnAlreadyDeletedRow pins board row
// A14: [assetManager.UpdateAsset] reads the current row through GetAsset
// (`WHERE id = ? AND deleted = false`) but writes through a bare
// `Updates(...).Where("id = ?", a.ID)` with no `deleted` guard at all — see
// asset.go. A DeleteAsset that lands in the gap between UpdateAsset's own
// read and its own write is invisible to the write: the update still
// applies its new Name/Category/etc to the now-deleted row, and UpdateAsset
// returns 200 with no error, even though the row is (and stays) soft
// deleted.
//
// Driven through the real `routes.Routes(am)` mux behind `httptest.
// NewServer`, with two real *http.Client requests fired concurrently —
// PATCH {assetID} racing DELETE {assetID} — repeated across 200 fresh
// assets so the test does not depend on hitting one particular
// interleaving. This race is far wider than A13's (no extra work happens
// between DeleteAsset's own read and write), so it reproduces on almost
// every iteration: empirically 197-199/200 iterations land both requests
// as success, and on every one of those the persisted row is confirmed
// `deleted=true` with the racing PATCH's new Name applied anyway (verified
// by reading the raw row directly, since GetAsset/ListAssets would filter
// it out and report nothing there).
//
// Once A14 is fixed (e.g. `Updates(...).Where("id = ? AND deleted = ?",
// a.ID, false)`, checking RowsAffected and returning ErrAssetNotFound on 0
// rows affected — the same compare-and-swap shape A13 and
// mwanachama-backend-git's `advanceBranchHead` already use), the deleted
// count here should drop to 0 and this test should be rewritten to assert
// exactly that.
func TestPinsA14_UpdateAssetCanWriteIntoAnAlreadyDeletedRow(t *testing.T) {
	ctx := context.Background()
	const iterations = 200
	bothSucceeded := 0
	writtenIntoDeletedRow := 0

	for i := 0; i < iterations; i++ {
		am, db, tables := raceTestManagerAndDB(t)
		srv := httptest.NewServer(raceMux(am))
		client := srv.Client()

		asset, err := am.CreateAsset(ctx, models.Asset{Name: "original", TrackingMode: models.AssetTrackingModeFungible})
		if err != nil {
			t.Fatalf("CreateAsset: %v", err)
		}

		var wg sync.WaitGroup
		var updateStatus, deleteStatus int
		wg.Add(2)
		go func() {
			defer wg.Done()
			resp := patchJSON(t, client, fmt.Sprintf("%s/assets/%s", srv.URL, asset.ID), map[string]any{
				"name": "raced-update",
			})
			updateStatus = resp.StatusCode
		}()
		go func() {
			defer wg.Done()
			resp := deleteReq(t, client, fmt.Sprintf("%s/assets/%s", srv.URL, asset.ID))
			deleteStatus = resp.StatusCode
		}()
		wg.Wait()

		if updateStatus == http.StatusOK && deleteStatus == http.StatusNoContent {
			bothSucceeded++
			var row struct {
				Name    string
				Deleted bool
			}
			if err := db.Table(tables.Assets).Select("name, deleted").Where("id = ?", asset.ID).Scan(&row).Error; err != nil {
				t.Fatalf("raw select: %v", err)
			}
			if row.Deleted && row.Name == "raced-update" {
				writtenIntoDeletedRow++
			}
		}

		srv.Close()
	}

	t.Logf("UpdateAsset returned 200 concurrently with DeleteAsset returning 204: %d/%d iterations", bothSucceeded, iterations)
	t.Logf("of those, the persisted row ended up deleted=true with the racing update's Name silently applied anyway: %d/%d", writtenIntoDeletedRow, bothSucceeded)

	if writtenIntoDeletedRow == 0 {
		t.Fatalf("expected at least one iteration where a soft-deleted Asset row still absorbed a racing update (race not reproduced across %d iterations — current broken behavior no longer confirmed)", iterations)
	}
}

// TestPinsA14_UpdateLocationCanWriteIntoAnAlreadyDeletedRow is A14's second
// instance: [assetManager.UpdateLocation]/[assetManager.DeleteLocation] in
// location.go share the identical shape as UpdateAsset/DeleteAsset above —
// GetLocation filters deleted=false, but UpdateLocation's own write is a
// bare `Updates(...).Where("id = ?", l.ID)` with no deleted guard.
func TestPinsA14_UpdateLocationCanWriteIntoAnAlreadyDeletedRow(t *testing.T) {
	ctx := context.Background()
	const iterations = 200
	bothSucceeded := 0
	writtenIntoDeletedRow := 0

	for i := 0; i < iterations; i++ {
		am, db, tables := raceTestManagerAndDB(t)
		srv := httptest.NewServer(raceMux(am))
		client := srv.Client()

		loc, err := am.CreateLocation(ctx, models.Location{Name: "original"})
		if err != nil {
			t.Fatalf("CreateLocation: %v", err)
		}

		var wg sync.WaitGroup
		var updateStatus, deleteStatus int
		wg.Add(2)
		go func() {
			defer wg.Done()
			resp := patchJSON(t, client, fmt.Sprintf("%s/locations/%s", srv.URL, loc.ID), map[string]any{
				"name": "raced-update",
			})
			updateStatus = resp.StatusCode
		}()
		go func() {
			defer wg.Done()
			resp := deleteReq(t, client, fmt.Sprintf("%s/locations/%s", srv.URL, loc.ID))
			deleteStatus = resp.StatusCode
		}()
		wg.Wait()

		if updateStatus == http.StatusOK && deleteStatus == http.StatusNoContent {
			bothSucceeded++
			var row struct {
				Name    string
				Deleted bool
			}
			if err := db.Table(tables.Locations).Select("name, deleted").Where("id = ?", loc.ID).Scan(&row).Error; err != nil {
				t.Fatalf("raw select: %v", err)
			}
			if row.Deleted && row.Name == "raced-update" {
				writtenIntoDeletedRow++
			}
		}

		srv.Close()
	}

	t.Logf("UpdateLocation returned 200 concurrently with DeleteLocation returning 204: %d/%d iterations", bothSucceeded, iterations)
	t.Logf("of those, the persisted row ended up deleted=true with the racing update's Name silently applied anyway: %d/%d", writtenIntoDeletedRow, bothSucceeded)

	if writtenIntoDeletedRow == 0 {
		t.Fatalf("expected at least one iteration where a soft-deleted Location row still absorbed a racing update (race not reproduced across %d iterations — current broken behavior no longer confirmed)", iterations)
	}
}

package routes_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func raceTestManagerAndDB(t *testing.T) (mwanachamaassetmanager.AssetManager, *gorm.DB, *spec.Spec) {
	t.Helper()
	db := openDB(t, true)
	s := testSpec(t)
	return managerOn(t, db, s), db, s
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

func deleteRowBeforeTheNextUpdate(db *gorm.DB, table, id string) {
	fired := false
	db.Callback().Update().Before("gorm:update").Register("a14:delete_first", func(tx *gorm.DB) {
		if fired {
			return
		}
		fired = true
		tx.Session(&gorm.Session{NewDB: true}).Exec("UPDATE "+table+" SET deleted = ? WHERE id = ?", true, id)
	})
}

func TestA14_UpdateAssetLosesToADeleteThatLandsBetweenItsReadAndItsWrite(t *testing.T) {
	ctx := context.Background()
	am, db, s := raceTestManagerAndDB(t)
	asset, err := am.CreateAsset(ctx, models.Asset{Name: "original", TrackingMode: models.AssetTrackingModeFungible})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	deleteRowBeforeTheNextUpdate(db, tableFor(t, s, "asset"), asset.ID)

	asset.Name = "raced-update"
	if _, err := am.UpdateAsset(ctx, asset); !errors.Is(err, mwanachamaassetmanager.ErrAssetNotFound) {
		t.Fatalf("UpdateAsset after a delete landed in its gap: %v, want ErrAssetNotFound", err)
	}
	var name string
	if err := db.Table(tableFor(t, s, "asset")).Select("name").Where("id = ?", asset.ID).Scan(&name).Error; err != nil {
		t.Fatalf("raw select: %v", err)
	}
	if name != "original" {
		t.Fatalf("the update wrote %q into a deleted row", name)
	}
}

func TestA14_UpdateLocationLosesToADeleteThatLandsBetweenItsReadAndItsWrite(t *testing.T) {
	ctx := context.Background()
	am, db, s := raceTestManagerAndDB(t)
	loc, err := am.CreateLocation(ctx, models.Location{Name: "original"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	deleteRowBeforeTheNextUpdate(db, tableFor(t, s, "location"), loc.ID)

	loc.Name = "raced-update"
	if _, err := am.UpdateLocation(ctx, loc); !errors.Is(err, mwanachamaassetmanager.ErrLocationNotFound) {
		t.Fatalf("UpdateLocation after a delete landed in its gap: %v, want ErrLocationNotFound", err)
	}
	var name string
	if err := db.Table(tableFor(t, s, "location")).Select("name").Where("id = ?", loc.ID).Scan(&name).Error; err != nil {
		t.Fatalf("raw select: %v", err)
	}
	if name != "original" {
		t.Fatalf("the update wrote %q into a deleted row", name)
	}
}

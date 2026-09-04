package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/aosanya/mwanachama-backend-assetmanager/routes"
)

func newAsset(t *testing.T, am mwanachamaassetmanager.AssetManager, name string, mode models.AssetTrackingMode) models.Asset {
	t.Helper()
	a, err := am.CreateAsset(context.Background(), models.Asset{Name: name, TrackingMode: mode})
	if err != nil {
		t.Fatalf("seed CreateAsset: %v", err)
	}
	return a
}

func TestCreateAsset(t *testing.T) {
	am := newTestManager(t)
	handler := routes.CreateAsset(am)

	body := `{"name":"Rice, 50kg bag","tracking_mode":"fungible"}`
	req := httptest.NewRequest(http.MethodPost, "/assets", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out models.Asset
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID == "" || out.Name != "Rice, 50kg bag" {
		t.Fatalf("unexpected asset: %+v", out)
	}
}

func TestCreateAsset_InvalidTrackingMode(t *testing.T) {
	am := newTestManager(t)
	handler := routes.CreateAsset(am)

	req := httptest.NewRequest(http.MethodPost, "/assets", strings.NewReader(`{"name":"Mystery","tracking_mode":"bogus"}`))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGetAsset_NotFound(t *testing.T) {
	am := newTestManager(t)
	handler := routes.GetAsset(am)

	req := withPathValue(httptest.NewRequest(http.MethodGet, "/assets/nope", nil), "assetID", "nope")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateAsset(t *testing.T) {
	am := newTestManager(t)
	a := newAsset(t, am, "Rice", models.AssetTrackingModeFungible)
	handler := routes.UpdateAsset(am)

	body := `{"name":"Rice, 25kg bag","category":"grocery"}`
	req := withPathValue(httptest.NewRequest(http.MethodPatch, "/assets/"+a.ID, strings.NewReader(body)), "assetID", a.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out models.Asset
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Name != "Rice, 25kg bag" || out.Category != "grocery" {
		t.Fatalf("unexpected asset: %+v", out)
	}
}

func TestDeleteAsset(t *testing.T) {
	am := newTestManager(t)
	a := newAsset(t, am, "Rice", models.AssetTrackingModeFungible)
	handler := routes.DeleteAsset(am)

	req := withPathValue(httptest.NewRequest(http.MethodDelete, "/assets/"+a.ID, nil), "assetID", a.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestListAssets_FilterByCategory(t *testing.T) {
	am := newTestManager(t)
	if _, err := am.CreateAsset(context.Background(), models.Asset{Name: "Rice", TrackingMode: models.AssetTrackingModeFungible, Category: "grocery"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := am.CreateAsset(context.Background(), models.Asset{Name: "Bessie", TrackingMode: models.AssetTrackingModeSerialized, SerialTag: "COW-1", Category: "livestock"}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	handler := routes.ListAssets(am)

	req := httptest.NewRequest(http.MethodGet, "/assets?category=grocery", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out []models.Asset
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out) != 1 || out[0].Name != "Rice" {
		t.Fatalf("expected [Rice], got %+v", out)
	}
}

func TestGetAssetBalance(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", models.AssetTrackingModeFungible)
	if _, err := am.PostMovement(context.Background(), models.Movement{
		AssetID: a.ID, Kind: models.MovementKindArrived, Quantity: 100, ToLocationID: loc.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("seed PostMovement: %v", err)
	}
	handler := routes.GetAssetBalance(am)

	req := withPathValue(httptest.NewRequest(http.MethodGet, "/assets/"+a.ID+"/balance?location_id="+loc.ID, nil), "assetID", a.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out map[string]int64
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out["balance"] != 100 {
		t.Fatalf("expected balance 100, got %+v", out)
	}
}

func TestGetAssetBalance_MissingLocationID(t *testing.T) {
	am := newTestManager(t)
	a := newAsset(t, am, "Rice", models.AssetTrackingModeFungible)
	handler := routes.GetAssetBalance(am)

	req := withPathValue(httptest.NewRequest(http.MethodGet, "/assets/"+a.ID+"/balance", nil), "assetID", a.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

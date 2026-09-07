package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/routes"
)

func TestPostMovement(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", mwanachamaassetmanager.AssetTrackingModeFungible)
	handler := routes.PostMovement(am)

	body := `{"asset_id":"` + a.ID + `","kind":"arrived","quantity":100,"to_location_id":"` + loc.ID + `","performed_by":"` + testActor + `"}`
	req := httptest.NewRequest(http.MethodPost, "/movements", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out mwanachamaassetmanager.Movement
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID == "" || out.Quantity != 100 {
		t.Fatalf("unexpected movement: %+v", out)
	}
}

func TestPostMovement_InvalidShape(t *testing.T) {
	am := newTestManager(t)
	a := newAsset(t, am, "Rice", mwanachamaassetmanager.AssetTrackingModeFungible)
	handler := routes.PostMovement(am)

	body := `{"asset_id":"` + a.ID + `","kind":"arrived","quantity":0,"performed_by":"` + testActor + `"}`
	req := httptest.NewRequest(http.MethodPost, "/movements", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestPostMovement_AssetNotFound(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	handler := routes.PostMovement(am)

	body := `{"asset_id":"nope","kind":"arrived","quantity":1,"to_location_id":"` + loc.ID + `","performed_by":"` + testActor + `"}`
	req := httptest.NewRequest(http.MethodPost, "/movements", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGetMovement_NotFound(t *testing.T) {
	am := newTestManager(t)
	handler := routes.GetMovement(am)

	req := withPathValue(httptest.NewRequest(http.MethodGet, "/movements/nope", nil), "movementID", "nope")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestListMovements_FilterByAssetID(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", mwanachamaassetmanager.AssetTrackingModeFungible)
	if _, err := am.PostMovement(context.Background(), mwanachamaassetmanager.Movement{
		AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 10, ToLocationID: loc.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	handler := routes.ListMovements(am)

	req := httptest.NewRequest(http.MethodGet, "/movements?asset_id="+a.ID, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out []mwanachamaassetmanager.Movement
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out) != 1 {
		t.Fatalf("expected 1 movement, got %+v", out)
	}
}

func TestReverseMovement(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", mwanachamaassetmanager.AssetTrackingModeFungible)
	mv, err := am.PostMovement(context.Background(), mwanachamaassetmanager.Movement{
		AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 40, ToLocationID: loc.ID, PerformedBy: testActor,
	})
	if err != nil {
		t.Fatalf("seed PostMovement: %v", err)
	}
	handler := routes.ReverseMovement(am)

	body := `{"performed_by":"` + testActor + `","note":"counted wrong"}`
	req := withPathValue(httptest.NewRequest(http.MethodPost, "/movements/"+mv.ID+"/reverse", strings.NewReader(body)), "movementID", mv.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out mwanachamaassetmanager.Movement
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Kind != mwanachamaassetmanager.MovementKindReversed || out.ReversesMovementID != mv.ID {
		t.Fatalf("unexpected reversal: %+v", out)
	}
}

func TestReverseMovement_MissingPerformedBy(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", mwanachamaassetmanager.AssetTrackingModeFungible)
	mv, err := am.PostMovement(context.Background(), mwanachamaassetmanager.Movement{
		AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 5, ToLocationID: loc.ID, PerformedBy: testActor,
	})
	if err != nil {
		t.Fatalf("seed PostMovement: %v", err)
	}
	handler := routes.ReverseMovement(am)

	req := withPathValue(httptest.NewRequest(http.MethodPost, "/movements/"+mv.ID+"/reverse", strings.NewReader(`{}`)), "movementID", mv.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

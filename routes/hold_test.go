package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/aosanya/mwanachama-backend-assetmanager/routes"
)

func futureRFC3339(t *testing.T) string {
	t.Helper()
	return time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
}

func TestCreateHold(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", models.AssetTrackingModeFungible)
	if _, err := am.PostMovement(context.Background(), models.Movement{
		AssetID: a.ID, Kind: models.MovementKindArrived, Quantity: 20, ToLocationID: loc.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("seed PostMovement: %v", err)
	}
	handler := routes.CreateHold(am)

	body := `{"asset_id":"` + a.ID + `","location_id":"` + loc.ID + `","quantity":10,"expires_at":"` + futureRFC3339(t) + `","placed_by":"` + testActor + `"}`
	req := httptest.NewRequest(http.MethodPost, "/holds", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out models.Hold
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID == "" || out.Status != models.HoldStatusReserved {
		t.Fatalf("unexpected hold: %+v", out)
	}
}

func TestCreateHold_InsufficientAvailable(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", models.AssetTrackingModeFungible)
	if _, err := am.PostMovement(context.Background(), models.Movement{
		AssetID: a.ID, Kind: models.MovementKindArrived, Quantity: 5, ToLocationID: loc.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("seed PostMovement: %v", err)
	}
	handler := routes.CreateHold(am)

	body := `{"asset_id":"` + a.ID + `","location_id":"` + loc.ID + `","quantity":10,"expires_at":"` + futureRFC3339(t) + `","placed_by":"` + testActor + `"}`
	req := httptest.NewRequest(http.MethodPost, "/holds", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGetHold_NotFound(t *testing.T) {
	am := newTestManager(t)
	handler := routes.GetHold(am)

	req := withPathValue(httptest.NewRequest(http.MethodGet, "/holds/nope", nil), "holdID", "nope")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestCommitHold(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", models.AssetTrackingModeFungible)
	if _, err := am.PostMovement(context.Background(), models.Movement{
		AssetID: a.ID, Kind: models.MovementKindArrived, Quantity: 20, ToLocationID: loc.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("seed PostMovement: %v", err)
	}
	h, err := am.CreateHold(context.Background(), models.Hold{
		AssetID: a.ID, LocationID: loc.ID, Quantity: 10, ExpiresAt: futureRFC3339(t), PlacedBy: testActor,
	})
	if err != nil {
		t.Fatalf("seed CreateHold: %v", err)
	}
	handler := routes.CommitHold(am)

	body := `{"quantity":10,"kind":"departed","performed_by":"` + testActor + `"}`
	req := withPathValue(httptest.NewRequest(http.MethodPost, "/holds/"+h.ID+"/commit", strings.NewReader(body)), "holdID", h.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Hold     models.Hold     `json:"hold"`
		Movement models.Movement `json:"movement"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Hold.Status != models.HoldStatusCommitted || out.Movement.Kind != models.MovementKindDeparted {
		t.Fatalf("unexpected commit response: %+v", out)
	}
}

func TestReleaseHold(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", models.AssetTrackingModeFungible)
	if _, err := am.PostMovement(context.Background(), models.Movement{
		AssetID: a.ID, Kind: models.MovementKindArrived, Quantity: 20, ToLocationID: loc.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("seed PostMovement: %v", err)
	}
	h, err := am.CreateHold(context.Background(), models.Hold{
		AssetID: a.ID, LocationID: loc.ID, Quantity: 20, ExpiresAt: futureRFC3339(t), PlacedBy: testActor,
	})
	if err != nil {
		t.Fatalf("seed CreateHold: %v", err)
	}
	handler := routes.ReleaseHold(am)

	req := withPathValue(httptest.NewRequest(http.MethodPost, "/holds/"+h.ID+"/release", nil), "holdID", h.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out models.Hold
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Status != models.HoldStatusReleased {
		t.Fatalf("unexpected hold: %+v", out)
	}
}

func TestListHolds_FilterByStatus(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", models.AssetTrackingModeFungible)
	if _, err := am.PostMovement(context.Background(), models.Movement{
		AssetID: a.ID, Kind: models.MovementKindArrived, Quantity: 20, ToLocationID: loc.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("seed PostMovement: %v", err)
	}
	if _, err := am.CreateHold(context.Background(), models.Hold{
		AssetID: a.ID, LocationID: loc.ID, Quantity: 5, ExpiresAt: futureRFC3339(t), PlacedBy: testActor,
	}); err != nil {
		t.Fatalf("seed CreateHold: %v", err)
	}
	handler := routes.ListHolds(am)

	req := httptest.NewRequest(http.MethodGet, "/holds?status=reserved", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out []models.Hold
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out) != 1 {
		t.Fatalf("expected 1 hold, got %+v", out)
	}
}

func TestListHoldsExpiredAsOf(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	a := newAsset(t, am, "Rice", models.AssetTrackingModeFungible)
	if _, err := am.PostMovement(context.Background(), models.Movement{
		AssetID: a.ID, Kind: models.MovementKindArrived, Quantity: 20, ToLocationID: loc.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("seed PostMovement: %v", err)
	}
	nearFuture := time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	h, err := am.CreateHold(context.Background(), models.Hold{
		AssetID: a.ID, LocationID: loc.ID, Quantity: 5, ExpiresAt: nearFuture, PlacedBy: testActor,
	})
	if err != nil {
		t.Fatalf("seed CreateHold: %v", err)
	}
	handler := routes.ListHoldsExpiredAsOf(am)

	cutoff := time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/holds/expired?cutoff="+cutoff, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out []models.Hold
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out) != 1 || out[0].ID != h.ID {
		t.Fatalf("expected [hold], got %+v", out)
	}
}

func TestListHoldsExpiredAsOf_MissingCutoff(t *testing.T) {
	am := newTestManager(t)
	handler := routes.ListHoldsExpiredAsOf(am)

	req := httptest.NewRequest(http.MethodGet, "/holds/expired", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

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

// a12Mux builds a real net/http.ServeMux from routes.Routes, mirroring
// this package's other real-mux fixtures.
func a12Mux(am mwanachamaassetmanager.AssetManager) *http.ServeMux {
	m := http.NewServeMux()
	for _, rt := range routes.Routes(am) {
		m.Handle(rt.Pattern(""), rt.Handler)
	}
	return m
}

// Pins board row A12: every Create<Type> method (CreateAsset, CreateLocation,
// CreateHold, PostMovement) honors a caller-supplied "id" instead of always
// minting its own — directly contradicting each type's own doc comment
// ("ID is ... Set by the backend on creation; callers should leave it empty
// in create requests", see models/asset.go, location.go, hold.go,
// movement.go). The row struct's BeforeCreate hook only mints a UUID
// `if r.ID == ""` (gormstore), and none of the four manager methods clears
// the caller's value first.
//
// Once A12 is fixed (each Create<Type> clearing .ID before building the
// row, the same fix mwanachama-backend-agency's AG21 and
// mwanachama-backend-taskmanager's W11 applied), these four assertions
// flip: the returned id must NOT equal the caller-supplied value.
func TestCreateAsset_PinsCallerSuppliedIDIsHonored(t *testing.T) {
	am := newTestManager(t)
	m := a12Mux(am)

	const wanted = "attacker-chosen-asset-id"
	req := httptest.NewRequest("POST", "/assets", strings.NewReader(
		`{"id":"`+wanted+`","name":"probe","tracking_mode":"fungible"}`))
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create asset: got %d, body %s", rec.Code, rec.Body.String())
	}
	var out mwanachamaassetmanager.Asset
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID != wanted {
		t.Fatalf("A12 appears fixed for CreateAsset (got %q) — update this pin", out.ID)
	}
}

// Pins A12's second half on Asset: re-POSTing the same id hits the sqlite/
// Postgres primary-key constraint, which this repo's errors.go has no
// sentinel for at all (unlike mwanachama-backend-actor's ErrDuplicateID or
// mwanachama-backend-comm's ErrConflict) — so writeAssetErr's default arm
// answers an opaque 500 instead of a clean 409.
func TestCreateAsset_PinsDuplicateCallerSuppliedIDReturns500NotConflict(t *testing.T) {
	am := newTestManager(t)
	m := a12Mux(am)

	body := `{"id":"attacker-chosen-asset-id-2","name":"probe","tracking_mode":"fungible"}`
	first := httptest.NewRequest("POST", "/assets", strings.NewReader(body))
	firstRec := httptest.NewRecorder()
	m.ServeHTTP(firstRec, first)
	if firstRec.Code != http.StatusCreated {
		t.Fatalf("first create: got %d, body %s", firstRec.Code, firstRec.Body.String())
	}

	second := httptest.NewRequest("POST", "/assets", strings.NewReader(body))
	secondRec := httptest.NewRecorder()
	m.ServeHTTP(secondRec, second)
	if secondRec.Code != http.StatusInternalServerError {
		t.Fatalf("A12 appears fixed: duplicate id now returns %d (body %s), not the unmapped 500 this pin expects", secondRec.Code, secondRec.Body.String())
	}
}

func TestCreateLocation_PinsCallerSuppliedIDIsHonored(t *testing.T) {
	am := newTestManager(t)
	m := a12Mux(am)

	const wanted = "attacker-chosen-location-id"
	req := httptest.NewRequest("POST", "/locations", strings.NewReader(
		`{"id":"`+wanted+`","name":"probe loc"}`))
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create location: got %d, body %s", rec.Code, rec.Body.String())
	}
	var out mwanachamaassetmanager.Location
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID != wanted {
		t.Fatalf("A12 appears fixed for CreateLocation (got %q) — update this pin", out.ID)
	}
}

func TestCreateHold_PinsCallerSuppliedIDIsHonored(t *testing.T) {
	am := newTestManager(t)
	m := a12Mux(am)

	asset := newAsset(t, am, "probe asset", mwanachamaassetmanager.AssetTrackingModeFungible)
	loc, err := am.CreateLocation(context.Background(), mwanachamaassetmanager.Location{Name: "probe loc"})
	if err != nil {
		t.Fatalf("seed location: %v", err)
	}
	if _, err := am.PostMovement(context.Background(), mwanachamaassetmanager.Movement{
		AssetID: asset.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 10,
		ToLocationID: loc.ID, PerformedBy: "tester",
	}); err != nil {
		t.Fatalf("seed movement: %v", err)
	}

	const wanted = "attacker-chosen-hold-id"
	req := httptest.NewRequest("POST", "/holds", strings.NewReader(
		`{"id":"`+wanted+`","asset_id":"`+asset.ID+`","location_id":"`+loc.ID+`","quantity":1,"placed_by":"tester","expires_at":"2099-01-01T00:00:00Z"}`))
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create hold: got %d, body %s", rec.Code, rec.Body.String())
	}
	var out mwanachamaassetmanager.Hold
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID != wanted {
		t.Fatalf("A12 appears fixed for CreateHold (got %q) — update this pin", out.ID)
	}
}

func TestPostMovement_PinsCallerSuppliedIDIsHonored(t *testing.T) {
	am := newTestManager(t)
	m := a12Mux(am)

	asset := newAsset(t, am, "probe asset", mwanachamaassetmanager.AssetTrackingModeFungible)
	loc, err := am.CreateLocation(context.Background(), mwanachamaassetmanager.Location{Name: "probe loc"})
	if err != nil {
		t.Fatalf("seed location: %v", err)
	}

	const wanted = "attacker-chosen-movement-id"
	req := httptest.NewRequest("POST", "/movements", strings.NewReader(
		`{"id":"`+wanted+`","asset_id":"`+asset.ID+`","kind":"arrived","quantity":5,"to_location_id":"`+loc.ID+`","performed_by":"tester"}`))
	rec := httptest.NewRecorder()
	m.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("post movement: got %d, body %s", rec.Code, rec.Body.String())
	}
	var out mwanachamaassetmanager.Movement
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ID != wanted {
		t.Fatalf("A12 appears fixed for PostMovement (got %q) — update this pin", out.ID)
	}
}

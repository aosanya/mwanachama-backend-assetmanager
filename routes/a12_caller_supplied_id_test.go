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

// Board row A12: every Create<Type> method (CreateAsset, CreateLocation,
// CreateHold, PostMovement) clears a caller-supplied "id" and mints its own,
// as each type's doc comment promises.
func TestCreateAsset_IgnoresCallerSuppliedID(t *testing.T) {
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
	if out.ID == wanted {
		t.Fatalf("a caller-supplied id was honoured (got %q) — the server must mint its own", out.ID)
	}
}

// A12's second half on Asset: re-POSTing the same body mints a second
// distinct row instead of colliding on the primary key.
func TestCreateAsset_DuplicateCallerSuppliedIDMintsDistinctIDs(t *testing.T) {
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
	if secondRec.Code != http.StatusCreated {
		t.Fatalf("second create: got %d, body %s", secondRec.Code, secondRec.Body.String())
	}
	var a, b mwanachamaassetmanager.Asset
	if err := json.Unmarshal(firstRec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode first: %v", err)
	}
	if err := json.Unmarshal(secondRec.Body.Bytes(), &b); err != nil {
		t.Fatalf("decode second: %v", err)
	}
	if a.ID == b.ID || a.ID == "attacker-chosen-asset-id-2" || b.ID == "attacker-chosen-asset-id-2" {
		t.Fatalf("expected two distinct server-minted ids, got %q and %q", a.ID, b.ID)
	}
}

func TestCreateLocation_IgnoresCallerSuppliedID(t *testing.T) {
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
	if out.ID == wanted {
		t.Fatalf("a caller-supplied id was honoured (got %q) — the server must mint its own", out.ID)
	}
}

func TestCreateHold_IgnoresCallerSuppliedID(t *testing.T) {
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
	if out.ID == wanted {
		t.Fatalf("a caller-supplied id was honoured (got %q) — the server must mint its own", out.ID)
	}
}

func TestPostMovement_IgnoresCallerSuppliedID(t *testing.T) {
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
	if out.ID == wanted {
		t.Fatalf("a caller-supplied id was honoured (got %q) — the server must mint its own", out.ID)
	}
}

package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/routes"
)

func newTestManager(t *testing.T) mwanachamaassetmanager.AssetManager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	tables := mwanachamaassetmanager.DefaultTableNames("routes_test")
	if err := mwanachamaassetmanager.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	mgr, err := mwanachamaassetmanager.NewAssetManager(db, tables)
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}
	return mgr
}

func withPathValue(req *http.Request, key, value string) *http.Request {
	req.SetPathValue(key, value)
	return req
}

const testActor = "actor-1"

func TestCreateLocation(t *testing.T) {
	am := newTestManager(t)
	handler := routes.CreateLocation(am)

	req := httptest.NewRequest(http.MethodPost, "/locations", strings.NewReader(`{"name":"Warehouse","kind":"warehouse"}`))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out mwanachamaassetmanager.Location
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID == "" || out.Name != "Warehouse" {
		t.Fatalf("unexpected location: %+v", out)
	}
}

func TestCreateLocation_MissingName(t *testing.T) {
	am := newTestManager(t)
	handler := routes.CreateLocation(am)

	req := httptest.NewRequest(http.MethodPost, "/locations", strings.NewReader(`{"kind":"bin"}`))
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func newLocation(t *testing.T, am mwanachamaassetmanager.AssetManager, name, parentID string) mwanachamaassetmanager.Location {
	t.Helper()
	l, err := am.CreateLocation(context.Background(), mwanachamaassetmanager.Location{Name: name, ParentLocationID: parentID})
	if err != nil {
		t.Fatalf("seed CreateLocation: %v", err)
	}
	return l
}

func TestGetLocation(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	handler := routes.GetLocation(am)

	req := withPathValue(httptest.NewRequest(http.MethodGet, "/locations/"+loc.ID, nil), "locationID", loc.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestGetLocation_NotFound(t *testing.T) {
	am := newTestManager(t)
	handler := routes.GetLocation(am)

	req := withPathValue(httptest.NewRequest(http.MethodGet, "/locations/nope", nil), "locationID", "nope")
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateLocation(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Warehouse", "")
	handler := routes.UpdateLocation(am)

	body := `{"name":"Main Warehouse","kind":"warehouse"}`
	req := withPathValue(httptest.NewRequest(http.MethodPatch, "/locations/"+loc.ID, strings.NewReader(body)), "locationID", loc.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out mwanachamaassetmanager.Location
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.Name != "Main Warehouse" {
		t.Fatalf("unexpected location: %+v", out)
	}
}

func TestDeleteLocation(t *testing.T) {
	am := newTestManager(t)
	loc := newLocation(t, am, "Empty Shelf", "")
	handler := routes.DeleteLocation(am)

	req := withPathValue(httptest.NewRequest(http.MethodDelete, "/locations/"+loc.ID, nil), "locationID", loc.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestDeleteLocation_NotEmpty(t *testing.T) {
	am := newTestManager(t)
	parent := newLocation(t, am, "Parent", "")
	_ = newLocation(t, am, "Child", parent.ID)
	handler := routes.DeleteLocation(am)

	req := withPathValue(httptest.NewRequest(http.MethodDelete, "/locations/"+parent.ID, nil), "locationID", parent.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestListLocations_RootOnly(t *testing.T) {
	am := newTestManager(t)
	root := newLocation(t, am, "Root", "")
	_ = newLocation(t, am, "Child", root.ID)
	handler := routes.ListLocations(am)

	req := httptest.NewRequest(http.MethodGet, "/locations?root_only=true", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out []mwanachamaassetmanager.Location
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out) != 1 || out[0].ID != root.ID {
		t.Fatalf("expected [root], got %+v", out)
	}
}

func TestListDescendantLocations(t *testing.T) {
	am := newTestManager(t)
	parent := newLocation(t, am, "Warehouse", "")
	child := newLocation(t, am, "Bin", parent.ID)
	handler := routes.ListDescendantLocations(am)

	req := withPathValue(httptest.NewRequest(http.MethodGet, "/locations/"+parent.ID+"/descendants", nil), "locationID", parent.ID)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var out []mwanachamaassetmanager.Location
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if len(out) != 1 || out[0].ID != child.ID {
		t.Fatalf("expected [child], got %+v", out)
	}
}

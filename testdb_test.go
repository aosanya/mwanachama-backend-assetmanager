package mwanachamaassetmanager_test

import (
	"path/filepath"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/gormtest"
	"github.com/aosanya/mwanachama-backend-shared/spec"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

const warehouseSpec = "warehouse.assetmanager.json"

func testSpec(t *testing.T) *spec.Spec {
	t.Helper()
	s, err := mwanachamaassetmanager.LoadSpec(filepath.Join("spec", "examples", warehouseSpec))
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	return s
}

func newTestManager(t *testing.T) mwanachamaassetmanager.AssetManager {
	t.Helper()
	db := gormtest.OpenSQLiteDB(t)
	s := testSpec(t)
	if err := mwanachamaassetmanager.Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	mgr, err := mwanachamaassetmanager.NewAssetManager(db, s)
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}
	return mgr
}

func TestNewAssetManager_NilDB(t *testing.T) {
	if _, err := mwanachamaassetmanager.NewAssetManager(nil, testSpec(t)); err == nil {
		t.Fatal("expected error for nil db")
	}
}

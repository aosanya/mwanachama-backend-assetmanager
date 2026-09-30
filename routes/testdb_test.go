package routes_test

import (
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/spec"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

func testSpec(t *testing.T) *spec.Spec {
	t.Helper()
	s, err := mwanachamaassetmanager.LoadSpec(
		filepath.Join("..", "spec", "examples", "warehouse.assetmanager.json"))
	if err != nil {
		t.Fatalf("LoadSpec: %v", err)
	}
	return s
}

func openDB(t *testing.T, pinned bool) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	if pinned {
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("db.DB(): %v", err)
		}
		sqlDB.SetMaxOpenConns(1)
	}
	return db
}

func managerOn(t *testing.T, db *gorm.DB, s *spec.Spec) mwanachamaassetmanager.AssetManager {
	t.Helper()
	if err := mwanachamaassetmanager.Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	mgr, err := mwanachamaassetmanager.NewAssetManager(db, s)
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}
	return mgr
}

func tableFor(t *testing.T, s *spec.Spec, role string) string {
	t.Helper()
	o, ok := s.ByRole(role)
	if !ok {
		t.Fatalf("the spec fills no %q role", role)
	}
	return s.TableFor(o)
}

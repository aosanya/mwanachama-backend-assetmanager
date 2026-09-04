package mwanachamaassetmanager_test

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

// newTestManager builds a [mwanachamaassetmanager.AssetManager] backed by a
// fresh in-memory sqlite database, migrated the same way a real deployment
// would via [mwanachamaassetmanager.Migrate]. Replaces the old
// hand-maintained fakeDataManager: exercising real GORM/SQL behavior
// catches more than a Go map fake ever could, while staying fully
// in-process — no containers, no POSTGRES_URL, consistent with this repo's
// existing separation between fast unit tests here and the opt-in
// postgres_integration_test.go.
func newTestManager(t *testing.T) mwanachamaassetmanager.AssetManager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	tables := mwanachamaassetmanager.DefaultTableNames("test")
	if err := mwanachamaassetmanager.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	mgr, err := mwanachamaassetmanager.NewAssetManager(db, tables)
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}
	return mgr
}

func TestNewAssetManager_NilDB(t *testing.T) {
	if _, err := mwanachamaassetmanager.NewAssetManager(nil, mwanachamaassetmanager.DefaultTableNames("test")); err == nil {
		t.Fatal("expected error for nil db")
	}
}

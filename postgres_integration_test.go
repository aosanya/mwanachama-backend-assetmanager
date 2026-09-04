// postgres_integration_test.go exercises AssetManager against a real
// Postgres database, rather than the in-memory sqlite-backed manager the
// rest of this package's tests use.
//
// Skipped unless POSTGRES_URL is set. The unit tests elsewhere in this
// package already exhaustively cover AssetManager's business logic; this
// file's job is narrower — prove the real Postgres wiring (GORM
// AutoMigrate, the serial_tag partial unique index) works end-to-end, and
// specifically prove multi-level ListDescendantLocations against a real
// database.
package mwanachamaassetmanager_test

import (
	"context"
	"os"
	"testing"
	"time"

	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/aosanya/mwanachama-backend-shared/postgres"
)

// newPostgresAssetManager opens POSTGRES_URL via
// mwanachama-backend-shared/postgres.Open (the same DSN parsing, pgx
// driver, and pooling every other repo already uses), wraps that connection
// with GORM's Postgres dialector, migrates a unique-enough table prefix,
// and returns a ready-to-use AssetManager. Skips the calling test if
// POSTGRES_URL is unset. Tables are dropped on cleanup.
func newPostgresAssetManager(t *testing.T) mwanachamaassetmanager.AssetManager {
	t.Helper()
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping Postgres integration test (see Makefile's test-pg target)")
	}

	ctx := context.Background()
	sqlDB, err := postgres.Open(ctx, postgres.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("postgres.Open: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	db, err := gorm.Open(gormpostgres.New(gormpostgres.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}

	// A unique-enough prefix per test keeps concurrent -run invocations from
	// colliding on the same physical tables.
	tables := mwanachamaassetmanager.DefaultTableNames("assetit")
	if err := mwanachamaassetmanager.Migrate(db, tables); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Migrator().DropTable(tables.Holds, tables.Movements, tables.Assets, tables.Locations)
	})

	mgr, err := mwanachamaassetmanager.NewAssetManager(db, tables)
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}
	return mgr
}

func TestPostgres_LocationAndAssetCRUD_RoundTrip(t *testing.T) {
	mgr := newPostgresAssetManager(t)
	ctx := context.Background()

	loc, err := mgr.CreateLocation(ctx, models.Location{Name: "Warehouse", Kind: "warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	a, err := mgr.CreateAsset(ctx, models.Asset{
		Name: "Rice, 50kg bag", TrackingMode: models.AssetTrackingModeFungible, LocationID: loc.ID,
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	got, err := mgr.GetAsset(ctx, a.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if got.Name != "Rice, 50kg bag" || got.LocationID != loc.ID {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestPostgres_MovementLedger_BalanceFold(t *testing.T) {
	mgr := newPostgresAssetManager(t)
	ctx := context.Background()

	warehouse, err := mgr.CreateLocation(ctx, models.Location{Name: "Warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	shop, err := mgr.CreateLocation(ctx, models.Location{Name: "Shop"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	a, err := mgr.CreateAsset(ctx, models.Asset{Name: "Rice", TrackingMode: models.AssetTrackingModeFungible})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	if _, err := mgr.PostMovement(ctx, models.Movement{
		AssetID: a.ID, Kind: models.MovementKindArrived, Quantity: 100, ToLocationID: warehouse.ID, PerformedBy: "pg-actor",
	}); err != nil {
		t.Fatalf("PostMovement arrived: %v", err)
	}
	if _, err := mgr.PostMovement(ctx, models.Movement{
		AssetID: a.ID, Kind: models.MovementKindTransferred, Quantity: 30, FromLocationID: warehouse.ID, ToLocationID: shop.ID, PerformedBy: "pg-actor",
	}); err != nil {
		t.Fatalf("PostMovement transferred: %v", err)
	}

	warehouseBalance, err := mgr.GetAssetBalance(ctx, a.ID, warehouse.ID)
	if err != nil {
		t.Fatalf("GetAssetBalance warehouse: %v", err)
	}
	shopBalance, err := mgr.GetAssetBalance(ctx, a.ID, shop.ID)
	if err != nil {
		t.Fatalf("GetAssetBalance shop: %v", err)
	}
	if warehouseBalance != 70 || shopBalance != 30 {
		t.Fatalf("expected 70/30, got warehouse=%d shop=%d", warehouseBalance, shopBalance)
	}
}

// TestPostgres_ListDescendantLocations_MultiLevel proves the recursive-tree
// design decision actually holds against real Postgres: three levels deep
// (warehouse > aisle > bin).
func TestPostgres_ListDescendantLocations_MultiLevel(t *testing.T) {
	mgr := newPostgresAssetManager(t)
	ctx := context.Background()

	warehouse, err := mgr.CreateLocation(ctx, models.Location{Name: "Warehouse", Kind: "warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation warehouse: %v", err)
	}
	aisle, err := mgr.CreateLocation(ctx, models.Location{Name: "Aisle 3", Kind: "aisle", ParentLocationID: warehouse.ID})
	if err != nil {
		t.Fatalf("CreateLocation aisle: %v", err)
	}
	bin, err := mgr.CreateLocation(ctx, models.Location{Name: "Bin 12", Kind: "bin", ParentLocationID: aisle.ID})
	if err != nil {
		t.Fatalf("CreateLocation bin: %v", err)
	}

	descendants, err := mgr.ListDescendantLocations(ctx, warehouse.ID)
	if err != nil {
		t.Fatalf("ListDescendantLocations: %v", err)
	}
	if len(descendants) != 2 {
		t.Fatalf("expected [aisle, bin] (2 descendants), got %d: %+v", len(descendants), descendants)
	}
	seen := map[string]bool{}
	for _, d := range descendants {
		seen[d.ID] = true
	}
	if !seen[aisle.ID] || !seen[bin.ID] {
		t.Fatalf("expected both aisle %q and bin %q in descendants, got %+v", aisle.ID, bin.ID, descendants)
	}

	// A grandchild reparent that would create a cycle through two hops is
	// rejected — proof reparentLocation's descendant check also holds
	// multi-level.
	warehouse.ParentLocationID = bin.ID
	if _, err := mgr.UpdateLocation(ctx, warehouse); err == nil {
		t.Fatal("expected a multi-hop cycle to be rejected")
	}
}

func TestPostgres_HoldLifecycle(t *testing.T) {
	mgr := newPostgresAssetManager(t)
	ctx := context.Background()

	loc, err := mgr.CreateLocation(ctx, models.Location{Name: "Shop floor"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	a, err := mgr.CreateAsset(ctx, models.Asset{Name: "T-shirt M", TrackingMode: models.AssetTrackingModeFungible})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if _, err := mgr.PostMovement(ctx, models.Movement{
		AssetID: a.ID, Kind: models.MovementKindArrived, Quantity: 20, ToLocationID: loc.ID, PerformedBy: "pg-actor",
	}); err != nil {
		t.Fatalf("PostMovement: %v", err)
	}

	h, err := mgr.CreateHold(ctx, models.Hold{
		AssetID: a.ID, LocationID: loc.ID, Quantity: 10, ExpiresAt: time.Now().UTC().Add(time.Hour).Format(time.RFC3339), PlacedBy: "pg-actor",
	})
	if err != nil {
		t.Fatalf("CreateHold: %v", err)
	}
	committed, _, err := mgr.CommitHold(ctx, h.ID, 10, models.MovementKindDeparted, "", "checkout")
	if err != nil {
		t.Fatalf("CommitHold: %v", err)
	}
	if committed.Status != models.HoldStatusCommitted {
		t.Fatalf("expected committed, got %q", committed.Status)
	}

	balance, err := mgr.GetAssetBalance(ctx, a.ID, loc.ID)
	if err != nil {
		t.Fatalf("GetAssetBalance: %v", err)
	}
	if balance != 10 {
		t.Fatalf("expected balance 10, got %d", balance)
	}
}

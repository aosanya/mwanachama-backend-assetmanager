// postgres_integration_test.go exercises AssetManager against a real
// Postgres-backed entitygraph.DataManager (mwanachama-backend-shared's
// postgres.Backend), rather than the in-memory fakeDataManager the rest of
// this package's tests use.
//
// Skipped unless POSTGRES_URL is set — mirrors mwanachama-backend-shared's
// own postgres/backend_test.go split, the same pattern
// mwanachama-backend-taskmanager's postgres_integration_test.go uses. The
// unit tests elsewhere in this package already exhaustively cover
// AssetManager's business logic against fakeDataManager, whose TraverseGraph
// is single-hop only; this file's job is narrower — prove the real Postgres
// wiring works end-to-end, and specifically prove multi-level
// ListDescendantLocations (a real recursive-CTE traversal, not something
// the fake can honestly test).
package mwanachamaassetmanager_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-shared/postgres"
)

func applyAssetDDL(ctx context.Context, db *sql.DB, script string) error {
	_, err := db.ExecContext(ctx, script)
	return err
}

// newPostgresAssetManager opens POSTGRES_URL, creates a scratch set of
// assetit_-prefixed tables, seeds+activates DefaultAssetSchema for
// agencyID, and returns a ready-to-use AssetManager. Skips the calling test
// if POSTGRES_URL is unset. Tables are dropped on cleanup.
func newPostgresAssetManager(t *testing.T, agencyID string) mwanachamaassetmanager.AssetManager {
	t.Helper()
	dsn := os.Getenv("POSTGRES_URL")
	if dsn == "" {
		t.Skip("POSTGRES_URL not set; skipping Postgres integration test (see Makefile's test-pg target)")
	}

	ctx := context.Background()
	db, err := postgres.Open(ctx, postgres.Config{DSN: dsn})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	tables := postgres.DefaultTableNames("assetit_")
	if err := applyAssetDDL(ctx, db, postgres.DDL(tables)); err != nil {
		t.Fatalf("applying DDL: %v", err)
	}
	t.Cleanup(func() {
		_ = applyAssetDDL(context.Background(), db, postgres.DropDDL(tables))
	})

	backend := postgres.NewBackend(db, tables)

	s := mwanachamaassetmanager.DefaultAssetSchema()
	s.AgencyID = agencyID
	if err := backend.SetSchema(ctx, s); err != nil {
		t.Fatalf("SetSchema: %v", err)
	}
	if err := backend.Publish(ctx, agencyID); err != nil {
		t.Fatalf("Publish: %v", err)
	}
	if err := backend.Activate(ctx, agencyID, 1); err != nil {
		t.Fatalf("Activate: %v", err)
	}

	mgr, err := mwanachamaassetmanager.NewAssetManager(backend)
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}
	return mgr
}

func TestPostgres_LocationAndAssetCRUD_RoundTrip(t *testing.T) {
	const agencyID = "pg-agency-asset"
	mgr := newPostgresAssetManager(t, agencyID)
	ctx := context.Background()

	loc, err := mgr.CreateLocation(ctx, agencyID, mwanachamaassetmanager.Location{Name: "Warehouse", Kind: "warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	a, err := mgr.CreateAsset(ctx, agencyID, mwanachamaassetmanager.Asset{
		Name: "Rice, 50kg bag", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible, LocationID: loc.ID,
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	got, err := mgr.GetAsset(ctx, agencyID, a.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if got.Name != "Rice, 50kg bag" || got.LocationID != loc.ID {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

func TestPostgres_MovementLedger_BalanceFold(t *testing.T) {
	const agencyID = "pg-agency-movement"
	mgr := newPostgresAssetManager(t, agencyID)
	ctx := context.Background()

	warehouse, err := mgr.CreateLocation(ctx, agencyID, mwanachamaassetmanager.Location{Name: "Warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	shop, err := mgr.CreateLocation(ctx, agencyID, mwanachamaassetmanager.Location{Name: "Shop"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	a, err := mgr.CreateAsset(ctx, agencyID, mwanachamaassetmanager.Asset{Name: "Rice", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	if _, err := mgr.PostMovement(ctx, agencyID, mwanachamaassetmanager.Movement{
		AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 100, ToLocationID: warehouse.ID,
	}); err != nil {
		t.Fatalf("PostMovement arrived: %v", err)
	}
	if _, err := mgr.PostMovement(ctx, agencyID, mwanachamaassetmanager.Movement{
		AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindTransferred, Quantity: 30, FromLocationID: warehouse.ID, ToLocationID: shop.ID,
	}); err != nil {
		t.Fatalf("PostMovement transferred: %v", err)
	}

	warehouseBalance, err := mgr.GetAssetBalance(ctx, agencyID, a.ID, warehouse.ID)
	if err != nil {
		t.Fatalf("GetAssetBalance warehouse: %v", err)
	}
	shopBalance, err := mgr.GetAssetBalance(ctx, agencyID, a.ID, shop.ID)
	if err != nil {
		t.Fatalf("GetAssetBalance shop: %v", err)
	}
	if warehouseBalance != 70 || shopBalance != 30 {
		t.Fatalf("expected 70/30, got warehouse=%d shop=%d", warehouseBalance, shopBalance)
	}
}

// TestPostgres_ListDescendantLocations_MultiLevel proves the recursive-tree
// design decision actually holds against real Postgres: three levels deep
// (warehouse > aisle > bin), which fakeDataManager's single-hop
// TraverseGraph cannot honestly exercise.
func TestPostgres_ListDescendantLocations_MultiLevel(t *testing.T) {
	const agencyID = "pg-agency-location-tree"
	mgr := newPostgresAssetManager(t, agencyID)
	ctx := context.Background()

	warehouse, err := mgr.CreateLocation(ctx, agencyID, mwanachamaassetmanager.Location{Name: "Warehouse", Kind: "warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation warehouse: %v", err)
	}
	aisle, err := mgr.CreateLocation(ctx, agencyID, mwanachamaassetmanager.Location{Name: "Aisle 3", Kind: "aisle", ParentLocationID: warehouse.ID})
	if err != nil {
		t.Fatalf("CreateLocation aisle: %v", err)
	}
	bin, err := mgr.CreateLocation(ctx, agencyID, mwanachamaassetmanager.Location{Name: "Bin 12", Kind: "bin", ParentLocationID: aisle.ID})
	if err != nil {
		t.Fatalf("CreateLocation bin: %v", err)
	}

	descendants, err := mgr.ListDescendantLocations(ctx, agencyID, warehouse.ID)
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
	// multi-level against the real recursive CTE.
	warehouse.ParentLocationID = bin.ID
	if _, err := mgr.UpdateLocation(ctx, agencyID, warehouse); err == nil {
		t.Fatal("expected a multi-hop cycle to be rejected")
	}
}

func TestPostgres_HoldLifecycle(t *testing.T) {
	const agencyID = "pg-agency-hold"
	mgr := newPostgresAssetManager(t, agencyID)
	ctx := context.Background()

	loc, err := mgr.CreateLocation(ctx, agencyID, mwanachamaassetmanager.Location{Name: "Shop floor"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	a, err := mgr.CreateAsset(ctx, agencyID, mwanachamaassetmanager.Asset{Name: "T-shirt M", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if _, err := mgr.PostMovement(ctx, agencyID, mwanachamaassetmanager.Movement{
		AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 20, ToLocationID: loc.ID,
	}); err != nil {
		t.Fatalf("PostMovement: %v", err)
	}

	h, err := mgr.CreateHold(ctx, agencyID, mwanachamaassetmanager.Hold{
		AssetID: a.ID, LocationID: loc.ID, Quantity: 10, ExpiresAt: time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("CreateHold: %v", err)
	}
	committed, _, err := mgr.CommitHold(ctx, agencyID, h.ID, 10, mwanachamaassetmanager.MovementKindDeparted, "", "checkout")
	if err != nil {
		t.Fatalf("CommitHold: %v", err)
	}
	if committed.Status != mwanachamaassetmanager.HoldStatusCommitted {
		t.Fatalf("expected committed, got %q", committed.Status)
	}

	balance, err := mgr.GetAssetBalance(ctx, agencyID, a.ID, loc.ID)
	if err != nil {
		t.Fatalf("GetAssetBalance: %v", err)
	}
	if balance != 10 {
		t.Fatalf("expected balance 10, got %d", balance)
	}
}

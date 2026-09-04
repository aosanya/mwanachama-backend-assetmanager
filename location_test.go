package mwanachamaassetmanager_test

import (
	"context"
	"errors"
	"testing"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func TestCreateLocation_Root(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	loc, err := m.CreateLocation(ctx, models.Location{Name: "Main Warehouse", Kind: "warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	if loc.ID == "" {
		t.Fatal("expected a generated ID")
	}
	if loc.ParentLocationID != "" {
		t.Fatalf("expected root location, got parent %q", loc.ParentLocationID)
	}
}

func TestCreateLocation_MissingName(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, err := m.CreateLocation(ctx, models.Location{Kind: "bin"})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidLocation) {
		t.Fatalf("expected ErrInvalidLocation, got %v", err)
	}
}

func TestCreateLocation_ParentNotFound(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, err := m.CreateLocation(ctx, models.Location{Name: "Bin 1", ParentLocationID: "does-not-exist"})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidLocation) {
		t.Fatalf("expected ErrInvalidLocation, got %v", err)
	}
}

func TestCreateLocation_WithParent(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	parent, err := m.CreateLocation(ctx, models.Location{Name: "Warehouse", Kind: "warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation parent: %v", err)
	}
	child, err := m.CreateLocation(ctx, models.Location{Name: "Bin 12", Kind: "bin", ParentLocationID: parent.ID})
	if err != nil {
		t.Fatalf("CreateLocation child: %v", err)
	}
	if child.ParentLocationID != parent.ID {
		t.Fatalf("expected parent %q, got %q", parent.ID, child.ParentLocationID)
	}

	descendants, err := m.ListDescendantLocations(ctx, parent.ID)
	if err != nil {
		t.Fatalf("ListDescendantLocations: %v", err)
	}
	if len(descendants) != 1 || descendants[0].ID != child.ID {
		t.Fatalf("expected [child], got %+v", descendants)
	}
}

// TestListDescendantLocations_MultiLevelFanOut proves the hand-rolled BFS
// in location.go (ListDescendantLocations) correctly walks more than one
// hop. Builds a 4-level tree with a wide fan-out at level 2 (warehouse -> 3
// aisles -> 2 bins each -> 1 item under one bin) and checks both the full
// descendant set (order-independent membership) and that the start node
// itself is excluded.
func TestListDescendantLocations_MultiLevelFanOut(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	warehouse, err := m.CreateLocation(ctx, models.Location{Name: "Warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation warehouse: %v", err)
	}

	wantIDs := map[string]bool{}
	var deepestBin models.Location
	for a := 0; a < 3; a++ {
		aisle, err := m.CreateLocation(ctx, models.Location{
			Name: "Aisle", ParentLocationID: warehouse.ID,
		})
		if err != nil {
			t.Fatalf("CreateLocation aisle %d: %v", a, err)
		}
		wantIDs[aisle.ID] = true
		for b := 0; b < 2; b++ {
			bin, err := m.CreateLocation(ctx, models.Location{
				Name: "Bin", ParentLocationID: aisle.ID,
			})
			if err != nil {
				t.Fatalf("CreateLocation bin a=%d b=%d: %v", a, b, err)
			}
			wantIDs[bin.ID] = true
			deepestBin = bin
		}
	}
	// A fourth level under just one bin, to prove the walk doesn't stop
	// after a fixed number of hops regardless of fan-out elsewhere.
	shelf, err := m.CreateLocation(ctx, models.Location{
		Name: "Shelf", ParentLocationID: deepestBin.ID,
	})
	if err != nil {
		t.Fatalf("CreateLocation shelf: %v", err)
	}
	wantIDs[shelf.ID] = true

	descendants, err := m.ListDescendantLocations(ctx, warehouse.ID)
	if err != nil {
		t.Fatalf("ListDescendantLocations: %v", err)
	}
	if len(descendants) != len(wantIDs) {
		t.Fatalf("expected %d descendants, got %d: %+v", len(wantIDs), len(descendants), descendants)
	}
	got := map[string]bool{}
	for _, d := range descendants {
		got[d.ID] = true
		if d.ID == warehouse.ID {
			t.Fatal("expected the start location itself to be excluded from its own descendant list")
		}
	}
	for id := range wantIDs {
		if !got[id] {
			t.Fatalf("expected descendant %q in result, got %+v", id, descendants)
		}
	}

	// The BFS's cycle-safety guard (visited checked before a node is
	// queued into the next frontier) should never reprocess a node — a
	// scoped sub-check that a deeper node reachable through only one path
	// appears exactly once.
	count := 0
	for _, d := range descendants {
		if d.ID == shelf.ID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected shelf to appear exactly once, got %d", count)
	}
}

func TestUpdateLocation_Reparent(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	warehouseA, _ := m.CreateLocation(ctx, models.Location{Name: "Warehouse A"})
	warehouseB, _ := m.CreateLocation(ctx, models.Location{Name: "Warehouse B"})
	bin, err := m.CreateLocation(ctx, models.Location{Name: "Bin 1", ParentLocationID: warehouseA.ID})
	if err != nil {
		t.Fatalf("CreateLocation bin: %v", err)
	}

	bin.ParentLocationID = warehouseB.ID
	updated, err := m.UpdateLocation(ctx, bin)
	if err != nil {
		t.Fatalf("UpdateLocation reparent: %v", err)
	}
	if updated.ParentLocationID != warehouseB.ID {
		t.Fatalf("expected new parent %q, got %q", warehouseB.ID, updated.ParentLocationID)
	}

	aChildren, _ := m.ListLocations(ctx, mwanachamaassetmanager.LocationFilter{ParentLocationID: warehouseA.ID})
	if len(aChildren) != 0 {
		t.Fatalf("expected warehouse A to have no children after reparent, got %+v", aChildren)
	}
	bChildren, _ := m.ListLocations(ctx, mwanachamaassetmanager.LocationFilter{ParentLocationID: warehouseB.ID})
	if len(bChildren) != 1 || bChildren[0].ID != bin.ID {
		t.Fatalf("expected warehouse B to have [bin], got %+v", bChildren)
	}
}

func TestUpdateLocation_SelfParentRejected(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	loc, _ := m.CreateLocation(ctx, models.Location{Name: "Loop"})
	loc.ParentLocationID = loc.ID
	_, err := m.UpdateLocation(ctx, loc)
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidLocation) {
		t.Fatalf("expected ErrInvalidLocation for self-parent, got %v", err)
	}
}

func TestUpdateLocation_DescendantCycleRejected(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	parent, _ := m.CreateLocation(ctx, models.Location{Name: "Parent"})
	child, _ := m.CreateLocation(ctx, models.Location{Name: "Child", ParentLocationID: parent.ID})

	// Attempt to make parent a child of its own direct child — a one-hop
	// cycle. See TestPostgres_ListDescendantLocations_MultiLevel for the
	// multi-hop case against real Postgres.
	parent.ParentLocationID = child.ID
	_, err := m.UpdateLocation(ctx, parent)
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidLocation) {
		t.Fatalf("expected ErrInvalidLocation for a descendant cycle, got %v", err)
	}
}

func TestDeleteLocation_NotEmptyWithChildren(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	parent, _ := m.CreateLocation(ctx, models.Location{Name: "Parent"})
	_, _ = m.CreateLocation(ctx, models.Location{Name: "Child", ParentLocationID: parent.ID})

	err := m.DeleteLocation(ctx, parent.ID)
	if !errors.Is(err, mwanachamaassetmanager.ErrLocationNotEmpty) {
		t.Fatalf("expected ErrLocationNotEmpty, got %v", err)
	}
}

func TestDeleteLocation_NotEmptyWithAssets(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	loc, _ := m.CreateLocation(ctx, models.Location{Name: "Pantry"})
	_, err := m.CreateAsset(ctx, models.Asset{
		Name: "Rice", TrackingMode: models.AssetTrackingModeFungible, LocationID: loc.ID,
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	if err := m.DeleteLocation(ctx, loc.ID); !errors.Is(err, mwanachamaassetmanager.ErrLocationNotEmpty) {
		t.Fatalf("expected ErrLocationNotEmpty, got %v", err)
	}
}

func TestDeleteLocation_OK(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	loc, _ := m.CreateLocation(ctx, models.Location{Name: "Empty Shelf"})
	if err := m.DeleteLocation(ctx, loc.ID); err != nil {
		t.Fatalf("DeleteLocation: %v", err)
	}
	if _, err := m.GetLocation(ctx, loc.ID); !errors.Is(err, mwanachamaassetmanager.ErrLocationNotFound) {
		t.Fatalf("expected ErrLocationNotFound after delete, got %v", err)
	}
}

func TestListLocations_RootOnly(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	root, _ := m.CreateLocation(ctx, models.Location{Name: "Root"})
	_, _ = m.CreateLocation(ctx, models.Location{Name: "Child", ParentLocationID: root.ID})

	roots, err := m.ListLocations(ctx, mwanachamaassetmanager.LocationFilter{RootOnly: true})
	if err != nil {
		t.Fatalf("ListLocations: %v", err)
	}
	if len(roots) != 1 || roots[0].ID != root.ID {
		t.Fatalf("expected [root], got %+v", roots)
	}
}

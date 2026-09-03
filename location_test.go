package mwanachamaassetmanager_test

import (
	"context"
	"errors"
	"testing"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

const testAgency = "agency-1"

func newTestManager(t *testing.T) mwanachamaassetmanager.AssetManager {
	t.Helper()
	m, err := mwanachamaassetmanager.NewAssetManager(newFakeDataManager())
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}
	return m
}

func TestCreateLocation_Root(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	loc, err := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Main Warehouse", Kind: "warehouse"})
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

	_, err := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Kind: "bin"})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidLocation) {
		t.Fatalf("expected ErrInvalidLocation, got %v", err)
	}
}

func TestCreateLocation_ParentNotFound(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, err := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Bin 1", ParentLocationID: "does-not-exist"})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidLocation) {
		t.Fatalf("expected ErrInvalidLocation, got %v", err)
	}
}

func TestCreateLocation_WithParent(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	parent, err := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Warehouse", Kind: "warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation parent: %v", err)
	}
	child, err := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Bin 12", Kind: "bin", ParentLocationID: parent.ID})
	if err != nil {
		t.Fatalf("CreateLocation child: %v", err)
	}
	if child.ParentLocationID != parent.ID {
		t.Fatalf("expected parent %q, got %q", parent.ID, child.ParentLocationID)
	}

	descendants, err := m.ListDescendantLocations(ctx, testAgency, parent.ID)
	if err != nil {
		t.Fatalf("ListDescendantLocations: %v", err)
	}
	if len(descendants) != 1 || descendants[0].ID != child.ID {
		t.Fatalf("expected [child], got %+v", descendants)
	}
}

func TestUpdateLocation_Reparent(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	warehouseA, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Warehouse A"})
	warehouseB, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Warehouse B"})
	bin, err := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Bin 1", ParentLocationID: warehouseA.ID})
	if err != nil {
		t.Fatalf("CreateLocation bin: %v", err)
	}

	bin.ParentLocationID = warehouseB.ID
	updated, err := m.UpdateLocation(ctx, testAgency, bin)
	if err != nil {
		t.Fatalf("UpdateLocation reparent: %v", err)
	}
	if updated.ParentLocationID != warehouseB.ID {
		t.Fatalf("expected new parent %q, got %q", warehouseB.ID, updated.ParentLocationID)
	}

	aChildren, _ := m.ListLocations(ctx, testAgency, mwanachamaassetmanager.LocationFilter{ParentLocationID: warehouseA.ID})
	if len(aChildren) != 0 {
		t.Fatalf("expected warehouse A to have no children after reparent, got %+v", aChildren)
	}
	bChildren, _ := m.ListLocations(ctx, testAgency, mwanachamaassetmanager.LocationFilter{ParentLocationID: warehouseB.ID})
	if len(bChildren) != 1 || bChildren[0].ID != bin.ID {
		t.Fatalf("expected warehouse B to have [bin], got %+v", bChildren)
	}
}

func TestUpdateLocation_SelfParentRejected(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	loc, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Loop"})
	loc.ParentLocationID = loc.ID
	_, err := m.UpdateLocation(ctx, testAgency, loc)
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidLocation) {
		t.Fatalf("expected ErrInvalidLocation for self-parent, got %v", err)
	}
}

func TestUpdateLocation_DescendantCycleRejected(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	parent, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Parent"})
	child, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Child", ParentLocationID: parent.ID})

	// Attempt to make parent a child of its own direct child — a one-hop
	// cycle, which the fake's single-hop TraverseGraph can detect. Deeper
	// (multi-hop) cycle rejection is only provable against a real
	// recursive-CTE-backed DataManager — see postgres_integration_test.go.
	parent.ParentLocationID = child.ID
	_, err := m.UpdateLocation(ctx, testAgency, parent)
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidLocation) {
		t.Fatalf("expected ErrInvalidLocation for a descendant cycle, got %v", err)
	}
}

func TestDeleteLocation_NotEmptyWithChildren(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	parent, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Parent"})
	_, _ = m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Child", ParentLocationID: parent.ID})

	err := m.DeleteLocation(ctx, testAgency, parent.ID)
	if !errors.Is(err, mwanachamaassetmanager.ErrLocationNotEmpty) {
		t.Fatalf("expected ErrLocationNotEmpty, got %v", err)
	}
}

func TestDeleteLocation_NotEmptyWithAssets(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	loc, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Pantry"})
	_, err := m.CreateAsset(ctx, testAgency, mwanachamaassetmanager.Asset{
		Name: "Rice", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible, LocationID: loc.ID,
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}

	if err := m.DeleteLocation(ctx, testAgency, loc.ID); !errors.Is(err, mwanachamaassetmanager.ErrLocationNotEmpty) {
		t.Fatalf("expected ErrLocationNotEmpty, got %v", err)
	}
}

func TestDeleteLocation_OK(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	loc, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Empty Shelf"})
	if err := m.DeleteLocation(ctx, testAgency, loc.ID); err != nil {
		t.Fatalf("DeleteLocation: %v", err)
	}
	if _, err := m.GetLocation(ctx, testAgency, loc.ID); !errors.Is(err, mwanachamaassetmanager.ErrLocationNotFound) {
		t.Fatalf("expected ErrLocationNotFound after delete, got %v", err)
	}
}

func TestListLocations_RootOnly(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	root, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Root"})
	_, _ = m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Child", ParentLocationID: root.ID})

	roots, err := m.ListLocations(ctx, testAgency, mwanachamaassetmanager.LocationFilter{RootOnly: true})
	if err != nil {
		t.Fatalf("ListLocations: %v", err)
	}
	if len(roots) != 1 || roots[0].ID != root.ID {
		t.Fatalf("expected [root], got %+v", roots)
	}
}

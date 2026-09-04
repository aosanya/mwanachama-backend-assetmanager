package mwanachamaassetmanager_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

func TestCreateAsset_Fungible(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	a, err := m.CreateAsset(ctx, mwanachamaassetmanager.Asset{
		Name: "T-shirt, size M", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible, Category: "merchandise",
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected a generated ID")
	}
}

func TestCreateAsset_Serialized(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	a, err := m.CreateAsset(ctx, mwanachamaassetmanager.Asset{
		Name: "Bessie", TrackingMode: mwanachamaassetmanager.AssetTrackingModeSerialized, SerialTag: "COW-001",
	})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if a.SerialTag != "COW-001" {
		t.Fatalf("expected serial tag preserved, got %q", a.SerialTag)
	}
}

func TestCreateAsset_MissingName(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, err := m.CreateAsset(ctx, mwanachamaassetmanager.Asset{TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidAsset) {
		t.Fatalf("expected ErrInvalidAsset, got %v", err)
	}
}

func TestCreateAsset_InvalidTrackingMode(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, err := m.CreateAsset(ctx, mwanachamaassetmanager.Asset{Name: "Mystery", TrackingMode: "bogus"})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidAsset) {
		t.Fatalf("expected ErrInvalidAsset, got %v", err)
	}
}

func TestCreateAsset_SerializedMissingSerialTag(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, err := m.CreateAsset(ctx, mwanachamaassetmanager.Asset{Name: "Laptop", TrackingMode: mwanachamaassetmanager.AssetTrackingModeSerialized})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidAsset) {
		t.Fatalf("expected ErrInvalidAsset, got %v", err)
	}
}

func TestCreateAsset_DuplicateSerialTag(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, err := m.CreateAsset(ctx, mwanachamaassetmanager.Asset{
		Name: "Laptop 1", TrackingMode: mwanachamaassetmanager.AssetTrackingModeSerialized, SerialTag: "SN-1",
	})
	if err != nil {
		t.Fatalf("CreateAsset first: %v", err)
	}
	_, err = m.CreateAsset(ctx, mwanachamaassetmanager.Asset{
		Name: "Laptop 2", TrackingMode: mwanachamaassetmanager.AssetTrackingModeSerialized, SerialTag: "SN-1",
	})
	if !errors.Is(err, mwanachamaassetmanager.ErrAssetSerialTagExists) {
		t.Fatalf("expected ErrAssetSerialTagExists, got %v", err)
	}
}

func TestUpdateAsset_TrackingModeImmutable(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	a, _ := m.CreateAsset(ctx, mwanachamaassetmanager.Asset{Name: "Rice", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible})
	a.TrackingMode = mwanachamaassetmanager.AssetTrackingModeSerialized
	_, err := m.UpdateAsset(ctx, a)
	if !errors.Is(err, mwanachamaassetmanager.ErrAssetTrackingModeImmutable) {
		t.Fatalf("expected ErrAssetTrackingModeImmutable, got %v", err)
	}
}

func TestDeleteAsset_OpenHoldsBlocks(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	loc, _ := m.CreateLocation(ctx, mwanachamaassetmanager.Location{Name: "Shelf"})
	a, _ := m.CreateAsset(ctx, mwanachamaassetmanager.Asset{Name: "Rice", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible})
	if _, err := m.PostMovement(ctx, mwanachamaassetmanager.Movement{
		AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 10, ToLocationID: loc.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("PostMovement arrived: %v", err)
	}
	if _, err := m.CreateHold(ctx, mwanachamaassetmanager.Hold{
		AssetID: a.ID, LocationID: loc.ID, Quantity: 5, ExpiresAt: futureRFC3339(t), PlacedBy: testActor,
	}); err != nil {
		t.Fatalf("CreateHold: %v", err)
	}

	if err := m.DeleteAsset(ctx, a.ID); !errors.Is(err, mwanachamaassetmanager.ErrAssetHasOpenHolds) {
		t.Fatalf("expected ErrAssetHasOpenHolds, got %v", err)
	}
}

func TestListAssets_FilterByCategory(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)

	_, _ = m.CreateAsset(ctx, mwanachamaassetmanager.Asset{Name: "Rice", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible, Category: "grocery"})
	_, _ = m.CreateAsset(ctx, mwanachamaassetmanager.Asset{Name: "Bessie", TrackingMode: mwanachamaassetmanager.AssetTrackingModeSerialized, SerialTag: "COW-1", Category: "livestock"})

	groceries, err := m.ListAssets(ctx, mwanachamaassetmanager.AssetFilter{Category: "grocery"})
	if err != nil {
		t.Fatalf("ListAssets: %v", err)
	}
	if len(groceries) != 1 || groceries[0].Name != "Rice" {
		t.Fatalf("expected [Rice], got %+v", groceries)
	}
}

// futureRFC3339 returns an RFC 3339 timestamp comfortably in the future, for
// tests that need a valid Hold.ExpiresAt.
func futureRFC3339(t *testing.T) string {
	t.Helper()
	return time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
}

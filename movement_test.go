package mwanachamaassetmanager_test

import (
	"context"
	"errors"
	"testing"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

const testActor = "actor-1"

func setupFungibleAsset(t *testing.T, m mwanachamaassetmanager.AssetManager, ctx context.Context) (mwanachamaassetmanager.Asset, mwanachamaassetmanager.Location) {
	t.Helper()
	loc, err := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Warehouse"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	a, err := m.CreateAsset(ctx, testAgency, mwanachamaassetmanager.Asset{Name: "Rice, 50kg bag", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	return a, loc
}

func TestPostMovement_ArrivedThenBalance(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupFungibleAsset(t, m, ctx)

	mv, err := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{
		AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 100, ToLocationID: loc.ID, PerformedBy: testActor,
	})
	if err != nil {
		t.Fatalf("PostMovement: %v", err)
	}
	if mv.ID == "" {
		t.Fatal("expected a generated ID")
	}

	balance, err := m.GetAssetBalance(ctx, testAgency, a.ID, loc.ID)
	if err != nil {
		t.Fatalf("GetAssetBalance: %v", err)
	}
	if balance != 100 {
		t.Fatalf("expected balance 100, got %d", balance)
	}

	updated, err := m.GetAsset(ctx, testAgency, a.ID)
	if err != nil {
		t.Fatalf("GetAsset: %v", err)
	}
	if updated.LocationID != loc.ID {
		t.Fatalf("expected denormalized LocationID %q, got %q", loc.ID, updated.LocationID)
	}
}

func TestPostMovement_TransferredMovesBalance(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, warehouse := setupFungibleAsset(t, m, ctx)
	shop, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Shop"})

	if _, err := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{
		AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 50, ToLocationID: warehouse.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("PostMovement arrived: %v", err)
	}
	if _, err := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{
		AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindTransferred, Quantity: 20, FromLocationID: warehouse.ID, ToLocationID: shop.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("PostMovement transferred: %v", err)
	}

	warehouseBalance, _ := m.GetAssetBalance(ctx, testAgency, a.ID, warehouse.ID)
	shopBalance, _ := m.GetAssetBalance(ctx, testAgency, a.ID, shop.ID)
	if warehouseBalance != 30 {
		t.Fatalf("expected warehouse balance 30, got %d", warehouseBalance)
	}
	if shopBalance != 20 {
		t.Fatalf("expected shop balance 20, got %d", shopBalance)
	}
}

func TestPostMovement_DepartedReducesBalance(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupFungibleAsset(t, m, ctx)

	_, _ = m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 30, ToLocationID: loc.ID, PerformedBy: testActor})
	_, err := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindDeparted, Quantity: 10, FromLocationID: loc.ID, PerformedBy: testActor})
	if err != nil {
		t.Fatalf("PostMovement departed: %v", err)
	}
	balance, _ := m.GetAssetBalance(ctx, testAgency, a.ID, loc.ID)
	if balance != 20 {
		t.Fatalf("expected balance 20, got %d", balance)
	}
}

func TestPostMovement_AdjustedCanBeNegative(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupFungibleAsset(t, m, ctx)

	_, _ = m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 30, ToLocationID: loc.ID, PerformedBy: testActor})
	_, err := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindAdjusted, Quantity: -3, ToLocationID: loc.ID, Note: "stock count variance", PerformedBy: testActor})
	if err != nil {
		t.Fatalf("PostMovement adjusted: %v", err)
	}
	balance, _ := m.GetAssetBalance(ctx, testAgency, a.ID, loc.ID)
	if balance != 27 {
		t.Fatalf("expected balance 27, got %d", balance)
	}
}

func TestPostMovement_InvalidShapes(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupFungibleAsset(t, m, ctx)

	cases := []struct {
		name string
		mv   mwanachamaassetmanager.Movement
	}{
		{"arrived with FromLocationID", mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 1, ToLocationID: loc.ID, FromLocationID: loc.ID, PerformedBy: testActor}},
		{"arrived non-positive quantity", mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 0, ToLocationID: loc.ID, PerformedBy: testActor}},
		{"transferred same location", mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindTransferred, Quantity: 1, FromLocationID: loc.ID, ToLocationID: loc.ID, PerformedBy: testActor}},
		{"departed with ToLocationID", mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindDeparted, Quantity: 1, FromLocationID: loc.ID, ToLocationID: loc.ID, PerformedBy: testActor}},
		{"adjusted zero quantity", mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindAdjusted, Quantity: 0, ToLocationID: loc.ID, PerformedBy: testActor}},
		{"reversed posted directly", mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindReversed, Quantity: 1, ToLocationID: loc.ID, PerformedBy: testActor}},
		{"unrecognized kind", mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: "bogus", Quantity: 1, ToLocationID: loc.ID, PerformedBy: testActor}},
		{"missing asset id", mwanachamaassetmanager.Movement{Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 1, ToLocationID: loc.ID, PerformedBy: testActor}},
		{"missing performed by", mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 1, ToLocationID: loc.ID}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := m.PostMovement(ctx, testAgency, c.mv)
			if !errors.Is(err, mwanachamaassetmanager.ErrInvalidMovement) {
				t.Fatalf("expected ErrInvalidMovement, got %v", err)
			}
		})
	}
}

func TestPostMovement_SerializedQuantityMustBeOne(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	loc, _ := m.CreateLocation(ctx, testAgency, mwanachamaassetmanager.Location{Name: "Pasture"})
	a, _ := m.CreateAsset(ctx, testAgency, mwanachamaassetmanager.Asset{Name: "Bessie", TrackingMode: mwanachamaassetmanager.AssetTrackingModeSerialized, SerialTag: "COW-1"})

	_, err := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 2, ToLocationID: loc.ID, PerformedBy: testActor})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidMovement) {
		t.Fatalf("expected ErrInvalidMovement for quantity != 1 on a serialized asset, got %v", err)
	}

	if _, err := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 1, ToLocationID: loc.ID, PerformedBy: testActor}); err != nil {
		t.Fatalf("expected quantity 1 to succeed, got %v", err)
	}
}

func TestReverseMovement_CancelsBalance(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupFungibleAsset(t, m, ctx)

	mv, err := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 40, ToLocationID: loc.ID, PerformedBy: testActor})
	if err != nil {
		t.Fatalf("PostMovement: %v", err)
	}

	reversal, err := m.ReverseMovement(ctx, testAgency, mv.ID, testActor, "counted wrong")
	if err != nil {
		t.Fatalf("ReverseMovement: %v", err)
	}
	if reversal.Kind != mwanachamaassetmanager.MovementKindReversed {
		t.Fatalf("expected reversed kind, got %q", reversal.Kind)
	}
	if reversal.ReversesMovementID != mv.ID {
		t.Fatalf("expected ReversesMovementID %q, got %q", mv.ID, reversal.ReversesMovementID)
	}

	balance, err := m.GetAssetBalance(ctx, testAgency, a.ID, loc.ID)
	if err != nil {
		t.Fatalf("GetAssetBalance: %v", err)
	}
	if balance != 0 {
		t.Fatalf("expected balance 0 after reversal, got %d", balance)
	}

	// Original untouched.
	original, err := m.GetMovement(ctx, testAgency, mv.ID)
	if err != nil {
		t.Fatalf("GetMovement: %v", err)
	}
	if original.Quantity != 40 || original.Kind != mwanachamaassetmanager.MovementKindArrived {
		t.Fatalf("expected original untouched, got %+v", original)
	}
}

func TestReverseMovement_RequiresPerformedBy(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupFungibleAsset(t, m, ctx)

	mv, _ := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 5, ToLocationID: loc.ID, PerformedBy: testActor})
	if _, err := m.ReverseMovement(ctx, testAgency, mv.ID, "", "no actor"); !errors.Is(err, mwanachamaassetmanager.ErrInvalidMovement) {
		t.Fatalf("expected ErrInvalidMovement for empty performedBy, got %v", err)
	}
}

func TestReverseMovement_DoubleReverseRejected(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupFungibleAsset(t, m, ctx)

	mv, _ := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 5, ToLocationID: loc.ID, PerformedBy: testActor})
	if _, err := m.ReverseMovement(ctx, testAgency, mv.ID, testActor, "first"); err != nil {
		t.Fatalf("first ReverseMovement: %v", err)
	}
	if _, err := m.ReverseMovement(ctx, testAgency, mv.ID, testActor, "second"); !errors.Is(err, mwanachamaassetmanager.ErrMovementAlreadyReversed) {
		t.Fatalf("expected ErrMovementAlreadyReversed, got %v", err)
	}
}

func TestReverseMovement_CannotReverseAReversal(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupFungibleAsset(t, m, ctx)

	mv, _ := m.PostMovement(ctx, testAgency, mwanachamaassetmanager.Movement{AssetID: a.ID, Kind: mwanachamaassetmanager.MovementKindArrived, Quantity: 5, ToLocationID: loc.ID, PerformedBy: testActor})
	reversal, err := m.ReverseMovement(ctx, testAgency, mv.ID, testActor, "first")
	if err != nil {
		t.Fatalf("ReverseMovement: %v", err)
	}
	if _, err := m.ReverseMovement(ctx, testAgency, reversal.ID, testActor, "again"); !errors.Is(err, mwanachamaassetmanager.ErrInvalidMovement) {
		t.Fatalf("expected ErrInvalidMovement reversing a reversal, got %v", err)
	}
}

package mwanachamaassetmanager_test

import (
	"context"
	"errors"
	"testing"
	"time"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func setupStockedAsset(t *testing.T, m mwanachamaassetmanager.AssetManager, ctx context.Context, qty int64) (models.Asset, models.Location) {
	t.Helper()
	a, loc := setupFungibleAsset(t, m, ctx)
	if _, err := m.PostMovement(ctx, models.Movement{
		AssetID: a.ID, Kind: models.MovementKindArrived, Quantity: qty, ToLocationID: loc.ID, PerformedBy: testActor,
	}); err != nil {
		t.Fatalf("PostMovement arrived: %v", err)
	}
	return a, loc
}

func TestCreateHold_ReservesAgainstBalance(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 20)

	h, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 10, ExpiresAt: futureRFC3339(t), PlacedBy: testActor})
	if err != nil {
		t.Fatalf("CreateHold: %v", err)
	}
	if h.Status != models.HoldStatusReserved {
		t.Fatalf("expected reserved status, got %q", h.Status)
	}

	// A second hold for more than what's left (10 available of 20) is
	// rejected.
	if _, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 11, ExpiresAt: futureRFC3339(t), PlacedBy: testActor}); !errors.Is(err, mwanachamaassetmanager.ErrInsufficientAvailable) {
		t.Fatalf("expected ErrInsufficientAvailable, got %v", err)
	}
	// Exactly what's left succeeds.
	if _, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 10, ExpiresAt: futureRFC3339(t), PlacedBy: testActor}); err != nil {
		t.Fatalf("expected exact remaining quantity to succeed, got %v", err)
	}
}

func TestCreateHold_ExpiresAtMustBeFuture(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 20)

	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	_, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 1, ExpiresAt: past, PlacedBy: testActor})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidHold) {
		t.Fatalf("expected ErrInvalidHold for a past ExpiresAt, got %v", err)
	}
}

func TestCreateHold_MissingLocation(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, _ := setupStockedAsset(t, m, ctx, 20)

	_, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, Quantity: 1, ExpiresAt: futureRFC3339(t), PlacedBy: testActor})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidHold) {
		t.Fatalf("expected ErrInvalidHold for missing LocationID, got %v", err)
	}
}

func TestCreateHold_MissingPlacedBy(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 20)

	_, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 1, ExpiresAt: futureRFC3339(t)})
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidHold) {
		t.Fatalf("expected ErrInvalidHold for missing PlacedBy, got %v", err)
	}
}

func TestCommitHold_Full(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 20)

	h, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 10, ExpiresAt: futureRFC3339(t), PlacedBy: testActor})
	if err != nil {
		t.Fatalf("CreateHold: %v", err)
	}

	committed, mv, err := m.CommitHold(ctx, h.ID, 10, models.MovementKindDeparted, "", testActor)
	if err != nil {
		t.Fatalf("CommitHold: %v", err)
	}
	if committed.Status != models.HoldStatusCommitted {
		t.Fatalf("expected committed status, got %q", committed.Status)
	}
	if committed.CommittedQuantity != 10 {
		t.Fatalf("expected committed quantity 10, got %d", committed.CommittedQuantity)
	}
	if mv.Kind != models.MovementKindDeparted {
		t.Fatalf("expected departed movement, got %q", mv.Kind)
	}

	balance, _ := m.GetAssetBalance(ctx, a.ID, loc.ID)
	if balance != 10 {
		t.Fatalf("expected balance 10 after committing 10 of 20, got %d", balance)
	}
}

func TestCommitHold_PartialReleasesRemainderAndFreesAvailability(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 20)

	h, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 20, ExpiresAt: futureRFC3339(t), PlacedBy: testActor})
	if err != nil {
		t.Fatalf("CreateHold: %v", err)
	}

	committed, _, err := m.CommitHold(ctx, h.ID, 12, models.MovementKindDeparted, "", "picker-1")
	if err != nil {
		t.Fatalf("CommitHold: %v", err)
	}
	if committed.Status != models.HoldStatusPartiallyCommitted {
		t.Fatalf("expected partially_committed status, got %q", committed.Status)
	}
	if committed.CommittedQuantity != 12 {
		t.Fatalf("expected committed quantity 12, got %d", committed.CommittedQuantity)
	}

	// Balance: 20 arrived - 12 departed = 8. The hold is now terminal
	// (partially_committed), so it no longer reserves anything — the full
	// remaining 8 must be available to a fresh hold.
	if _, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 8, ExpiresAt: futureRFC3339(t), PlacedBy: testActor}); err != nil {
		t.Fatalf("expected the released remainder to be available, got %v", err)
	}
}

func TestCommitHold_WrongStatusRejected(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 20)

	h, _ := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 10, ExpiresAt: futureRFC3339(t), PlacedBy: testActor})
	if _, _, err := m.CommitHold(ctx, h.ID, 10, models.MovementKindDeparted, "", testActor); err != nil {
		t.Fatalf("first CommitHold: %v", err)
	}
	if _, _, err := m.CommitHold(ctx, h.ID, 10, models.MovementKindDeparted, "", testActor); !errors.Is(err, mwanachamaassetmanager.ErrInvalidHoldStatusTransition) {
		t.Fatalf("expected ErrInvalidHoldStatusTransition on a second commit, got %v", err)
	}
}

func TestReleaseHold(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 20)

	h, _ := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 20, ExpiresAt: futureRFC3339(t), PlacedBy: testActor})
	released, err := m.ReleaseHold(ctx, h.ID)
	if err != nil {
		t.Fatalf("ReleaseHold: %v", err)
	}
	if released.Status != models.HoldStatusReleased {
		t.Fatalf("expected released status, got %q", released.Status)
	}

	// Full quantity available again.
	if _, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 20, ExpiresAt: futureRFC3339(t), PlacedBy: testActor}); err != nil {
		t.Fatalf("expected released quantity to be available, got %v", err)
	}
}

func TestListHoldsExpiredAsOf(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 20)

	nearFuture := time.Now().UTC().Add(time.Minute).Format(time.RFC3339)
	h, err := m.CreateHold(ctx, models.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 5, ExpiresAt: nearFuture, PlacedBy: testActor})
	if err != nil {
		t.Fatalf("CreateHold: %v", err)
	}

	cutoffBefore := time.Now().UTC().Format(time.RFC3339)
	stillOpen, err := m.ListHoldsExpiredAsOf(ctx, cutoffBefore)
	if err != nil {
		t.Fatalf("ListHoldsExpiredAsOf: %v", err)
	}
	if len(stillOpen) != 0 {
		t.Fatalf("expected no expired holds yet, got %+v", stillOpen)
	}

	cutoffAfter := time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339)
	expired, err := m.ListHoldsExpiredAsOf(ctx, cutoffAfter)
	if err != nil {
		t.Fatalf("ListHoldsExpiredAsOf: %v", err)
	}
	if len(expired) != 1 || expired[0].ID != h.ID {
		t.Fatalf("expected [hold], got %+v", expired)
	}

	if _, err := m.ReleaseHold(ctx, expired[0].ID); err != nil {
		t.Fatalf("ReleaseHold (watchdog sweep): %v", err)
	}
	stillExpired, _ := m.ListHoldsExpiredAsOf(ctx, cutoffAfter)
	if len(stillExpired) != 0 {
		t.Fatalf("expected the swept hold to no longer be reserved, got %+v", stillExpired)
	}
}

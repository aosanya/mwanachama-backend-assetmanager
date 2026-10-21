package mwanachamaassetmanager_test

import (
	"context"
	"errors"
	"testing"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

func slotHold(assetID, locationID, start, end string, t *testing.T) mwanachamaassetmanager.Hold {
	return mwanachamaassetmanager.Hold{AssetID: assetID, LocationID: locationID, Quantity: 1, ExpiresAt: futureRFC3339(t), PlacedBy: testActor, StartsAt: start, EndsAt: end}
}

func TestCreateHold_OverlappingSlotOnTheSameAssetIsRefused(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 10)

	first, err := m.CreateHold(ctx, slotHold(a.ID, loc.ID, "2030-01-01T09:00:00Z", "2030-01-01T11:00:00Z", t))
	if err != nil {
		t.Fatalf("first slotted hold: %v", err)
	}
	if first.StartsAt != "2030-01-01T09:00:00Z" || first.EndsAt != "2030-01-01T11:00:00Z" {
		t.Fatalf("slot not kept: %+v", first)
	}
	read, err := m.GetHold(ctx, first.ID)
	if err != nil || read.StartsAt != first.StartsAt || read.EndsAt != first.EndsAt {
		t.Fatalf("slot not stored: %+v %v", read, err)
	}
	if _, err := m.CreateHold(ctx, slotHold(a.ID, loc.ID, "2030-01-01T10:00:00Z", "2030-01-01T12:00:00Z", t)); !errors.Is(err, mwanachamaassetmanager.ErrHoldSlotOverlaps) {
		t.Fatalf("expected ErrHoldSlotOverlaps, got %v", err)
	}
	if _, err := m.CreateHold(ctx, slotHold(a.ID, loc.ID, "2030-01-01T11:00:00Z", "2030-01-01T12:00:00Z", t)); err != nil {
		t.Fatalf("a slot starting when the other ends must be accepted: %v", err)
	}
}

func TestCreateHold_ReleasedHoldFreesItsSlot(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 10)

	first, err := m.CreateHold(ctx, slotHold(a.ID, loc.ID, "2030-01-01T09:00:00Z", "2030-01-01T11:00:00Z", t))
	if err != nil {
		t.Fatalf("first slotted hold: %v", err)
	}
	if _, err := m.ReleaseHold(ctx, first.ID); err != nil {
		t.Fatalf("ReleaseHold: %v", err)
	}
	if _, err := m.CreateHold(ctx, slotHold(a.ID, loc.ID, "2030-01-01T09:00:00Z", "2030-01-01T11:00:00Z", t)); err != nil {
		t.Fatalf("a released hold must not keep its slot: %v", err)
	}
}

func TestCreateHold_SlotsOnDifferentAssetsDoNotCollide(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 10)
	b, locB := setupStockedAsset(t, m, ctx, 10)

	if _, err := m.CreateHold(ctx, slotHold(a.ID, loc.ID, "2030-01-01T09:00:00Z", "2030-01-01T11:00:00Z", t)); err != nil {
		t.Fatalf("hold on a: %v", err)
	}
	if _, err := m.CreateHold(ctx, slotHold(b.ID, locB.ID, "2030-01-01T09:00:00Z", "2030-01-01T11:00:00Z", t)); err != nil {
		t.Fatalf("hold on b: %v", err)
	}
}

func TestCreateHold_HoldsWithoutASlotNeverCollide(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 10)

	if _, err := m.CreateHold(ctx, slotHold(a.ID, loc.ID, "2030-01-01T09:00:00Z", "2030-01-01T11:00:00Z", t)); err != nil {
		t.Fatalf("slotted hold: %v", err)
	}
	if _, err := m.CreateHold(ctx, slotHold(a.ID, loc.ID, "", "", t)); err != nil {
		t.Fatalf("unslotted hold beside a slotted one: %v", err)
	}
	if _, err := m.CreateHold(ctx, slotHold(a.ID, loc.ID, "", "", t)); err != nil {
		t.Fatalf("second unslotted hold: %v", err)
	}
}

func TestCreateHold_MalformedSlotIsInvalid(t *testing.T) {
	ctx := context.Background()
	m := newTestManager(t)
	a, loc := setupStockedAsset(t, m, ctx, 10)

	cases := map[string][2]string{
		"start only":       {"2030-01-01T09:00:00Z", ""},
		"end only":         {"", "2030-01-01T09:00:00Z"},
		"end before start": {"2030-01-01T11:00:00Z", "2030-01-01T09:00:00Z"},
		"empty slot":       {"2030-01-01T09:00:00Z", "2030-01-01T09:00:00Z"},
		"not rfc3339":      {"tomorrow", "2030-01-01T09:00:00Z"},
	}
	for name, c := range cases {
		if _, err := m.CreateHold(ctx, slotHold(a.ID, loc.ID, c[0], c[1], t)); !errors.Is(err, mwanachamaassetmanager.ErrInvalidHold) {
			t.Fatalf("%s: expected ErrInvalidHold, got %v", name, err)
		}
	}
}

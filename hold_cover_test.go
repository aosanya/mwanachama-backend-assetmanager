package mwanachamaassetmanager

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

const holdCoverDeclaration = `{
  "module": "assetmanager",
  "domain": "wakala",
  "instance": "holdcover",
  "mount": "assets",
  "objects": [
    {"role": "location", "name": "location", "table": "locations", "description": "d"},
    {"role": "asset", "name": "asset", "table": "assets", "description": "d"},
    {"role": "movement", "name": "movement", "table": "movements", "description": "d"},
    {"role": "hold", "name": "hold", "table": "holds", "description": "d"},
    {"role": "code_sequence", "name": "code_sequence", "table": "code_sequences", "description": "d"}
  ]
}`

type holdFixture struct {
	m     AssetManager
	asset models.Asset
	store models.Location
	other models.Location
}

func newHoldFixture(t *testing.T, onHand int64) holdFixture {
	t.Helper()
	file := filepath.Join(t.TempDir(), "holds.db") + "?_pragma=busy_timeout(10000)"
	db, err := gorm.Open(sqlite.Open(file), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	s, err := ParseSpec([]byte(holdCoverDeclaration))
	if err != nil {
		t.Fatal(err)
	}
	if err := Provision(db, s); err != nil {
		t.Fatal(err)
	}
	m, err := NewAssetManager(db, s)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	store, err := m.CreateLocation(ctx, models.Location{Name: "store"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := m.CreateLocation(ctx, models.Location{Name: "shelf"})
	if err != nil {
		t.Fatal(err)
	}
	asset, err := m.CreateAsset(ctx, models.Asset{Name: "eggs", TrackingMode: models.AssetTrackingModeFungible})
	if err != nil {
		t.Fatal(err)
	}
	if onHand > 0 {
		_, err = m.PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindArrived, Quantity: onHand, ToLocationID: store.ID, PerformedBy: "clerk"})
		if err != nil {
			t.Fatal(err)
		}
	}
	return holdFixture{m: m, asset: asset, store: store, other: other}
}

func (f holdFixture) hold(quantity int64) models.Hold {
	return models.Hold{
		AssetID:    f.asset.ID,
		LocationID: f.store.ID,
		Quantity:   quantity,
		ExpiresAt:  time.Now().UTC().Add(time.Hour).Format(time.RFC3339),
		PlacedBy:   "buyer",
	}
}

func inAnHour(offset time.Duration) string {
	return time.Now().UTC().Add(time.Hour + offset).Format(time.RFC3339)
}

func TestCreateHoldRefusesWhatCannotBeHeld(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	cases := map[string]func(h *models.Hold){
		"zero quantity":         func(h *models.Hold) { h.Quantity = 0 },
		"negative quantity":     func(h *models.Hold) { h.Quantity = -3 },
		"expiry not a time":     func(h *models.Hold) { h.ExpiresAt = "tomorrow" },
		"expiry already passed": func(h *models.Hold) { h.ExpiresAt = time.Now().UTC().Add(-time.Minute).Format(time.RFC3339) },
		"start without end":     func(h *models.Hold) { h.StartsAt = inAnHour(0) },
		"end without start":     func(h *models.Hold) { h.EndsAt = inAnHour(time.Hour) },
		"start not a time":      func(h *models.Hold) { h.StartsAt = "soon"; h.EndsAt = inAnHour(time.Hour) },
		"end not a time":        func(h *models.Hold) { h.StartsAt = inAnHour(0); h.EndsAt = "later" },
		"end before start":      func(h *models.Hold) { h.StartsAt = inAnHour(time.Hour); h.EndsAt = inAnHour(0) },
		"end equals start":      func(h *models.Hold) { h.StartsAt = inAnHour(0); h.EndsAt = inAnHour(0) },
		"no asset":              func(h *models.Hold) { h.AssetID = "" },
		"no location":           func(h *models.Hold) { h.LocationID = "" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			h := f.hold(2)
			mutate(&h)
			if _, err := f.m.CreateHold(ctx, h); !errors.Is(err, ErrInvalidHold) {
				t.Fatalf("want ErrInvalidHold, got %v", err)
			}
		})
	}
	listed, err := f.m.ListHolds(ctx, HoldFilter{})
	if err != nil || len(listed) != 0 {
		t.Fatalf("a refused hold must leave nothing behind, got %v %v", listed, err)
	}
}

func TestCreateHoldRefusesAnUnknownAssetOrLocation(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	h := f.hold(1)
	h.AssetID = "no-such-asset"
	if _, err := f.m.CreateHold(ctx, h); !errors.Is(err, ErrAssetNotFound) {
		t.Fatalf("want ErrAssetNotFound, got %v", err)
	}
	h = f.hold(1)
	h.LocationID = "no-such-location"
	if _, err := f.m.CreateHold(ctx, h); !errors.Is(err, ErrLocationNotFound) {
		t.Fatalf("want ErrLocationNotFound, got %v", err)
	}
}

func TestCreateHoldStartsReservedWithServerOwnedFields(t *testing.T) {
	f := newHoldFixture(t, 10)
	h := f.hold(4)
	h.ID = "chosen-by-caller"
	h.Status = models.HoldStatusCommitted
	h.CommittedQuantity = 4
	got, err := f.m.CreateHold(context.Background(), h)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID == "chosen-by-caller" || got.ID == "" {
		t.Fatalf("id must be minted by the store, got %q", got.ID)
	}
	if got.Status != models.HoldStatusReserved || got.CommittedQuantity != 0 {
		t.Fatalf("a new hold is reserved with nothing committed, got %s/%d", got.Status, got.CommittedQuantity)
	}
	read, err := f.m.GetHold(context.Background(), got.ID)
	if err != nil {
		t.Fatal(err)
	}
	if read.Status != models.HoldStatusReserved || read.Quantity != 4 || read.PlacedBy != "buyer" {
		t.Fatalf("stored hold differs: %+v", read)
	}
}

func TestCreateHoldCannotReserveMoreThanIsAvailable(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	if _, err := f.m.CreateHold(ctx, f.hold(7)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.CreateHold(ctx, f.hold(4)); !errors.Is(err, ErrInsufficientAvailable) {
		t.Fatalf("7 of 10 held, 4 more must be refused, got %v", err)
	}
	if _, err := f.m.CreateHold(ctx, f.hold(3)); err != nil {
		t.Fatalf("the remaining 3 are still holdable: %v", err)
	}
	if _, err := f.m.CreateHold(ctx, f.hold(1)); !errors.Is(err, ErrInsufficientAvailable) {
		t.Fatalf("everything is held, got %v", err)
	}
}

func TestCreateHoldOnAnEmptyLocationIsRefused(t *testing.T) {
	f := newHoldFixture(t, 10)
	h := f.hold(1)
	h.LocationID = f.other.ID
	if _, err := f.m.CreateHold(context.Background(), h); !errors.Is(err, ErrInsufficientAvailable) {
		t.Fatalf("nothing is on the shelf, got %v", err)
	}
}

func TestReleasedHoldFreesItsQuantityForAnotherHold(t *testing.T) {
	f := newHoldFixture(t, 5)
	ctx := context.Background()
	first, err := f.m.CreateHold(ctx, f.hold(5))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.CreateHold(ctx, f.hold(1)); !errors.Is(err, ErrInsufficientAvailable) {
		t.Fatalf("want ErrInsufficientAvailable, got %v", err)
	}
	released, err := f.m.ReleaseHold(ctx, first.ID)
	if err != nil {
		t.Fatal(err)
	}
	if released.Status != models.HoldStatusReleased {
		t.Fatalf("want released, got %s", released.Status)
	}
	if _, err := f.m.CreateHold(ctx, f.hold(5)); err != nil {
		t.Fatalf("released quantity is available again: %v", err)
	}
}

func TestOverlappingSlotsOnOneAssetAreRefused(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	slotted := func(start, end time.Duration) models.Hold {
		h := f.hold(1)
		h.StartsAt = inAnHour(start)
		h.EndsAt = inAnHour(end)
		return h
	}
	if _, err := f.m.CreateHold(ctx, slotted(0, 2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	for name, h := range map[string]models.Hold{
		"inside":      slotted(30*time.Minute, time.Hour),
		"straddles":   slotted(time.Hour, 3*time.Hour),
		"same window": slotted(0, 2*time.Hour),
		"covers":      slotted(-time.Hour, 3*time.Hour),
	} {
		if _, err := f.m.CreateHold(ctx, h); !errors.Is(err, ErrHoldSlotOverlaps) {
			t.Fatalf("%s: want ErrHoldSlotOverlaps, got %v", name, err)
		}
	}
	if _, err := f.m.CreateHold(ctx, slotted(2*time.Hour, 3*time.Hour)); err != nil {
		t.Fatalf("a slot starting exactly when the other ends does not overlap: %v", err)
	}
	if _, err := f.m.CreateHold(ctx, f.hold(1)); err != nil {
		t.Fatalf("a hold with no slot never conflicts on time: %v", err)
	}
}

func TestReleasedSlotCanBeBookedAgain(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	h := f.hold(1)
	h.StartsAt = inAnHour(0)
	h.EndsAt = inAnHour(time.Hour)
	first, err := f.m.CreateHold(ctx, h)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.ReleaseHold(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.CreateHold(ctx, h); err != nil {
		t.Fatalf("a released slot is free: %v", err)
	}
}

func TestGetHoldOfAnUnknownIdIsNotFound(t *testing.T) {
	f := newHoldFixture(t, 1)
	if _, err := f.m.GetHold(context.Background(), "missing"); !errors.Is(err, ErrHoldNotFound) {
		t.Fatalf("want ErrHoldNotFound, got %v", err)
	}
	if _, err := f.m.ReleaseHold(context.Background(), "missing"); !errors.Is(err, ErrHoldNotFound) {
		t.Fatalf("release: want ErrHoldNotFound, got %v", err)
	}
	if _, _, err := f.m.CommitHold(context.Background(), "missing", 1, models.MovementKindDeparted, "", "clerk"); !errors.Is(err, ErrHoldNotFound) {
		t.Fatalf("commit: want ErrHoldNotFound, got %v", err)
	}
}

func TestCommitHoldInFullDepartsStockAndClosesTheHold(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	hold, err := f.m.CreateHold(ctx, f.hold(4))
	if err != nil {
		t.Fatal(err)
	}
	committed, movement, err := f.m.CommitHold(ctx, hold.ID, 4, models.MovementKindDeparted, "", "clerk")
	if err != nil {
		t.Fatal(err)
	}
	if committed.Status != models.HoldStatusCommitted || committed.CommittedQuantity != 4 {
		t.Fatalf("want committed/4, got %s/%d", committed.Status, committed.CommittedQuantity)
	}
	if movement.Kind != models.MovementKindDeparted || movement.Quantity != 4 || movement.FromLocationID != f.store.ID {
		t.Fatalf("movement differs: %+v", movement)
	}
	balance, err := f.m.GetAssetBalance(ctx, f.asset.ID, f.store.ID)
	if err != nil || balance != 6 {
		t.Fatalf("want balance 6, got %d %v", balance, err)
	}
	stored, err := f.m.GetHold(ctx, hold.ID)
	if err != nil || stored.Status != models.HoldStatusCommitted {
		t.Fatalf("stored hold must be committed, got %+v %v", stored, err)
	}
}

func TestCommitHoldForLessMarksItPartiallyCommittedAndFreesTheRest(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	hold, err := f.m.CreateHold(ctx, f.hold(6))
	if err != nil {
		t.Fatal(err)
	}
	committed, _, err := f.m.CommitHold(ctx, hold.ID, 2, models.MovementKindTransferred, f.other.ID, "clerk")
	if err != nil {
		t.Fatal(err)
	}
	if committed.Status != models.HoldStatusPartiallyCommitted || committed.CommittedQuantity != 2 {
		t.Fatalf("want partially_committed/2, got %s/%d", committed.Status, committed.CommittedQuantity)
	}
	onShelf, err := f.m.GetAssetBalance(ctx, f.asset.ID, f.other.ID)
	if err != nil || onShelf != 2 {
		t.Fatalf("transfer should land 2 on the shelf, got %d %v", onShelf, err)
	}
	if _, err := f.m.CreateHold(ctx, f.hold(8)); err != nil {
		t.Fatalf("the uncommitted rest is released with the hold: %v", err)
	}
}

func TestCommitHoldRefusals(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	hold, err := f.m.CreateHold(ctx, f.hold(3))
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []int64{0, -1, 4} {
		if _, _, err := f.m.CommitHold(ctx, hold.ID, q, models.MovementKindDeparted, "", "clerk"); !errors.Is(err, ErrInvalidHold) {
			t.Fatalf("quantity %d: want ErrInvalidHold, got %v", q, err)
		}
	}
	for _, kind := range []models.MovementKind{models.MovementKindArrived, models.MovementKindAdjusted, models.MovementKindReversed, ""} {
		if _, _, err := f.m.CommitHold(ctx, hold.ID, 1, kind, "", "clerk"); !errors.Is(err, ErrInvalidMovement) {
			t.Fatalf("kind %q: want ErrInvalidMovement, got %v", kind, err)
		}
	}
	stored, err := f.m.GetHold(ctx, hold.ID)
	if err != nil || stored.Status != models.HoldStatusReserved || stored.CommittedQuantity != 0 {
		t.Fatalf("refused commits must leave the hold reserved, got %+v %v", stored, err)
	}
}

func TestRefusedMovementUnclaimsTheHoldSoItCanBeCommittedAgain(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	hold, err := f.m.CreateHold(ctx, f.hold(3))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.m.CommitHold(ctx, hold.ID, 3, models.MovementKindTransferred, "", "clerk"); !errors.Is(err, ErrInvalidMovement) {
		t.Fatalf("a transfer with no destination must be refused, got %v", err)
	}
	stored, err := f.m.GetHold(ctx, hold.ID)
	if err != nil || stored.Status != models.HoldStatusReserved || stored.CommittedQuantity != 0 {
		t.Fatalf("hold must be back to reserved, got %+v %v", stored, err)
	}
	if _, _, err := f.m.CommitHold(ctx, hold.ID, 3, models.MovementKindDeparted, "", "clerk"); err != nil {
		t.Fatalf("the hold is committable after the refusal: %v", err)
	}
	balance, _ := f.m.GetAssetBalance(ctx, f.asset.ID, f.store.ID)
	if balance != 7 {
		t.Fatalf("only the successful commit moved stock, got %d", balance)
	}
}

func TestAHoldClosedOnceCannotBeCommittedOrReleasedAgain(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	committed, err := f.m.CreateHold(ctx, f.hold(2))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.m.CommitHold(ctx, committed.ID, 2, models.MovementKindDeparted, "", "clerk"); err != nil {
		t.Fatal(err)
	}
	released, err := f.m.CreateHold(ctx, f.hold(2))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.ReleaseHold(ctx, released.ID); err != nil {
		t.Fatal(err)
	}
	for name, id := range map[string]string{"committed": committed.ID, "released": released.ID} {
		if _, err := f.m.ReleaseHold(ctx, id); !errors.Is(err, ErrInvalidHoldStatusTransition) {
			t.Fatalf("%s release: want ErrInvalidHoldStatusTransition, got %v", name, err)
		}
		if _, _, err := f.m.CommitHold(ctx, id, 1, models.MovementKindDeparted, "", "clerk"); !errors.Is(err, ErrInvalidHoldStatusTransition) {
			t.Fatalf("%s commit: want ErrInvalidHoldStatusTransition, got %v", name, err)
		}
	}
	balance, _ := f.m.GetAssetBalance(ctx, f.asset.ID, f.store.ID)
	if balance != 8 {
		t.Fatalf("a refused second commit must not move stock again, got %d", balance)
	}
}

func TestListHoldsFiltersByStatusAssetAndLocationAndPages(t *testing.T) {
	f := newHoldFixture(t, 20)
	ctx := context.Background()
	var ids []string
	for i := 0; i < 3; i++ {
		h, err := f.m.CreateHold(ctx, f.hold(1))
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, h.ID)
	}
	if _, err := f.m.ReleaseHold(ctx, ids[0]); err != nil {
		t.Fatal(err)
	}

	reserved, err := f.m.ListHolds(ctx, HoldFilter{Status: models.HoldStatusReserved})
	if err != nil || len(reserved) != 2 {
		t.Fatalf("want 2 reserved, got %d %v", len(reserved), err)
	}
	released, err := f.m.ListHolds(ctx, HoldFilter{Status: models.HoldStatusReleased})
	if err != nil || len(released) != 1 || released[0].ID != ids[0] {
		t.Fatalf("want the released one, got %+v %v", released, err)
	}
	none, err := f.m.ListHolds(ctx, HoldFilter{AssetID: "other-asset"})
	if err != nil || len(none) != 0 {
		t.Fatalf("an asset with no holds lists nothing, got %d %v", len(none), err)
	}
	none, err = f.m.ListHolds(ctx, HoldFilter{LocationID: f.other.ID})
	if err != nil || len(none) != 0 {
		t.Fatalf("a location with no holds lists nothing, got %d %v", len(none), err)
	}
	all, err := f.m.ListHolds(ctx, HoldFilter{AssetID: f.asset.ID, LocationID: f.store.ID})
	if err != nil || len(all) != 3 {
		t.Fatalf("want 3, got %d %v", len(all), err)
	}
	for i := 1; i < len(all); i++ {
		if all[i-1].ID >= all[i].ID {
			t.Fatalf("holds must list in id order: %s then %s", all[i-1].ID, all[i].ID)
		}
	}
	first, err := f.m.ListHolds(ctx, HoldFilter{Limit: 2})
	if err != nil || len(first) != 2 {
		t.Fatalf("limit 2 gives 2, got %d %v", len(first), err)
	}
	rest, err := f.m.ListHolds(ctx, HoldFilter{Limit: 2, Offset: 2})
	if err != nil || len(rest) != 1 || rest[0].ID != all[2].ID {
		t.Fatalf("offset 2 gives the third hold, got %+v %v", rest, err)
	}
}

func TestListHoldsExpiredAsOfReturnsOnlyReservedHoldsPastTheCutoff(t *testing.T) {
	f := newHoldFixture(t, 10)
	ctx := context.Background()
	soon := f.hold(1)
	soon.ExpiresAt = time.Now().UTC().Add(2 * time.Minute).Format(time.RFC3339)
	early, err := f.m.CreateHold(ctx, soon)
	if err != nil {
		t.Fatal(err)
	}
	late, err := f.m.CreateHold(ctx, f.hold(1))
	if err != nil {
		t.Fatal(err)
	}
	closedSoon := soon
	closed, err := f.m.CreateHold(ctx, closedSoon)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.m.ReleaseHold(ctx, closed.ID); err != nil {
		t.Fatal(err)
	}

	cutoff := time.Now().UTC().Add(10 * time.Minute).Format(time.RFC3339)
	expired, err := f.m.ListHoldsExpiredAsOf(ctx, cutoff)
	if err != nil {
		t.Fatal(err)
	}
	if len(expired) != 1 || expired[0].ID != early.ID {
		t.Fatalf("want only the early reserved hold, got %+v (late is %s)", expired, late.ID)
	}
	before, err := f.m.ListHoldsExpiredAsOf(ctx, time.Now().UTC().Format(time.RFC3339))
	if err != nil || len(before) != 0 {
		t.Fatalf("nothing has expired yet, got %+v %v", before, err)
	}
}

func TestListHoldsExpiredAsOfRefusesACutoffThatIsNotATime(t *testing.T) {
	f := newHoldFixture(t, 1)
	if _, err := f.m.ListHoldsExpiredAsOf(context.Background(), "now"); !errors.Is(err, ErrInvalidHold) {
		t.Fatalf("want ErrInvalidHold, got %v", err)
	}
}

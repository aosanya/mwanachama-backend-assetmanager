package mwanachamaassetmanager_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

type interferingManager struct {
	mwanachamaassetmanager.AssetManager
	db     *gorm.DB
	tables map[string]string
	hooks  atomic.Int64
}

func newInterferingManager(t *testing.T) *interferingManager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	return managerOnDB(t, db)
}

func managerOnDB(t *testing.T, db *gorm.DB) *interferingManager {
	t.Helper()
	s := testSpec(t)
	if err := mwanachamaassetmanager.Provision(db, s); err != nil {
		t.Fatalf("Provision: %v", err)
	}
	mgr, err := mwanachamaassetmanager.NewAssetManager(db, s)
	if err != nil {
		t.Fatalf("NewAssetManager: %v", err)
	}
	tables := map[string]string{}
	for _, role := range []string{"location", "asset", "movement", "hold", "code_sequence"} {
		o, ok := s.ByRole(role)
		if !ok {
			t.Fatalf("the spec fills no %q role", role)
		}
		tables[role] = s.TableFor(o)
	}
	return &interferingManager{AssetManager: mgr, db: db, tables: tables}
}

func (im *interferingManager) afterEveryReadOf(t *testing.T, role string, statement string, args ...any) {
	t.Helper()
	table := im.tables[role]
	name := fmt.Sprintf("interfere-%s-%d", role, im.hooks.Add(1))
	err := im.db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table != table || tx.Error != nil {
			return
		}
		if _, err := tx.Statement.ConnPool.ExecContext(tx.Statement.Context, statement, args...); err != nil {
			tx.AddError(err)
		}
	})
	if err != nil {
		t.Fatalf("register read hook: %v", err)
	}
	t.Cleanup(func() { _ = im.db.Callback().Query().Remove(name) })
}

func (im *interferingManager) afterTheNextReadOf(t *testing.T, role string, statement string, args ...any) {
	t.Helper()
	table := im.tables[role]
	var fired atomic.Bool
	name := fmt.Sprintf("interfere-once-%s-%d", role, im.hooks.Add(1))
	err := im.db.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		if tx.Statement.Table != table || tx.Error != nil || !fired.CompareAndSwap(false, true) {
			return
		}
		if _, err := tx.Statement.ConnPool.ExecContext(tx.Statement.Context, statement, args...); err != nil {
			tx.AddError(err)
		}
	})
	if err != nil {
		t.Fatalf("register read hook: %v", err)
	}
	t.Cleanup(func() { _ = im.db.Callback().Query().Remove(name) })
}

func (im *interferingManager) rawString(t *testing.T, role, column, id string) string {
	t.Helper()
	var out string
	if err := im.db.Table(im.tables[role]).Select(column).Where("id = ?", id).Scan(&out).Error; err != nil {
		t.Fatalf("raw select %s.%s: %v", role, column, err)
	}
	return out
}

func (im *interferingManager) rawNextNumber(t *testing.T, entityType string) int64 {
	t.Helper()
	var out int64
	if err := im.db.Table(im.tables["code_sequence"]).Select("next_number").Where("entity_type = ?", entityType).Scan(&out).Error; err != nil {
		t.Fatalf("raw select next_number: %v", err)
	}
	return out
}

func bumpCounter(im *interferingManager, entityType string) (string, string) {
	return fmt.Sprintf("UPDATE %s SET next_number = next_number + 1 WHERE entity_type = ?", im.tables["code_sequence"]), entityType
}

func TestNextCode_RetriesWhenAnotherMinterMovedTheCounterAfterItsRead(t *testing.T) {
	ctx := context.Background()
	im := newInterferingManager(t)
	if _, err := im.CreateAsset(ctx, mwanachamaassetmanager.Asset{Name: "first", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible}); err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	statement, entity := bumpCounter(im, "asset")
	im.afterTheNextReadOf(t, "code_sequence", statement, entity)

	a, err := im.CreateAsset(ctx, mwanachamaassetmanager.Asset{Name: "second", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	if a.Code != "A-3" {
		t.Fatalf("code = %q, want A-3 (A-2 was taken by the other minter between read and write)", a.Code)
	}
	if got := im.rawNextNumber(t, "asset"); got != 4 {
		t.Fatalf("counter = %d, want 4", got)
	}
}

func TestNextCode_GivesUpWithErrConflictWhenTheCounterKeepsMoving(t *testing.T) {
	ctx := context.Background()
	im := newInterferingManager(t)
	if _, err := im.CreateLocation(ctx, mwanachamaassetmanager.Location{Name: "first"}); err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	statement, entity := bumpCounter(im, "location")
	im.afterEveryReadOf(t, "code_sequence", statement, entity)

	_, err := im.CreateLocation(ctx, mwanachamaassetmanager.Location{Name: "second"})
	if !errors.Is(err, mwanachamaassetmanager.ErrConflict) {
		t.Fatalf("err = %v, want ErrConflict", err)
	}
}

func newSharedCacheManager(t *testing.T, name string) *interferingManager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", name)), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm.Open: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(4)
	t.Cleanup(func() { sqlDB.Close() })
	return managerOnDB(t, db)
}

type mintOutcome struct {
	code string
	err  error
}

func raceMints(writers int, mint func(i int) (string, error)) []mintOutcome {
	outcomes := make([]mintOutcome, writers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			code, err := mint(i)
			outcomes[i] = mintOutcome{code: code, err: err}
		}()
	}
	close(start)
	wg.Wait()
	return outcomes
}

func requireDistinctCodesAndClassifiedLosers(t *testing.T, round int, outcomes []mintOutcome) {
	t.Helper()
	seen := map[string]bool{}
	for _, o := range outcomes {
		if o.err != nil {
			if !errors.Is(o.err, mwanachamaassetmanager.ErrConflict) {
				t.Errorf("round %d: a losing writer saw %v, want ErrConflict", round, o.err)
			}
			continue
		}
		if seen[o.code] {
			t.Fatalf("round %d: two writers were handed the code %q", round, o.code)
		}
		seen[o.code] = true
	}
	if len(seen) == 0 {
		t.Errorf("round %d: no writer minted a code", round)
	}
}

func TestConcurrentCreateAssetMintsDistinctCodesAndClassifiesTheRest(t *testing.T) {
	ctx := context.Background()
	im := newSharedCacheManager(t, "amcreateasset")
	for round := 0; round < 10; round++ {
		outcomes := raceMints(8, func(i int) (string, error) {
			a, err := im.CreateAsset(ctx, mwanachamaassetmanager.Asset{Name: fmt.Sprintf("a%d-%d", round, i), TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible})
			return a.Code, err
		})
		requireDistinctCodesAndClassifiedLosers(t, round, outcomes)
	}
}

func TestConcurrentCreateLocationMintsDistinctCodesAndClassifiesTheRest(t *testing.T) {
	ctx := context.Background()
	im := newSharedCacheManager(t, "amcreatelocation")
	for round := 0; round < 10; round++ {
		outcomes := raceMints(8, func(i int) (string, error) {
			l, err := im.CreateLocation(ctx, mwanachamaassetmanager.Location{Name: fmt.Sprintf("l%d-%d", round, i)})
			return l.Code, err
		})
		requireDistinctCodesAndClassifiedLosers(t, round, outcomes)
	}
}

func reservedHold(t *testing.T, im *interferingManager, ctx context.Context) (mwanachamaassetmanager.Hold, mwanachamaassetmanager.Asset, mwanachamaassetmanager.Location) {
	t.Helper()
	a, loc := setupStockedAsset(t, im, ctx, 10)
	h, err := im.CreateHold(ctx, mwanachamaassetmanager.Hold{AssetID: a.ID, LocationID: loc.ID, Quantity: 5, ExpiresAt: futureRFC3339(t), PlacedBy: testActor})
	if err != nil {
		t.Fatalf("CreateHold: %v", err)
	}
	return h, a, loc
}

func setHoldStatus(im *interferingManager, status, id string) (string, string, string) {
	return fmt.Sprintf("UPDATE %s SET status = ? WHERE id = ?", im.tables["hold"]), status, id
}

func TestCommitHold_RefusesAHoldReleasedAfterItReadItAndPostsNoMovement(t *testing.T) {
	ctx := context.Background()
	im := newInterferingManager(t)
	h, a, loc := reservedHold(t, im, ctx)
	statement, status, id := setHoldStatus(im, "released", h.ID)
	im.afterTheNextReadOf(t, "hold", statement, status, id)

	_, _, err := im.CommitHold(ctx, h.ID, 5, mwanachamaassetmanager.MovementKindDeparted, "", testActor)
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidHoldStatusTransition) {
		t.Fatalf("err = %v, want ErrInvalidHoldStatusTransition", err)
	}
	if got := im.rawString(t, "hold", "status", h.ID); got != "released" {
		t.Fatalf("hold status = %q, want released", got)
	}
	balance, err := im.GetAssetBalance(ctx, a.ID, loc.ID)
	if err != nil || balance != 10 {
		t.Fatalf("balance = %d, %v; want 10, nil (the losing commit must not post a departure)", balance, err)
	}
}

func TestReleaseHold_RefusesAHoldCommittedAfterItReadItAndLeavesItCommitted(t *testing.T) {
	ctx := context.Background()
	im := newInterferingManager(t)
	h, _, _ := reservedHold(t, im, ctx)
	statement, status, id := setHoldStatus(im, "committed", h.ID)
	im.afterTheNextReadOf(t, "hold", statement, status, id)

	_, err := im.ReleaseHold(ctx, h.ID)
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidHoldStatusTransition) {
		t.Fatalf("err = %v, want ErrInvalidHoldStatusTransition", err)
	}
	if got := im.rawString(t, "hold", "status", h.ID); got != "committed" {
		t.Fatalf("hold status = %q, want committed", got)
	}
}

func TestCommitHold_AMovementThatIsRefusedLeavesTheHoldReserved(t *testing.T) {
	ctx := context.Background()
	im := newInterferingManager(t)
	h, a, loc := reservedHold(t, im, ctx)

	_, _, err := im.CommitHold(ctx, h.ID, 5, mwanachamaassetmanager.MovementKindTransferred, "", testActor)
	if !errors.Is(err, mwanachamaassetmanager.ErrInvalidMovement) {
		t.Fatalf("err = %v, want ErrInvalidMovement", err)
	}
	if got := im.rawString(t, "hold", "status", h.ID); got != "reserved" {
		t.Fatalf("hold status = %q, want reserved", got)
	}
	if _, _, err := im.CommitHold(ctx, h.ID, 5, mwanachamaassetmanager.MovementKindDeparted, "", testActor); err != nil {
		t.Fatalf("a retry with a valid movement must still commit: %v", err)
	}
	balance, err := im.GetAssetBalance(ctx, a.ID, loc.ID)
	if err != nil || balance != 5 {
		t.Fatalf("balance = %d, %v; want 5, nil", balance, err)
	}
}

func TestUpdateAsset_RefusesARowDeletedAfterItReadItAndLeavesTheNameAlone(t *testing.T) {
	ctx := context.Background()
	im := newInterferingManager(t)
	a, err := im.CreateAsset(ctx, mwanachamaassetmanager.Asset{Name: "original", TrackingMode: mwanachamaassetmanager.AssetTrackingModeFungible})
	if err != nil {
		t.Fatalf("CreateAsset: %v", err)
	}
	im.afterTheNextReadOf(t, "asset", fmt.Sprintf("UPDATE %s SET deleted = ? WHERE id = ?", im.tables["asset"]), true, a.ID)

	a.Name = "raced-update"
	_, err = im.UpdateAsset(ctx, a)
	if !errors.Is(err, mwanachamaassetmanager.ErrAssetNotFound) {
		t.Fatalf("err = %v, want ErrAssetNotFound", err)
	}
	if got := im.rawString(t, "asset", "name", a.ID); got != "original" {
		t.Fatalf("name = %q, want original", got)
	}
}

func TestUpdateLocation_RefusesARowDeletedAfterItReadItAndLeavesTheNameAlone(t *testing.T) {
	ctx := context.Background()
	im := newInterferingManager(t)
	l, err := im.CreateLocation(ctx, mwanachamaassetmanager.Location{Name: "original"})
	if err != nil {
		t.Fatalf("CreateLocation: %v", err)
	}
	im.afterTheNextReadOf(t, "location", fmt.Sprintf("UPDATE %s SET deleted = ? WHERE id = ?", im.tables["location"]), true, l.ID)

	l.Name = "raced-update"
	_, err = im.UpdateLocation(ctx, l)
	if !errors.Is(err, mwanachamaassetmanager.ErrLocationNotFound) {
		t.Fatalf("err = %v, want ErrLocationNotFound", err)
	}
	if got := im.rawString(t, "location", "name", l.ID); got != "original" {
		t.Fatalf("name = %q, want original", got)
	}
}

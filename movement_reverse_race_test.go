package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/aosanya/mwanachama-backend-shared/gormutil"
)

const reverseRaceDeclaration = `{
  "module": "assetmanager",
  "domain": "wakala",
  "instance": "reverserace",
  "mount": "assets",
  "objects": [
    {"role": "location", "name": "location", "table": "locations", "description": "d"},
    {"role": "asset", "name": "asset", "table": "assets", "description": "d"},
    {"role": "movement", "name": "movement", "table": "movements", "description": "d"},
    {"role": "hold", "name": "hold", "table": "holds", "description": "d"},
    {"role": "code_sequence", "name": "code_sequence", "table": "code_sequences", "description": "d"}
  ]
}`

func reverseRaceDatabases(t *testing.T) map[string]func() *gorm.DB {
	t.Helper()
	quiet := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}
	file := filepath.Join(t.TempDir(), "reverse.db") + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)"
	out := map[string]func() *gorm.DB{
		"sqlite": func() *gorm.DB {
			db, err := gorm.Open(sqlite.Open(file), quiet)
			if err != nil {
				t.Fatal(err)
			}
			return db
		},
	}
	if base := os.Getenv("ASSETMANAGER_REVERSE_PG"); base != "" {
		name := fmt.Sprintf("reverse_%d", time.Now().UnixNano()%1e9)
		admin, err := gorm.Open(postgres.Open(base+" dbname=postgres"), quiet)
		if err != nil {
			t.Fatal(err)
		}
		if err := admin.Exec("create database " + name).Error; err != nil {
			t.Fatal(err)
		}
		out["postgres"] = func() *gorm.DB {
			db, err := gorm.Open(postgres.Open(base+" dbname="+name), quiet)
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			sqlDB.SetMaxOpenConns(40)
			t.Cleanup(func() { sqlDB.Close() })
			return db
		}
	}
	return out
}

func reverseRaceManagers(t *testing.T, open func() *gorm.DB) []AssetManager {
	t.Helper()
	var managers []AssetManager
	for i := 0; i < 2; i++ {
		db := open()
		s, err := ParseSpec([]byte(reverseRaceDeclaration))
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
		managers = append(managers, m)
	}
	return managers
}

func reverseRaceStock(t *testing.T, m AssetManager) (models.Asset, models.Location, models.Movement) {
	t.Helper()
	ctx := context.Background()
	store, err := m.CreateLocation(ctx, models.Location{Name: "store"})
	if err != nil {
		t.Fatal(err)
	}
	asset, err := m.CreateAsset(ctx, models.Asset{Name: "eggs", TrackingMode: models.AssetTrackingModeFungible})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindArrived, Quantity: 10, ToLocationID: store.ID, PerformedBy: "clerk"}); err != nil {
		t.Fatal(err)
	}
	departure, err := m.PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindDeparted, Quantity: 2, FromLocationID: store.ID, PerformedBy: "clerk"})
	if err != nil {
		t.Fatal(err)
	}
	return asset, store, departure
}

func TestReversalsSentAtOnceReverseTheMovementOnce(t *testing.T) {
	for name, open := range reverseRaceDatabases(t) {
		t.Run(name, func(t *testing.T) {
			managers := reverseRaceManagers(t, open)
			ctx := context.Background()
			const sends, rounds = 8, 6
			for round := 0; round < rounds; round++ {
				asset, store, departure := reverseRaceStock(t, managers[0])
				results := make([]error, sends)
				start := make(chan struct{})
				var wg sync.WaitGroup
				for i := 0; i < sends; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						<-start
						_, results[i] = managers[i%2].ReverseMovement(ctx, departure.ID, "clerk", "again")
					}(i)
				}
				close(start)
				wg.Wait()
				reversed := 0
				for _, err := range results {
					switch {
					case err == nil:
						reversed++
					case errors.Is(err, ErrMovementAlreadyReversed):
					default:
						t.Fatalf("round %d: a reversal failed with %v, want it reversed or answered already reversed", round, err)
					}
				}
				if reversed != 1 {
					t.Fatalf("round %d: %d reversals of one movement succeeded, want 1", round, reversed)
				}
				balance, err := managers[0].GetAssetBalance(ctx, asset.ID, store.ID)
				if err != nil {
					t.Fatal(err)
				}
				if balance != 10 {
					t.Fatalf("round %d: the balance is %d after one departure of 2 was reversed, want 10", round, balance)
				}
			}
		})
	}
}

func TestTwoReversalRowsOfOneMovementAreRefusedByTheStore(t *testing.T) {
	for name, open := range reverseRaceDatabases(t) {
		t.Run(name, func(t *testing.T) {
			manager := reverseRaceManagers(t, open)[0].(*assetManager)
			ctx := context.Background()
			_, _, departure := reverseRaceStock(t, manager)
			row := func() models.Movement {
				return models.Movement{ID: newID(), AssetID: departure.AssetID, Kind: models.MovementKindReversed, Quantity: 2, ToLocationID: departure.FromLocationID,
					PerformedBy: "clerk", ReversesMovementID: departure.ID, OccurredAt: "2026-11-11T00:00:00Z", CreatedAt: "2026-11-11T00:00:00Z"}
			}
			if err := manager.store.Insert(ctx, roleMovement, row()); err != nil {
				t.Fatal(err)
			}
			if err := manager.store.Insert(ctx, roleMovement, row()); !gormutil.IsUniqueViolation(err) {
				t.Fatalf("a second reversal row of one movement must be a unique violation, got %v", err)
			}
		})
	}
}

func TestMovementsThatReverseNothingMayShareTheEmptyKey(t *testing.T) {
	for name, open := range reverseRaceDatabases(t) {
		t.Run(name, func(t *testing.T) {
			manager := reverseRaceManagers(t, open)[0]
			asset, store, _ := reverseRaceStock(t, manager)
			for i := 0; i < 3; i++ {
				if _, err := manager.PostMovement(context.Background(), models.Movement{AssetID: asset.ID, Kind: models.MovementKindArrived, Quantity: 1, ToLocationID: store.ID, PerformedBy: "clerk"}); err != nil {
					t.Fatalf("arrival %d: %v", i, err)
				}
			}
		})
	}
}

func TestProvisionAgainDropsDuplicateReversalsAndKeepsTheIndex(t *testing.T) {
	for name, open := range reverseRaceDatabases(t) {
		t.Run(name, func(t *testing.T) {
			db := open()
			s, err := ParseSpec([]byte(reverseRaceDeclaration))
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
			manager := m.(*assetManager)
			ctx := context.Background()
			asset, store, departure := reverseRaceStock(t, m)
			for _, idx := range manager.store.Object(roleMovement).Indexes {
				if idx.Unique {
					if err := db.Exec("drop index if exists " + s.IndexFor(manager.store.Object(roleMovement), idx)).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			for i := 0; i < 3; i++ {
				dup := models.Movement{ID: newID(), AssetID: asset.ID, Kind: models.MovementKindReversed, Quantity: 2, ToLocationID: store.ID,
					PerformedBy: "clerk", ReversesMovementID: departure.ID, OccurredAt: "2026-11-11T00:00:00Z", CreatedAt: "2026-11-11T00:00:00Z"}
				if err := manager.store.Insert(ctx, roleMovement, dup); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				if err := Provision(db, s); err != nil {
					t.Fatal(err)
				}
			}
			balance, err := m.GetAssetBalance(ctx, asset.ID, store.ID)
			if err != nil {
				t.Fatal(err)
			}
			if balance != 10 {
				t.Fatalf("after provisioning the balance is %d, want 10 with one reversal left", balance)
			}
			if _, err := m.ReverseMovement(ctx, departure.ID, "clerk", ""); !errors.Is(err, ErrMovementAlreadyReversed) {
				t.Fatalf("the movement is still reversed once, got %v", err)
			}
		})
	}
}

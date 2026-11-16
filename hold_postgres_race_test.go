package mwanachamaassetmanager_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

const holdRaceDeclaration = `{
  "module": "assetmanager",
  "domain": "wakala",
  "instance": "holdrace",
  "mount": "assets",
  "objects": [
    {"role": "location", "name": "location", "table": "locations", "description": "x"},
    {"role": "asset", "name": "asset", "table": "assets", "description": "x"},
    {"role": "movement", "name": "movement", "table": "movements", "description": "x"},
    {"role": "hold", "name": "hold", "table": "holds", "description": "x"},
    {"role": "code_sequence", "name": "code_sequence", "table": "code_sequences", "description": "x"}
  ]
}`

func holdRaceSchemaOrSkip(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("DATABASE_URL is not set")
	}
	u, err := url.Parse(dsn)
	if err != nil || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") {
		t.Skip("DATABASE_URL does not point at 127.0.0.1 or localhost")
	}
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open postgres: %v", err)
	}
	schema := fmt.Sprintf("hold_race_%d", time.Now().UnixNano())
	if err := admin.Exec("create schema " + schema).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	query := u.Query()
	query.Set("search_path", schema)
	u.RawQuery = query.Encode()
	db, err := gorm.Open(postgres.Open(u.String()), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open postgres in %s: %v", schema, err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(40)
	t.Cleanup(func() {
		sqlDB.Close()
		admin.Exec("drop schema " + schema + " cascade")
		if adminDB, err := admin.DB(); err == nil {
			adminDB.Close()
		}
	})
	return db
}

func TestHoldCreateNeverReservesMoreThanTheBalanceWhenHoldsArriveAtOnceOnPostgres(t *testing.T) {
	db := holdRaceSchemaOrSkip(t)
	s, err := mwanachamaassetmanager.ParseSpec([]byte(holdRaceDeclaration))
	if err != nil {
		t.Fatal(err)
	}
	if err := mwanachamaassetmanager.Provision(db, s); err != nil {
		t.Fatal(err)
	}
	manager, err := mwanachamaassetmanager.NewAssetManager(db, s)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	const rounds, holders, stock, wanted = 6, 8, 5, 3
	for round := 0; round < rounds; round++ {
		location, err := manager.CreateLocation(ctx, models.Location{Name: "Store"})
		if err != nil {
			t.Fatal(err)
		}
		asset, err := manager.CreateAsset(ctx, models.Asset{Name: "Rice", TrackingMode: models.AssetTrackingModeFungible})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := manager.PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindArrived, Quantity: stock, ToLocationID: location.ID, PerformedBy: "u"}); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		errs := make([]error, holders)
		var wg sync.WaitGroup
		for i := 0; i < holders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				_, errs[i] = manager.CreateHold(ctx, models.Hold{
					AssetID:    asset.ID,
					LocationID: location.ID,
					Quantity:   wanted,
					ExpiresAt:  time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
					PlacedBy:   "u",
				})
			}()
		}
		close(start)
		wg.Wait()
		wins := 0
		for _, err := range errs {
			switch {
			case err == nil:
				wins++
			case !errors.Is(err, mwanachamaassetmanager.ErrInsufficientAvailable):
				t.Fatalf("round %d: a refused hold answered %v, want ErrInsufficientAvailable", round, err)
			}
		}
		held, err := manager.ListHolds(ctx, mwanachamaassetmanager.HoldFilter{AssetID: asset.ID})
		if err != nil {
			t.Fatal(err)
		}
		reserved := int64(0)
		for _, h := range held {
			reserved += h.Quantity
		}
		if wins != 1 || reserved != wanted {
			t.Fatalf("round %d: %d holds succeeded and %d is reserved of a stock of %d, want 1 and %d", round, wins, reserved, int64(stock), int64(wanted))
		}
	}
}

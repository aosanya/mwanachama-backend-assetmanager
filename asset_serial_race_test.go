package mwanachamaassetmanager

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/aosanya/mwanachama-backend-shared/gormtest"
)

func serialRaceStores(t *testing.T) map[string]func() []AssetManager {
	t.Helper()
	stores := map[string]func() []AssetManager{}
	for name, open := range reverseRaceDatabases(t) {
		open := open
		stores[name] = func() []AssetManager { return reverseRaceManagers(t, open) }
	}
	stores["memory"] = func() []AssetManager {
		db := gormtest.OpenSQLiteDB(t)
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		sqlDB.SetMaxOpenConns(1)
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
		return []AssetManager{m}
	}
	return stores
}

func countAssetsWithTag(t *testing.T, m AssetManager, tag string) int {
	t.Helper()
	assets, err := m.ListAssets(context.Background(), AssetFilter{})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, a := range assets {
		if a.SerialTag == tag {
			count++
		}
	}
	return count
}

func TestCreatesSentAtOnceWithOneSerialTagCreateOneAsset(t *testing.T) {
	for name, managersOf := range serialRaceStores(t) {
		t.Run(name, func(t *testing.T) {
			managers := managersOf()
			ctx := context.Background()
			const sends, rounds = 8, 6
			for round := 0; round < rounds; round++ {
				tag := "ear-" + newID()
				results := make([]error, sends)
				start := make(chan struct{})
				var wg sync.WaitGroup
				for i := 0; i < sends; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						<-start
						_, results[i] = managers[i%len(managers)].CreateAsset(ctx, models.Asset{Name: "cow", TrackingMode: models.AssetTrackingModeSerialized, SerialTag: tag})
					}(i)
				}
				close(start)
				wg.Wait()
				created := 0
				for _, err := range results {
					switch {
					case err == nil:
						created++
					case errors.Is(err, ErrAssetSerialTagExists):
					default:
						t.Fatalf("round %d: a create failed with %v, want it created or answered serial tag already in use", round, err)
					}
				}
				if created != 1 {
					t.Fatalf("round %d: %d creates of one serial tag succeeded, want 1", round, created)
				}
				if rows := countAssetsWithTag(t, managers[0], tag); rows != 1 {
					t.Fatalf("round %d: %d assets carry the tag, want 1", round, rows)
				}
			}
		})
	}
}

func TestAnAssetRenamedToAnotherAssetsSerialTagIsRefusedAsInUse(t *testing.T) {
	for name, managersOf := range serialRaceStores(t) {
		t.Run(name, func(t *testing.T) {
			m := managersOf()[0]
			ctx := context.Background()
			first, err := m.CreateAsset(ctx, models.Asset{Name: "cow", TrackingMode: models.AssetTrackingModeSerialized, SerialTag: "ear-1"})
			if err != nil {
				t.Fatal(err)
			}
			second, err := m.CreateAsset(ctx, models.Asset{Name: "cow", TrackingMode: models.AssetTrackingModeSerialized, SerialTag: "ear-2"})
			if err != nil {
				t.Fatal(err)
			}
			second.SerialTag = first.SerialTag
			if _, err := m.UpdateAsset(ctx, renamed(second, second.Name, "")); !errors.Is(err, ErrAssetSerialTagExists) {
				t.Fatalf("updating onto a tag in use answered %v, want ErrAssetSerialTagExists", err)
			}
		})
	}
}

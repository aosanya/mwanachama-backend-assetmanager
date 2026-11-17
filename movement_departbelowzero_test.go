package mwanachamaassetmanager

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func departStock(t *testing.T, m AssetManager, quantity int64) (models.Asset, models.Location, models.Location) {
	t.Helper()
	ctx := context.Background()
	store, err := m.CreateLocation(ctx, models.Location{Name: "store"})
	if err != nil {
		t.Fatal(err)
	}
	shelf, err := m.CreateLocation(ctx, models.Location{Name: "shelf"})
	if err != nil {
		t.Fatal(err)
	}
	asset, err := m.CreateAsset(ctx, models.Asset{Name: "eggs", TrackingMode: models.AssetTrackingModeFungible})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindArrived, Quantity: quantity, ToLocationID: store.ID, PerformedBy: "clerk"}); err != nil {
		t.Fatal(err)
	}
	return asset, store, shelf
}

func movementCount(t *testing.T, m AssetManager, assetID string) int {
	t.Helper()
	all, err := m.ListMovements(context.Background(), MovementFilter{AssetID: assetID})
	if err != nil {
		t.Fatal(err)
	}
	return len(all)
}

func TestADepartureAboveTheBalanceIsRefusedAndWritesNothing(t *testing.T) {
	for name, open := range reverseRaceDatabases(t) {
		t.Run(name, func(t *testing.T) {
			m := reverseRaceManagers(t, open)[0]
			asset, store, _ := departStock(t, m, 5)
			_, err := m.PostMovement(context.Background(), models.Movement{AssetID: asset.ID, Kind: models.MovementKindDeparted, Quantity: 50, FromLocationID: store.ID, PerformedBy: "clerk"})
			if !errors.Is(err, ErrInsufficientAvailable) {
				t.Fatalf("departing 50 from a balance of 5 = %v, want ErrInsufficientAvailable", err)
			}
			if got := movementCount(t, m, asset.ID); got != 1 {
				t.Fatalf("%d movements on the ledger, want only the arrival", got)
			}
			balance, _ := m.GetAssetBalance(context.Background(), asset.ID, store.ID)
			if balance != 5 {
				t.Fatalf("the balance is %d, want 5", balance)
			}
		})
	}
}

func TestATransferAboveTheBalanceIsRefused(t *testing.T) {
	for name, open := range reverseRaceDatabases(t) {
		t.Run(name, func(t *testing.T) {
			m := reverseRaceManagers(t, open)[0]
			asset, store, shelf := departStock(t, m, 5)
			_, err := m.PostMovement(context.Background(), models.Movement{AssetID: asset.ID, Kind: models.MovementKindTransferred, Quantity: 6, FromLocationID: store.ID, ToLocationID: shelf.ID, PerformedBy: "clerk"})
			if !errors.Is(err, ErrInsufficientAvailable) {
				t.Fatalf("transferring 6 from a balance of 5 = %v, want ErrInsufficientAvailable", err)
			}
			if balance, _ := m.GetAssetBalance(context.Background(), asset.ID, shelf.ID); balance != 0 {
				t.Fatalf("the shelf holds %d, want 0", balance)
			}
		})
	}
}

func TestADepartureOfTheWholeBalanceAndATransferWithinItPass(t *testing.T) {
	for name, open := range reverseRaceDatabases(t) {
		t.Run(name, func(t *testing.T) {
			m := reverseRaceManagers(t, open)[0]
			ctx := context.Background()
			asset, store, shelf := departStock(t, m, 5)
			if _, err := m.PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindTransferred, Quantity: 2, FromLocationID: store.ID, ToLocationID: shelf.ID, PerformedBy: "clerk"}); err != nil {
				t.Fatalf("transferring 2 of 5: %v", err)
			}
			if _, err := m.PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindDeparted, Quantity: 3, FromLocationID: store.ID, PerformedBy: "clerk"}); err != nil {
				t.Fatalf("departing the remaining 3: %v", err)
			}
			if balance, _ := m.GetAssetBalance(ctx, asset.ID, store.ID); balance != 0 {
				t.Fatalf("the store holds %d, want 0", balance)
			}
			if _, err := m.PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindDeparted, Quantity: 1, FromLocationID: store.ID, PerformedBy: "clerk"}); !errors.Is(err, ErrInsufficientAvailable) {
				t.Fatalf("departing 1 from an empty store = %v, want ErrInsufficientAvailable", err)
			}
		})
	}
}

func TestAnArrivalAndAnAdjustmentAreNeverRefusedForBalance(t *testing.T) {
	for name, open := range reverseRaceDatabases(t) {
		t.Run(name, func(t *testing.T) {
			m := reverseRaceManagers(t, open)[0]
			ctx := context.Background()
			asset, store, _ := departStock(t, m, 5)
			if _, err := m.PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindAdjusted, Quantity: -9, ToLocationID: store.ID, PerformedBy: "clerk"}); err != nil {
				t.Fatalf("an adjustment below zero must stay allowed: %v", err)
			}
			if _, err := m.PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindArrived, Quantity: 4, ToLocationID: store.ID, PerformedBy: "clerk"}); err != nil {
				t.Fatalf("an arrival: %v", err)
			}
		})
	}
}

func TestADepartureCommittedFromAHoldStillPasses(t *testing.T) {
	for name, open := range reverseRaceDatabases(t) {
		t.Run(name, func(t *testing.T) {
			m := reverseRaceManagers(t, open)[0]
			ctx := context.Background()
			asset, store, _ := departStock(t, m, 5)
			hold, err := m.CreateHold(ctx, models.Hold{AssetID: asset.ID, LocationID: store.ID, Quantity: 5, PlacedBy: "clerk", ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339)})
			if err != nil {
				t.Fatal(err)
			}
			if _, _, err := m.CommitHold(ctx, hold.ID, 5, models.MovementKindDeparted, "", "clerk"); err != nil {
				t.Fatalf("committing a hold of the whole balance: %v", err)
			}
			if balance, _ := m.GetAssetBalance(ctx, asset.ID, store.ID); balance != 0 {
				t.Fatalf("the store holds %d, want 0", balance)
			}
		})
	}
}

func TestDeparturesSentAtOnceNeverTakeMoreThanTheBalance(t *testing.T) {
	for name, open := range reverseRaceDatabases(t) {
		t.Run(name, func(t *testing.T) {
			managers := reverseRaceManagers(t, open)
			ctx := context.Background()
			const sends, rounds = 8, 6
			for round := 0; round < rounds; round++ {
				asset, store, _ := departStock(t, managers[0], 5)
				results := make([]error, sends)
				start := make(chan struct{})
				var wg sync.WaitGroup
				for i := 0; i < sends; i++ {
					wg.Add(1)
					go func(i int) {
						defer wg.Done()
						<-start
						_, results[i] = managers[i%2].PostMovement(ctx, models.Movement{AssetID: asset.ID, Kind: models.MovementKindDeparted, Quantity: 3, FromLocationID: store.ID, PerformedBy: "clerk"})
					}(i)
				}
				close(start)
				wg.Wait()
				departed := 0
				for _, err := range results {
					switch {
					case err == nil:
						departed++
					case errors.Is(err, ErrInsufficientAvailable), errors.Is(err, ErrConflict):
					default:
						t.Fatalf("round %d: a departure failed with %v, want it posted, refused or in conflict", round, err)
					}
				}
				if departed > 1 {
					t.Fatalf("round %d: %d departures of 3 succeeded against a balance of 5, want at most 1", round, departed)
				}
				balance, err := managers[0].GetAssetBalance(ctx, asset.ID, store.ID)
				if err != nil {
					t.Fatal(err)
				}
				if balance < 0 {
					t.Fatalf("round %d: the balance is %d, want 0 or more", round, balance)
				}
			}
		})
	}
}

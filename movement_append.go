package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func drawsDownFromPlace(kind models.MovementKind) bool {
	return kind == models.MovementKindDeparted || kind == models.MovementKindTransferred
}

func (m *assetManager) appendMovement(ctx context.Context, mv models.Movement) error {
	if !drawsDownFromPlace(mv.Kind) {
		return m.store.Insert(ctx, roleMovement, mv)
	}
	err := m.store.InTransaction(ctx, func(tx *store) error {
		if err := lockAssetForHold(ctx, tx, mv.AssetID); err != nil {
			return err
		}
		inTx := &assetManager{db: m.db, store: tx}
		return inTx.insertWithinBalance(ctx, mv)
	})
	if err == nil || errors.Is(err, ErrInsufficientAvailable) {
		return err
	}
	return classifyConflict(err)
}

func (m *assetManager) insertWithinBalance(ctx context.Context, mv models.Movement) error {
	balance, err := m.GetAssetBalance(ctx, mv.AssetID, mv.FromLocationID)
	if err != nil {
		return err
	}
	if mv.Quantity > balance {
		return fmt.Errorf("%w: moving %d from a place that holds %d", ErrInsufficientAvailable, mv.Quantity, balance)
	}
	return m.store.Insert(ctx, roleMovement, mv)
}

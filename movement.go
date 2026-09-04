// movement.go — the append-only Movement ledger for [assetManager]:
// PostMovement, ReverseMovement, and GetAssetBalance's fold logic.
package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-assetmanager/gormstore"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

// PostMovement validates and appends a new Movement entry, then updates the
// affected Asset's denormalized LocationID to match. Kind must be one of
// [models.MovementKindArrived], [models.MovementKindTransferred],
// [models.MovementKindDeparted], or [models.MovementKindAdjusted] —
// [models.MovementKindReversed] entries are only ever created by
// [assetManager.ReverseMovement], never posted directly.
//
// Movement creation and the Asset.location_id patch are two separate calls,
// not one atomic transaction — same gap mwanachama-backend-taskmanager's
// WorkflowRun rollback (W6) documents and defers, unaffected by this
// storage swap. A crash between the two leaves the ledger correct (it's the
// only source of truth for balance) and the denormalized LocationID stale
// until the next movement corrects it.
func (m *assetManager) PostMovement(ctx context.Context, mv models.Movement) (models.Movement, error) {
	shape, err := validateMovementShape(mv)
	if err != nil {
		return models.Movement{}, err
	}
	current, err := m.GetAsset(ctx, mv.AssetID)
	if err != nil {
		return models.Movement{}, err
	}
	if current.TrackingMode == models.AssetTrackingModeSerialized && shape.absQuantity() != 1 {
		return models.Movement{}, fmt.Errorf("%w: a serialized asset's movement quantity must be exactly 1", ErrInvalidMovement)
	}
	if err := m.validateMovementLocations(ctx, mv); err != nil {
		return models.Movement{}, err
	}

	now := time.Now().UTC().Format(time.RFC3339)
	mv.ReversesMovementID = ""
	mv.CreatedAt = now
	if mv.OccurredAt == "" {
		mv.OccurredAt = now
	}

	row := gormstore.MovementToRow(mv)
	if err := m.db.WithContext(ctx).Table(m.tables.Movements).Create(&row).Error; err != nil {
		return models.Movement{}, fmt.Errorf("PostMovement: %w", err)
	}
	out := gormstore.MovementFromRow(row)

	if err := m.syncAssetLocation(ctx, out); err != nil {
		return out, fmt.Errorf("PostMovement: entry posted (id=%s) but denormalized location sync failed: %w", out.ID, err)
	}
	return out, nil
}

// movementShape is the outcome of validating a Movement's Kind/Quantity/
// location combination — carried so callers don't have to re-derive
// absQuantity from a possibly-negative (adjusted) Quantity.
type movementShape struct {
	quantity int64
}

func (s movementShape) absQuantity() int64 {
	if s.quantity < 0 {
		return -s.quantity
	}
	return s.quantity
}

// validateMovementShape checks that Kind/Quantity/FromLocationID/
// ToLocationID form one of the four postable shapes.
func validateMovementShape(mv models.Movement) (movementShape, error) {
	if mv.AssetID == "" {
		return movementShape{}, fmt.Errorf("%w: AssetID is required", ErrInvalidMovement)
	}
	if mv.PerformedBy == "" {
		return movementShape{}, fmt.Errorf("%w: PerformedBy is required — every ledger entry needs an accountable actor, matching merchandise-entry's issued_by not-null invariant", ErrInvalidMovement)
	}
	switch mv.Kind {
	case models.MovementKindArrived:
		if mv.Quantity <= 0 {
			return movementShape{}, fmt.Errorf("%w: arrived requires a positive Quantity", ErrInvalidMovement)
		}
		if mv.ToLocationID == "" || mv.FromLocationID != "" {
			return movementShape{}, fmt.Errorf("%w: arrived requires ToLocationID and no FromLocationID", ErrInvalidMovement)
		}
	case models.MovementKindTransferred:
		if mv.Quantity <= 0 {
			return movementShape{}, fmt.Errorf("%w: transferred requires a positive Quantity", ErrInvalidMovement)
		}
		if mv.FromLocationID == "" || mv.ToLocationID == "" {
			return movementShape{}, fmt.Errorf("%w: transferred requires both FromLocationID and ToLocationID", ErrInvalidMovement)
		}
		if mv.FromLocationID == mv.ToLocationID {
			return movementShape{}, fmt.Errorf("%w: transferred requires distinct FromLocationID and ToLocationID", ErrInvalidMovement)
		}
	case models.MovementKindDeparted:
		if mv.Quantity <= 0 {
			return movementShape{}, fmt.Errorf("%w: departed requires a positive Quantity", ErrInvalidMovement)
		}
		if mv.FromLocationID == "" || mv.ToLocationID != "" {
			return movementShape{}, fmt.Errorf("%w: departed requires FromLocationID and no ToLocationID", ErrInvalidMovement)
		}
	case models.MovementKindAdjusted:
		if mv.Quantity == 0 {
			return movementShape{}, fmt.Errorf("%w: adjusted requires a non-zero Quantity", ErrInvalidMovement)
		}
		if mv.ToLocationID == "" || mv.FromLocationID != "" {
			return movementShape{}, fmt.Errorf("%w: adjusted requires ToLocationID and no FromLocationID", ErrInvalidMovement)
		}
	case models.MovementKindReversed:
		return movementShape{}, fmt.Errorf("%w: reversed entries are only created via ReverseMovement", ErrInvalidMovement)
	default:
		return movementShape{}, fmt.Errorf("%w: unrecognized Kind %q", ErrInvalidMovement, mv.Kind)
	}
	return movementShape{quantity: mv.Quantity}, nil
}

// validateMovementLocations confirms every non-empty location field
// references a real Location.
func (m *assetManager) validateMovementLocations(ctx context.Context, mv models.Movement) error {
	for _, locID := range []string{mv.FromLocationID, mv.ToLocationID} {
		if locID == "" {
			continue
		}
		if _, err := m.GetLocation(ctx, locID); err != nil {
			if errors.Is(err, ErrLocationNotFound) {
				return fmt.Errorf("%w: location %q not found", ErrInvalidMovement, locID)
			}
			return fmt.Errorf("validateMovementLocations: %w", err)
		}
	}
	return nil
}

// syncAssetLocation patches Asset.location_id to reflect mv's destination —
// ToLocationID if the movement arrived somewhere, otherwise cleared (the
// asset left the system, or a reversal undid its last recorded position).
func (m *assetManager) syncAssetLocation(ctx context.Context, mv models.Movement) error {
	err := m.db.WithContext(ctx).Table(m.tables.Assets).Where("id = ?", mv.AssetID).
		Updates(map[string]any{
			"location_id": mv.ToLocationID,
			"updated_at":  time.Now().UTC().Format(time.RFC3339),
		}).Error
	if err != nil {
		return fmt.Errorf("syncAssetLocation: %w", err)
	}
	return nil
}

// ReverseMovement posts the mirror of an existing Movement — the same rule
// merchandise-entry.md's rpc_resolve_merchandise_dispute follows: "reverse
// posts the mirror, original untouched". The reversal's From/To locations
// are the original's swapped, which is what makes GetAssetBalance's fold
// cancel the pair out to zero net effect regardless of the original Kind's
// sign convention.
func (m *assetManager) ReverseMovement(ctx context.Context, movementID, performedBy, note string) (models.Movement, error) {
	if performedBy == "" {
		return models.Movement{}, fmt.Errorf("%w: performedBy is required — every ledger entry needs an accountable actor", ErrInvalidMovement)
	}
	original, err := m.GetMovement(ctx, movementID)
	if err != nil {
		return models.Movement{}, err
	}
	if original.Kind == models.MovementKindReversed {
		return models.Movement{}, fmt.Errorf("%w: cannot reverse a reversal", ErrInvalidMovement)
	}
	existing, err := m.ListMovements(ctx, MovementFilter{AssetID: original.AssetID})
	if err != nil {
		return models.Movement{}, fmt.Errorf("ReverseMovement: %w", err)
	}
	for _, e := range existing {
		if e.ReversesMovementID == original.ID {
			return models.Movement{}, ErrMovementAlreadyReversed
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	reversal := models.Movement{
		AssetID:            original.AssetID,
		Kind:               models.MovementKindReversed,
		Quantity:           original.Quantity,
		FromLocationID:     original.ToLocationID,
		ToLocationID:       original.FromLocationID,
		PerformedBy:        performedBy,
		ReversesMovementID: original.ID,
		Note:               note,
		OccurredAt:         now,
		CreatedAt:          now,
	}

	row := gormstore.MovementToRow(reversal)
	if err := m.db.WithContext(ctx).Table(m.tables.Movements).Create(&row).Error; err != nil {
		return models.Movement{}, fmt.Errorf("ReverseMovement: %w", err)
	}
	out := gormstore.MovementFromRow(row)

	if err := m.syncAssetLocation(ctx, out); err != nil {
		return out, fmt.Errorf("ReverseMovement: entry posted (id=%s) but denormalized location sync failed: %w", out.ID, err)
	}
	return out, nil
}

// GetMovement reads a single Movement row.
func (m *assetManager) GetMovement(ctx context.Context, movementID string) (models.Movement, error) {
	var row gormstore.MovementRow
	err := m.db.WithContext(ctx).Table(m.tables.Movements).Where("id = ?", movementID).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Movement{}, ErrMovementNotFound
		}
		return models.Movement{}, fmt.Errorf("GetMovement: %w", err)
	}
	return gormstore.MovementFromRow(row), nil
}

// ListMovements returns all Movement rows that match the filter, in no
// particular guaranteed order (callers folding a balance don't need one —
// sum is order-independent).
func (m *assetManager) ListMovements(ctx context.Context, filter MovementFilter) ([]models.Movement, error) {
	q := m.db.WithContext(ctx).Table(m.tables.Movements)
	if filter.AssetID != "" {
		q = q.Where("asset_id = ?", filter.AssetID)
	}
	if filter.Kind != "" {
		q = q.Where("kind = ?", string(filter.Kind))
	}

	var rows []gormstore.MovementRow
	if err := q.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("ListMovements: %w", err)
	}
	out := make([]models.Movement, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.MovementFromRow(r))
	}
	return out, nil
}

// GetAssetBalance folds every Movement for assetID into the net quantity
// currently at locationID: each movement contributes +Quantity where
// ToLocationID == locationID and -Quantity where FromLocationID ==
// locationID (both can apply to a single "adjusted" entry's ToLocationID
// only; a reversal is just another Movement in the fold, so it cancels its
// original by construction — see ReverseMovement's swapped From/To). Never
// a stored running total — always recomputed from the ledger.
func (m *assetManager) GetAssetBalance(ctx context.Context, assetID, locationID string) (int64, error) {
	movements, err := m.ListMovements(ctx, MovementFilter{AssetID: assetID})
	if err != nil {
		return 0, fmt.Errorf("GetAssetBalance: %w", err)
	}
	var balance int64
	for _, mv := range movements {
		if mv.ToLocationID == locationID {
			balance += mv.Quantity
		}
		if mv.FromLocationID == locationID {
			balance -= mv.Quantity
		}
	}
	return balance, nil
}

package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/gormutil"
	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func (m *assetManager) PostMovement(ctx context.Context, mv models.Movement) (models.Movement, error) {
	mv.ID = newID()
	if err := m.check(roleMovement, mv, ErrInvalidMovement); err != nil {
		return models.Movement{}, err
	}
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

	if err := m.appendMovement(ctx, mv); err != nil {
		return models.Movement{}, fmt.Errorf("PostMovement: %w", err)
	}
	out := mv

	if err := m.syncAssetLocation(ctx, out); err != nil {
		return out, fmt.Errorf("PostMovement: entry posted (id=%s) but denormalized location sync failed: %w", out.ID, err)
	}
	return out, nil
}

type movementShape struct {
	quantity int64
}

func (s movementShape) absQuantity() int64 {
	if s.quantity < 0 {
		return -s.quantity
	}
	return s.quantity
}

func validateMovementShape(mv models.Movement) (movementShape, error) {
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

func (m *assetManager) syncAssetLocation(ctx context.Context, mv models.Movement) error {
	err := m.store.Query(ctx, roleAsset).Where("id = ?", mv.AssetID).
		Updates(map[string]any{
			columnName("LocationID"): mv.ToLocationID,
			columnName("UpdatedAt"):  time.Now().UTC().Format(time.RFC3339),
		}).Error
	if err != nil {
		return fmt.Errorf("syncAssetLocation: %w", err)
	}
	return nil
}

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
	existing, err := m.everyMovement(ctx, MovementFilter{AssetID: original.AssetID})
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

	reversal.ID = newID()
	if err := m.store.Insert(ctx, roleMovement, reversal); err != nil {
		if gormutil.IsUniqueViolation(err) {
			return models.Movement{}, ErrMovementAlreadyReversed
		}
		return models.Movement{}, fmt.Errorf("ReverseMovement: %w", err)
	}
	out := reversal

	if err := m.syncAssetLocation(ctx, out); err != nil {
		return out, fmt.Errorf("ReverseMovement: entry posted (id=%s) but denormalized location sync failed: %w", out.ID, err)
	}
	return out, nil
}

func (m *assetManager) GetMovement(ctx context.Context, movementID string) (models.Movement, error) {
	var out models.Movement
	q := m.store.Query(ctx, roleMovement).Where("id = ?", movementID)
	if err := m.store.Take(q, roleMovement, &out, ErrMovementNotFound); err != nil {
		if errors.Is(err, ErrMovementNotFound) {
			return models.Movement{}, err
		}
		return models.Movement{}, fmt.Errorf("GetMovement: %w", err)
	}
	return out, nil
}

func (m *assetManager) movementQuery(ctx context.Context, filter MovementFilter) *gorm.DB {
	q := m.store.Query(ctx, roleMovement)
	if filter.AssetID != "" {
		q = q.Where("asset_id = ?", filter.AssetID)
	}
	if filter.Kind != "" {
		q = q.Where("kind = ?", string(filter.Kind))
	}
	return q.Order("id")
}

func (m *assetManager) ListMovements(ctx context.Context, filter MovementFilter) ([]models.Movement, error) {
	q, err := page(m.movementQuery(ctx, filter), filter.Limit, filter.Offset)
	if err != nil {
		return nil, fmt.Errorf("ListMovements: %w", err)
	}
	out, err := specstore.List[models.Movement](m.store, q, roleMovement)
	if err != nil {
		return nil, fmt.Errorf("ListMovements: %w", err)
	}
	return out, nil
}

func (m *assetManager) everyMovement(ctx context.Context, filter MovementFilter) ([]models.Movement, error) {
	out, err := specstore.List[models.Movement](m.store, m.movementQuery(ctx, filter), roleMovement)
	if err != nil {
		return nil, fmt.Errorf("everyMovement: %w", err)
	}
	return out, nil
}

func (m *assetManager) GetAssetBalance(ctx context.Context, assetID, locationID string) (int64, error) {
	if locationID == "" {
		return 0, fmt.Errorf("%w: a balance is only meaningful at a named location", ErrInvalidAsset)
	}
	movements, err := m.everyMovement(ctx, MovementFilter{AssetID: assetID})
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

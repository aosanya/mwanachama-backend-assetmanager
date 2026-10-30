package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func (m *assetManager) CreateHold(ctx context.Context, h models.Hold) (models.Hold, error) {
	h.ID = newID()
	if err := m.check(roleHold, h, ErrInvalidHold); err != nil {
		return models.Hold{}, err
	}
	if h.Quantity <= 0 {
		return models.Hold{}, fmt.Errorf("%w: Quantity must be positive", ErrInvalidHold)
	}
	expiresAt, err := time.Parse(time.RFC3339, h.ExpiresAt)
	if err != nil {
		return models.Hold{}, fmt.Errorf("%w: ExpiresAt must be RFC 3339: %v", ErrInvalidHold, err)
	}
	if !expiresAt.After(time.Now().UTC()) {
		return models.Hold{}, fmt.Errorf("%w: ExpiresAt must be in the future", ErrInvalidHold)
	}
	slot, err := holdSlotOf(h)
	if err != nil {
		return models.Hold{}, err
	}
	if _, err := m.GetAsset(ctx, h.AssetID); err != nil {
		return models.Hold{}, err
	}
	if _, err := m.GetLocation(ctx, h.LocationID); err != nil {
		return models.Hold{}, err
	}
	if slot != nil {
		if err := m.refuseOverlappingSlot(ctx, h.AssetID, *slot); err != nil {
			return models.Hold{}, err
		}
	}

	available, err := m.availableQuantity(ctx, h.AssetID, h.LocationID)
	if err != nil {
		return models.Hold{}, fmt.Errorf("CreateHold: %w", err)
	}
	if h.Quantity > available {
		return models.Hold{}, fmt.Errorf("%w: requested %d, available %d", ErrInsufficientAvailable, h.Quantity, available)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	h.Status = models.HoldStatusReserved
	h.CommittedQuantity = 0
	h.CreatedAt = now
	h.UpdatedAt = now

	if err := m.store.Insert(ctx, roleHold, h); err != nil {
		return models.Hold{}, fmt.Errorf("CreateHold: %w", err)
	}
	return h, nil
}

func (m *assetManager) availableQuantity(ctx context.Context, assetID, locationID string) (int64, error) {
	balance, err := m.GetAssetBalance(ctx, assetID, locationID)
	if err != nil {
		return 0, err
	}
	openHolds, err := m.ListHolds(ctx, HoldFilter{AssetID: assetID, LocationID: locationID, Status: models.HoldStatusReserved})
	if err != nil {
		return 0, err
	}
	reserved := int64(0)
	for _, h := range openHolds {
		reserved += h.Quantity
	}
	return balance - reserved, nil
}

func (m *assetManager) GetHold(ctx context.Context, holdID string) (models.Hold, error) {
	var out models.Hold
	q := m.store.Query(ctx, roleHold).Where("id = ?", holdID)
	if err := m.store.Take(q, roleHold, &out, ErrHoldNotFound); err != nil {
		if errors.Is(err, ErrHoldNotFound) {
			return models.Hold{}, err
		}
		return models.Hold{}, fmt.Errorf("GetHold: %w", err)
	}
	return out, nil
}

func (m *assetManager) CommitHold(ctx context.Context, holdID string, quantity int64, kind models.MovementKind, toLocationID, performedBy string) (models.Hold, models.Movement, error) {
	hold, err := m.GetHold(ctx, holdID)
	if err != nil {
		return models.Hold{}, models.Movement{}, err
	}
	if hold.Status != models.HoldStatusReserved {
		return models.Hold{}, models.Movement{}, ErrInvalidHoldStatusTransition
	}
	if quantity <= 0 || quantity > hold.Quantity {
		return models.Hold{}, models.Movement{}, fmt.Errorf("%w: quantity must be in (0, %d]", ErrInvalidHold, hold.Quantity)
	}
	if kind != models.MovementKindDeparted && kind != models.MovementKindTransferred {
		return models.Hold{}, models.Movement{}, fmt.Errorf("%w: CommitHold requires kind %q or %q, got %q",
			ErrInvalidMovement, models.MovementKindDeparted, models.MovementKindTransferred, kind)
	}

	committed := hold
	committed.CommittedQuantity = quantity
	committed.Status = models.HoldStatusCommitted
	if quantity != hold.Quantity {
		committed.Status = models.HoldStatusPartiallyCommitted
	}
	committed.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := m.claimReservedHold(ctx, committed); err != nil {
		return models.Hold{}, models.Movement{}, fmt.Errorf("CommitHold: %w", err)
	}

	movement, err := m.PostMovement(ctx, models.Movement{
		AssetID:        hold.AssetID,
		Kind:           kind,
		Quantity:       quantity,
		FromLocationID: hold.LocationID,
		ToLocationID:   toLocationID,
		PerformedBy:    performedBy,
	})
	if err != nil && movement.ID == "" {
		if undone := m.unclaimHold(ctx, committed); undone != nil {
			return models.Hold{}, models.Movement{}, errors.Join(fmt.Errorf("CommitHold: %w", err), undone)
		}
		return models.Hold{}, models.Movement{}, fmt.Errorf("CommitHold: %w", err)
	}
	if err != nil {
		return models.Hold{}, movement, fmt.Errorf("CommitHold: %w", err)
	}
	return committed, movement, nil
}

func (m *assetManager) ReleaseHold(ctx context.Context, holdID string) (models.Hold, error) {
	hold, err := m.GetHold(ctx, holdID)
	if err != nil {
		return models.Hold{}, err
	}
	if hold.Status != models.HoldStatusReserved {
		return models.Hold{}, ErrInvalidHoldStatusTransition
	}
	hold.Status = models.HoldStatusReleased
	hold.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := m.claimReservedHold(ctx, hold); err != nil {
		return models.Hold{}, fmt.Errorf("ReleaseHold: %w", err)
	}
	return hold, nil
}

func (m *assetManager) claimReservedHold(ctx context.Context, next models.Hold) error {
	written := m.store.Query(ctx, roleHold).
		Where("id = ? AND status = ?", next.ID, string(models.HoldStatusReserved)).
		Updates(map[string]any{
			columnName("Status"):            string(next.Status),
			columnName("CommittedQuantity"): next.CommittedQuantity,
			columnName("UpdatedAt"):         next.UpdatedAt,
		})
	if written.Error != nil {
		return classifyConflict(written.Error)
	}
	if written.RowsAffected == 0 {
		return ErrInvalidHoldStatusTransition
	}
	return nil
}

func (m *assetManager) unclaimHold(ctx context.Context, claimed models.Hold) error {
	written := m.store.Query(ctx, roleHold).
		Where("id = ? AND status = ?", claimed.ID, string(claimed.Status)).
		Updates(map[string]any{
			columnName("Status"):            string(models.HoldStatusReserved),
			columnName("CommittedQuantity"): int64(0),
			columnName("UpdatedAt"):         time.Now().UTC().Format(time.RFC3339),
		})
	if written.Error != nil {
		return fmt.Errorf("CommitHold: hold %s stays %s after its movement was refused: %w", claimed.ID, claimed.Status, written.Error)
	}
	return nil
}

func (m *assetManager) ListHolds(ctx context.Context, filter HoldFilter) ([]models.Hold, error) {
	q := m.store.Query(ctx, roleHold)
	if filter.AssetID != "" {
		q = q.Where("asset_id = ?", filter.AssetID)
	}
	if filter.LocationID != "" {
		q = q.Where("location_id = ?", filter.LocationID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", string(filter.Status))
	}

	out, err := specstore.List[models.Hold](m.store, q.Order("id"), roleHold)
	if err != nil {
		return nil, fmt.Errorf("ListHolds: %w", err)
	}
	return out, nil
}

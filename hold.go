// hold.go — Hold CRUD + the commit/release lifecycle for [assetManager].
//
// A Hold is single-shot: [assetManager.CommitHold] is called at most once
// per hold, committing up to its full reserved Quantity in that one call —
// whatever isn't committed is released in the same call, matching
// [models.HoldStatus.CanTransitionTo]'s transition table (every transition
// out of reserved is terminal; there is no partially_committed →
// committed path for a second, later commit).
package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

// CreateHold reserves h.Quantity of h.AssetID at h.LocationID (required —
// availability is only meaningful against a specific location) until
// h.ExpiresAt, provided that much is actually available: the asset's
// balance at that location minus what every other still-open
// ([models.HoldStatusReserved]) hold there has already reserved.
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
	if _, err := m.GetAsset(ctx, h.AssetID); err != nil {
		return models.Hold{}, err
	}
	if _, err := m.GetLocation(ctx, h.LocationID); err != nil {
		return models.Hold{}, err
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

// availableQuantity is the asset's balance at locationID minus the
// remaining Quantity of every other open (status = reserved) Hold there.
// A partially_committed or released Hold no longer reserves anything —
// it's terminal, its remainder was already given back.
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

// GetHold reads a single Hold row.
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

// CommitHold fulfills a reserved Hold — up to quantity of it — by posting a
// Movement of the given kind (must be [models.MovementKindDeparted] or
// [models.MovementKindTransferred]; committing a hold always removes stock
// from where it was held) from the hold's LocationID, to toLocationID when
// kind is transferred. quantity may be less than the hold's full reserved
// amount; whatever's left is released in this same call — see the package
// doc for why there is no later, second commit on the same hold.
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

	mv := models.Movement{
		AssetID:        hold.AssetID,
		Kind:           kind,
		Quantity:       quantity,
		FromLocationID: hold.LocationID,
		ToLocationID:   toLocationID,
		PerformedBy:    performedBy,
	}
	movement, err := m.PostMovement(ctx, mv)
	if err != nil {
		return models.Hold{}, models.Movement{}, fmt.Errorf("CommitHold: %w", err)
	}

	hold.CommittedQuantity = quantity
	if quantity == hold.Quantity {
		hold.Status = models.HoldStatusCommitted
	} else {
		hold.Status = models.HoldStatusPartiallyCommitted
	}
	hold.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	err = m.store.Query(ctx, roleHold).Where("id = ?", hold.ID).
		Updates(map[string]any{
			columnName("Status"):            string(hold.Status),
			columnName("CommittedQuantity"): hold.CommittedQuantity,
			columnName("UpdatedAt"):         hold.UpdatedAt,
		}).Error
	if err != nil {
		return models.Hold{}, movement, fmt.Errorf("CommitHold: movement posted (id=%s) but hold update failed: %w", movement.ID, err)
	}
	return hold, movement, nil
}

// ReleaseHold releases a reserved Hold with nothing committed — manual
// release, or the outcome the watchdog applies once ExpiresAt has passed
// (see hold_watchdog.go).
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

	err = m.store.Query(ctx, roleHold).Where("id = ?", hold.ID).
		Updates(map[string]any{
			columnName("Status"):    string(hold.Status),
			columnName("UpdatedAt"): hold.UpdatedAt,
		}).Error
	if err != nil {
		return models.Hold{}, fmt.Errorf("ReleaseHold: %w", err)
	}
	return hold, nil
}

// ListHolds returns all Hold rows that match the filter, id order.
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

// hold.go — Hold CRUD + the commit/release lifecycle for [assetManager].
//
// A Hold is single-shot: [assetManager.CommitHold] is called at most once
// per hold, committing up to its full reserved Quantity in that one call —
// whatever isn't committed is released in the same call, matching
// [HoldStatus.CanTransitionTo]'s transition table (every transition out of
// reserved is terminal; there is no partially_committed → committed path
// for a second, later commit).
package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
)

// CreateHold reserves h.Quantity of h.AssetID at h.LocationID (required —
// availability is only meaningful against a specific location) until
// h.ExpiresAt, provided that much is actually available: the asset's
// balance at that location minus what every other still-open
// ([HoldStatusReserved]) hold there has already reserved.
func (m *assetManager) CreateHold(ctx context.Context, agencyID string, h Hold) (Hold, error) {
	if h.AssetID == "" {
		return Hold{}, fmt.Errorf("%w: AssetID is required", ErrInvalidHold)
	}
	if h.LocationID == "" {
		return Hold{}, fmt.Errorf("%w: LocationID is required", ErrInvalidHold)
	}
	if h.Quantity <= 0 {
		return Hold{}, fmt.Errorf("%w: Quantity must be positive", ErrInvalidHold)
	}
	if h.PlacedBy == "" {
		return Hold{}, fmt.Errorf("%w: PlacedBy is required — every hold needs an accountable actor", ErrInvalidHold)
	}
	expiresAt, err := time.Parse(time.RFC3339, h.ExpiresAt)
	if err != nil {
		return Hold{}, fmt.Errorf("%w: ExpiresAt must be RFC 3339: %v", ErrInvalidHold, err)
	}
	if !expiresAt.After(time.Now().UTC()) {
		return Hold{}, fmt.Errorf("%w: ExpiresAt must be in the future", ErrInvalidHold)
	}
	if _, err := m.GetAsset(ctx, agencyID, h.AssetID); err != nil {
		return Hold{}, err
	}
	if _, err := m.GetLocation(ctx, agencyID, h.LocationID); err != nil {
		return Hold{}, err
	}

	available, err := m.availableQuantity(ctx, agencyID, h.AssetID, h.LocationID)
	if err != nil {
		return Hold{}, fmt.Errorf("CreateHold: %w", err)
	}
	if h.Quantity > available {
		return Hold{}, fmt.Errorf("%w: requested %d, available %d", ErrInsufficientAvailable, h.Quantity, available)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	h.AgencyID = agencyID
	h.Status = HoldStatusReserved
	h.CommittedQuantity = 0
	h.CreatedAt = now
	h.UpdatedAt = now

	created, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		AgencyID:   agencyID,
		TypeID:     holdTypeID,
		Properties: holdToProperties(h),
	})
	if err != nil {
		return Hold{}, fmt.Errorf("CreateHold: %w", err)
	}
	return holdFromEntity(created), nil
}

// availableQuantity is the asset's balance at locationID minus the
// remaining Quantity of every other open (status = reserved) Hold there.
// A partially_committed or released Hold no longer reserves anything —
// it's terminal, its remainder was already given back.
func (m *assetManager) availableQuantity(ctx context.Context, agencyID, assetID, locationID string) (int64, error) {
	balance, err := m.GetAssetBalance(ctx, agencyID, assetID, locationID)
	if err != nil {
		return 0, err
	}
	openHolds, err := m.ListHolds(ctx, agencyID, HoldFilter{AssetID: assetID, LocationID: locationID, Status: HoldStatusReserved})
	if err != nil {
		return 0, err
	}
	reserved := int64(0)
	for _, h := range openHolds {
		reserved += h.Quantity
	}
	return balance - reserved, nil
}

// GetHold reads a single Hold entity from the agency graph.
func (m *assetManager) GetHold(ctx context.Context, agencyID, holdID string) (Hold, error) {
	e, err := m.dm.GetEntity(ctx, agencyID, holdID)
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return Hold{}, ErrHoldNotFound
		}
		return Hold{}, fmt.Errorf("GetHold: %w", err)
	}
	if e.AgencyID != agencyID || e.TypeID != holdTypeID {
		return Hold{}, ErrHoldNotFound
	}
	return holdFromEntity(e), nil
}

// CommitHold fulfills a reserved Hold — up to quantity of it — by posting a
// Movement of the given kind (must be [MovementKindDeparted] or
// [MovementKindTransferred]; committing a hold always removes stock from
// where it was held) from the hold's LocationID, to toLocationID when kind
// is transferred. quantity may be less than the hold's full reserved
// amount; whatever's left is released in this same call — see the package
// doc for why there is no later, second commit on the same hold.
func (m *assetManager) CommitHold(ctx context.Context, agencyID, holdID string, quantity int64, kind MovementKind, toLocationID, performedBy string) (Hold, Movement, error) {
	hold, err := m.GetHold(ctx, agencyID, holdID)
	if err != nil {
		return Hold{}, Movement{}, err
	}
	if hold.Status != HoldStatusReserved {
		return Hold{}, Movement{}, ErrInvalidHoldStatusTransition
	}
	if quantity <= 0 || quantity > hold.Quantity {
		return Hold{}, Movement{}, fmt.Errorf("%w: quantity must be in (0, %d]", ErrInvalidHold, hold.Quantity)
	}
	if kind != MovementKindDeparted && kind != MovementKindTransferred {
		return Hold{}, Movement{}, fmt.Errorf("%w: CommitHold requires kind %q or %q, got %q",
			ErrInvalidMovement, MovementKindDeparted, MovementKindTransferred, kind)
	}

	mv := Movement{
		AssetID:        hold.AssetID,
		Kind:           kind,
		Quantity:       quantity,
		FromLocationID: hold.LocationID,
		ToLocationID:   toLocationID,
		PerformedBy:    performedBy,
	}
	movement, err := m.PostMovement(ctx, agencyID, mv)
	if err != nil {
		return Hold{}, Movement{}, fmt.Errorf("CommitHold: %w", err)
	}

	hold.CommittedQuantity = quantity
	if quantity == hold.Quantity {
		hold.Status = HoldStatusCommitted
	} else {
		hold.Status = HoldStatusPartiallyCommitted
	}
	hold.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	updated, err := m.dm.UpdateEntity(ctx, agencyID, hold.ID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"status":             string(hold.Status),
			"committed_quantity": hold.CommittedQuantity,
			"updated_at":         hold.UpdatedAt,
		},
	})
	if err != nil {
		return Hold{}, movement, fmt.Errorf("CommitHold: movement posted (id=%s) but hold update failed: %w", movement.ID, err)
	}
	return holdFromEntity(updated), movement, nil
}

// ReleaseHold releases a reserved Hold with nothing committed — manual
// release, or the outcome the watchdog applies once ExpiresAt has passed
// (see hold_watchdog.go).
func (m *assetManager) ReleaseHold(ctx context.Context, agencyID, holdID string) (Hold, error) {
	hold, err := m.GetHold(ctx, agencyID, holdID)
	if err != nil {
		return Hold{}, err
	}
	if hold.Status != HoldStatusReserved {
		return Hold{}, ErrInvalidHoldStatusTransition
	}
	hold.Status = HoldStatusReleased
	hold.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	updated, err := m.dm.UpdateEntity(ctx, agencyID, hold.ID, entitygraph.UpdateEntityRequest{
		Properties: map[string]any{
			"status":     string(hold.Status),
			"updated_at": hold.UpdatedAt,
		},
	})
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return Hold{}, ErrHoldNotFound
		}
		return Hold{}, fmt.Errorf("ReleaseHold: %w", err)
	}
	return holdFromEntity(updated), nil
}

// ListHolds returns all non-deleted Hold entities for the agency that match
// the filter.
func (m *assetManager) ListHolds(ctx context.Context, agencyID string, filter HoldFilter) ([]Hold, error) {
	props := map[string]any{}
	if filter.AssetID != "" {
		props["asset_id"] = filter.AssetID
	}
	if filter.LocationID != "" {
		props["location_id"] = filter.LocationID
	}
	if filter.Status != "" {
		props["status"] = string(filter.Status)
	}

	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
		TypeID:     holdTypeID,
		Properties: props,
	})
	if err != nil {
		return nil, fmt.Errorf("ListHolds: %w", err)
	}
	out := make([]Hold, 0, len(entities))
	for _, e := range entities {
		out = append(out, holdFromEntity(e))
	}
	return out, nil
}

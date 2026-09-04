// asset.go — Asset CRUD for [assetManager].
package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
)

// CreateAsset creates a new Asset entity in the graph. It does not
// establish any balance — an Asset row with no Movement against it holds
// nothing anywhere; balance always comes from folding the Movement ledger
// (see movement.go's GetAssetBalance). LocationID, if set, is an informational
// starting point only.
func (m *assetManager) CreateAsset(ctx context.Context, a Asset) (Asset, error) {
	if a.Name == "" {
		return Asset{}, fmt.Errorf("%w: Asset.Name is required", ErrInvalidAsset)
	}
	switch a.TrackingMode {
	case AssetTrackingModeSerialized:
		if a.SerialTag == "" {
			return Asset{}, fmt.Errorf("%w: SerialTag is required for a serialized asset", ErrInvalidAsset)
		}
		existing, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
			TypeID:     assetTypeID,
			Properties: map[string]any{"serial_tag": a.SerialTag},
		})
		if err != nil {
			return Asset{}, fmt.Errorf("CreateAsset: %w", err)
		}
		if len(existing) > 0 {
			return Asset{}, ErrAssetSerialTagExists
		}
	case AssetTrackingModeFungible:
		// No serial tag expected; nothing further to validate here.
	default:
		return Asset{}, fmt.Errorf("%w: TrackingMode must be %q or %q, got %q",
			ErrInvalidAsset, AssetTrackingModeSerialized, AssetTrackingModeFungible, a.TrackingMode)
	}
	if a.LocationID != "" {
		if _, err := m.GetLocation(ctx, a.LocationID); err != nil {
			if errors.Is(err, ErrLocationNotFound) {
				return Asset{}, fmt.Errorf("%w: location %q not found", ErrInvalidAsset, a.LocationID)
			}
			return Asset{}, fmt.Errorf("CreateAsset: %w", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	a.CreatedAt = now
	a.UpdatedAt = now

	created, err := m.dm.CreateEntity(ctx, entitygraph.CreateEntityRequest{
		TypeID:     assetTypeID,
		Properties: assetToProperties(a),
	})
	if err != nil {
		return Asset{}, fmt.Errorf("CreateAsset: %w", err)
	}
	return assetFromEntity(created), nil
}

// GetAsset reads a single Asset entity from the graph.
func (m *assetManager) GetAsset(ctx context.Context, assetID string) (Asset, error) {
	e, err := m.dm.GetEntity(ctx, assetID)
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return Asset{}, ErrAssetNotFound
		}
		return Asset{}, fmt.Errorf("GetAsset: %w", err)
	}
	if e.TypeID != assetTypeID {
		return Asset{}, ErrAssetNotFound
	}
	return assetFromEntity(e), nil
}

// UpdateAsset patches Name/Category/AttributesJSON/SerialTag. TrackingMode
// and LocationID are not caller-writable here: changing TrackingMode after
// creation would silently invalidate every prior Movement's balance
// semantics ([ErrAssetTrackingModeImmutable]), and LocationID is a
// derived field only PostMovement/ReverseMovement may update.
func (m *assetManager) UpdateAsset(ctx context.Context, a Asset) (Asset, error) {
	current, err := m.GetAsset(ctx, a.ID)
	if err != nil {
		return Asset{}, err
	}
	if a.Name == "" {
		return Asset{}, fmt.Errorf("%w: Asset.Name is required", ErrInvalidAsset)
	}
	if a.TrackingMode != "" && a.TrackingMode != current.TrackingMode {
		return Asset{}, ErrAssetTrackingModeImmutable
	}

	a.TrackingMode = current.TrackingMode
	a.LocationID = current.LocationID
	a.CreatedAt = current.CreatedAt
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	updated, err := m.dm.UpdateEntity(ctx, a.ID, entitygraph.UpdateEntityRequest{
		Properties: assetToProperties(a),
	})
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return Asset{}, ErrAssetNotFound
		}
		return Asset{}, fmt.Errorf("UpdateAsset: %w", err)
	}
	return assetFromEntity(updated), nil
}

// DeleteAsset soft-deletes the Asset entity. Refused with
// [ErrAssetHasOpenHolds] if the asset has any Hold still in
// [HoldStatusReserved] — release or commit those first.
func (m *assetManager) DeleteAsset(ctx context.Context, assetID string) error {
	if _, err := m.GetAsset(ctx, assetID); err != nil {
		return err
	}
	openHolds, err := m.ListHolds(ctx, HoldFilter{AssetID: assetID, Status: HoldStatusReserved})
	if err != nil {
		return fmt.Errorf("DeleteAsset: %w", err)
	}
	if len(openHolds) > 0 {
		return ErrAssetHasOpenHolds
	}
	if err := m.dm.DeleteEntity(ctx, assetID); err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return ErrAssetNotFound
		}
		return fmt.Errorf("DeleteAsset: %w", err)
	}
	return nil
}

// ListAssets returns all non-deleted Asset entities that match the filter.
func (m *assetManager) ListAssets(ctx context.Context, filter AssetFilter) ([]Asset, error) {
	props := map[string]any{}
	if filter.Category != "" {
		props["category"] = filter.Category
	}
	if filter.TrackingMode != "" {
		props["tracking_mode"] = string(filter.TrackingMode)
	}
	if filter.LocationID != "" {
		props["location_id"] = filter.LocationID
	}

	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		TypeID:     assetTypeID,
		Properties: props,
	})
	if err != nil {
		return nil, fmt.Errorf("ListAssets: %w", err)
	}
	out := make([]Asset, 0, len(entities))
	for _, e := range entities {
		out = append(out, assetFromEntity(e))
	}
	return out, nil
}

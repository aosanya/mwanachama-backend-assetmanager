// asset.go — Asset CRUD for [assetManager].
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

// CreateAsset creates a new Asset row. It does not establish any balance —
// an Asset row with no Movement against it holds nothing anywhere; balance
// always comes from folding the Movement ledger (see movement.go's
// GetAssetBalance). LocationID, if set, is an informational starting point
// only.
//
// SerialTag uniqueness is checked here as a pre-check (friendly error
// before any write); gormstore.Migrate's partial unique index is the
// database-level guard against two concurrent creates both passing this
// check.
func (m *assetManager) CreateAsset(ctx context.Context, a models.Asset) (models.Asset, error) {
	if a.Name == "" {
		return models.Asset{}, fmt.Errorf("%w: Asset.Name is required", ErrInvalidAsset)
	}
	switch a.TrackingMode {
	case models.AssetTrackingModeSerialized:
		if a.SerialTag == "" {
			return models.Asset{}, fmt.Errorf("%w: SerialTag is required for a serialized asset", ErrInvalidAsset)
		}
		var count int64
		err := m.db.WithContext(ctx).Table(m.tables.Assets).
			Where("serial_tag = ? AND deleted = ?", a.SerialTag, false).Count(&count).Error
		if err != nil {
			return models.Asset{}, fmt.Errorf("CreateAsset: %w", err)
		}
		if count > 0 {
			return models.Asset{}, ErrAssetSerialTagExists
		}
	case models.AssetTrackingModeFungible:
		// No serial tag expected; nothing further to validate here.
	default:
		return models.Asset{}, fmt.Errorf("%w: TrackingMode must be %q or %q, got %q",
			ErrInvalidAsset, models.AssetTrackingModeSerialized, models.AssetTrackingModeFungible, a.TrackingMode)
	}
	if a.LocationID != "" {
		if _, err := m.GetLocation(ctx, a.LocationID); err != nil {
			if errors.Is(err, ErrLocationNotFound) {
				return models.Asset{}, fmt.Errorf("%w: location %q not found", ErrInvalidAsset, a.LocationID)
			}
			return models.Asset{}, fmt.Errorf("CreateAsset: %w", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	a.CreatedAt = now
	a.UpdatedAt = now

	row := gormstore.AssetToRow(a)
	if err := m.db.WithContext(ctx).Table(m.tables.Assets).Create(&row).Error; err != nil {
		return models.Asset{}, fmt.Errorf("CreateAsset: %w", err)
	}
	return gormstore.AssetFromRow(row), nil
}

// GetAsset reads a single non-deleted Asset row.
func (m *assetManager) GetAsset(ctx context.Context, assetID string) (models.Asset, error) {
	var row gormstore.AssetRow
	err := m.db.WithContext(ctx).Table(m.tables.Assets).
		Where("id = ? AND deleted = ?", assetID, false).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Asset{}, ErrAssetNotFound
		}
		return models.Asset{}, fmt.Errorf("GetAsset: %w", err)
	}
	return gormstore.AssetFromRow(row), nil
}

// UpdateAsset patches Name/Category/AttributesJSON/SerialTag. TrackingMode
// and LocationID are not caller-writable here: changing TrackingMode after
// creation would silently invalidate every prior Movement's balance
// semantics ([ErrAssetTrackingModeImmutable]), and LocationID is a
// derived field only PostMovement/ReverseMovement may update.
func (m *assetManager) UpdateAsset(ctx context.Context, a models.Asset) (models.Asset, error) {
	current, err := m.GetAsset(ctx, a.ID)
	if err != nil {
		return models.Asset{}, err
	}
	if a.Name == "" {
		return models.Asset{}, fmt.Errorf("%w: Asset.Name is required", ErrInvalidAsset)
	}
	if a.TrackingMode != "" && a.TrackingMode != current.TrackingMode {
		return models.Asset{}, ErrAssetTrackingModeImmutable
	}

	now := time.Now().UTC().Format(time.RFC3339)
	err = m.db.WithContext(ctx).Table(m.tables.Assets).Where("id = ?", a.ID).
		Updates(map[string]any{
			"name":            a.Name,
			"category":        a.Category,
			"attributes_json": a.AttributesJSON,
			"serial_tag":      a.SerialTag,
			"updated_at":      now,
		}).Error
	if err != nil {
		return models.Asset{}, fmt.Errorf("UpdateAsset: %w", err)
	}
	current.Name = a.Name
	current.Category = a.Category
	current.AttributesJSON = a.AttributesJSON
	current.SerialTag = a.SerialTag
	current.UpdatedAt = now
	return current, nil
}

// DeleteAsset soft-deletes the Asset row. Refused with
// [ErrAssetHasOpenHolds] if the asset has any Hold still in
// [models.HoldStatusReserved] — release or commit those first.
func (m *assetManager) DeleteAsset(ctx context.Context, assetID string) error {
	if _, err := m.GetAsset(ctx, assetID); err != nil {
		return err
	}
	openHolds, err := m.ListHolds(ctx, HoldFilter{AssetID: assetID, Status: models.HoldStatusReserved})
	if err != nil {
		return fmt.Errorf("DeleteAsset: %w", err)
	}
	if len(openHolds) > 0 {
		return ErrAssetHasOpenHolds
	}

	now := time.Now().UTC().Format(time.RFC3339)
	err = m.db.WithContext(ctx).Table(m.tables.Assets).Where("id = ?", assetID).
		Updates(map[string]any{"deleted": true, "updated_at": now}).Error
	if err != nil {
		return fmt.Errorf("DeleteAsset: %w", err)
	}
	return nil
}

// ListAssets returns all non-deleted Asset rows that match the filter, id
// order.
func (m *assetManager) ListAssets(ctx context.Context, filter AssetFilter) ([]models.Asset, error) {
	q := m.db.WithContext(ctx).Table(m.tables.Assets).Where("deleted = ?", false)
	if filter.Category != "" {
		q = q.Where("category = ?", filter.Category)
	}
	if filter.TrackingMode != "" {
		q = q.Where("tracking_mode = ?", string(filter.TrackingMode))
	}
	if filter.LocationID != "" {
		q = q.Where("location_id = ?", filter.LocationID)
	}

	var rows []gormstore.AssetRow
	if err := q.Order("id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("ListAssets: %w", err)
	}
	out := make([]models.Asset, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.AssetFromRow(r))
	}
	return out, nil
}

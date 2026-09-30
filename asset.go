package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func (m *assetManager) CreateAsset(ctx context.Context, a models.Asset) (models.Asset, error) {
	a.ID = newID()
	if a.Name == "" {
		return models.Asset{}, fmt.Errorf("%w: Asset.Name is required", ErrInvalidAsset)
	}
	switch a.TrackingMode {
	case models.AssetTrackingModeSerialized:
		if a.SerialTag == "" {
			return models.Asset{}, fmt.Errorf("%w: SerialTag is required for a serialized asset", ErrInvalidAsset)
		}
		var count int64
		err := m.store.Query(ctx, roleAsset).
			Where("serial_tag = ? AND deleted = ?", a.SerialTag, false).Count(&count).Error
		if err != nil {
			return models.Asset{}, fmt.Errorf("CreateAsset: %w", err)
		}
		if count > 0 {
			return models.Asset{}, ErrAssetSerialTagExists
		}
	case models.AssetTrackingModeFungible:
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

	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		code, err := m.nextCode(ctx, tx, roleAsset, "A")
		if err != nil {
			return err
		}
		a.Code = code
		row, err := encode(m.store.Object(roleAsset), a)
		if err != nil {
			return err
		}
		return tx.WithContext(ctx).Table(m.store.Table(roleAsset)).Create(row).Error
	})
	if err != nil {
		return models.Asset{}, fmt.Errorf("CreateAsset: %w", err)
	}
	return a, nil
}

func (m *assetManager) GetAsset(ctx context.Context, assetID string) (models.Asset, error) {
	var out models.Asset
	q := m.store.Query(ctx, roleAsset).Where("id = ? AND deleted = ?", assetID, false)
	if err := m.store.Take(q, roleAsset, &out, ErrAssetNotFound); err != nil {
		if errors.Is(err, ErrAssetNotFound) {
			return models.Asset{}, err
		}
		return models.Asset{}, fmt.Errorf("GetAsset: %w", err)
	}
	return out, nil
}

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
	err = m.store.Query(ctx, roleAsset).Where("id = ?", a.ID).
		Updates(map[string]any{
			columnName("Name"):           a.Name,
			columnName("Category"):       a.Category,
			columnName("AttributesJSON"): a.AttributesJSON,
			columnName("SerialTag"):      a.SerialTag,
			columnName("UpdatedAt"):      now,
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
	err = m.store.Query(ctx, roleAsset).Where("id = ?", assetID).
		Updates(map[string]any{
			columnName("Deleted"):   true,
			columnName("UpdatedAt"): now,
		}).Error
	if err != nil {
		return fmt.Errorf("DeleteAsset: %w", err)
	}
	return nil
}

func (m *assetManager) ListAssets(ctx context.Context, filter AssetFilter) ([]models.Asset, error) {
	q := m.store.Query(ctx, roleAsset).Where("deleted = ?", false)
	if filter.Category != "" {
		q = q.Where("category = ?", filter.Category)
	}
	if filter.TrackingMode != "" {
		q = q.Where("tracking_mode = ?", string(filter.TrackingMode))
	}
	if filter.LocationID != "" {
		q = q.Where("location_id = ?", filter.LocationID)
	}

	out, err := specstore.List[models.Asset](m.store, q.Order("id"), roleAsset)
	if err != nil {
		return nil, fmt.Errorf("ListAssets: %w", err)
	}
	return out, nil
}

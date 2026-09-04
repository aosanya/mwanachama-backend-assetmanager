package gormstore

import (
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AssetRow is the GORM row for a [models.Asset]. SerialTag carries a
// partial unique index (see Migrate) scoped to non-deleted, non-empty
// values — the database-level guarantee the old entitygraph version only
// had as a pre-check query (see asset.go's CreateAsset).
type AssetRow struct {
	ID             string `gorm:"primaryKey"`
	Name           string
	TrackingMode   string
	SerialTag      string
	Category       string
	AttributesJSON string
	LocationID     string
	CreatedAt      string
	UpdatedAt      string
	Deleted        bool
}

func (r *AssetRow) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}

// AssetToRow converts a domain Asset to its row shape.
func AssetToRow(a models.Asset) AssetRow {
	return AssetRow{
		ID:             a.ID,
		Name:           a.Name,
		TrackingMode:   string(a.TrackingMode),
		SerialTag:      a.SerialTag,
		Category:       a.Category,
		AttributesJSON: a.AttributesJSON,
		LocationID:     a.LocationID,
		CreatedAt:      a.CreatedAt,
		UpdatedAt:      a.UpdatedAt,
		Deleted:        a.Deleted,
	}
}

// AssetFromRow converts a row back to the domain Asset.
func AssetFromRow(r AssetRow) models.Asset {
	return models.Asset{
		ID:             r.ID,
		Name:           r.Name,
		TrackingMode:   models.AssetTrackingMode(r.TrackingMode),
		SerialTag:      r.SerialTag,
		Category:       r.Category,
		AttributesJSON: r.AttributesJSON,
		LocationID:     r.LocationID,
		CreatedAt:      r.CreatedAt,
		UpdatedAt:      r.UpdatedAt,
		Deleted:        r.Deleted,
	}
}

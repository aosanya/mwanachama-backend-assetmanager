package gormstore

import (
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// HoldRow is the GORM row for a [models.Hold]. No Deleted column — there is
// no DeleteHold operation; a hold's terminal states (committed, released,
// partially_committed) are reached via status transitions instead.
type HoldRow struct {
	ID                string `gorm:"primaryKey"`
	AssetID           string `gorm:"index"`
	LocationID        string `gorm:"index"`
	Status            string
	Quantity          int64
	CommittedQuantity int64
	ExpiresAt         string
	PlacedBy          string
	CreatedAt         string
	UpdatedAt         string
}

func (r *HoldRow) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}

// HoldToRow converts a domain Hold to its row shape.
func HoldToRow(h models.Hold) HoldRow {
	return HoldRow{
		ID:                h.ID,
		AssetID:           h.AssetID,
		LocationID:        h.LocationID,
		Status:            string(h.Status),
		Quantity:          h.Quantity,
		CommittedQuantity: h.CommittedQuantity,
		ExpiresAt:         h.ExpiresAt,
		PlacedBy:          h.PlacedBy,
		CreatedAt:         h.CreatedAt,
		UpdatedAt:         h.UpdatedAt,
	}
}

// HoldFromRow converts a row back to the domain Hold.
func HoldFromRow(r HoldRow) models.Hold {
	return models.Hold{
		ID:                r.ID,
		AssetID:           r.AssetID,
		LocationID:        r.LocationID,
		Status:            models.HoldStatus(r.Status),
		Quantity:          r.Quantity,
		CommittedQuantity: r.CommittedQuantity,
		ExpiresAt:         r.ExpiresAt,
		PlacedBy:          r.PlacedBy,
		CreatedAt:         r.CreatedAt,
		UpdatedAt:         r.UpdatedAt,
	}
}

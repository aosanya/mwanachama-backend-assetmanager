package gormstore

import (
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MovementRow is the GORM row for a [models.Movement]. No UpdatedAt/Deleted
// columns — a Movement is append-only, never updated or deleted (see
// models.Movement's doc).
type MovementRow struct {
	ID                 string `gorm:"primaryKey"`
	AssetID            string `gorm:"index"`
	Kind               string
	Quantity           int64
	FromLocationID     string
	ToLocationID       string
	PerformedBy        string
	ReversesMovementID string
	Note               string
	OccurredAt         string
	CreatedAt          string
}

func (r *MovementRow) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}

// MovementToRow converts a domain Movement to its row shape.
func MovementToRow(mv models.Movement) MovementRow {
	return MovementRow{
		ID:                 mv.ID,
		AssetID:            mv.AssetID,
		Kind:               string(mv.Kind),
		Quantity:           mv.Quantity,
		FromLocationID:     mv.FromLocationID,
		ToLocationID:       mv.ToLocationID,
		PerformedBy:        mv.PerformedBy,
		ReversesMovementID: mv.ReversesMovementID,
		Note:               mv.Note,
		OccurredAt:         mv.OccurredAt,
		CreatedAt:          mv.CreatedAt,
	}
}

// MovementFromRow converts a row back to the domain Movement.
func MovementFromRow(r MovementRow) models.Movement {
	return models.Movement{
		ID:                 r.ID,
		AssetID:            r.AssetID,
		Kind:               models.MovementKind(r.Kind),
		Quantity:           r.Quantity,
		FromLocationID:     r.FromLocationID,
		ToLocationID:       r.ToLocationID,
		PerformedBy:        r.PerformedBy,
		ReversesMovementID: r.ReversesMovementID,
		Note:               r.Note,
		OccurredAt:         r.OccurredAt,
		CreatedAt:          r.CreatedAt,
	}
}

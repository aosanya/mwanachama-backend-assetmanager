package gormstore

import (
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// LocationRow is the GORM row for a [models.Location].
//
// ParentLocationID is a nullable *string (NULL for a root location), not
// the domain Location's plain "" — LocationToRow/LocationFromRow map ""
// <-> nil at the boundary. It replaces both the old entitygraph child_of
// edge and the denormalized parent_location_id property that duplicated it
// — a real relational table needs only the one column.
type LocationRow struct {
	ID               string `gorm:"primaryKey"`
	Code             string `gorm:"uniqueIndex"`
	Name             string
	Kind             string
	ParentLocationID *string `gorm:"index"`
	CreatedAt        string
	UpdatedAt        string
	Deleted          bool
}

func (r *LocationRow) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}

// LocationToRow converts a domain Location to its row shape.
func LocationToRow(l models.Location) LocationRow {
	return LocationRow{
		ID:               l.ID,
		Code:             l.Code,
		Name:             l.Name,
		Kind:             l.Kind,
		ParentLocationID: StringToNullable(l.ParentLocationID),
		CreatedAt:        l.CreatedAt,
		UpdatedAt:        l.UpdatedAt,
		Deleted:          l.Deleted,
	}
}

// LocationFromRow converts a row back to the domain Location.
func LocationFromRow(r LocationRow) models.Location {
	return models.Location{
		ID:               r.ID,
		Code:             r.Code,
		Name:             r.Name,
		Kind:             r.Kind,
		ParentLocationID: nullableToString(r.ParentLocationID),
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
		Deleted:          r.Deleted,
	}
}

// StringToNullable maps the domain Location's "" (root) to a nil *string,
// the nullable ParentLocationID column's spelling of "no parent".
func StringToNullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func nullableToString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

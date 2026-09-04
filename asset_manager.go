package mwanachamaassetmanager

import (
	"context"
	"fmt"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

// LocationFilter scopes a [AssetManager.ListLocations] query. Zero-value
// fields are ignored (no filtering applied for that field).
type LocationFilter struct {
	// ParentLocationID restricts results to direct children of this
	// location. Ignored when RootOnly is true.
	ParentLocationID string

	// RootOnly restricts results to locations with no parent.
	RootOnly bool
}

// AssetFilter scopes a [AssetManager.ListAssets] query. Zero-value fields
// are ignored (no filtering applied for that field).
type AssetFilter struct {
	// Category restricts results to this exact category.
	Category string

	// TrackingMode restricts results to this tracking mode.
	TrackingMode models.AssetTrackingMode

	// LocationID restricts results to assets currently denormalized to
	// this location.
	LocationID string
}

// MovementFilter scopes a [AssetManager.ListMovements] query. Zero-value
// fields are ignored (no filtering applied for that field).
type MovementFilter struct {
	// AssetID restricts results to movements affecting this asset.
	AssetID string

	// Kind restricts results to this movement kind.
	Kind models.MovementKind
}

// HoldFilter scopes a [AssetManager.ListHolds] query. Zero-value fields are
// ignored (no filtering applied for that field).
type HoldFilter struct {
	// AssetID restricts results to holds against this asset.
	AssetID string

	// LocationID restricts results to holds at this location.
	LocationID string

	// Status restricts results to this hold status.
	Status models.HoldStatus
}

// AssetManager is the primary interface for asset/inventory lifecycle
// management. This is a single-tenant package — one deployment serves one
// owner, so no method takes a tenant-scoping argument, mirroring
// mwanachama-backend-actor.UserManager's shape.
//
// Implementations must be safe for concurrent use.
type AssetManager interface {
	// Location

	CreateLocation(ctx context.Context, l models.Location) (models.Location, error)
	GetLocation(ctx context.Context, locationID string) (models.Location, error)
	UpdateLocation(ctx context.Context, l models.Location) (models.Location, error)
	DeleteLocation(ctx context.Context, locationID string) error
	ListLocations(ctx context.Context, filter LocationFilter) ([]models.Location, error)
	ListDescendantLocations(ctx context.Context, locationID string) ([]models.Location, error)

	// Asset

	CreateAsset(ctx context.Context, a models.Asset) (models.Asset, error)
	GetAsset(ctx context.Context, assetID string) (models.Asset, error)
	UpdateAsset(ctx context.Context, a models.Asset) (models.Asset, error)
	DeleteAsset(ctx context.Context, assetID string) error
	ListAssets(ctx context.Context, filter AssetFilter) ([]models.Asset, error)

	// Movement

	PostMovement(ctx context.Context, mv models.Movement) (models.Movement, error)
	ReverseMovement(ctx context.Context, movementID, performedBy, note string) (models.Movement, error)
	GetMovement(ctx context.Context, movementID string) (models.Movement, error)
	ListMovements(ctx context.Context, filter MovementFilter) ([]models.Movement, error)
	GetAssetBalance(ctx context.Context, assetID, locationID string) (int64, error)

	// Hold

	CreateHold(ctx context.Context, h models.Hold) (models.Hold, error)
	GetHold(ctx context.Context, holdID string) (models.Hold, error)
	CommitHold(ctx context.Context, holdID string, quantity int64, kind models.MovementKind, toLocationID, performedBy string) (models.Hold, models.Movement, error)
	ReleaseHold(ctx context.Context, holdID string) (models.Hold, error)
	ListHolds(ctx context.Context, filter HoldFilter) ([]models.Hold, error)
	ListHoldsExpiredAsOf(ctx context.Context, cutoffRFC3339 string) ([]models.Hold, error)
}

// assetManager is the GORM-backed implementation of [AssetManager].
type assetManager struct {
	db     *gorm.DB
	tables TableNames
}

// NewAssetManager constructs an [AssetManager] backed by db, reading and
// writing the four tables named by t (see [DefaultTableNames]). Callers
// must run [Migrate] against the same db and t before use. Returns an error
// if db is nil.
func NewAssetManager(db *gorm.DB, t TableNames) (AssetManager, error) {
	if db == nil {
		return nil, fmt.Errorf("NewAssetManager: db must not be nil")
	}
	return &assetManager{db: db, tables: t}, nil
}

// Package mwanachamaassetmanager provides general-purpose asset/inventory
// tracking — warehouses, supermarkets, household inventories, livestock. It
// exposes [AssetManager] — the single interface for managing Locations,
// Assets, the append-only Movement ledger, and Hold reservations.
//
// Storage is delegated to a
// [github.com/aosanya/mwanachama-backend-shared/entitygraph.DataManager], the
// same contract mwanachama-backend-taskmanager is built on. Construct a
// Postgres-backed DataManager and pass it to [NewAssetManager].
//
// Implementation is split across focused files:
//   - models.go     — domain types
//   - schema.go      — the entitygraph schema (DefaultAssetSchema)
//   - converters.go — entity↔domain converters
//   - location.go    — Location CRUD + child_of tree queries
//   - asset.go        — Asset CRUD
//   - movement.go     — PostMovement, ReverseMovement, GetAssetBalance
//   - hold.go          — Hold CRUD + CommitHold/ReleaseHold lifecycle
//   - hold_watchdog.go — ListHoldsExpiredAsOf, the watchdog query helper
//
// Modelled on mwanachama-backend-taskmanager's task.go.
package mwanachamaassetmanager

import (
	"context"
	"fmt"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
)

// locationTypeID is the TypeDefinition.Name used for Location entities.
const locationTypeID = "Location"

// assetTypeID is the TypeDefinition.Name used for Asset entities.
const assetTypeID = "Asset"

// movementTypeID is the TypeDefinition.Name used for Movement entities.
const movementTypeID = "Movement"

// holdTypeID is the TypeDefinition.Name used for Hold entities.
const holdTypeID = "Hold"

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
	TrackingMode AssetTrackingMode

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
	Kind MovementKind
}

// HoldFilter scopes a [AssetManager.ListHolds] query. Zero-value fields are
// ignored (no filtering applied for that field).
type HoldFilter struct {
	// AssetID restricts results to holds against this asset.
	AssetID string

	// LocationID restricts results to holds at this location.
	LocationID string

	// Status restricts results to this hold status.
	Status HoldStatus
}

// AssetManager is the primary interface for asset/inventory lifecycle
// management. Every method takes agencyID explicitly — this backend is
// multi-tenant, and entitygraph.DataManager already scopes every entity by
// agency, so "owner" (a household, a store, a warehouse operator) maps
// directly onto that existing concept.
type AssetManager interface {
	// Location

	CreateLocation(ctx context.Context, agencyID string, l Location) (Location, error)
	GetLocation(ctx context.Context, agencyID, locationID string) (Location, error)
	UpdateLocation(ctx context.Context, agencyID string, l Location) (Location, error)
	DeleteLocation(ctx context.Context, agencyID, locationID string) error
	ListLocations(ctx context.Context, agencyID string, filter LocationFilter) ([]Location, error)
	ListDescendantLocations(ctx context.Context, agencyID, locationID string) ([]Location, error)

	// Asset

	CreateAsset(ctx context.Context, agencyID string, a Asset) (Asset, error)
	GetAsset(ctx context.Context, agencyID, assetID string) (Asset, error)
	UpdateAsset(ctx context.Context, agencyID string, a Asset) (Asset, error)
	DeleteAsset(ctx context.Context, agencyID, assetID string) error
	ListAssets(ctx context.Context, agencyID string, filter AssetFilter) ([]Asset, error)

	// Movement

	PostMovement(ctx context.Context, agencyID string, mv Movement) (Movement, error)
	ReverseMovement(ctx context.Context, agencyID, movementID, performedBy, note string) (Movement, error)
	GetMovement(ctx context.Context, agencyID, movementID string) (Movement, error)
	ListMovements(ctx context.Context, agencyID string, filter MovementFilter) ([]Movement, error)
	GetAssetBalance(ctx context.Context, agencyID, assetID, locationID string) (int64, error)

	// Hold

	CreateHold(ctx context.Context, agencyID string, h Hold) (Hold, error)
	GetHold(ctx context.Context, agencyID, holdID string) (Hold, error)
	CommitHold(ctx context.Context, agencyID, holdID string, quantity int64, kind MovementKind, toLocationID, performedBy string) (Hold, Movement, error)
	ReleaseHold(ctx context.Context, agencyID, holdID string) (Hold, error)
	ListHolds(ctx context.Context, agencyID string, filter HoldFilter) ([]Hold, error)
	ListHoldsExpiredAsOf(ctx context.Context, agencyID string, cutoffRFC3339 string) ([]Hold, error)
}

// assetManager is the entitygraph-backed implementation of [AssetManager].
type assetManager struct {
	dm entitygraph.DataManager
}

// NewAssetManager constructs an [AssetManager] backed by the given
// [entitygraph.DataManager]. Returns an error if dm is nil.
func NewAssetManager(dm entitygraph.DataManager) (AssetManager, error) {
	if dm == nil {
		return nil, fmt.Errorf("NewAssetManager: data manager must not be nil")
	}
	return &assetManager{dm: dm}, nil
}

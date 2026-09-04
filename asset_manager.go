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
// management.
type AssetManager interface {
	// Location

	CreateLocation(ctx context.Context, l Location) (Location, error)
	GetLocation(ctx context.Context, locationID string) (Location, error)
	UpdateLocation(ctx context.Context, l Location) (Location, error)
	DeleteLocation(ctx context.Context, locationID string) error
	ListLocations(ctx context.Context, filter LocationFilter) ([]Location, error)
	ListDescendantLocations(ctx context.Context, locationID string) ([]Location, error)

	// Asset

	CreateAsset(ctx context.Context, a Asset) (Asset, error)
	GetAsset(ctx context.Context, assetID string) (Asset, error)
	UpdateAsset(ctx context.Context, a Asset) (Asset, error)
	DeleteAsset(ctx context.Context, assetID string) error
	ListAssets(ctx context.Context, filter AssetFilter) ([]Asset, error)

	// Movement

	PostMovement(ctx context.Context, mv Movement) (Movement, error)
	ReverseMovement(ctx context.Context, movementID, performedBy, note string) (Movement, error)
	GetMovement(ctx context.Context, movementID string) (Movement, error)
	ListMovements(ctx context.Context, filter MovementFilter) ([]Movement, error)
	GetAssetBalance(ctx context.Context, assetID, locationID string) (int64, error)

	// Hold

	CreateHold(ctx context.Context, h Hold) (Hold, error)
	GetHold(ctx context.Context, holdID string) (Hold, error)
	CommitHold(ctx context.Context, holdID string, quantity int64, kind MovementKind, toLocationID, performedBy string) (Hold, Movement, error)
	ReleaseHold(ctx context.Context, holdID string) (Hold, error)
	ListHolds(ctx context.Context, filter HoldFilter) ([]Hold, error)
	ListHoldsExpiredAsOf(ctx context.Context, cutoffRFC3339 string) ([]Hold, error)
}

// dataManager is entitygraph.DataManager plus the relationship methods this
// package needs — CreateRelationship/DeleteRelationship/ListRelationships
// are no longer part of the shared interface (see its doc comment), since
// each consumer knows its own fixed set of relationship labels.
type dataManager interface {
	entitygraph.DataManager
	CreateRelationship(ctx context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error)
	DeleteRelationship(ctx context.Context, relationshipID string) error
	ListRelationships(ctx context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error)
}

// assetManager is the entitygraph-backed implementation of [AssetManager].
type assetManager struct {
	dm dataManager
}

// NewAssetManager constructs an [AssetManager] backed by the given
// [entitygraph.DataManager]. Returns an error if dm is nil.
func NewAssetManager(dm dataManager) (AssetManager, error) {
	if dm == nil {
		return nil, fmt.Errorf("NewAssetManager: data manager must not be nil")
	}
	return &assetManager{dm: dm}, nil
}

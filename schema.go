// Package mwanachamaassetmanager — pre-delivered schema definition.
//
// This file exposes [DefaultAssetSchema], which returns the fixed
// [schema.Schema] for mwanachama-backend-assetmanager. Wiring code in
// whatever imports this package seeds this schema per agency at startup via
// SchemaManager.SetSchema (set s.AgencyID before calling; DefaultAssetSchema
// itself returns an agency-agnostic template) — same pattern as
// mwanachama-backend-taskmanager's DefaultWorkSchema.
//
// The schema declares four TypeDefinitions:
//   - Asset    — a trackable thing, serialized (one physical unit) or
//     fungible (a type + quantity), distinguished by tracking_mode (mutable)
//   - Location — a place an Asset can be, nested via child_of (mutable)
//   - Movement — an append-only ledger entry recording a quantity/unit
//     change; current balance is always the fold of Movement entries, never
//     a stored running total (immutable once posted)
//   - Hold     — a reservation against an Asset, with its own lifecycle
//     (reserved → committed / released / partially_committed) (mutable)
//
// Graph topology — Location is the only type with a real graph edge, since
// it is the only relation ever walked multi-hop (the recursive-descendant
// closure architecture.md calls for):
//
//	Location ──child_of──► Location   (child points at its parent; at most one)
//
// Asset's current location, and Movement/Hold's asset and location
// references, are plain denormalized string properties (location_id,
// asset_id, from_location_id, to_location_id) instead of edges — the same
// choice mwanachama-backend-taskmanager made for Task.WorkflowRunID
// ("denormalises the started_task edge onto the Task row so queries can
// filter by run-id without traversing the graph"). None of these are ever
// walked multi-hop, so a property filter on ListEntities is strictly
// simpler than an edge plus a traversal. This is a deliberate simplification
// from the original scaffold's edge-per-reference draft — see
// documentation/3. implementation/todo_done.md, A2.
//
// Deliberately NOT a typed edge: who performed a Movement or placed a
// Hold. performed_by/placed_by are required plain string properties (a
// caller-supplied external actor id), not references to a modelled Actor
// vertex — this package owns no auth model of its own (decision #10, see
// documentation/1. requirements/requirements.md: imported as a package,
// no HTTP boundary where auth would live). What they are NOT is optional:
// every ledger entry and every hold needs an accountable actor, matching
// merchandise-entry's issued_by not-null invariant.
//
// Storage: every entity lives in mwanachama-backend-shared's single Postgres
// `entities` table, keyed by TypeID; TypeDefinition.StorageCollection below
// is a label only, no functional effect. All edges live in the single
// `relationships` table.
//
// Modelled on mwanachama-backend-taskmanager's schema.go. Version: 1 / Tag:
// "v1" — this is schema history starting fresh.
package mwanachamaassetmanager

import "github.com/aosanya/mwanachama-backend-shared/schema"

// RelLabelChildOf is the single forward graph edge label this schema
// declares — a Location points at its parent via child_of (at most one; a
// root location has none). Named to match mwanachama-backend-taskmanager's
// subtask_of convention: the edge reads "X is child_of Y", source is the
// child.
const RelLabelChildOf = "child_of"

// RelLabelHasChildLocation is child_of's declared inverse — entitygraph's
// ValidateSchema requires both directions of a relationship to be real
// RelationshipDefinitions (see mwanachama-backend-taskmanager/schema.go's
// blocked_by/has_subtask/depended_on_by, the same self-referencing-type
// shape). Not used directly by this package's code — descendant queries
// walk child_of inbound instead (see location.go's
// ListDescendantLocations) — but it must exist for the schema to validate.
const RelLabelHasChildLocation = "has_child_location"

// DefaultAssetSchema returns the pre-delivered [schema.Schema] for
// mwanachama-backend-assetmanager. The operation is idempotent — calling it
// multiple times with the same schema ID is safe.
func DefaultAssetSchema() schema.Schema {
	return schema.Schema{
		ID:      "asset-schema-v1",
		Version: 1,
		Tag:     "v1",
		Types: []schema.TypeDefinition{
			{
				Name:              "Location",
				DisplayName:       "Location",
				StorageCollection: "asset_locations",
				Properties: []schema.PropertyDefinition{
					// name is the short human-readable label (e.g. "Bin 12", "Pantry shelf").
					{Name: "name", Type: schema.PropertyTypeString, Required: true},
					// kind is open-ended and descriptive only — "warehouse", "aisle",
					// "bin", "room", "shelf", "pasture"... nothing in the schema
					// constrains what kind may nest under what.
					{Name: "kind", Type: schema.PropertyTypeString},
					// parent_location_id denormalises the child_of edge so a Location's
					// immediate parent can be read without a graph traversal; the
					// child_of edge below remains the source of truth for recursive
					// descendant/ancestor queries.
					{Name: "parent_location_id", Type: schema.PropertyTypeString},
					{Name: "created_at", Type: schema.PropertyTypeString},
					{Name: "updated_at", Type: schema.PropertyTypeString},
				},
				Relationships: []schema.RelationshipDefinition{
					{
						Name:    RelLabelChildOf,
						Label:   "Child of",
						ToType:  "Location",
						ToMany:  false,
						Inverse: RelLabelHasChildLocation,
						Properties: []schema.PropertyDefinition{
							{Name: "created_at", Type: schema.PropertyTypeString},
						},
					},
					{
						Name:    RelLabelHasChildLocation,
						Label:   "Child locations",
						ToType:  "Location",
						ToMany:  true,
						Inverse: RelLabelChildOf,
					},
				},
			},
			{
				Name:              "Asset",
				DisplayName:       "Asset",
				StorageCollection: "asset_assets",
				Properties: []schema.PropertyDefinition{
					// name is the short human-readable label (e.g. "T-shirt, size M", "Bessie").
					{Name: "name", Type: schema.PropertyTypeString, Required: true},
					// tracking_mode selects how this Asset's on-hand amount is
					// interpreted. See [AssetTrackingMode].
					// Well-known values: "serialized", "fungible".
					{Name: "tracking_mode", Type: schema.PropertyTypeOption, Required: true, Options: []string{"serialized", "fungible"}},
					// serial_tag is the unique-per-agency tag/serial for a serialized
					// Asset (e.g. an ear tag, an equipment serial number). Empty for
					// fungible Assets.
					{Name: "serial_tag", Type: schema.PropertyTypeString},
					// category is a free-form grouping label (e.g. "livestock", "grocery",
					// "electronics"). Not schema-enforced — see requirements.md's
					// flexible-attribute open question for whether/how this becomes
					// a closed set later.
					{Name: "category", Type: schema.PropertyTypeString},
					// attributes_json is a JSON-encoded object of domain-specific
					// attributes (breed, warranty_expiry, barcode, ...). See
					// requirements.md — entitygraph's schema.PropertyType has no
					// object/map type, so this is a v1 stand-in, not a final answer.
					{Name: "attributes_json", Type: schema.PropertyTypeString},
					// location_id denormalises this Asset's current location — the
					// most recent Movement's ToLocationID (or FromLocationID, cleared,
					// if the asset departed the system). See the package doc's note
					// on why this is a property, not an edge, and models.go's
					// LocationID field doc for the single-location v1 caveat.
					{Name: "location_id", Type: schema.PropertyTypeString},
					{Name: "created_at", Type: schema.PropertyTypeString},
					{Name: "updated_at", Type: schema.PropertyTypeString},
				},
			},
			{
				Name:              "Movement",
				DisplayName:       "Movement",
				StorageCollection: "asset_movements",
				Immutable:         true,
				Properties: []schema.PropertyDefinition{
					// kind classifies the entry. See [MovementKind]. Provisional list,
					// expected to grow the way merchandise-entry's did.
					// Well-known values: "arrived", "transferred", "departed",
					// "adjusted", "reversed".
					{Name: "kind", Type: schema.PropertyTypeOption, Required: true, Options: []string{"arrived", "transferred", "departed", "adjusted", "reversed"}},
					// asset_id is the Asset this movement affects — see the package
					// doc's note on why this is a property, not an edge.
					{Name: "asset_id", Type: schema.PropertyTypeString, Required: true},
					// quantity is the amount this entry applies. A positive magnitude
					// for every kind except "adjusted", where it may be negative (a
					// downward variance correction) — see models.go's MovementKind
					// doc for the full sign convention.
					{Name: "quantity", Type: schema.PropertyTypeInteger, Required: true},
					// from_location_id is where this movement originated. Empty for
					// "arrived".
					{Name: "from_location_id", Type: schema.PropertyTypeString},
					// to_location_id is where this movement ends up. Empty for
					// "departed".
					{Name: "to_location_id", Type: schema.PropertyTypeString},
					// performed_by is a denormalized external actor id — see the
					// package doc's note on why this is not yet a typed edge. Required:
					// every ledger entry needs an accountable actor, matching
					// merchandise-entry's issued_by not-null invariant — see
					// movement.go's validateMovementShape.
					{Name: "performed_by", Type: schema.PropertyTypeString, Required: true},
					// reverses_movement_id points at the Movement this entry mirrors,
					// for kind = "reversed". Empty otherwise.
					{Name: "reverses_movement_id", Type: schema.PropertyTypeString},
					// note is a free-form reason (e.g. a stock-count variance
					// explanation).
					{Name: "note", Type: schema.PropertyTypeString},
					// occurred_at is when the movement actually happened (RFC 3339),
					// which may differ from created_at for an offline-queued entry
					// posted on reconnect.
					{Name: "occurred_at", Type: schema.PropertyTypeString},
					{Name: "created_at", Type: schema.PropertyTypeString},
				},
			},
			{
				Name:              "Hold",
				DisplayName:       "Hold",
				StorageCollection: "asset_holds",
				Properties: []schema.PropertyDefinition{
					// status is the current lifecycle state — see [HoldStatus].
					// Well-known values: "reserved", "committed", "released",
					// "partially_committed".
					{Name: "status", Type: schema.PropertyTypeOption, Required: true, Options: []string{"reserved", "committed", "released", "partially_committed"}},
					// asset_id is the Asset this hold reserves against — see the
					// package doc's note on why this is a property, not an edge.
					{Name: "asset_id", Type: schema.PropertyTypeString, Required: true},
					// location_id is the Location this hold reserves stock at.
					{Name: "location_id", Type: schema.PropertyTypeString},
					// quantity is the amount originally reserved.
					{Name: "quantity", Type: schema.PropertyTypeInteger, Required: true},
					// committed_quantity is how much of quantity has actually been
					// committed so far (posted as a Movement). Always <= quantity.
					{Name: "committed_quantity", Type: schema.PropertyTypeInteger},
					// expires_at is when an uncommitted Hold is released automatically
					// (RFC 3339). Checked by a watchdog process — see
					// documentation/2. design/architecture.md's Hold section.
					{Name: "expires_at", Type: schema.PropertyTypeString, Required: true},
					// placed_by is a denormalized external actor id — see the package
					// doc's note on why this is not yet a typed edge. Required: every
					// hold needs an accountable actor — see hold.go's CreateHold.
					{Name: "placed_by", Type: schema.PropertyTypeString, Required: true},
					{Name: "created_at", Type: schema.PropertyTypeString},
					{Name: "updated_at", Type: schema.PropertyTypeString},
				},
			},
		},
	}
}

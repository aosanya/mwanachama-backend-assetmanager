// Package mwanachamaassetmanager — domain entity types.
//
// This file mirrors the TypeDefinitions declared in DefaultAssetSchema:
//   - Location — a place, nested via child_of (the one real graph edge)
//   - Asset    — a trackable thing, serialized or fungible
//   - Movement — an append-only ledger entry
//   - Hold     — a reservation against an Asset, with its own lifecycle
//
// All domain structs use string timestamps (ISO 8601 / RFC 3339) to match
// the entitygraph property storage convention used across
// mwanachama-backend-shared.
//
// Modelled on mwanachama-backend-taskmanager's models.go.
package mwanachamaassetmanager

// AssetTrackingMode selects how an [Asset]'s on-hand amount is interpreted.
type AssetTrackingMode string

const (
	// AssetTrackingModeSerialized means one Asset row is one physical unit
	// (a specific cow, a specific laptop) — its balance at any Location is
	// always 0 or 1.
	AssetTrackingModeSerialized AssetTrackingMode = "serialized"

	// AssetTrackingModeFungible means one Asset row is a type
	// (e.g. "50kg rice bag") — its on-hand quantity at a Location is
	// derived by folding [Movement] entries, never stored directly.
	AssetTrackingModeFungible AssetTrackingMode = "fungible"
)

// Location is a place an [Asset] can be — a warehouse bin, a household
// room, a pasture. Nested via ParentLocationID; unbounded depth.
type Location struct {
	// ID is the unique identifier for this location. Set by the backend on
	// creation; callers should leave it empty in create requests.
	ID string `json:"id"`

	// Name is the short human-readable label (e.g. "Bin 12", "Pantry shelf").
	Name string `json:"name"`

	// Kind is open-ended and descriptive only — "warehouse", "aisle",
	// "bin", "room", "shelf", "pasture"... Nothing constrains what kind
	// may nest under what.
	Kind string `json:"kind,omitempty"`

	// ParentLocationID is the parent Location's ID, denormalized from the
	// child_of edge. Empty for a root location.
	ParentLocationID string `json:"parent_location_id,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when this location was created.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the RFC 3339 timestamp of the last update.
	UpdatedAt string `json:"updated_at"`
}

// Asset is a trackable thing — either one physical unit (serialized) or a
// type tracked as a quantity (fungible). See [AssetTrackingMode].
type Asset struct {
	// ID is the unique identifier for this asset. Set by the backend on
	// creation; callers should leave it empty in create requests.
	ID string `json:"id"`

	// Name is the short human-readable label (e.g. "T-shirt, size M", "Bessie").
	Name string `json:"name"`

	// TrackingMode selects how this asset's on-hand amount is interpreted.
	TrackingMode AssetTrackingMode `json:"tracking_mode"`

	// SerialTag is the unique tag/serial for a serialized asset (an ear
	// tag, an equipment serial number). Empty for fungible assets.
	SerialTag string `json:"serial_tag,omitempty"`

	// Category is a free-form grouping label (e.g. "livestock", "grocery",
	// "electronics"). Not schema-enforced.
	Category string `json:"category,omitempty"`

	// AttributesJSON is a JSON-encoded object of domain-specific attributes
	// (breed, warranty_expiry, barcode, ...). A v1 stand-in for a proper
	// flexible-attribute mechanism — see requirements.md's open question.
	AttributesJSON string `json:"attributes_json,omitempty"`

	// LocationID is where this asset currently is — a convenience
	// denormalization, updated by the most recent Movement affecting this
	// asset. For a fungible asset this is one of potentially several
	// locations holding stock of this type — see Movement's
	// fold-to-balance model in architecture.md; a single LocationID field
	// on Asset is a v1 simplification for the common single-location case
	// and is expected to be revisited once multi-location fungible stock
	// is actually built. The real balance at any given location always
	// comes from folding Movement entries, never from this field.
	LocationID string `json:"location_id,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when this asset was created.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the RFC 3339 timestamp of the last update.
	UpdatedAt string `json:"updated_at"`
}

// MovementKind classifies a [Movement] entry. Provisional — expected to
// grow the way merchandise-entry's kind vocabulary did.
type MovementKind string

const (
	// MovementKindArrived is stock/a unit entering the system from outside
	// (no FromLocationID).
	MovementKindArrived MovementKind = "arrived"

	// MovementKindTransferred is a move from one Location to another.
	MovementKindTransferred MovementKind = "transferred"

	// MovementKindDeparted is stock/a unit leaving the system (no
	// ToLocationID) — sold, consumed, given away.
	MovementKindDeparted MovementKind = "departed"

	// MovementKindAdjusted is a counted correction (e.g. a stock-count
	// variance) with no corresponding physical move — the one kind whose
	// Quantity may be negative (a downward correction), applied at
	// ToLocationID with FromLocationID empty.
	MovementKindAdjusted MovementKind = "adjusted"

	// MovementKindReversed is the mirror of a prior entry — see
	// ReversesMovementID. The original entry is never edited or deleted.
	MovementKindReversed MovementKind = "reversed"
)

// Movement is one append-only ledger entry. Current balance at any
// (Asset, Location) pair is the fold of every committed Movement affecting
// it — never a stored running total. A correction is a new Movement with
// Kind [MovementKindReversed], not an edit to a prior one.
type Movement struct {
	// ID is the unique identifier for this movement. Set by the backend on
	// creation; callers should leave it empty in create requests.
	ID string `json:"id"`

	// AssetID is the Asset this movement affects.
	AssetID string `json:"asset_id"`

	// Kind classifies this entry.
	Kind MovementKind `json:"kind"`

	// Quantity is the amount this entry applies. A positive magnitude for
	// every Kind except [MovementKindAdjusted], which may be negative (a
	// downward variance correction). Direction is otherwise encoded by
	// which of FromLocationID/ToLocationID is set, not by the sign of
	// Quantity — see GetAssetBalance's fold logic in movement.go.
	Quantity int64 `json:"quantity"`

	// FromLocationID is where this movement originated. Empty for
	// [MovementKindArrived].
	FromLocationID string `json:"from_location_id,omitempty"`

	// ToLocationID is where this movement ends up. Empty for
	// [MovementKindDeparted].
	ToLocationID string `json:"to_location_id,omitempty"`

	// PerformedBy is a denormalized external actor id — who recorded this
	// movement. Required (validateMovementShape rejects an empty value):
	// every ledger entry needs an accountable actor, matching
	// merchandise-entry's issued_by not-null invariant. A plain caller-
	// supplied string, not a typed edge to a modelled Actor vertex — this
	// package owns no auth model of its own (decision #10: it's imported
	// as a package, with no HTTP boundary where auth would live; whatever
	// authenticates the caller is expected to hand this package an actor
	// id it already trusts).
	PerformedBy string `json:"performed_by"`

	// ReversesMovementID points at the Movement this entry mirrors, for
	// Kind = [MovementKindReversed]. Empty otherwise.
	ReversesMovementID string `json:"reverses_movement_id,omitempty"`

	// Note is a free-form reason (e.g. a stock-count variance explanation).
	Note string `json:"note,omitempty"`

	// OccurredAt is when the movement actually happened (RFC 3339), which
	// may differ from CreatedAt for an offline-queued entry posted on
	// reconnect.
	OccurredAt string `json:"occurred_at"`

	// CreatedAt is the RFC 3339 timestamp when this entry was posted.
	CreatedAt string `json:"created_at"`
}

// HoldStatus represents the lifecycle state of a [Hold].
type HoldStatus string

const (
	// HoldStatusReserved is the initial state of every new hold — quantity
	// is reserved but nothing has been committed yet.
	HoldStatusReserved HoldStatus = "reserved"

	// HoldStatusCommitted is a terminal state — the full held quantity was
	// committed (posted as a Movement).
	HoldStatusCommitted HoldStatus = "committed"

	// HoldStatusReleased is a terminal state — the hold was released,
	// either manually or automatically on expiry, with nothing committed.
	HoldStatusReleased HoldStatus = "released"

	// HoldStatusPartiallyCommitted is a terminal state — less than the
	// full held quantity was committed and the remainder was released.
	HoldStatusPartiallyCommitted HoldStatus = "partially_committed"
)

// CanTransitionTo reports whether transitioning from the receiver status to
// next is a valid move in the hold lifecycle.
//
// Allowed transitions:
//
//	reserved             → committed, released, partially_committed
//	committed            → (none — terminal)
//	released             → (none — terminal)
//	partially_committed  → (none — terminal)
func (s HoldStatus) CanTransitionTo(next HoldStatus) bool {
	switch s {
	case HoldStatusReserved:
		return next == HoldStatusCommitted || next == HoldStatusReleased || next == HoldStatusPartiallyCommitted
	default:
		return false
	}
}

// Hold is a reservation against an [Asset] — quantity set aside before it
// actually moves (checkout, order picking). Auto-expires via ExpiresAt if
// never committed; supports partial fulfillment (CommittedQuantity may be
// less than Quantity, with the remainder released).
type Hold struct {
	// ID is the unique identifier for this hold. Set by the backend on
	// creation; callers should leave it empty in create requests.
	ID string `json:"id"`

	// AssetID is the Asset this hold reserves against.
	AssetID string `json:"asset_id"`

	// LocationID is the Location this hold reserves stock at.
	LocationID string `json:"location_id,omitempty"`

	// Status is the current lifecycle state. Always starts as
	// [HoldStatusReserved] on creation.
	Status HoldStatus `json:"status"`

	// Quantity is the amount originally reserved.
	Quantity int64 `json:"quantity"`

	// CommittedQuantity is how much of Quantity has actually been
	// committed so far. Always <= Quantity.
	CommittedQuantity int64 `json:"committed_quantity"`

	// ExpiresAt is when an uncommitted hold is released automatically
	// (RFC 3339). Checked by a watchdog process.
	ExpiresAt string `json:"expires_at"`

	// PlacedBy is a denormalized external actor id — who placed this hold.
	// Required (CreateHold rejects an empty value), same reasoning as
	// Movement.PerformedBy: a plain caller-supplied string, not a typed
	// edge — this package owns no auth model of its own.
	PlacedBy string `json:"placed_by"`

	// CreatedAt is the RFC 3339 timestamp when this hold was created.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the RFC 3339 timestamp of the last update.
	UpdatedAt string `json:"updated_at"`
}

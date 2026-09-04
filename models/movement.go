package models

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
	// Quantity.
	Quantity int64 `json:"quantity"`

	// FromLocationID is where this movement originated. Empty for
	// [MovementKindArrived].
	FromLocationID string `json:"from_location_id,omitempty"`

	// ToLocationID is where this movement ends up. Empty for
	// [MovementKindDeparted].
	ToLocationID string `json:"to_location_id,omitempty"`

	// PerformedBy is a denormalized external actor id — who recorded this
	// movement. Required: every ledger entry needs an accountable actor,
	// matching merchandise-entry's issued_by not-null invariant. A plain
	// caller-supplied string, not a typed edge to a modelled Actor — this
	// package owns no auth model of its own.
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

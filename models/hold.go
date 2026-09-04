package models

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
	// Required, same reasoning as Movement.PerformedBy: a plain
	// caller-supplied string, not a typed edge — this package owns no auth
	// model of its own.
	PlacedBy string `json:"placed_by"`

	// CreatedAt is the RFC 3339 timestamp when this hold was created.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the RFC 3339 timestamp of the last update.
	UpdatedAt string `json:"updated_at"`
}

package models

// Location is a place an Asset can be — a warehouse bin, a household room,
// a pasture. Nested via ParentLocationID; unbounded depth. ParentLocationID
// is empty for a root location.
type Location struct {
	// ID is the unique identifier for this location. Set by the backend on
	// creation; callers should leave it empty in create requests.
	ID string `json:"id"`

	// Code is a stable, human-readable identifier (e.g. "L-1") assigned
	// once at creation. Set by the backend; callers should leave it empty
	// in create requests. Unlike Name, it never changes on update.
	Code string `json:"code"`

	// Name is the short human-readable label (e.g. "Bin 12", "Pantry shelf").
	Name string `json:"name"`

	// Kind is open-ended and descriptive only — "warehouse", "aisle",
	// "bin", "room", "shelf", "pasture"... Nothing constrains what kind
	// may nest under what.
	Kind string `json:"kind,omitempty"`

	// ParentLocationID is the parent Location's ID. Empty for a root
	// location.
	ParentLocationID string `json:"parent_location_id,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when this location was created.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the RFC 3339 timestamp of the last update.
	UpdatedAt string `json:"updated_at"`

	// Deleted marks a soft-deleted location. Not `omitempty`: false is a
	// real, meaningful answer, not an absent one. Every read that lists
	// rather than names one by id filters it out.
	Deleted bool `json:"deleted"`
}

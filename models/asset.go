package models

// AssetTrackingMode selects how an [Asset]'s on-hand amount is interpreted.
type AssetTrackingMode string

const (
	// AssetTrackingModeSerialized means one Asset row is one physical unit
	// (a specific cow, a specific laptop) — its balance at any Location is
	// always 0 or 1.
	AssetTrackingModeSerialized AssetTrackingMode = "serialized"

	// AssetTrackingModeFungible means one Asset row is a type
	// (e.g. "50kg rice bag") — its on-hand quantity at a Location is
	// derived by folding Movement entries, never stored directly.
	AssetTrackingModeFungible AssetTrackingMode = "fungible"
)

// Asset is a trackable thing — either one physical unit (serialized) or a
// type tracked as a quantity (fungible). See [AssetTrackingMode].
type Asset struct {
	// ID is the unique identifier for this asset. Set by the backend on
	// creation; callers should leave it empty in create requests.
	ID string `json:"id"`

	// Code is a stable, human-readable identifier (e.g. "A-1") assigned
	// once at creation. Set by the backend; callers should leave it empty
	// in create requests. Unlike Name, it never changes on update.
	Code string `json:"code"`

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
	// flexible-attribute mechanism — see requirements.md's decision #12.
	AttributesJSON string `json:"attributes_json,omitempty"`

	// LocationID is where this asset currently is — a convenience
	// denormalization, updated by the most recent Movement affecting this
	// asset. For a fungible asset this is one of potentially several
	// locations holding stock of this type — see Movement's
	// fold-to-balance model; a single LocationID field on Asset is a v1
	// simplification for the common single-location case. The real
	// balance at any given location always comes from folding Movement
	// entries, never from this field.
	LocationID string `json:"location_id,omitempty"`

	// CreatedAt is the RFC 3339 timestamp when this asset was created.
	CreatedAt string `json:"created_at"`

	// UpdatedAt is the RFC 3339 timestamp of the last update.
	UpdatedAt string `json:"updated_at"`

	// Deleted marks a soft-deleted asset. Not `omitempty` — see Location's
	// identical field for why.
	Deleted bool `json:"deleted"`
}

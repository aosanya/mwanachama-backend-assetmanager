package models

type AssetTrackingMode string

const (
	AssetTrackingModeSerialized AssetTrackingMode = "serialized"
	AssetTrackingModeFungible   AssetTrackingMode = "fungible"
)

type Asset struct {
	ID             string            `json:"id"`
	Code           string            `json:"code"`
	Name           string            `json:"name"`
	TrackingMode   AssetTrackingMode `json:"tracking_mode"`
	SerialTag      string            `json:"serial_tag,omitempty"`
	Category       string            `json:"category,omitempty"`
	AttributesJSON string            `json:"attributes_json,omitempty"`
	LocationID     string            `json:"location_id,omitempty"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
	Deleted        bool              `json:"deleted"`
}

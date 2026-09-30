package models

type HoldStatus string

const (
	HoldStatusReserved           HoldStatus = "reserved"
	HoldStatusCommitted          HoldStatus = "committed"
	HoldStatusReleased           HoldStatus = "released"
	HoldStatusPartiallyCommitted HoldStatus = "partially_committed"
)

func (s HoldStatus) CanTransitionTo(next HoldStatus) bool {
	switch s {
	case HoldStatusReserved:
		return next == HoldStatusCommitted || next == HoldStatusReleased || next == HoldStatusPartiallyCommitted
	default:
		return false
	}
}

type Hold struct {
	ID                string     `json:"id"`
	AssetID           string     `json:"asset_id"`
	LocationID        string     `json:"location_id,omitempty"`
	Status            HoldStatus `json:"status"`
	Quantity          int64      `json:"quantity"`
	CommittedQuantity int64      `json:"committed_quantity"`
	ExpiresAt         string     `json:"expires_at"`
	PlacedBy          string     `json:"placed_by"`
	CreatedAt         string     `json:"created_at"`
	UpdatedAt         string     `json:"updated_at"`
}

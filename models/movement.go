package models

type MovementKind string

const (
	MovementKindArrived     MovementKind = "arrived"
	MovementKindTransferred MovementKind = "transferred"
	MovementKindDeparted    MovementKind = "departed"
	MovementKindAdjusted    MovementKind = "adjusted"
	MovementKindReversed    MovementKind = "reversed"
)

type Movement struct {
	ID                 string       `json:"id"`
	AssetID            string       `json:"asset_id"`
	Kind               MovementKind `json:"kind"`
	Quantity           int64        `json:"quantity"`
	FromLocationID     string       `json:"from_location_id,omitempty"`
	ToLocationID       string       `json:"to_location_id,omitempty"`
	PerformedBy        string       `json:"performed_by"`
	ReversesMovementID string       `json:"reverses_movement_id,omitempty"`
	Note               string       `json:"note,omitempty"`
	OccurredAt         string       `json:"occurred_at"`
	CreatedAt          string       `json:"created_at"`
}

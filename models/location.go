package models

type Location struct {
	ID               string `json:"id"`
	Code             string `json:"code"`
	Name             string `json:"name"`
	Kind             string `json:"kind,omitempty"`
	ParentLocationID string `json:"parent_location_id,omitempty"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
	Deleted          bool   `json:"deleted"`
}

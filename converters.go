// converters.go — entity↔domain converters.
//
// Property helpers (StringProp, Int64Prop, …) live in
// [github.com/aosanya/mwanachama-backend-shared/entitygraph] and are used
// directly.
package mwanachamaassetmanager

import "github.com/aosanya/mwanachama-backend-shared/entitygraph"

// ── Location ─────────────────────────────────────────────────────────────

func locationToProperties(l Location) map[string]any {
	return map[string]any{
		"name":               l.Name,
		"kind":               l.Kind,
		"parent_location_id": l.ParentLocationID,
		"created_at":         l.CreatedAt,
		"updated_at":         l.UpdatedAt,
	}
}

func locationFromEntity(e entitygraph.Entity) Location {
	return Location{
		ID:               e.ID,
		Name:             entitygraph.StringProp(e.Properties, "name"),
		Kind:             entitygraph.StringProp(e.Properties, "kind"),
		ParentLocationID: entitygraph.StringProp(e.Properties, "parent_location_id"),
		CreatedAt:        entitygraph.StringProp(e.Properties, "created_at"),
		UpdatedAt:        entitygraph.StringProp(e.Properties, "updated_at"),
	}
}

// ── Asset ────────────────────────────────────────────────────────────────

func assetToProperties(a Asset) map[string]any {
	return map[string]any{
		"name":            a.Name,
		"tracking_mode":   string(a.TrackingMode),
		"serial_tag":      a.SerialTag,
		"category":        a.Category,
		"attributes_json": a.AttributesJSON,
		"location_id":     a.LocationID,
		"created_at":      a.CreatedAt,
		"updated_at":      a.UpdatedAt,
	}
}

func assetFromEntity(e entitygraph.Entity) Asset {
	return Asset{
		ID:             e.ID,
		Name:           entitygraph.StringProp(e.Properties, "name"),
		TrackingMode:   AssetTrackingMode(entitygraph.StringProp(e.Properties, "tracking_mode")),
		SerialTag:      entitygraph.StringProp(e.Properties, "serial_tag"),
		Category:       entitygraph.StringProp(e.Properties, "category"),
		AttributesJSON: entitygraph.StringProp(e.Properties, "attributes_json"),
		LocationID:     entitygraph.StringProp(e.Properties, "location_id"),
		CreatedAt:      entitygraph.StringProp(e.Properties, "created_at"),
		UpdatedAt:      entitygraph.StringProp(e.Properties, "updated_at"),
	}
}

// ── Movement ─────────────────────────────────────────────────────────────

func movementToProperties(mv Movement) map[string]any {
	return map[string]any{
		"kind":                 string(mv.Kind),
		"asset_id":             mv.AssetID,
		"quantity":             mv.Quantity,
		"from_location_id":     mv.FromLocationID,
		"to_location_id":       mv.ToLocationID,
		"performed_by":         mv.PerformedBy,
		"reverses_movement_id": mv.ReversesMovementID,
		"note":                 mv.Note,
		"occurred_at":          mv.OccurredAt,
		"created_at":           mv.CreatedAt,
	}
}

func movementFromEntity(e entitygraph.Entity) Movement {
	return Movement{
		ID:                 e.ID,
		AssetID:            entitygraph.StringProp(e.Properties, "asset_id"),
		Kind:               MovementKind(entitygraph.StringProp(e.Properties, "kind")),
		Quantity:           entitygraph.Int64Prop(e.Properties, "quantity"),
		FromLocationID:     entitygraph.StringProp(e.Properties, "from_location_id"),
		ToLocationID:       entitygraph.StringProp(e.Properties, "to_location_id"),
		PerformedBy:        entitygraph.StringProp(e.Properties, "performed_by"),
		ReversesMovementID: entitygraph.StringProp(e.Properties, "reverses_movement_id"),
		Note:               entitygraph.StringProp(e.Properties, "note"),
		OccurredAt:         entitygraph.StringProp(e.Properties, "occurred_at"),
		CreatedAt:          entitygraph.StringProp(e.Properties, "created_at"),
	}
}

// ── Hold ─────────────────────────────────────────────────────────────────

func holdToProperties(h Hold) map[string]any {
	return map[string]any{
		"status":             string(h.Status),
		"asset_id":           h.AssetID,
		"location_id":        h.LocationID,
		"quantity":           h.Quantity,
		"committed_quantity": h.CommittedQuantity,
		"expires_at":         h.ExpiresAt,
		"placed_by":          h.PlacedBy,
		"created_at":         h.CreatedAt,
		"updated_at":         h.UpdatedAt,
	}
}

func holdFromEntity(e entitygraph.Entity) Hold {
	return Hold{
		ID:                e.ID,
		AssetID:           entitygraph.StringProp(e.Properties, "asset_id"),
		LocationID:        entitygraph.StringProp(e.Properties, "location_id"),
		Status:            HoldStatus(entitygraph.StringProp(e.Properties, "status")),
		Quantity:          entitygraph.Int64Prop(e.Properties, "quantity"),
		CommittedQuantity: entitygraph.Int64Prop(e.Properties, "committed_quantity"),
		ExpiresAt:         entitygraph.StringProp(e.Properties, "expires_at"),
		PlacedBy:          entitygraph.StringProp(e.Properties, "placed_by"),
		CreatedAt:         entitygraph.StringProp(e.Properties, "created_at"),
		UpdatedAt:         entitygraph.StringProp(e.Properties, "updated_at"),
	}
}

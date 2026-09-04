// location.go — Location CRUD + child_of tree queries for [assetManager].
package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
)

// descendantTraversalDepth bounds how far [assetManager.ListDescendantLocations]
// walks the child_of tree. entitygraph.DataManager has no TraverseGraph
// helper, so ListDescendantLocations walks the tree itself, one level of
// child_of edges per round trip (see the function below) — this is a real
// bound on the number of round trips, not just a nominal safety cap, but
// still comfortably supports depth 200 for any realistic warehouse/
// household/chapter hierarchy.
const descendantTraversalDepth = 200

// CreateLocation creates a new Location entity in the graph. When
// l.ParentLocationID is set, the parent must already exist; the child_of
// edge is created atomically with the entity via entitygraph's
// inline-relationship support.
func (m *assetManager) CreateLocation(ctx context.Context, l Location) (Location, error) {
	if l.Name == "" {
		return Location{}, fmt.Errorf("%w: Location.Name is required", ErrInvalidLocation)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	l.CreatedAt = now
	l.UpdatedAt = now

	req := entitygraph.CreateEntityRequest{
		TypeID:     locationTypeID,
		Properties: locationToProperties(l),
	}
	if l.ParentLocationID != "" {
		parent, err := m.GetLocation(ctx, l.ParentLocationID)
		if err != nil {
			if errors.Is(err, ErrLocationNotFound) {
				return Location{}, fmt.Errorf("%w: parent location %q not found", ErrInvalidLocation, l.ParentLocationID)
			}
			return Location{}, fmt.Errorf("CreateLocation: %w", err)
		}
		req.Relationships = []entitygraph.EntityRelationshipRequest{
			{Name: RelLabelChildOf, ToID: parent.ID},
		}
	}

	created, err := m.dm.CreateEntity(ctx, req)
	if err != nil {
		return Location{}, fmt.Errorf("CreateLocation: %w", err)
	}
	return locationFromEntity(created), nil
}

// GetLocation reads a single Location entity from the graph.
func (m *assetManager) GetLocation(ctx context.Context, locationID string) (Location, error) {
	e, err := m.dm.GetEntity(ctx, locationID)
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return Location{}, ErrLocationNotFound
		}
		return Location{}, fmt.Errorf("GetLocation: %w", err)
	}
	if e.TypeID != locationTypeID {
		return Location{}, ErrLocationNotFound
	}
	return locationFromEntity(e), nil
}

// UpdateLocation patches Name/Kind, and re-parents the location if
// ParentLocationID differs from the stored value. Re-parenting is rejected
// if the new parent is the location itself or one of its own descendants —
// that would introduce a cycle in the child_of tree.
func (m *assetManager) UpdateLocation(ctx context.Context, l Location) (Location, error) {
	current, err := m.GetLocation(ctx, l.ID)
	if err != nil {
		return Location{}, err
	}
	if l.Name == "" {
		return Location{}, fmt.Errorf("%w: Location.Name is required", ErrInvalidLocation)
	}

	if l.ParentLocationID != current.ParentLocationID {
		if err := m.reparentLocation(ctx, current, l.ParentLocationID); err != nil {
			return Location{}, err
		}
	}

	l.CreatedAt = current.CreatedAt
	l.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	updated, err := m.dm.UpdateEntity(ctx, l.ID, entitygraph.UpdateEntityRequest{
		Properties: locationToProperties(l),
	})
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return Location{}, ErrLocationNotFound
		}
		return Location{}, fmt.Errorf("UpdateLocation: %w", err)
	}
	return locationFromEntity(updated), nil
}

// reparentLocation validates and applies a parent change: rejects
// self-parenting and cycles, then replaces the child_of edge (deleting the
// old one, if any, and creating the new one, if any — an empty
// newParentID means "become a root location").
func (m *assetManager) reparentLocation(ctx context.Context, current Location, newParentID string) error {
	if newParentID == current.ID {
		return fmt.Errorf("%w: a location cannot be its own parent", ErrInvalidLocation)
	}
	if newParentID != "" {
		if _, err := m.GetLocation(ctx, newParentID); err != nil {
			if errors.Is(err, ErrLocationNotFound) {
				return fmt.Errorf("%w: parent location %q not found", ErrInvalidLocation, newParentID)
			}
			return fmt.Errorf("reparentLocation: %w", err)
		}
		descendants, err := m.ListDescendantLocations(ctx, current.ID)
		if err != nil {
			return fmt.Errorf("reparentLocation: %w", err)
		}
		for _, d := range descendants {
			if d.ID == newParentID {
				return fmt.Errorf("%w: %q is a descendant of %q; re-parenting would create a cycle", ErrInvalidLocation, newParentID, current.ID)
			}
		}
	}

	if current.ParentLocationID != "" {
		if err := m.deleteChildOfEdge(ctx, current.ID); err != nil {
			return fmt.Errorf("reparentLocation: %w", err)
		}
	}
	if newParentID != "" {
		if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
			Name:   RelLabelChildOf,
			FromID: current.ID,
			ToID:   newParentID,
		}); err != nil {
			return fmt.Errorf("reparentLocation: create child_of: %w", err)
		}
	}
	return nil
}

// deleteChildOfEdge removes locationID's outbound child_of edge, if any.
// A location with no parent has none — a no-op, not an error.
func (m *assetManager) deleteChildOfEdge(ctx context.Context, locationID string) error {
	edges, err := m.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		FromID: locationID,
		Name:   RelLabelChildOf,
	})
	if err != nil {
		return fmt.Errorf("deleteChildOfEdge: list: %w", err)
	}
	for _, e := range edges {
		if err := m.dm.DeleteRelationship(ctx, e.ID); err != nil && !errors.Is(err, entitygraph.ErrRelationshipNotFound) {
			return fmt.Errorf("deleteChildOfEdge: %w", err)
		}
	}
	return nil
}

// DeleteLocation soft-deletes the Location entity. Refused with
// [ErrLocationNotEmpty] if the location currently has child locations or
// assets denormalized to it — move or delete those first.
func (m *assetManager) DeleteLocation(ctx context.Context, locationID string) error {
	if _, err := m.GetLocation(ctx, locationID); err != nil {
		return err
	}
	children, err := m.ListLocations(ctx, LocationFilter{ParentLocationID: locationID})
	if err != nil {
		return fmt.Errorf("DeleteLocation: %w", err)
	}
	if len(children) > 0 {
		return ErrLocationNotEmpty
	}
	assetsHere, err := m.ListAssets(ctx, AssetFilter{LocationID: locationID})
	if err != nil {
		return fmt.Errorf("DeleteLocation: %w", err)
	}
	if len(assetsHere) > 0 {
		return ErrLocationNotEmpty
	}

	if err := m.deleteChildOfEdge(ctx, locationID); err != nil {
		return fmt.Errorf("DeleteLocation: %w", err)
	}
	if err := m.dm.DeleteEntity(ctx, locationID); err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return ErrLocationNotFound
		}
		return fmt.Errorf("DeleteLocation: %w", err)
	}
	return nil
}

// ListLocations returns all non-deleted Location entities that match the
// filter.
func (m *assetManager) ListLocations(ctx context.Context, filter LocationFilter) ([]Location, error) {
	props := map[string]any{}
	if filter.RootOnly {
		props["parent_location_id"] = ""
	} else if filter.ParentLocationID != "" {
		props["parent_location_id"] = filter.ParentLocationID
	}

	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		TypeID:     locationTypeID,
		Properties: props,
	})
	if err != nil {
		return nil, fmt.Errorf("ListLocations: %w", err)
	}
	out := make([]Location, 0, len(entities))
	for _, e := range entities {
		out = append(out, locationFromEntity(e))
	}
	return out, nil
}

// ListDescendantLocations returns every Location transitively nested under
// locationID (children, grandchildren, ...), found by walking inbound
// child_of edges — a child's child_of edge points at its parent, so
// "everyone who points at me, transitively" is exactly the descendant set.
//
// entitygraph.DataManager has no built-in graph traversal, so this walks
// the tree itself: a bounded breadth-first search that, at each level,
// lists every child_of edge whose ToID is in the current frontier, then
// fetches the entity for each edge's FromID (the child) to seed the next
// level. Depth is capped by descendantTraversalDepth.
func (m *assetManager) ListDescendantLocations(ctx context.Context, locationID string) ([]Location, error) {
	visited := map[string]bool{locationID: true}
	frontier := []string{locationID}
	var out []Location

	for depth := 0; depth < descendantTraversalDepth && len(frontier) > 0; depth++ {
		var next []string
		for _, id := range frontier {
			edges, err := m.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
				ToID: id,
				Name: RelLabelChildOf,
			})
			if err != nil {
				return nil, fmt.Errorf("ListDescendantLocations: list relationships: %w", err)
			}
			for _, e := range edges {
				if visited[e.FromID] {
					continue
				}
				visited[e.FromID] = true
				next = append(next, e.FromID)
			}
		}
		for _, id := range next {
			e, err := m.dm.GetEntity(ctx, id)
			if err != nil {
				if errors.Is(err, entitygraph.ErrEntityNotFound) {
					// Soft-deleted (or otherwise gone) between the edge
					// lookup and this read — same "excluded" outcome the
					// old vertex list gave a deleted entity.
					continue
				}
				return nil, fmt.Errorf("ListDescendantLocations: get entity: %w", err)
			}
			out = append(out, locationFromEntity(e))
		}
		frontier = next
	}
	if out == nil {
		out = []Location{}
	}
	return out, nil
}

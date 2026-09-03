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
// walks the child_of tree. entitygraph.TraverseGraph compiles a single call
// to one recursive CTE (see mwanachama-backend-taskmanager's W5 port notes),
// so this is a safety cap, not a real limit on tree depth in practice — no
// realistic warehouse/household/chapter hierarchy nests this deep.
const descendantTraversalDepth = 200

// CreateLocation creates a new Location entity in the agency graph. When
// l.ParentLocationID is set, the parent must already exist in the same
// agency; the child_of edge is created atomically with the entity via
// entitygraph's inline-relationship support.
func (m *assetManager) CreateLocation(ctx context.Context, agencyID string, l Location) (Location, error) {
	if l.Name == "" {
		return Location{}, fmt.Errorf("%w: Location.Name is required", ErrInvalidLocation)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	l.AgencyID = agencyID
	l.CreatedAt = now
	l.UpdatedAt = now

	req := entitygraph.CreateEntityRequest{
		AgencyID:   agencyID,
		TypeID:     locationTypeID,
		Properties: locationToProperties(l),
	}
	if l.ParentLocationID != "" {
		parent, err := m.GetLocation(ctx, agencyID, l.ParentLocationID)
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

// GetLocation reads a single Location entity from the agency graph.
func (m *assetManager) GetLocation(ctx context.Context, agencyID, locationID string) (Location, error) {
	e, err := m.dm.GetEntity(ctx, agencyID, locationID)
	if err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return Location{}, ErrLocationNotFound
		}
		return Location{}, fmt.Errorf("GetLocation: %w", err)
	}
	if e.AgencyID != agencyID || e.TypeID != locationTypeID {
		return Location{}, ErrLocationNotFound
	}
	return locationFromEntity(e), nil
}

// UpdateLocation patches Name/Kind, and re-parents the location if
// ParentLocationID differs from the stored value. Re-parenting is rejected
// if the new parent is the location itself or one of its own descendants —
// that would introduce a cycle in the child_of tree.
func (m *assetManager) UpdateLocation(ctx context.Context, agencyID string, l Location) (Location, error) {
	current, err := m.GetLocation(ctx, agencyID, l.ID)
	if err != nil {
		return Location{}, err
	}
	if l.Name == "" {
		return Location{}, fmt.Errorf("%w: Location.Name is required", ErrInvalidLocation)
	}

	if l.ParentLocationID != current.ParentLocationID {
		if err := m.reparentLocation(ctx, agencyID, current, l.ParentLocationID); err != nil {
			return Location{}, err
		}
	}

	l.AgencyID = agencyID
	l.CreatedAt = current.CreatedAt
	l.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	updated, err := m.dm.UpdateEntity(ctx, agencyID, l.ID, entitygraph.UpdateEntityRequest{
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
func (m *assetManager) reparentLocation(ctx context.Context, agencyID string, current Location, newParentID string) error {
	if newParentID == current.ID {
		return fmt.Errorf("%w: a location cannot be its own parent", ErrInvalidLocation)
	}
	if newParentID != "" {
		if _, err := m.GetLocation(ctx, agencyID, newParentID); err != nil {
			if errors.Is(err, ErrLocationNotFound) {
				return fmt.Errorf("%w: parent location %q not found", ErrInvalidLocation, newParentID)
			}
			return fmt.Errorf("reparentLocation: %w", err)
		}
		descendants, err := m.ListDescendantLocations(ctx, agencyID, current.ID)
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
		if err := m.deleteChildOfEdge(ctx, agencyID, current.ID); err != nil {
			return fmt.Errorf("reparentLocation: %w", err)
		}
	}
	if newParentID != "" {
		if _, err := m.dm.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
			AgencyID: agencyID,
			Name:     RelLabelChildOf,
			FromID:   current.ID,
			ToID:     newParentID,
		}); err != nil {
			return fmt.Errorf("reparentLocation: create child_of: %w", err)
		}
	}
	return nil
}

// deleteChildOfEdge removes locationID's outbound child_of edge, if any.
// A location with no parent has none — a no-op, not an error.
func (m *assetManager) deleteChildOfEdge(ctx context.Context, agencyID, locationID string) error {
	edges, err := m.dm.ListRelationships(ctx, entitygraph.RelationshipFilter{
		AgencyID: agencyID,
		FromID:   locationID,
		Name:     RelLabelChildOf,
	})
	if err != nil {
		return fmt.Errorf("deleteChildOfEdge: list: %w", err)
	}
	for _, e := range edges {
		if err := m.dm.DeleteRelationship(ctx, agencyID, e.ID); err != nil && !errors.Is(err, entitygraph.ErrRelationshipNotFound) {
			return fmt.Errorf("deleteChildOfEdge: %w", err)
		}
	}
	return nil
}

// DeleteLocation soft-deletes the Location entity. Refused with
// [ErrLocationNotEmpty] if the location currently has child locations or
// assets denormalized to it — move or delete those first.
func (m *assetManager) DeleteLocation(ctx context.Context, agencyID, locationID string) error {
	if _, err := m.GetLocation(ctx, agencyID, locationID); err != nil {
		return err
	}
	children, err := m.ListLocations(ctx, agencyID, LocationFilter{ParentLocationID: locationID})
	if err != nil {
		return fmt.Errorf("DeleteLocation: %w", err)
	}
	if len(children) > 0 {
		return ErrLocationNotEmpty
	}
	assetsHere, err := m.ListAssets(ctx, agencyID, AssetFilter{LocationID: locationID})
	if err != nil {
		return fmt.Errorf("DeleteLocation: %w", err)
	}
	if len(assetsHere) > 0 {
		return ErrLocationNotEmpty
	}

	if err := m.deleteChildOfEdge(ctx, agencyID, locationID); err != nil {
		return fmt.Errorf("DeleteLocation: %w", err)
	}
	if err := m.dm.DeleteEntity(ctx, agencyID, locationID); err != nil {
		if errors.Is(err, entitygraph.ErrEntityNotFound) {
			return ErrLocationNotFound
		}
		return fmt.Errorf("DeleteLocation: %w", err)
	}
	return nil
}

// ListLocations returns all non-deleted Location entities for the agency
// that match the filter.
func (m *assetManager) ListLocations(ctx context.Context, agencyID string, filter LocationFilter) ([]Location, error) {
	props := map[string]any{}
	if filter.RootOnly {
		props["parent_location_id"] = ""
	} else if filter.ParentLocationID != "" {
		props["parent_location_id"] = filter.ParentLocationID
	}

	entities, err := m.dm.ListEntities(ctx, entitygraph.EntityFilter{
		AgencyID:   agencyID,
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
func (m *assetManager) ListDescendantLocations(ctx context.Context, agencyID, locationID string) ([]Location, error) {
	res, err := m.dm.TraverseGraph(ctx, entitygraph.TraverseGraphRequest{
		AgencyID:  agencyID,
		StartID:   locationID,
		Direction: "inbound",
		Depth:     descendantTraversalDepth,
		Names:     []string{RelLabelChildOf},
	})
	if err != nil {
		return nil, fmt.Errorf("ListDescendantLocations: %w", err)
	}
	out := make([]Location, 0, len(res.Vertices))
	for _, e := range res.Vertices {
		if e.ID == locationID {
			continue
		}
		out = append(out, locationFromEntity(e))
	}
	return out, nil
}

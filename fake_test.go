package mwanachamaassetmanager_test

import (
	"context"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
	"github.com/google/uuid"
)

// fakeDataManager is an in-memory entitygraph.DataManager used for unit
// tests. Modelled on mwanachama-backend-taskmanager's fake_test.go.
//
// ListDescendantLocations no longer relies on any traversal helper from
// entitygraph — it walks child_of edges itself via ListRelationships/
// GetEntity (see location.go), so this fake only needs to implement those
// correctly, which it does for arbitrary depth. Deeper tree/traversal
// correctness against a real recursive-CTE-backed DataManager is still
// left to postgres_integration_test.go for the Postgres-specific bits
// (e.g. multi-hop cycle rejection), matching how taskmanager defers
// WorkflowRun closure correctness to its own Postgres integration tests.
type fakeDataManager struct {
	entities      map[string]entitygraph.Entity
	relationships map[string]entitygraph.Relationship
}

func newFakeDataManager() *fakeDataManager {
	return &fakeDataManager{
		entities:      make(map[string]entitygraph.Entity),
		relationships: make(map[string]entitygraph.Relationship),
	}
}

func (f *fakeDataManager) CreateEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	id := uuid.NewString()
	now := time.Now().UTC()
	props := make(map[string]any, len(req.Properties))
	for k, v := range req.Properties {
		props[k] = v
	}
	e := entitygraph.Entity{
		ID:         id,
		TypeID:     req.TypeID,
		Properties: props,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	f.entities[id] = e

	for _, rel := range req.Relationships {
		if _, err := f.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
			Name:   rel.Name,
			FromID: id,
			ToID:   rel.ToID,
		}); err != nil {
			return entitygraph.Entity{}, err
		}
	}
	return e, nil
}

func (f *fakeDataManager) GetEntity(_ context.Context, entityID string) (entitygraph.Entity, error) {
	e, ok := f.entities[entityID]
	if !ok || e.Deleted {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	return e, nil
}

func (f *fakeDataManager) UpdateEntity(_ context.Context, entityID string, req entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	e, ok := f.entities[entityID]
	if !ok || e.Deleted {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	if e.Properties == nil {
		e.Properties = map[string]any{}
	}
	for k, v := range req.Properties {
		e.Properties[k] = v
	}
	e.UpdatedAt = time.Now().UTC()
	f.entities[entityID] = e
	return e, nil
}

func (f *fakeDataManager) DeleteEntity(_ context.Context, entityID string) error {
	e, ok := f.entities[entityID]
	if !ok || e.Deleted {
		return entitygraph.ErrEntityNotFound
	}
	now := time.Now().UTC()
	e.Deleted = true
	e.DeletedAt = &now
	f.entities[entityID] = e
	return nil
}

func (f *fakeDataManager) ListEntities(_ context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	out := make([]entitygraph.Entity, 0)
	for _, e := range f.entities {
		if e.Deleted {
			continue
		}
		if filter.TypeID != "" && e.TypeID != filter.TypeID {
			continue
		}
		match := true
		for k, want := range filter.Properties {
			got, ok := e.Properties[k]
			if !ok || got != want {
				match = false
				break
			}
		}
		if !match {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func (f *fakeDataManager) UpsertEntity(ctx context.Context, req entitygraph.CreateEntityRequest) (entitygraph.Entity, error) {
	return entitygraph.Entity{}, entitygraph.ErrUniqueKeyNotDefined
}

func (f *fakeDataManager) CreateRelationship(_ context.Context, req entitygraph.CreateRelationshipRequest) (entitygraph.Relationship, error) {
	if _, ok := f.entities[req.FromID]; !ok {
		return entitygraph.Relationship{}, entitygraph.ErrEntityNotFound
	}
	if _, ok := f.entities[req.ToID]; !ok {
		return entitygraph.Relationship{}, entitygraph.ErrEntityNotFound
	}
	id := uuid.NewString()
	props := make(map[string]any, len(req.Properties))
	for k, v := range req.Properties {
		props[k] = v
	}
	r := entitygraph.Relationship{
		ID:         id,
		Name:       req.Name,
		FromID:     req.FromID,
		ToID:       req.ToID,
		Properties: props,
		CreatedAt:  time.Now().UTC(),
	}
	f.relationships[id] = r
	return r, nil
}

func (f *fakeDataManager) GetRelationship(_ context.Context, relID string) (entitygraph.Relationship, error) {
	r, ok := f.relationships[relID]
	if !ok {
		return entitygraph.Relationship{}, entitygraph.ErrRelationshipNotFound
	}
	return r, nil
}

func (f *fakeDataManager) DeleteRelationship(_ context.Context, relID string) error {
	if _, ok := f.relationships[relID]; !ok {
		return entitygraph.ErrRelationshipNotFound
	}
	delete(f.relationships, relID)
	return nil
}

func (f *fakeDataManager) ListRelationships(_ context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	out := make([]entitygraph.Relationship, 0)
	for _, r := range f.relationships {
		if filter.FromID != "" && r.FromID != filter.FromID {
			continue
		}
		if filter.ToID != "" && r.ToID != filter.ToID {
			continue
		}
		if filter.Name != "" && r.Name != filter.Name {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

package mwanachamaassetmanager_test

import (
	"context"
	"time"

	"github.com/aosanya/mwanachama-backend-shared/entitygraph"
	"github.com/google/uuid"
)

// fakeDataManager is an in-memory entitygraph.DataManager used for unit
// tests. Modelled on mwanachama-backend-taskmanager's fake_test.go — same
// reasoning: mwanachama-backend-shared's schema-aware memory.Backend
// requires an active published schema via GetActive, which would force
// schema-seeding boilerplate onto every call site these tests don't
// otherwise need.
//
// TraverseGraph here is single-hop only, same as taskmanager's fake — real
// multi-level child_of descendant-closure correctness (what
// ListDescendantLocations needs beyond one level) is only provable against
// a DataManager that actually compiles Depth into a recursive CTE, so that
// case is left to postgres_integration_test.go, matching how taskmanager
// defers WorkflowRun closure correctness to its own Postgres integration
// tests rather than its fake.
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

func (f *fakeDataManager) key(agencyID, entityID string) string {
	return agencyID + "/" + entityID
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
		AgencyID:   req.AgencyID,
		TypeID:     req.TypeID,
		Properties: props,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	f.entities[f.key(req.AgencyID, id)] = e

	for _, rel := range req.Relationships {
		if _, err := f.CreateRelationship(ctx, entitygraph.CreateRelationshipRequest{
			AgencyID: req.AgencyID,
			Name:     rel.Name,
			FromID:   id,
			ToID:     rel.ToID,
		}); err != nil {
			return entitygraph.Entity{}, err
		}
	}
	return e, nil
}

func (f *fakeDataManager) GetEntity(_ context.Context, agencyID, entityID string) (entitygraph.Entity, error) {
	e, ok := f.entities[f.key(agencyID, entityID)]
	if !ok || e.Deleted {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	return e, nil
}

func (f *fakeDataManager) UpdateEntity(_ context.Context, agencyID, entityID string, req entitygraph.UpdateEntityRequest) (entitygraph.Entity, error) {
	k := f.key(agencyID, entityID)
	e, ok := f.entities[k]
	if !ok || e.Deleted {
		return entitygraph.Entity{}, entitygraph.ErrEntityNotFound
	}
	if e.Properties == nil {
		e.Properties = map[string]any{}
	}
	for k2, v := range req.Properties {
		e.Properties[k2] = v
	}
	e.UpdatedAt = time.Now().UTC()
	f.entities[k] = e
	return e, nil
}

func (f *fakeDataManager) DeleteEntity(_ context.Context, agencyID, entityID string) error {
	k := f.key(agencyID, entityID)
	e, ok := f.entities[k]
	if !ok || e.Deleted {
		return entitygraph.ErrEntityNotFound
	}
	now := time.Now().UTC()
	e.Deleted = true
	e.DeletedAt = &now
	f.entities[k] = e
	return nil
}

func (f *fakeDataManager) ListEntities(_ context.Context, filter entitygraph.EntityFilter) ([]entitygraph.Entity, error) {
	out := make([]entitygraph.Entity, 0)
	for _, e := range f.entities {
		if e.Deleted {
			continue
		}
		if filter.AgencyID != "" && e.AgencyID != filter.AgencyID {
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
	if _, ok := f.entities[f.key(req.AgencyID, req.FromID)]; !ok {
		return entitygraph.Relationship{}, entitygraph.ErrEntityNotFound
	}
	if _, ok := f.entities[f.key(req.AgencyID, req.ToID)]; !ok {
		return entitygraph.Relationship{}, entitygraph.ErrEntityNotFound
	}
	id := uuid.NewString()
	props := make(map[string]any, len(req.Properties))
	for k, v := range req.Properties {
		props[k] = v
	}
	r := entitygraph.Relationship{
		ID:         id,
		AgencyID:   req.AgencyID,
		Name:       req.Name,
		FromID:     req.FromID,
		ToID:       req.ToID,
		Properties: props,
		CreatedAt:  time.Now().UTC(),
	}
	f.relationships[f.key(req.AgencyID, id)] = r
	return r, nil
}

func (f *fakeDataManager) GetRelationship(_ context.Context, agencyID, relID string) (entitygraph.Relationship, error) {
	r, ok := f.relationships[f.key(agencyID, relID)]
	if !ok {
		return entitygraph.Relationship{}, entitygraph.ErrRelationshipNotFound
	}
	return r, nil
}

func (f *fakeDataManager) DeleteRelationship(_ context.Context, agencyID, relID string) error {
	k := f.key(agencyID, relID)
	if _, ok := f.relationships[k]; !ok {
		return entitygraph.ErrRelationshipNotFound
	}
	delete(f.relationships, k)
	return nil
}

func (f *fakeDataManager) ListRelationships(_ context.Context, filter entitygraph.RelationshipFilter) ([]entitygraph.Relationship, error) {
	out := make([]entitygraph.Relationship, 0)
	for _, r := range f.relationships {
		if filter.AgencyID != "" && r.AgencyID != filter.AgencyID {
			continue
		}
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

// TraverseGraph in the fake supports single-hop traversal only — see the
// type doc comment above for why multi-level descendant closure is left to
// the Postgres integration tests.
func (f *fakeDataManager) TraverseGraph(_ context.Context, req entitygraph.TraverseGraphRequest) (entitygraph.TraverseGraphResult, error) {
	res := entitygraph.TraverseGraphResult{Edges: []entitygraph.Relationship{}, Vertices: []entitygraph.Entity{}}
	allowedNames := map[string]struct{}{}
	for _, n := range req.Names {
		allowedNames[n] = struct{}{}
	}
	for _, r := range f.relationships {
		if r.AgencyID != req.AgencyID {
			continue
		}
		if len(allowedNames) > 0 {
			if _, ok := allowedNames[r.Name]; !ok {
				continue
			}
		}
		var otherID string
		switch req.Direction {
		case "outbound":
			if r.FromID != req.StartID {
				continue
			}
			otherID = r.ToID
		case "inbound":
			if r.ToID != req.StartID {
				continue
			}
			otherID = r.FromID
		default: // "any" or empty
			if r.FromID != req.StartID && r.ToID != req.StartID {
				continue
			}
			if r.FromID == req.StartID {
				otherID = r.ToID
			} else {
				otherID = r.FromID
			}
		}
		res.Edges = append(res.Edges, r)
		if e, ok := f.entities[f.key(req.AgencyID, otherID)]; ok && !e.Deleted {
			res.Vertices = append(res.Vertices, e)
		}
	}
	return res, nil
}

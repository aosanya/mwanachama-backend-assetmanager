package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-shared/specstore"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func (m *assetManager) CreateLocation(ctx context.Context, l models.Location) (models.Location, error) {
	l.ID = newID()
	if err := m.check(roleLocation, l, ErrInvalidLocation); err != nil {
		return models.Location{}, err
	}
	if l.ParentLocationID != "" {
		if _, err := m.GetLocation(ctx, l.ParentLocationID); err != nil {
			if errors.Is(err, ErrLocationNotFound) {
				return models.Location{}, fmt.Errorf("%w: parent location %q not found", ErrInvalidLocation, l.ParentLocationID)
			}
			return models.Location{}, fmt.Errorf("CreateLocation: %w", err)
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	l.CreatedAt = now
	l.UpdatedAt = now

	err := m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		code, err := m.nextCode(ctx, tx, roleLocation, "L")
		if err != nil {
			return err
		}
		l.Code = code
		row, err := encode(m.store.Object(roleLocation), l)
		if err != nil {
			return err
		}
		return tx.WithContext(ctx).Table(m.store.Table(roleLocation)).Create(row).Error
	})
	if err != nil {
		return models.Location{}, fmt.Errorf("CreateLocation: %w", err)
	}
	return l, nil
}

func (m *assetManager) GetLocation(ctx context.Context, locationID string) (models.Location, error) {
	var out models.Location
	q := m.store.Query(ctx, roleLocation).Where("id = ? AND deleted = ?", locationID, false)
	if err := m.store.Take(q, roleLocation, &out, ErrLocationNotFound); err != nil {
		if errors.Is(err, ErrLocationNotFound) {
			return models.Location{}, err
		}
		return models.Location{}, fmt.Errorf("GetLocation: %w", err)
	}
	return out, nil
}

func (m *assetManager) UpdateLocation(ctx context.Context, l models.Location) (models.Location, error) {
	current, err := m.GetLocation(ctx, l.ID)
	if err != nil {
		return models.Location{}, err
	}
	if err := m.check(roleLocation, l, ErrInvalidLocation); err != nil {
		return models.Location{}, err
	}
	if l.ParentLocationID != current.ParentLocationID {
		if err := m.validateReparent(ctx, current, l.ParentLocationID); err != nil {
			return models.Location{}, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	err = m.store.Query(ctx, roleLocation).Where("id = ?", l.ID).
		Updates(map[string]any{
			columnName("Name"):             l.Name,
			columnName("Kind"):             l.Kind,
			columnName("ParentLocationID"): l.ParentLocationID,
			columnName("UpdatedAt"):        now,
		}).Error
	if err != nil {
		return models.Location{}, fmt.Errorf("UpdateLocation: %w", err)
	}
	current.Name = l.Name
	current.Kind = l.Kind
	current.ParentLocationID = l.ParentLocationID
	current.UpdatedAt = now
	return current, nil
}

func (m *assetManager) validateReparent(ctx context.Context, current models.Location, newParentID string) error {
	if newParentID == current.ID {
		return fmt.Errorf("%w: a location cannot be its own parent", ErrInvalidLocation)
	}
	if newParentID == "" {
		return nil
	}
	if _, err := m.GetLocation(ctx, newParentID); err != nil {
		if errors.Is(err, ErrLocationNotFound) {
			return fmt.Errorf("%w: parent location %q not found", ErrInvalidLocation, newParentID)
		}
		return fmt.Errorf("validateReparent: %w", err)
	}
	descendants, err := m.ListDescendantLocations(ctx, current.ID)
	if err != nil {
		return fmt.Errorf("validateReparent: %w", err)
	}
	for _, d := range descendants {
		if d.ID == newParentID {
			return fmt.Errorf("%w: %q is a descendant of %q; re-parenting would create a cycle", ErrInvalidLocation, newParentID, current.ID)
		}
	}
	return nil
}

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

	now := time.Now().UTC().Format(time.RFC3339)
	err = m.store.Query(ctx, roleLocation).Where("id = ?", locationID).
		Updates(map[string]any{
			columnName("Deleted"):   true,
			columnName("UpdatedAt"): now,
		}).Error
	if err != nil {
		return fmt.Errorf("DeleteLocation: %w", err)
	}
	return nil
}

func (m *assetManager) ListLocations(ctx context.Context, filter LocationFilter) ([]models.Location, error) {
	q := m.store.Query(ctx, roleLocation).Where("deleted = ?", false)
	if filter.RootOnly {
		q = q.Where("parent_location_id = ?", "")
	} else if filter.ParentLocationID != "" {
		q = q.Where("parent_location_id = ?", filter.ParentLocationID)
	}

	out, err := specstore.List[models.Location](m.store, q.Order("id"), roleLocation)
	if err != nil {
		return nil, fmt.Errorf("ListLocations: %w", err)
	}
	return out, nil
}

func (m *assetManager) ListDescendantLocations(ctx context.Context, locationID string) ([]models.Location, error) {
	all, err := m.ListLocations(ctx, LocationFilter{})
	if err != nil {
		return nil, fmt.Errorf("ListDescendantLocations: %w", err)
	}
	childrenOf := map[string][]models.Location{}
	for _, l := range all {
		childrenOf[l.ParentLocationID] = append(childrenOf[l.ParentLocationID], l)
	}

	out := []models.Location{}
	visited := map[string]bool{locationID: true}
	frontier := []string{locationID}
	for len(frontier) > 0 {
		var next []string
		for _, id := range frontier {
			for _, child := range childrenOf[id] {
				if visited[child.ID] {
					continue
				}
				visited[child.ID] = true
				out = append(out, child)
				next = append(next, child.ID)
			}
		}
		frontier = next
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

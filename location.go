// location.go — Location CRUD + parent/descendant tree queries for
// [assetManager].
package mwanachamaassetmanager

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/aosanya/mwanachama-backend-assetmanager/gormstore"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

// CreateLocation creates a new Location row. When l.ParentLocationID is
// set, the parent must already exist.
func (m *assetManager) CreateLocation(ctx context.Context, l models.Location) (models.Location, error) {
	if l.Name == "" {
		return models.Location{}, fmt.Errorf("%w: Location.Name is required", ErrInvalidLocation)
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

	row := gormstore.LocationToRow(l)
	if err := m.db.WithContext(ctx).Table(m.tables.Locations).Create(&row).Error; err != nil {
		return models.Location{}, fmt.Errorf("CreateLocation: %w", err)
	}
	return gormstore.LocationFromRow(row), nil
}

// GetLocation reads a single non-deleted Location row.
func (m *assetManager) GetLocation(ctx context.Context, locationID string) (models.Location, error) {
	var row gormstore.LocationRow
	err := m.db.WithContext(ctx).Table(m.tables.Locations).
		Where("id = ? AND deleted = ?", locationID, false).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return models.Location{}, ErrLocationNotFound
		}
		return models.Location{}, fmt.Errorf("GetLocation: %w", err)
	}
	return gormstore.LocationFromRow(row), nil
}

// UpdateLocation patches Name/Kind, and re-parents the location if
// ParentLocationID differs from the stored value. Re-parenting is rejected
// if the new parent is the location itself or one of its own descendants —
// that would introduce a cycle in the parent tree.
func (m *assetManager) UpdateLocation(ctx context.Context, l models.Location) (models.Location, error) {
	current, err := m.GetLocation(ctx, l.ID)
	if err != nil {
		return models.Location{}, err
	}
	if l.Name == "" {
		return models.Location{}, fmt.Errorf("%w: Location.Name is required", ErrInvalidLocation)
	}
	if l.ParentLocationID != current.ParentLocationID {
		if err := m.validateReparent(ctx, current, l.ParentLocationID); err != nil {
			return models.Location{}, err
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	err = m.db.WithContext(ctx).Table(m.tables.Locations).Where("id = ?", l.ID).
		Updates(map[string]any{
			"name":               l.Name,
			"kind":               l.Kind,
			"parent_location_id": gormstore.StringToNullable(l.ParentLocationID),
			"updated_at":         now,
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

// validateReparent rejects self-parenting and cycles for a proposed
// ParentLocationID change.
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

// DeleteLocation soft-deletes the Location row. Refused with
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

	now := time.Now().UTC().Format(time.RFC3339)
	err = m.db.WithContext(ctx).Table(m.tables.Locations).Where("id = ?", locationID).
		Updates(map[string]any{"deleted": true, "updated_at": now}).Error
	if err != nil {
		return fmt.Errorf("DeleteLocation: %w", err)
	}
	return nil
}

// ListLocations returns all non-deleted Location rows that match the
// filter, id order.
func (m *assetManager) ListLocations(ctx context.Context, filter LocationFilter) ([]models.Location, error) {
	q := m.db.WithContext(ctx).Table(m.tables.Locations).Where("deleted = ?", false)
	if filter.RootOnly {
		q = q.Where("parent_location_id IS NULL")
	} else if filter.ParentLocationID != "" {
		q = q.Where("parent_location_id = ?", filter.ParentLocationID)
	}

	var rows []gormstore.LocationRow
	if err := q.Order("id").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("ListLocations: %w", err)
	}
	out := make([]models.Location, 0, len(rows))
	for _, r := range rows {
		out = append(out, gormstore.LocationFromRow(r))
	}
	return out, nil
}

// ListDescendantLocations returns every Location transitively nested under
// locationID (children, grandchildren, ...), found by walking
// parent_location_id one level at a time. A hand-written recursive CTE
// would work fine against this real column, but mwanachama-backend-actor's
// comparable case (MoveGroup's cycle check) made the same call to keep a Go
// walk rather than write one as part of a storage-engine swap — mirrored
// here for the same reason: out of scope for this change, revisit if a
// deployment's location count ever makes this a real cost.
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

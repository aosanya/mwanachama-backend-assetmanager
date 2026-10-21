package mwanachamaassetmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

type holdSlot struct {
	start time.Time
	end   time.Time
}

func (s holdSlot) overlaps(o holdSlot) bool {
	return s.start.Before(o.end) && o.start.Before(s.end)
}

func holdSlotOf(h models.Hold) (*holdSlot, error) {
	if h.StartsAt == "" && h.EndsAt == "" {
		return nil, nil
	}
	if h.StartsAt == "" || h.EndsAt == "" {
		return nil, fmt.Errorf("%w: StartsAt and EndsAt are given together or not at all", ErrInvalidHold)
	}
	start, err := time.Parse(time.RFC3339, h.StartsAt)
	if err != nil {
		return nil, fmt.Errorf("%w: StartsAt must be RFC 3339: %v", ErrInvalidHold, err)
	}
	end, err := time.Parse(time.RFC3339, h.EndsAt)
	if err != nil {
		return nil, fmt.Errorf("%w: EndsAt must be RFC 3339: %v", ErrInvalidHold, err)
	}
	if !end.After(start) {
		return nil, fmt.Errorf("%w: EndsAt must be after StartsAt", ErrInvalidHold)
	}
	return &holdSlot{start: start, end: end}, nil
}

func (m *assetManager) refuseOverlappingSlot(ctx context.Context, assetID string, slot holdSlot) error {
	live, err := m.ListHolds(ctx, HoldFilter{AssetID: assetID, Status: models.HoldStatusReserved})
	if err != nil {
		return fmt.Errorf("CreateHold: %w", err)
	}
	for _, other := range live {
		otherSlot, err := holdSlotOf(other)
		if err != nil || otherSlot == nil {
			continue
		}
		if slot.overlaps(*otherSlot) {
			return fmt.Errorf("%w: hold %s holds %s to %s", ErrHoldSlotOverlaps, other.ID, other.StartsAt, other.EndsAt)
		}
	}
	return nil
}

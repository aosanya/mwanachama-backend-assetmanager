// hold_watchdog.go — stale-hold detection helper.
//
// This file adds the query helper a watchdog sweeper uses to find Holds
// past their expiry. mwanachama-backend-assetmanager has no scheduler of
// its own — the sweep loop lives in whatever wires this package in, the
// same split mwanachama-backend-taskmanager's workflow_run_watchdog.go
// documents for WorkflowRun timeouts. A sweeper calls
// [assetManager.ListHoldsExpiredAsOf] on an interval, then calls
// [assetManager.ReleaseHold] on each result.
package mwanachamaassetmanager

import (
	"context"
	"fmt"
	"time"
)

// ListHoldsExpiredAsOf returns every [HoldStatusReserved] Hold whose
// ExpiresAt is at or before cutoffRFC3339 (an RFC 3339 timestamp — the
// sweeper's own idea of "now", passed in rather than read here so a test
// can simulate time passing without a real clock).
func (m *assetManager) ListHoldsExpiredAsOf(ctx context.Context, cutoffRFC3339 string) ([]Hold, error) {
	cutoff, err := time.Parse(time.RFC3339, cutoffRFC3339)
	if err != nil {
		return nil, fmt.Errorf("ListHoldsExpiredAsOf: cutoff must be RFC 3339: %w", err)
	}
	open, err := m.ListHolds(ctx, HoldFilter{Status: HoldStatusReserved})
	if err != nil {
		return nil, fmt.Errorf("ListHoldsExpiredAsOf: %w", err)
	}
	var out []Hold
	for _, h := range open {
		expiresAt, err := time.Parse(time.RFC3339, h.ExpiresAt)
		if err != nil {
			continue
		}
		if !expiresAt.After(cutoff) {
			out = append(out, h)
		}
	}
	return out, nil
}

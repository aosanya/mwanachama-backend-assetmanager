package mwanachamaassetmanager

import "context"

func lockAssetForHold(ctx context.Context, tx *store, assetID string) error {
	q := tx.Query(ctx, roleHold)
	if q.Dialector.Name() != "postgres" {
		return nil
	}
	return q.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", tx.Table(roleHold)+"|"+assetID).Error
}

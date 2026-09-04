// Package mwanachamaassetmanager provides general-purpose asset/inventory
// tracking — warehouses, supermarkets, household inventories, livestock. It
// exposes [AssetManager] — the single interface for managing Locations,
// Assets, the append-only Movement ledger, and Hold reservations.
//
// Storage moved from mwanachama-backend-shared/entitygraph onto GORM,
// 2026-09-04, mirroring mwanachama-backend-actor's same-day migration: the
// org has moved away from entitygraph's generic entity/relationship graph
// engine in favor of fixed Go structs mapped to plain relational tables.
// This was a storage-layer swap only — AssetManager's method set is
// unchanged; only NewAssetManager's constructor and everything behind it
// changed shape. See the CLAUDE.md decision log for what else changed along
// the way (single-tenant, Location's child_of edge collapsing into a plain
// parent_location_id column).
//
// Layout:
//   - models/    — domain types (Location, Asset, Movement, Hold);
//     callers use models.Location etc. directly, no re-export in this
//     package
//   - gormstore/ — GORM row structs, row<->domain conversion, migration
//   - doc.go (this file), tables.go — table-name/migrate wrappers
//   - asset_manager.go — AssetManager interface, assetManager struct
//   - location.go      — Location CRUD + parent/descendant tree queries
//   - asset.go         — Asset CRUD
//   - movement.go      — PostMovement, ReverseMovement, GetAssetBalance
//   - hold.go          — Hold CRUD + CommitHold/ReleaseHold lifecycle
//   - hold_watchdog.go — ListHoldsExpiredAsOf, the watchdog query helper
//   - errors.go        — sentinel errors
package mwanachamaassetmanager

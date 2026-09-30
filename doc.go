// Package mwanachamaassetmanager tracks physical things — what exists,
// where it is, who holds it, and what moved. It exposes [AssetManager] over
// four objects: a Location tree, an Asset, an append-only Movement ledger,
// and a Hold reservation.
//
// # It names no domain, and that is the point
//
// Nothing here knows what is being tracked. The same module is expected to
// run a warehouse, a supermarket, a household inventory and a livestock
// register without a line changing, so every type is named for its shape
// rather than its subject.
//
// The rule is one sentence: a word that means something in one domain and
// nothing in another does not belong in this package. "Pallet" fails it —
// a household has belongings, not pallets. "Asset" passes it.
//
// A worked comparison — one store, two domains:
//
//	Location    warehouse > aisle > bin  |  house > kitchen > shelf
//	Asset       a pallet of one SKU      |  a washing machine
//	Movement    a putaway or a pick      |  something moved or used up
//	Hold        stock allocated to an    |  something packed for a trip
//	            order not yet picked     |
//
// domain_agnostic_test.go enforces it over identifiers, stored enum values,
// the blueprint and the route table's addresses.
//
// # Objects are declared, not written
//
// The tables come from JSON, not from Go row structs — see
// developer/documentation/2. design/architecture-spec-driven-modules.md.
//
//   - assetmanager.blueprint.json declares the five objects and every field,
//     embedded and reached through [Blueprint], [LoadSpec] and [ParseSpec].
//     Load a domain spec through those, never through spec.Load, or its
//     roled objects arrive with no fields.
//   - spec/examples/warehouse.assetmanager.json and
//     household.assetmanager.json are the same module under two domains.
//     A domain names the objects and adds its own indexes; it does not
//     re-declare fields.
//   - mwanachama-backend-shared/spec is the loader and migrator,
//     specstore the row codec, dispatch the route builder.
//
// A column is found by a Go field's name, never by its json tag, and every
// declared column is written on every write — a map missing a key means
// "leave it alone" to an update, which would make clearing a field
// impossible. The spec and the types are checked against each other in
// [NewAssetManager], so a declared column with no field fails at
// construction rather than dropping a value on every write.
//
// # Rules are declared too, where they can be
//
// validate.go reads required, an enum's values and matches off the spec;
// patterns.go says what a pattern name means. What stays in Go is what a
// spec cannot state, each with the type it is about: the four postable
// movement shapes, a serialized asset needing a tag, a quantity that must
// be positive, an expiry that must be in the future, and
// models.HoldStatus.CanTransitionTo.
//
// # The route table is declared too
//
// assetmanager.operations.json holds all 22 addresses and maps every
// sentinel to a status; routes/routes.go is the adapter. There are no
// hand-written handlers and no per-aggregate route builders — see
// documentation/2. design/routes.md.
//
// Layout:
//   - models/           — the domain types and the constants Go compares against
//   - assetmanager.blueprint.json, blueprint.go   — the declared objects
//   - assetmanager.operations.json, operations.go — the declared addresses
//   - store.go          — roles and the thin wrappers over specstore
//   - validate.go       — the declared rules; patterns.go — the registry
//   - provision.go      — spec.Migrate; cmd/ddl prints the SQL first
//   - asset_manager.go  — AssetManager interface, assetManager struct
//   - location.go, asset.go, movement.go, hold.go, hold_watchdog.go
//   - code.go           — the stable business code counter
//   - errors.go         — sentinel errors
package mwanachamaassetmanager

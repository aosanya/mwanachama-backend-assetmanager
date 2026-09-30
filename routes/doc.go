// Package routes is mwanachama-backend-assetmanager's HTTP surface.
//
// There are no handlers here. The 22 addresses are declared in
// assetmanager.operations.json and built by
// mwanachama-backend-shared/dispatch; this package is the adapter that
// supplies the sentinel map and the mount. Adding an address is an edit to
// that file, not a new Go function.
//
// AnonymousActions is the gate boundary, and it is empty. This module has
// no auth model and no modelled actor — performed_by and placed_by are
// plain caller-supplied strings it never interprets — so every address
// arrives gated and whatever mounts these routes names what it wants to
// open and wraps its own capability check around the rest.
//
// See documentation/2. design/routes.md for the table, the error map and
// the wire changes the 2026-09-30 conversion accepted.
package routes

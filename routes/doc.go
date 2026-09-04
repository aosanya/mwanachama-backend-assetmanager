// Package routes is mwanachama-backend-assetmanager's own HTTP surface:
// decode a request, call one [mwanachamaassetmanager.AssetManager] method,
// encode the response — the same shape location.go/asset.go/movement.go/
// hold.go's Go callers already get, just reachable from an HTTP mux.
//
// Added 2026-09-04, superseding requirements.md decision #10 ("no
// HTTP/gRPC layer of its own") at the user's explicit request, after the
// entitygraph→GORM storage swap (see CLAUDE.md and
// documentation/3. implementation/todo_done.md's A11). Mirrors
// mwanachama-backend-actor/routes' shape and conventions on purpose: [Route]/
// [Route.Pattern], one XRoutes function per aggregate concatenated by
// [Routes], writeJSON/writeErr/readJSON, and decode-call-encode handlers
// with no caller-identity gate of their own — a route built from this
// package still needs an auth/capability check wrapped around it by
// whatever mounts it, the same way actor's routes package leaves that to
// its own mounting process.
//
// Unlike actor, there is no per-type ResourceNames override: this package
// has no gateway-style "the org calls this something else" precedent (no
// DSN-1699-style renaming decision on record for Asset/Location/Movement/
// Hold), so [LocationRoutes]/[AssetRoutes]/[MovementRoutes]/[HoldRoutes]
// address their resources under the package's own fixed nouns —
// "locations", "assets", "movements", "holds".
//
// performed_by/placed_by keep flowing as plain, caller-supplied,
// unauthenticated request-body strings — decision #11 is unchanged by
// adding this package: there is still no modelled Actor type and no auth
// of any kind here, only a place to decode and validate the same string
// the Go API already required.
package routes

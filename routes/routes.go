package routes

import (
	"net/http"

	"github.com/aosanya/mwanachama-backend-shared/httpwire"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

// Route is httpwire.Route — one address this package answers, relative to
// wherever the mounting process prefixes it (e.g. "/v1/assets"). Kept as a
// local alias so this package's own exported signatures don't force every
// caller to import httpwire just to spell the type.
type Route = httpwire.Route

// LocationRoutes is the six Location operations CreateLocation/GetLocation/
// UpdateLocation/DeleteLocation/ListLocations/ListDescendantLocations,
// addressed under "/locations".
func LocationRoutes(am mwanachamaassetmanager.AssetManager) []Route {
	base := "/locations"
	return []Route{
		{Method: http.MethodPost, Path: base, Handler: CreateLocation(am)},
		{Method: http.MethodGet, Path: base, Handler: ListLocations(am)},
		{Method: http.MethodGet, Path: base + "/{locationID}", Handler: GetLocation(am)},
		{Method: http.MethodPatch, Path: base + "/{locationID}", Handler: UpdateLocation(am)},
		{Method: http.MethodDelete, Path: base + "/{locationID}", Handler: DeleteLocation(am)},
		{Method: http.MethodGet, Path: base + "/{locationID}/descendants", Handler: ListDescendantLocations(am)},
	}
}

// AssetRoutes is the five plain Asset operations CreateAsset/GetAsset/
// UpdateAsset/DeleteAsset/ListAssets, plus GetAssetBalance nested under the
// asset it reads, addressed under "/assets".
func AssetRoutes(am mwanachamaassetmanager.AssetManager) []Route {
	base := "/assets"
	return []Route{
		{Method: http.MethodPost, Path: base, Handler: CreateAsset(am)},
		{Method: http.MethodGet, Path: base, Handler: ListAssets(am)},
		{Method: http.MethodGet, Path: base + "/{assetID}", Handler: GetAsset(am)},
		{Method: http.MethodPatch, Path: base + "/{assetID}", Handler: UpdateAsset(am)},
		{Method: http.MethodDelete, Path: base + "/{assetID}", Handler: DeleteAsset(am)},
		{Method: http.MethodGet, Path: base + "/{assetID}/balance", Handler: GetAssetBalance(am)},
	}
}

// MovementRoutes is PostMovement/GetMovement/ListMovements/ReverseMovement,
// addressed under "/movements".
func MovementRoutes(am mwanachamaassetmanager.AssetManager) []Route {
	base := "/movements"
	return []Route{
		{Method: http.MethodPost, Path: base, Handler: PostMovement(am)},
		{Method: http.MethodGet, Path: base, Handler: ListMovements(am)},
		{Method: http.MethodGet, Path: base + "/{movementID}", Handler: GetMovement(am)},
		{Method: http.MethodPost, Path: base + "/{movementID}/reverse", Handler: ReverseMovement(am)},
	}
}

// HoldRoutes is CreateHold/GetHold/CommitHold/ReleaseHold/ListHolds, plus
// ListHoldsExpiredAsOf's watchdog query, addressed under "/holds".
func HoldRoutes(am mwanachamaassetmanager.AssetManager) []Route {
	base := "/holds"
	return []Route{
		{Method: http.MethodPost, Path: base, Handler: CreateHold(am)},
		{Method: http.MethodGet, Path: base, Handler: ListHolds(am)},
		{Method: http.MethodGet, Path: base + "/expired", Handler: ListHoldsExpiredAsOf(am)},
		{Method: http.MethodGet, Path: base + "/{holdID}", Handler: GetHold(am)},
		{Method: http.MethodPost, Path: base + "/{holdID}/commit", Handler: CommitHold(am)},
		{Method: http.MethodPost, Path: base + "/{holdID}/release", Handler: ReleaseHold(am)},
	}
}

// Routes is every address this package answers today: LocationRoutes,
// AssetRoutes, MovementRoutes and HoldRoutes concatenated. A mounting
// process that wants all of it in one loop uses this; one that wants to
// wrap one aggregate's writes differently from another's calls the four
// functions above separately instead.
func Routes(am mwanachamaassetmanager.AssetManager) []Route {
	out := LocationRoutes(am)
	out = append(out, AssetRoutes(am)...)
	out = append(out, MovementRoutes(am)...)
	out = append(out, HoldRoutes(am)...)
	return out
}

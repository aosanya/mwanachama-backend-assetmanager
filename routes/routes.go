package routes

import (
	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/httpwire"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

type Route = httpwire.Route

type Mount = dispatch.Mount

var sentinels = map[string]error{
	"ErrLocationNotFound":            mwanachamaassetmanager.ErrLocationNotFound,
	"ErrLocationNotEmpty":            mwanachamaassetmanager.ErrLocationNotEmpty,
	"ErrInvalidLocation":             mwanachamaassetmanager.ErrInvalidLocation,
	"ErrAssetNotFound":               mwanachamaassetmanager.ErrAssetNotFound,
	"ErrInvalidAsset":                mwanachamaassetmanager.ErrInvalidAsset,
	"ErrAssetSerialTagExists":        mwanachamaassetmanager.ErrAssetSerialTagExists,
	"ErrAssetTrackingModeImmutable":  mwanachamaassetmanager.ErrAssetTrackingModeImmutable,
	"ErrAssetHasOpenHolds":           mwanachamaassetmanager.ErrAssetHasOpenHolds,
	"ErrMovementNotFound":            mwanachamaassetmanager.ErrMovementNotFound,
	"ErrInvalidMovement":             mwanachamaassetmanager.ErrInvalidMovement,
	"ErrMovementAlreadyReversed":     mwanachamaassetmanager.ErrMovementAlreadyReversed,
	"ErrHoldNotFound":                mwanachamaassetmanager.ErrHoldNotFound,
	"ErrInvalidHold":                 mwanachamaassetmanager.ErrInvalidHold,
	"ErrInvalidHoldStatusTransition": mwanachamaassetmanager.ErrInvalidHoldStatusTransition,
	"ErrInsufficientAvailable":       mwanachamaassetmanager.ErrInsufficientAvailable,
}

var AnonymousActions = []string{}

var table = dispatch.NewTable(
	mwanachamaassetmanager.Operations(), sentinels, AnonymousActions...)

func Table() *dispatch.Table { return table }

func Build(am mwanachamaassetmanager.AssetManager, m Mount) ([]Route, error) {
	return table.Build(am, m)
}

func Routes(am mwanachamaassetmanager.AssetManager) []Route {
	return table.Routes(am, Mount{})
}

func RoutesFor(am mwanachamaassetmanager.AssetManager, m Mount) []Route {
	return table.Routes(am, m)
}

func Split(am mwanachamaassetmanager.AssetManager, m Mount) dispatch.Split {
	return table.Split(am, m)
}

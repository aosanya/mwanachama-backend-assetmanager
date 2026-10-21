package routes

import (
	"github.com/aosanya/mwanachama-backend-shared/dispatch"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

type Tool = dispatch.Tool

func BuildTools(am mwanachamaassetmanager.AssetManager, m Mount) ([]Tool, error) {
	s, err := table.Spec()
	if err != nil {
		return nil, err
	}
	return dispatch.Tools(s, dispatch.Deps{
		Manager: am, Errors: sentinels, Authorize: m.Authorize, Caller: m.Caller,
	})
}

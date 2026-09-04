package routes_test

import (
	"testing"

	"github.com/aosanya/mwanachama-backend-assetmanager/routes"
)

func patterns(rts []routes.Route, prefix string) []string {
	out := make([]string, len(rts))
	for i, rt := range rts {
		out[i] = rt.Pattern(prefix)
	}
	return out
}

func assertPatterns(t *testing.T, got []routes.Route, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d routes, want %d: %v", len(got), len(want), patterns(got, ""))
	}
	for i, p := range patterns(got, "") {
		if p != want[i] {
			t.Fatalf("route %d: got %q, want %q", i, p, want[i])
		}
	}
}

func TestLocationRoutes(t *testing.T) {
	am := newTestManager(t)
	rts := routes.LocationRoutes(am)
	assertPatterns(t, rts, []string{
		"POST /locations",
		"GET /locations",
		"GET /locations/{locationID}",
		"PATCH /locations/{locationID}",
		"DELETE /locations/{locationID}",
		"GET /locations/{locationID}/descendants",
	})
}

func TestAssetRoutes(t *testing.T) {
	am := newTestManager(t)
	rts := routes.AssetRoutes(am)
	assertPatterns(t, rts, []string{
		"POST /assets",
		"GET /assets",
		"GET /assets/{assetID}",
		"PATCH /assets/{assetID}",
		"DELETE /assets/{assetID}",
		"GET /assets/{assetID}/balance",
	})
}

func TestMovementRoutes(t *testing.T) {
	am := newTestManager(t)
	rts := routes.MovementRoutes(am)
	assertPatterns(t, rts, []string{
		"POST /movements",
		"GET /movements",
		"GET /movements/{movementID}",
		"POST /movements/{movementID}/reverse",
	})
}

func TestHoldRoutes(t *testing.T) {
	am := newTestManager(t)
	rts := routes.HoldRoutes(am)
	assertPatterns(t, rts, []string{
		"POST /holds",
		"GET /holds",
		"GET /holds/expired",
		"GET /holds/{holdID}",
		"POST /holds/{holdID}/commit",
		"POST /holds/{holdID}/release",
	})
}

func TestRoutes_ConcatenatesAllFour(t *testing.T) {
	am := newTestManager(t)
	all := routes.Routes(am)

	want := 6 + 6 + 4 + 6 // LocationRoutes + AssetRoutes + MovementRoutes + HoldRoutes
	if len(all) != want {
		t.Fatalf("got %d routes, want %d: %v", len(all), want, patterns(all, ""))
	}
}

func TestRoute_PatternWithPrefix(t *testing.T) {
	am := newTestManager(t)
	rts := routes.LocationRoutes(am)

	if got := rts[0].Pattern("/v1/assetmanager"); got != "POST /v1/assetmanager/locations" {
		t.Fatalf("got %q", got)
	}
}

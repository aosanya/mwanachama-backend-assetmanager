package routes_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-assetmanager/routes"
)

var addresses = []string{
	"DELETE /assets/{assetID}",
	"DELETE /locations/{locationID}",
	"GET /assets",
	"GET /assets/{assetID}",
	"GET /assets/{assetID}/balance",
	"GET /holds",
	"GET /holds/expired",
	"GET /holds/{holdID}",
	"GET /locations",
	"GET /locations/{locationID}",
	"GET /locations/{locationID}/descendants",
	"GET /movements",
	"GET /movements/{movementID}",
	"PATCH /assets/{assetID}",
	"PATCH /locations/{locationID}",
	"POST /assets",
	"POST /holds",
	"POST /holds/{holdID}/commit",
	"POST /holds/{holdID}/release",
	"POST /locations",
	"POST /movements",
	"POST /movements/{movementID}/reverse",
}

func patterns(rts []routes.Route, prefix string) []string {
	out := make([]string, len(rts))
	for i, rt := range rts {
		out[i] = rt.Pattern(prefix)
	}
	sort.Strings(out)
	return out
}

func TestTheDeclaredTableAnswersExactlyTheAddressesItAlwaysDid(t *testing.T) {
	got := patterns(routes.Routes(newTestManager(t)), "")
	if len(got) != len(addresses) {
		t.Fatalf("got %d addresses, want %d:\n%v", len(got), len(addresses), got)
	}
	for i := range addresses {
		if got[i] != addresses[i] {
			t.Errorf("address %d: got %q, want %q", i, got[i], addresses[i])
		}
	}
}

func TestEveryAddressCarriesItsAction(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range routes.Routes(newTestManager(t)) {
		if r.Action == "" {
			t.Errorf("%s carries no action, so no mount can name it", r.Pattern(""))
			continue
		}
		if seen[r.Action] {
			t.Errorf("%q is declared twice", r.Action)
		}
		seen[r.Action] = true
	}
}

func TestAPrefixReachesEveryAddress(t *testing.T) {
	for _, p := range patterns(routes.Routes(newTestManager(t)), "/v1/assetmanager") {
		if !strings.Contains(p, "/v1/assetmanager/") {
			t.Errorf("%q does not carry the mount's prefix", p)
		}
	}
}

func TestEveryAnonymousActionNamesADeclaredOperation(t *testing.T) {
	unknown, err := routes.Table().UnknownAnonymousActions()
	if err != nil {
		t.Fatalf("UnknownAnonymousActions: %v", err)
	}
	if len(unknown) > 0 {
		t.Errorf("the allowlist names %v, which no operation declares", unknown)
	}
}

func TestNothingIsPublicUntilSomethingSaysSo(t *testing.T) {
	split := routes.Split(newTestManager(t), routes.Mount{})
	if len(split.Anonymous) != 0 {
		t.Errorf("%d routes arrive ungated, and this module declares no anonymous action",
			len(split.Anonymous))
	}
	if len(split.Gated) != len(addresses) {
		t.Errorf("gated %d of %d addresses", len(split.Gated), len(addresses))
	}
}

func TestEverySuppliedSentinelHasADeclaredStatus(t *testing.T) {
	missing, err := routes.Table().UnmappedSentinels()
	if err != nil {
		t.Fatalf("UnmappedSentinels: %v", err)
	}
	if len(missing) > 0 {
		t.Errorf("%v have no declared status, so each would be redacted to a 500", missing)
	}
}

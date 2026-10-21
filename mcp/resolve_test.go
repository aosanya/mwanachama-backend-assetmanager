package mcp_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/gormtest"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	assetmcp "github.com/aosanya/mwanachama-backend-assetmanager/mcp"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
	"github.com/aosanya/mwanachama-backend-assetmanager/routes"
)

func managerFor(t *testing.T) mwanachamaassetmanager.AssetManager {
	t.Helper()
	db := gormtest.OpenSQLiteDB(t)
	s, err := mwanachamaassetmanager.LoadSpec(filepath.Join("..", "spec", "examples", "warehouse.assetmanager.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := mwanachamaassetmanager.Provision(db, s); err != nil {
		t.Fatal(err)
	}
	m, err := mwanachamaassetmanager.NewAssetManager(db, s)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func resolvedSession(t *testing.T, held map[string]mwanachamaassetmanager.AssetManager) *mcp.ClientSession {
	t.Helper()
	server := mcp.NewServer(&mcp.Implementation{Name: "t", Version: "1"}, nil)
	assetmcp.RegisterResolvedTools(server, func(_ context.Context, id string) (mwanachamaassetmanager.AssetManager, error) {
		if m, ok := held[id]; ok {
			return m, nil
		}
		return nil, errors.New("not found")
	}, routes.Mount{Caller: func(context.Context) string { return "actor-1" }})
	ct, st := mcp.NewInMemoryTransports()
	if _, err := server.Connect(context.Background(), st, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "c", Version: "1"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

func toolNamed(t *testing.T, cs *mcp.ClientSession, suffix string) string {
	t.Helper()
	list, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range list.Tools {
		if len(tool.Name) >= len(suffix) && tool.Name[len(tool.Name)-len(suffix):] == suffix {
			return tool.Name
		}
	}
	t.Fatalf("no tool ending %q", suffix)
	return ""
}

func TestAResolvedToolReachesTheNamedInstanceOnly(t *testing.T) {
	a, b := managerFor(t), managerFor(t)
	if _, err := a.CreateLocation(context.Background(), models.Location{Name: "Held by alpha", Kind: "site"}); err != nil {
		t.Fatal(err)
	}
	cs := resolvedSession(t, map[string]mwanachamaassetmanager.AssetManager{"alpha": a, "beta": b})
	list := toolNamed(t, cs, "location_list")
	for instance, want := range map[string]int{"alpha": 1, "beta": 0} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: list, Arguments: map[string]any{"instance_id": instance}})
		if err != nil || res.IsError {
			t.Fatalf("%s: %v %+v", instance, err, res)
		}
		items, _ := res.StructuredContent.(map[string]any)["items"].([]any)
		if len(items) != want {
			t.Fatalf("%s: %d locations, want %d", instance, len(items), want)
		}
	}
}

func TestAResolvedToolRefusesAnUnknownOrMissingInstance(t *testing.T) {
	cs := resolvedSession(t, map[string]mwanachamaassetmanager.AssetManager{"alpha": managerFor(t)})
	list := toolNamed(t, cs, "location_list")
	for _, args := range []map[string]any{{"instance_id": "nobody"}, {}} {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: list, Arguments: args})
		if err == nil && !res.IsError {
			t.Fatalf("%v: answered", args)
		}
	}
}

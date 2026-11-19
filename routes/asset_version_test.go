package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

const versionDeclaration = `{
  "module": "assetmanager",
  "domain": "wakala",
  "instance": "versionroutes",
  "mount": "assets",
  "objects": [
    {"role": "location", "name": "location", "table": "locations", "description": "d"},
    {"role": "asset", "name": "asset", "table": "assets", "description": "d"},
    {"role": "movement", "name": "movement", "table": "movements", "description": "d"},
    {"role": "hold", "name": "hold", "table": "holds", "description": "d"},
    {"role": "code_sequence", "name": "code_sequence", "table": "code_sequences", "description": "d"}
  ]
}`

const longAgo = "2001-01-01T00:00:00Z"

func versionedManager(t *testing.T) (mwanachamaassetmanager.AssetManager, models.Asset) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "versions.db") + "?_pragma=busy_timeout(10000)"
	db, err := gorm.Open(sqlite.Open(file), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	s, err := mwanachamaassetmanager.ParseSpec([]byte(versionDeclaration))
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
	asset, err := m.CreateAsset(context.Background(), models.Asset{Name: "eggs", TrackingMode: models.AssetTrackingModeFungible})
	if err != nil {
		t.Fatal(err)
	}
	return m, asset
}

func updateTool(t *testing.T, m mwanachamaassetmanager.AssetManager) Tool {
	t.Helper()
	tools, err := BuildTools(m, theSession)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools {
		if tool.Name == "assetmanager_asset_update" {
			return tool
		}
	}
	t.Fatal("no assetmanager_asset_update tool")
	return Tool{}
}

func updateArgs(id, name, version string) json.RawMessage {
	args := map[string]any{"assetID": id, "name": name}
	if version != "" {
		args["version"] = version
	}
	raw, _ := json.Marshal(args)
	return raw
}

func TestTheAssetUpdateToolDeclaresAnOptionalVersion(t *testing.T) {
	m, _ := versionedManager(t)
	var schema struct {
		Properties           map[string]json.RawMessage `json:"properties"`
		Required             []string                   `json:"required"`
		AdditionalProperties bool                       `json:"additionalProperties"`
	}
	if err := json.Unmarshal(updateTool(t, m).InputSchema, &schema); err != nil {
		t.Fatal(err)
	}
	if _, ok := schema.Properties["version"]; !ok {
		t.Fatalf("schema has no version: %v", schema.Properties)
	}
	for _, name := range schema.Required {
		if name == "version" {
			t.Fatal("version must stay optional")
		}
	}
	if schema.AdditionalProperties {
		t.Fatal("the schema must keep refusing unknown arguments")
	}
}

func TestTheAssetUpdateToolSavesOnTheCurrentVersionAndOnNoVersion(t *testing.T) {
	m, asset := versionedManager(t)
	tool := updateTool(t, m)
	if _, err := tool.Invoke(context.Background(), updateArgs(asset.ID, "duck eggs", asset.UpdatedAt)); err != nil {
		t.Fatalf("current version: %v", err)
	}
	if _, err := tool.Invoke(context.Background(), updateArgs(asset.ID, "goose eggs", "")); err != nil {
		t.Fatalf("no version: %v", err)
	}
}

func TestTheAssetUpdateToolRefusesAnOldVersionNamingTheField(t *testing.T) {
	m, asset := versionedManager(t)
	_, err := updateTool(t, m).Invoke(context.Background(), updateArgs(asset.ID, "duck eggs", longAgo))
	if err == nil || err.Error() != "name changed since you read it" {
		t.Fatalf("Invoke = %v", err)
	}
}

func TestTheAssetUpdateRouteAnswersConflictOnAnOldVersion(t *testing.T) {
	m, asset := versionedManager(t)
	list := RoutesFor(m, theSession)
	path := strings.Replace(routeOf(t, list, "assetmanager.asset.update").Path, "{assetID}", asset.ID, 1)
	stale := serve(list, http.MethodPatch, path, `{"name":"duck eggs","version":"`+longAgo+`"}`)
	if stale.Code != http.StatusConflict || !strings.Contains(stale.Body.String(), "name changed since you read it") {
		t.Fatalf("stale save answered %d %s", stale.Code, stale.Body.String())
	}
	fresh := serve(list, http.MethodPatch, path, `{"name":"duck eggs","version":"`+asset.UpdatedAt+`"}`)
	if fresh.Code != http.StatusOK {
		t.Fatalf("current save answered %d %s", fresh.Code, fresh.Body.String())
	}
}

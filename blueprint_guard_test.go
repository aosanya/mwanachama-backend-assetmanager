package mwanachamaassetmanager_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/dispatch"
	"github.com/aosanya/mwanachama-backend-shared/gormtest"
	"github.com/aosanya/mwanachama-backend-shared/spec"

	assetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

func routeTable(t *testing.T) (*dispatch.Spec, error) {
	t.Helper()
	return dispatch.Parse(assetmanager.Operations())
}

func TestEveryExampleFitsTheTypes(t *testing.T) {
	for _, path := range examplePaths(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			s := loadExample(t, path)
			db := gormtest.OpenSQLiteDB(t)
			if err := assetmanager.Provision(db, s); err != nil {
				t.Fatalf("provision: %v", err)
			}
			if _, err := assetmanager.NewAssetManager(db, s); err != nil {
				t.Fatalf("%s: the spec and the Go types disagree: %v", path, err)
			}
		})
	}
}

func TestRequiredFieldsHaveNoDefault(t *testing.T) {
	b, err := assetmanager.Blueprint()
	if err != nil {
		t.Fatalf("blueprint: %v", err)
	}
	for _, o := range b.Objects {
		for _, f := range o.Fields {
			if (f.Required || f.Primary) && f.Default != "" {
				t.Errorf("%s.%s is required and defaults to %q — a default is exactly what lets an omitted value pass unnoticed",
					o.Role, f.Name, f.Default)
			}
		}
	}

	for _, path := range examplePaths(t) {
		s := loadExample(t, path)
		for _, stmt := range s.DDL("postgres") {
			for _, line := range strings.Split(stmt, "\n") {
				column := strings.TrimSpace(line)
				if !strings.Contains(column, "not null") || !strings.Contains(column, "default") {
					continue
				}
				t.Errorf("%s emits a column that is both not null and defaulted, and a default is exactly what would let an omitted value pass: %s",
					path, column)
			}
		}
	}
}

func TestVocabularyMatchesTheBlueprint(t *testing.T) {
	b, err := assetmanager.Blueprint()
	if err != nil {
		t.Fatalf("blueprint: %v", err)
	}

	goConstants := map[string][]string{
		"asset.tracking_mode": {
			string(models.AssetTrackingModeSerialized),
			string(models.AssetTrackingModeFungible),
		},
		"movement.kind": {
			string(models.MovementKindArrived),
			string(models.MovementKindTransferred),
			string(models.MovementKindDeparted),
			string(models.MovementKindAdjusted),
			string(models.MovementKindReversed),
		},
		"hold.status": {
			string(models.HoldStatusReserved),
			string(models.HoldStatusCommitted),
			string(models.HoldStatusReleased),
			string(models.HoldStatusPartiallyCommitted),
		},
	}

	declared := map[string][]string{}
	for _, o := range b.Objects {
		for _, f := range o.Fields {
			if f.Type == spec.TypeEnum {
				declared[o.Role+"."+f.Name] = f.Values
			}
		}
	}

	for key, values := range declared {
		if _, ok := goConstants[key]; !ok {
			t.Errorf("the blueprint declares the enum %q, and Go compares against no constants for it", key)
			continue
		}
		for _, v := range values {
			if !holds(goConstants[key], v) {
				t.Errorf("the blueprint declares %s = %q, which no Go constant carries", key, v)
			}
		}
	}
	for key, values := range goConstants {
		if _, ok := declared[key]; !ok {
			t.Errorf("Go carries constants for %q, which the blueprint declares as no enum", key)
			continue
		}
		for _, v := range values {
			if !holds(declared[key], v) {
				t.Errorf("Go carries %s = %q, which the blueprint does not declare", key, v)
			}
		}
	}
}

func holds(values []string, s string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}

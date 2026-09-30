package mwanachamaassetmanager_test

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/aosanya/mwanachama-backend-shared/gormtest"
	"github.com/aosanya/mwanachama-backend-shared/spec"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

func examplePaths(t *testing.T) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("spec", "examples", "*.assetmanager.json"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(paths) < 2 {
		t.Fatalf("found %d example specs, want at least two so domain-neutrality is actually exercised", len(paths))
	}
	sort.Strings(paths)
	return paths
}

func loadExample(t *testing.T, path string) *spec.Spec {
	t.Helper()
	s, err := mwanachamaassetmanager.LoadSpec(path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return s
}

func TestEveryShippedDomainLoadsThroughTheBlueprint(t *testing.T) {
	for _, path := range examplePaths(t) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			s := loadExample(t, path)
			if err := s.RequireRoles("location", "asset", "movement", "hold", "code_sequence"); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			for _, role := range []string{"location", "asset", "movement", "hold"} {
				o, _ := s.ByRole(role)
				if len(o.Fields) == 0 {
					t.Errorf("%s fills %q with no fields, so it was not read through the blueprint", path, role)
				}
			}
		})
	}
}

func TestADomainNamesTheObjectsWithoutRedeclaringThem(t *testing.T) {
	warehouse := loadExample(t, filepath.Join("spec", "examples", "warehouse.assetmanager.json"))
	household := loadExample(t, filepath.Join("spec", "examples", "household.assetmanager.json"))

	w, _ := warehouse.ByRole("asset")
	h, _ := household.ByRole("asset")
	if w.Name == h.Name {
		t.Fatalf("both domains call the asset %q, so nothing is being exercised", w.Name)
	}
	if len(w.Fields) != len(h.Fields) {
		t.Errorf("the two domains see %d and %d fields for one role", len(w.Fields), len(h.Fields))
	}

	room, _ := household.ByRole("location")
	for _, f := range room.Fields {
		if f.Name == "kind" && f.Default != "room" {
			t.Errorf("the household's default for kind = %q, want it taken from the domain", f.Default)
		}
	}
	bin, _ := warehouse.ByRole("location")
	for _, f := range bin.Fields {
		if f.Name == "kind" && f.Default != "" {
			t.Errorf("the warehouse set no default for kind, got %q", f.Default)
		}
	}
}

func TestADomainsOwnObjectFillsNoRole(t *testing.T) {
	household := loadExample(t, filepath.Join("spec", "examples", "household.assetmanager.json"))
	warranty, ok := household.Object("warranty")
	if !ok {
		t.Fatal("the household declares no object of its own, so that path is untested")
	}
	if warranty.Role != "" {
		t.Errorf("warranty fills the role %q, and a domain's own object fills none", warranty.Role)
	}
}

func TestTwoDomainsCoexist(t *testing.T) {
	db := gormtest.OpenSQLiteDB(t)
	tables := map[string]bool{}
	for _, path := range examplePaths(t) {
		s := loadExample(t, path)
		if err := spec.Migrate(db, s); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, o := range s.Objects {
			name := s.TableFor(o)
			if tables[name] {
				t.Fatalf("%s: two domains landed on the table %q", path, name)
			}
			tables[name] = true
		}
	}
	if len(tables) != 11 {
		t.Fatalf("migrated %d tables, want 11 (five roles twice, plus the household's own warranty)", len(tables))
	}
}

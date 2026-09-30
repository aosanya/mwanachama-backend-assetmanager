package mwanachamaassetmanager_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	assetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
	"github.com/aosanya/mwanachama-backend-assetmanager/models"
)

var domainWords = []string{
	"warehouse", "pallet", "aisle", "bin",
	"supermarket", "grocery", "sku", "checkout", "till",
	"household", "pantry", "cupboard", "room", "shelf",
	"livestock", "pasture", "paddock", "cattle", "cow", "herd", "eartag",
	"merchandise", "handout", "rally", "campaign", "chapter", "tshirt",
}

func TestNoDomainWordsInIdentifiers(t *testing.T) {
	for _, dir := range []string{"models", "routes", "."} {
		fset := token.NewFileSet()
		pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", dir, err)
		}
		seen := map[string]bool{}
		for _, pkg := range pkgs {
			for path, file := range pkg.Files {
				ast.Inspect(file, func(n ast.Node) bool {
					id, ok := n.(*ast.Ident)
					if !ok {
						return true
					}
					w, bad := carriesDomainWord(id.Name)
					if !bad {
						return true
					}
					key := filepath.Base(path) + ":" + id.Name
					if seen[key] {
						return true
					}
					seen[key] = true
					t.Errorf("%s: identifier %q carries the domain word %q — a warehouse, a household and a livestock keeper all use this module, so it belongs in a domain's own spec",
						filepath.Base(path), id.Name, w)
					return true
				})
			}
		}
	}
}

func TestNoDomainWordsInStoredVocabularies(t *testing.T) {
	for _, v := range []string{
		string(models.AssetTrackingModeSerialized), string(models.AssetTrackingModeFungible),
		string(models.MovementKindArrived), string(models.MovementKindTransferred),
		string(models.MovementKindDeparted), string(models.MovementKindAdjusted),
		string(models.MovementKindReversed),
		string(models.HoldStatusReserved), string(models.HoldStatusCommitted),
		string(models.HoldStatusReleased), string(models.HoldStatusPartiallyCommitted),
	} {
		if w, bad := carriesDomainWord(v); bad {
			t.Errorf("stored value %q carries the domain word %q — a value outlives a rename", v, w)
		}
	}
}

func TestNoDomainWordsInTheBlueprintsRolesFieldsAndStoredValues(t *testing.T) {
	b, err := assetmanager.Blueprint()
	if err != nil {
		t.Fatalf("blueprint: %v", err)
	}
	report := func(kind, name string) {
		if w, bad := carriesDomainWord(name); bad {
			t.Errorf("blueprint %s %q carries the domain word %q — it belongs in a domain's own spec", kind, name, w)
		}
	}
	for _, o := range b.Objects {
		report("role", o.Role)
		if o.Name != "" || o.Table != "" {
			t.Errorf("blueprint role %q names a table, which is the domain's to choose", o.Role)
		}
		for _, f := range o.Fields {
			report("field", o.Role+"."+f.Name)
			for _, v := range f.Values {
				report("stored value", o.Role+"."+f.Name+"="+v)
			}
		}
		for _, idx := range o.Indexes {
			report("index", o.Role+"."+idx.Name)
		}
	}
}

func TestNoDomainWordsInTheDeclaredRouteTable(t *testing.T) {
	s, err := routeTable(t)
	if err != nil {
		t.Fatalf("operations: %v", err)
	}
	for name, op := range s.Operations {
		for _, candidate := range []string{name, op.Action, op.Path, op.Call} {
			if w, bad := carriesDomainWord(candidate); bad {
				t.Errorf("operation %q carries the domain word %q in %q — an address outlives a rename the way a stored value does",
					name, w, candidate)
			}
		}
	}
}

func carriesDomainWord(s string) (string, bool) {
	low := strings.ToLower(s)
	for _, w := range domainWords {
		if strings.Contains(low, w) {
			return w, true
		}
	}
	return "", false
}

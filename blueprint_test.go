package mwanachamaassetmanager_test

import (
	"sort"
	"testing"

	mwanachamaassetmanager "github.com/aosanya/mwanachama-backend-assetmanager"
)

func TestTheBlueprintDeclaresEveryRoleTheModuleOperatesOn(t *testing.T) {
	b, err := mwanachamaassetmanager.Blueprint()
	if err != nil {
		t.Fatalf("blueprint: %v", err)
	}
	got := b.Roles()
	sort.Strings(got)
	want := []string{"asset", "code_sequence", "hold", "location", "movement"}
	if len(got) != len(want) {
		t.Fatalf("roles = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("roles = %v, want %v", got, want)
		}
	}
}

func TestEveryDeclaredFieldSaysWhatItIsFor(t *testing.T) {
	b, err := mwanachamaassetmanager.Blueprint()
	if err != nil {
		t.Fatalf("blueprint: %v", err)
	}
	for _, o := range b.Objects {
		if o.Description == "" {
			t.Errorf("object %q has no description", o.Role)
		}
		for _, f := range o.Fields {
			if f.Description == "" {
				t.Errorf("%s.%s has no description", o.Role, f.Name)
			}
		}
	}
}

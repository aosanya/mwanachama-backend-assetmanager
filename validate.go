package mwanachamaassetmanager

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const (
	RoleLocation = roleLocation
	RoleAsset    = roleAsset
	RoleMovement = roleMovement
	RoleHold     = roleHold
)

var storeOwned = map[string]map[string]bool{
	roleLocation: {"id": true, "code": true, "created_at": true, "updated_at": true, "deleted": true},
	roleAsset: {"id": true, "code": true, "created_at": true, "updated_at": true, "deleted": true,
		"location_id": true},
	roleMovement: {"id": true, "created_at": true, "occurred_at": true, "reverses_movement_id": true},
	roleHold: {"id": true, "status": true, "committed_quantity": true,
		"created_at": true, "updated_at": true},
}

func Check(s *spec.Spec, role string, v any) error {
	o, ok := s.ByRole(role)
	if !ok {
		return fmt.Errorf("check: this domain fills no object for the role %q", role)
	}
	return check(o, storeOwned[role], v)
}

func check(o spec.Object, owned map[string]bool, v any) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return fmt.Errorf("check %s: want a struct, got %T", o.Name, v)
	}

	byColumn := map[string]reflect.Value{}
	rt := rv.Type()
	for i := 0; i < rt.NumField(); i++ {
		if rt.Field(i).PkgPath != "" {
			continue
		}
		byColumn[columnName(rt.Field(i).Name)] = rv.Field(i)
	}

	for _, f := range o.Fields {
		if owned[f.Name] {
			continue
		}
		fv, ok := byColumn[f.Name]
		if !ok || fv.Kind() != reflect.String {
			continue
		}
		if err := checkValue(o, f, fv.String()); err != nil {
			return err
		}
	}
	return nil
}

func checkValue(o spec.Object, f spec.Field, value string) error {
	if f.Required && strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", f.Name)
	}
	if f.Type == spec.TypeEnum && !declares(f.Values, value) {
		return fmt.Errorf("%s is %q, which is not one of %s",
			f.Name, value, strings.Join(f.Values, ", "))
	}
	if f.Matches != "" && value != "" {
		holds, known := patterns[f.Matches]
		if !known {
			return fmt.Errorf("%s names the pattern %q, which this module does not supply",
				f.Name, f.Matches)
		}
		if !holds(value) {
			return fmt.Errorf("%s does not match %s", f.Name, f.Matches)
		}
	}
	return nil
}

func declares(values []string, s string) bool {
	for _, v := range values {
		if v == s {
			return true
		}
	}
	return false
}

func (m *assetManager) check(role string, v any, sentinel error) error {
	if err := check(m.store.Object(role), storeOwned[role], v); err != nil {
		return fmt.Errorf("%w: %v", sentinel, err)
	}
	return nil
}

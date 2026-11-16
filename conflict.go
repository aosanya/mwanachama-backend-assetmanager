package mwanachamaassetmanager

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-shared/gormutil"
	"github.com/aosanya/mwanachama-backend-shared/spec"
)

const serialTagIndex = "serial_tag"

var busyMessages = []string{
	"database table is locked",
	"database is locked",
	"database is deadlocked",
	"SQLITE_BUSY",
	"could not serialize access",
	"deadlock detected",
}

func isBusy(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	for _, busy := range busyMessages {
		if strings.Contains(message, busy) {
			return true
		}
	}
	return false
}

func classifyConflict(err error) error {
	if err == nil || errors.Is(err, ErrConflict) {
		return err
	}
	if isBusy(err) {
		return fmt.Errorf("%w: %v", ErrConflict, err)
	}
	return err
}

func (m *assetManager) violatesSerialTagIndex(err error) bool {
	taken := false
	gormutil.ClassifyError(err, func(string) error { return err }, func(violated string) error {
		taken = m.namesSerialTagIndex(violated)
		return err
	})
	return taken
}

func (m *assetManager) namesSerialTagIndex(violated string) bool {
	if cut := strings.Index(violated, " ("); cut >= 0 {
		violated = violated[:cut]
	}
	violated = strings.TrimSpace(violated)
	if violated == "" {
		return false
	}
	asset := m.store.Object(roleAsset)
	postgresName := m.store.Spec().IndexFor(asset, spec.Index{Name: serialTagIndex})
	sqliteName := m.store.Table(roleAsset) + "." + serialTagIndex
	return violated == postgresName || violated == sqliteName
}

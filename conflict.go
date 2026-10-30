package mwanachamaassetmanager

import (
	"errors"
	"fmt"
	"strings"

	"github.com/aosanya/mwanachama-backend-shared/gormutil"
)

var busyMessages = []string{
	"database table is locked",
	"database is locked",
	"database is deadlocked",
	"SQLITE_BUSY",
	"could not serialize access",
	"deadlock detected",
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var conflicted bool
	gormutil.ClassifyError(err,
		func(string) error { return err },
		func(string) error { conflicted = true; return err },
	)
	return conflicted
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

package mwanachamaassetmanager

import (
	"errors"
	"fmt"
	"strings"
)

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

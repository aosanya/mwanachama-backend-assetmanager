package mwanachamaassetmanager

import "time"

func isRFC3339(s string) bool {
	_, err := time.Parse(time.RFC3339, s)
	return err == nil
}

var patterns = map[string]func(string) bool{
	"rfc3339": isRFC3339,
}

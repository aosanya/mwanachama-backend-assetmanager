package mwanachamaassetmanager

import _ "embed"

//go:embed assetmanager.operations.json
var operationsJSON []byte

func Operations() []byte { return operationsJSON }

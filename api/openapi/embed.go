// Package openapi embeds the published RideMesh HTTP contract.
package openapi

import _ "embed"

//go:embed ridemesh.json
var Specification []byte

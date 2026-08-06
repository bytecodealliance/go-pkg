// Package httpconv contains the pure-Go conversion logic shared by both the
// sync (WASI P2) and async (WASI P3) implementations of the wasihttp
// package. It has no dependency on generated wasm bindings, so it compiles,
// tests, and benchmarks on the host.
package httpconv

import (
	"net/http"

	witTypes "go.bytecodealliance.org/pkg/wit/types"
)

// FieldValues converts a list of HTTP header values into the [][]uint8 shape
// required by wasi:http `fields.set`.
func FieldValues(vals []string) [][]uint8 {
	fieldVals := make([][]uint8, 0, len(vals))
	for _, val := range vals {
		fieldVals = append(fieldVals, []uint8(val))
	}
	return fieldVals
}

// AddEntries copies wasi:http field entries (as returned by `fields.entries`
// / `fields.copy-all`) into a [net/http.Header] map.
func AddEntries(dest http.Header, entries []witTypes.Tuple2[string, []uint8]) {
	for _, pair := range entries {
		dest.Add(pair.F0, string(pair.F1))
	}
}

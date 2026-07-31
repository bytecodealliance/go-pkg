//go:build !wasm

package wasilog

import (
	_ "unsafe"
)

// wasmimportLog provides a host-side body for the wasi:logging `log` import
// so this package's tests can link and run on non-wasm platforms. On wasm
// targets the real //go:wasmimport declaration provides the symbol.
//
//go:linkname wasmimportLog go.bytecodealliance.org/pkg/imports/wasi_logging_0_1_0_draft_logging.wasm_import_log
func wasmimportLog(arg0 int32, arg1 uintptr, arg2 uint32, arg3 uintptr, arg4 uint32) {
}

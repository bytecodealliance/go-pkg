// Package wasiconfig provides helpers over the [wasi:config/store]
// interface.
//
// [wasi:config/store]: https://github.com/WebAssembly/wasi-config/blob/main/wit/store.wit
package wasiconfig

import (
	store "go.bytecodealliance.org/pkg/imports/wasi_config_0_2_0_rc_1_store"
)

// Get returns the configuration value for the provided key using
// [wasi:config/store.get]. The second return value reports whether a value
// was found.
//
// [wasi:config/store.get]: https://github.com/WebAssembly/wasi-config/blob/main/wit/store.wit
func Get(key string) (string, bool) {
	res := store.Get(key)
	if res.IsOk() {
		opt := res.Ok()
		if opt.IsSome() {
			return opt.Some(), true
		}
	}

	return "", false
}

// GetOrDefault tries to get a configuration value by the provided key using
// [wasi:config/store.get], falling back to the provided defaultValue if a
// configuration value for the key is not found.
//
// [wasi:config/store.get]: https://github.com/WebAssembly/wasi-config/blob/main/wit/store.wit
func GetOrDefault(key string, defaultValue string) string {
	if val, ok := Get(key); ok {
		return val
	}

	return defaultValue
}

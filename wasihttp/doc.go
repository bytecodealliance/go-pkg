// Package wasihttp adapts [net/http] to [wasi:http]: incoming requests are
// served by a standard [net/http.Handler] registered via [Handle] or
// [HandleFunc], and outbound requests are sent through the host via
// [Transport] / [DefaultClient].
//
// The package has two implementations selected at build time with the
// componentizego_async build tag; the exported API is identical under both:
//
//   - Default (no tag): sync WASI P2, wasi:http@0.2.8. Exports
//     wasi:http/incoming-handler@0.2.8 and sends outbound requests through
//     wasi:http/outgoing-handler@0.2.8. Matches the bytecodealliance:pkg/wasip2
//     world and builds with stock Go.
//   - -tags componentizego_async: async WASI P3, wasi:http@0.3.0. Exports
//     wasi:http/handler@0.3.0 with streaming bodies and native concurrency,
//     and sends outbound requests through wasi:http/client@0.3.0. Matches the
//     bytecodealliance:pkg/wasip3 world. componentize-go sets this tag
//     automatically when building an async world.
//
// [wasi:http]: https://github.com/WebAssembly/wasi-http
package wasihttp

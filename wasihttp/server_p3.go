//go:build componentizego_async

package wasihttp

import (
	"fmt"
	"net/http"
	"os"

	handler "go.bytecodealliance.org/pkg/exports/bytecodealliance_pkg_wasip3_0_1_0/export_wasi_http_0_3_0_handler"
	_ "go.bytecodealliance.org/pkg/exports/bytecodealliance_pkg_wasip3_0_1_0/wit_exports"
	types "go.bytecodealliance.org/pkg/imports/wasi_http_0_3_0_types"
	witTypes "go.bytecodealliance.org/pkg/wit/types"
)

func init() {
	handler.Exports.Handle = wasiHandle
}

// handlerFn is the function called by the wasi:http/handler export.
var handlerFn = defaultHandler

// defaultHandler is a placeholder for returning a useful error to stderr when
// the handler is not set.
var defaultHandler = func(http.ResponseWriter, *http.Request) {
	fmt.Fprintln(os.Stderr, "http handler undefined")
}

// Handle sets the [net/http.Handler] that will be called to handle the
// incoming request. It must be called from an init() function.
func Handle(h http.Handler) {
	handlerFn = h.ServeHTTP
}

// HandleFunc sets the [net/http.HandlerFunc] that will be called to handle the
// incoming request. It must be called from an init() function.
func HandleFunc(h http.HandlerFunc) {
	handlerFn = h
}

// wasiHandle bridges the async wasi:http/handler export to the registered
// net/http handler. The handler runs in a goroutine; wasiHandle returns the
// wasi Response as soon as the handler produces headers so the response body
// can stream while the handler is still writing.
func wasiHandle(request *types.Request) witTypes.Result[*types.Response, types.ErrorCode] {
	res := newResponseWriter()

	go func() {
		defer res.close()

		req, err := wasiToHTTPRequest(request)
		if err != nil {
			res.channel <- witTypes.Err[*types.Response](
				types.MakeErrorCodeInternalError(witTypes.Some(fmt.Sprintf(
					"failed to convert wasi:http request to http.Request: %v", err,
				))),
			)
			return
		}
		defer req.Body.Close()

		handlerFn(res, req)

		// If the handler never wrote to the body, the response has not been
		// sent yet; send it now with headers and status code only.
		if err := res.send(); err != nil {
			res.channel <- witTypes.Err[*types.Response](
				types.MakeErrorCodeInternalError(witTypes.Some(fmt.Sprintf(
					"failed to produce a response: %v", err,
				))),
			)
			return
		}

		res.writeTrailers()
	}()

	return <-res.channel
}

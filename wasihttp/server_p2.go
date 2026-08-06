//go:build !componentizego_async

package wasihttp

import (
	"fmt"
	"net/http"
	"os"

	incominghandler "go.bytecodealliance.org/pkg/exports/bytecodealliance_pkg_wasip2_0_1_0/export_wasi_http_0_2_8_incoming_handler"
	types "go.bytecodealliance.org/pkg/imports/wasi_http_0_2_8_types"
	witTypes "go.bytecodealliance.org/pkg/wit/types"

	// Pull in the //go:wasmexport glue for the component's exports.
	_ "go.bytecodealliance.org/pkg/exports/bytecodealliance_pkg_wasip2_0_1_0/wit_exports"
)

func init() {
	incominghandler.Exports.Handle = wasiHandle
}

// handlerFn is the function called by the wasi:http/incoming-handler export.
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

func wasiHandle(request *types.IncomingRequest, responseOut *types.ResponseOutparam) {
	httpReq, err := wasiToHTTPRequest(request)
	if err != nil {
		types.ResponseOutparamSet(responseOut, witTypes.Err[*types.OutgoingResponse, types.ErrorCode](
			types.MakeErrorCodeInternalError(witTypes.Some(err.Error()))),
		)
		return
	}
	if httpReq.Body != nil {
		defer func() { _ = httpReq.Body.Close() }()
	}

	httpRes := newResponseOutparamWriter(responseOut)
	defer func() { _ = httpRes.Close() }()

	handlerFn(httpRes, httpReq)
}

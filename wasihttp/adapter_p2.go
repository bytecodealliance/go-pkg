//go:build !componentizego_async

package wasihttp

// Refactored from https://github.com/rajatjindal/wasi-go-sdk/tree/d3e8665bef9fbf0794ad14f7114a9882e0d983c3/pkg/wasihttp

import (
	"fmt"
	"io"
	"net/http"
	"sync"

	types "go.bytecodealliance.org/pkg/imports/wasi_http_0_2_8_types"
	streams "go.bytecodealliance.org/pkg/imports/wasi_io_0_2_8_streams"
	"go.bytecodealliance.org/pkg/internal/httpconv"
	witTypes "go.bytecodealliance.org/pkg/wit/types"
)

var _ http.ResponseWriter = (*responseOutparamWriter)(nil)

// responseOutparamWriter implements a [net/http.ResponseWriter] for
// [wasi:http/types.response-outparam].
//
// [wasi:http/types.response-outparam]: https://github.com/WebAssembly/wasi-http/blob/main/wit/types.wit
type responseOutparamWriter struct {
	outparam    *types.ResponseOutparam
	response    *types.OutgoingResponse
	wasiHeaders *types.Fields
	httpHeaders http.Header
	body        *types.OutgoingBody
	stream      *streams.OutputStream

	headerOnce sync.Once
	headerErr  error

	statuscode int
}

// Header returns the header map that will be sent by WriteHeader.
func (row *responseOutparamWriter) Header() http.Header {
	return row.httpHeaders
}

// Write writes the data to the connection as part of an HTTP reply.
func (row *responseOutparamWriter) Write(buf []byte) (int, error) {
	// NOTE(lxf): If this is the first write, make sure we set the headers/statuscode
	row.headerOnce.Do(row.reconcile)
	if row.headerErr != nil {
		return 0, row.headerErr
	}

	writeResult := row.stream.Write(buf)
	if writeResult.IsErr() {
		if writeResult.Err().Tag() == streams.StreamErrorClosed {
			return 0, io.EOF
		}

		return 0, fmt.Errorf("failed to write to response body's stream: %s", writeResult.Err().LastOperationFailed().ToDebugString())
	}

	row.stream.BlockingFlush()

	return len(buf), nil
}

// WriteHeader sends an HTTP response header with the provided
// status code.
func (row *responseOutparamWriter) WriteHeader(statusCode int) {
	row.headerOnce.Do(func() {
		row.statuscode = statusCode
		row.reconcile()
	})
}

// reconcile headers from go to wasi
func (row *responseOutparamWriter) reconcileHeaders() error {
	for key, vals := range row.httpHeaders {
		if result := row.wasiHeaders.Set(key, httpconv.FieldValues(vals)); result.IsErr() {
			return fmt.Errorf("failed to set header %s: %v", key, result.Err())
		}
	}

	// NOTE(lxf): once headers are written we clear them out so they can emit http trailers
	row.httpHeaders = http.Header{}

	return nil
}

func (row *responseOutparamWriter) reconcile() {
	if row.headerErr = row.reconcileHeaders(); row.headerErr != nil {
		return
	}

	row.response = types.MakeOutgoingResponse(row.wasiHeaders)
	row.response.SetStatusCode(uint16(row.statuscode))

	bodyResult := row.response.Body()
	if bodyResult.IsErr() {
		row.headerErr = fmt.Errorf("failed to acquire resource handle to response body")
		return
	}
	row.body = bodyResult.Ok()

	writeResult := row.body.Write()
	if writeResult.IsErr() {
		row.headerErr = fmt.Errorf("failed to acquire resource handle for response body's stream")
		return
	}
	row.stream = writeResult.Ok()

	result := witTypes.Ok[*types.OutgoingResponse, types.ErrorCode](row.response)
	types.ResponseOutparamSet(row.outparam, result)
}

// Close closes out the underlying stream by flushing the response and making
// sure that the underlying resource handle is dropped.
func (row *responseOutparamWriter) Close() error {
	if row.stream == nil {
		return nil
	}

	row.stream.BlockingFlush()
	row.stream.Drop()
	row.stream = nil

	maybeTrailers := witTypes.None[*types.Fields]()
	wasiTrailers := types.MakeFields()
	for key, vals := range row.httpHeaders {
		if result := wasiTrailers.Set(key, httpconv.FieldValues(vals)); result.IsErr() {
			return fmt.Errorf("failed to set trailer %s: %v", key, result.Err())
		}
	}
	if len(row.httpHeaders) > 0 {
		maybeTrailers = witTypes.Some(wasiTrailers)
	}

	res := types.OutgoingBodyFinish(row.body, maybeTrailers)
	if res.IsErr() {
		return fmt.Errorf("failed to set trailer: %v", res.Err())
	}
	return nil
}

// newResponseOutparamWriter takes a [types.ResponseOutparam] representing
// [wasi:http/types.response-outparam] and instantiates a new
// [responseOutparamWriter] for writing to it.
func newResponseOutparamWriter(out *types.ResponseOutparam) *responseOutparamWriter {
	return &responseOutparamWriter{
		outparam:    out,
		httpHeaders: http.Header{},
		wasiHeaders: types.MakeFields(),
		statuscode:  http.StatusOK,
	}
}

// wasiToHTTPRequest takes a [types.IncomingRequest] and returns a
// [net/http.Request] representation of it.
func wasiToHTTPRequest(ir *types.IncomingRequest) (req *http.Request, err error) {
	method, err := methodToString(ir.Method())
	if err != nil {
		return nil, err
	}

	authority := "localhost"
	if auth := ir.Authority(); auth.IsSome() {
		authority = auth.Some()
	}

	pathWithQuery := "/"
	if p := ir.PathWithQuery(); p.IsSome() {
		pathWithQuery = p.Some()
	}

	body, trailers, err := newIncomingBodyTrailer(ir)
	if err != nil {
		switch method {
		case http.MethodGet,
			http.MethodHead,
			http.MethodDelete,
			http.MethodConnect,
			http.MethodOptions,
			http.MethodTrace:
		default:
			return nil, fmt.Errorf("failed to consume incoming request: %w", err)
		}
	}

	url := fmt.Sprintf("http://%s%s", authority, pathWithQuery)
	req, err = http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Trailer = trailers

	headers := ir.Headers()
	wasiToHTTPHeader(headers, &req.Header)
	headers.Drop()

	req.Host = authority
	req.URL.Host = authority
	req.RequestURI = pathWithQuery

	return req, nil
}

func methodToString(m types.Method) (string, error) {
	switch m.Tag() {
	case types.MethodConnect:
		return http.MethodConnect, nil
	case types.MethodDelete:
		return http.MethodDelete, nil
	case types.MethodGet:
		return http.MethodGet, nil
	case types.MethodHead:
		return http.MethodHead, nil
	case types.MethodOptions:
		return http.MethodOptions, nil
	case types.MethodPatch:
		return http.MethodPatch, nil
	case types.MethodPost:
		return http.MethodPost, nil
	case types.MethodPut:
		return http.MethodPut, nil
	case types.MethodTrace:
		return http.MethodTrace, nil
	case types.MethodOther:
		other := m.Other()
		return other, fmt.Errorf("unknown http method '%s'", other)
	}
	return "", fmt.Errorf("failed to convert http method")
}

// wasiToHTTPHeader takes a [types.Fields] and copies them to the provided [net/http.Header] map.
func wasiToHTTPHeader(src *types.Fields, dest *http.Header) {
	httpconv.AddEntries(*dest, src.Entries())
}

// httpToWASIHeader takes a [net/http.Header] map and copies them to the provided [types.Fields].
func httpToWASIHeader(src http.Header, dest *types.Fields) error {
	for k, v := range src {
		res := dest.Set(k, httpconv.FieldValues(v))
		if res.IsErr() {
			return fmt.Errorf("failed to set header %s: %v", k, res.Err())
		}
	}

	return nil
}

func toWASIMethod(s string) types.Method {
	switch s {
	case http.MethodConnect:
		return types.MakeMethodConnect()
	case http.MethodDelete:
		return types.MakeMethodDelete()
	case http.MethodGet:
		return types.MakeMethodGet()
	case http.MethodHead:
		return types.MakeMethodHead()
	case http.MethodOptions:
		return types.MakeMethodOptions()
	case http.MethodPatch:
		return types.MakeMethodPatch()
	case http.MethodPost:
		return types.MakeMethodPost()
	case http.MethodPut:
		return types.MakeMethodPut()
	case http.MethodTrace:
		return types.MakeMethodTrace()
	default:
		return types.MakeMethodOther(s)
	}
}

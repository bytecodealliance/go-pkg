package wasilog

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	logging "go.bytecodealliance.org/pkg/imports/wasi_logging_0_1_0_draft_logging"
)

// DefaultLogger is the default implementation that adapts the [wasi:logging] interface to a [slog.Handler].
//
// [wasi:logging]: https://github.com/WebAssembly/wasi-logging
var DefaultLogger = slog.New(DefaultOptions().NewHandler())

// ContextLogger returns a [DefaultLogger] implementation that has an additional "wasi-context" [slog.Attr] attached to it.
func ContextLogger(wasiContext string) *slog.Logger {
	return DefaultLogger.With(ContextAttr(wasiContext))
}

type contextKey string

func (k contextKey) String() string {
	return string(k)
}

// ContextKey is a predefined key used to track the wasi
const ContextKey = contextKey("wasi-context")

func ContextAttr(name string) slog.Attr {
	return slog.String(string(ContextKey), name)
}

type wasmLoggerFunc func(level logging.Level, context string, message string)

// WasiLoggingOption represents the available options for customizing the [WebassemblyHandler].
type WasiLoggingOption struct {
	// required: log function
	LoggerFunc wasmLoggerFunc
	// log level (default: info)
	Level slog.Leveler

	// optional: fetch attributes from context
	AttrFromContext []func(ctx context.Context) []slog.Attr

	// optional: replace attributes
	ReplaceAttr func(groups []string, a slog.Attr) slog.Attr
}

// WebassemblyHandler implements the [slog.Handler] interface to adapt [slog] to wasi:logging.
type WebassemblyHandler struct {
	option WasiLoggingOption
	attrs  []slog.Attr
	groups []string
}

var _ slog.Handler = (*WebassemblyHandler)(nil)

func wasiLevel(level slog.Level) logging.Level {
	switch level {
	case slog.LevelDebug:
		return logging.LevelDebug
	case slog.LevelInfo:
		return logging.LevelInfo
	case slog.LevelWarn:
		return logging.LevelWarn
	case slog.LevelError:
		return logging.LevelError
	default:
		return logging.LevelDebug
	}
}

// contextAttrs collects attributes from the context using the configured
// AttrFromContext extractors.
func contextAttrs(ctx context.Context, fns []func(ctx context.Context) []slog.Attr) []slog.Attr {
	var attrs []slog.Attr
	for _, fn := range fns {
		attrs = append(attrs, fn(ctx)...)
	}
	return attrs
}

// flatten reduces attrs to leaf attributes: group values are descended with
// their names appended to the group path, LogValuers are resolved, and
// replaceAttr is applied to each leaf with its group path (the default
// ReplaceAttr folds the path into the key, e.g. "a.b.c"). Attributes with an
// empty key or empty value are dropped.
func flatten(replaceAttr func(groups []string, a slog.Attr) slog.Attr, groups []string, attrs []slog.Attr) []slog.Attr {
	var output []slog.Attr
	for _, attr := range attrs {
		attr.Value = attr.Value.Resolve()
		if attr.Value.Kind() == slog.KindGroup {
			g := groups
			if attr.Key != "" {
				g = append(groups[:len(groups):len(groups)], attr.Key)
			}
			output = append(output, flatten(replaceAttr, g, attr.Value.Group())...)
			continue
		}
		if replaceAttr != nil {
			attr = replaceAttr(groups, attr)
			attr.Value = attr.Value.Resolve()
		}
		if attr.Key == "" || attr.Value.Equal(slog.Value{}) {
			continue
		}
		output = append(output, attr)
	}
	return output
}

func wasiConverter(replaceAttr func(groups []string, a slog.Attr) slog.Attr, loggerAttr []slog.Attr, groups []string, record *slog.Record) (string, string) {
	recordAttrs := make([]slog.Attr, 0, record.NumAttrs())
	record.Attrs(func(a slog.Attr) bool {
		recordAttrs = append(recordAttrs, a)
		return true
	})

	attrs := flatten(replaceAttr, nil, loggerAttr)
	attrs = append(attrs, flatten(replaceAttr, groups, recordAttrs)...)

	// The context key is moved to the 'Context' field in wasi:logging and
	// removed from the log message.
	var context string
	parts := make([]string, 0, len(attrs)+1)
	for _, attr := range attrs {
		if attr.Key == string(ContextKey) {
			context = attr.Value.String()
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=%q", attr.Key, attr.Value.String()))
	}
	parts = append(parts, record.Message)

	return strings.Join(parts, " "), context
}

// DefaultOptions represents the default set of values used in [WasiLoggingOption] that are used in setting up [DefaultLogger].
func DefaultOptions() WasiLoggingOption {
	return WasiLoggingOption{
		LoggerFunc: logging.Log,
		Level:      slog.LevelInfo,
		AttrFromContext: []func(ctx context.Context) []slog.Attr{
			func(ctx context.Context) []slog.Attr {
				if contextName, ok := ctx.Value(ContextKey).(string); ok {
					return []slog.Attr{slog.String(string(ContextKey), string(contextName))}
				}
				return nil
			},
		},
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Make so groups become a prefix of the key
			// Ex: groups = ["a", "b"], key = "c" => "a.b.c"
			if len(groups) == 0 {
				return a
			}

			a.Key = strings.Join(groups, ".") + "." + a.Key
			return a
		},
	}
}

// NewHandler is used to instantiate a new instance of a [WebassemblyHandler] that implements the [slog.Handler] interface.
func (o WasiLoggingOption) NewHandler() slog.Handler {
	return &WebassemblyHandler{
		option: o,
	}
}

// Enabled reports whether the handler handles records at the given level. The handler ignores records whose level is lower.
func (h *WebassemblyHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.option.Level.Level()
}

// Handle formats its argument [slog.Record] using the provided [context.Context] into a wasi:logging compatible output.
func (h *WebassemblyHandler) Handle(ctx context.Context, record slog.Record) error {
	fromContext := contextAttrs(ctx, h.option.AttrFromContext)
	loggerAttr := append(h.attrs[:len(h.attrs):len(h.attrs)], fromContext...)
	message, logContext := wasiConverter(h.option.ReplaceAttr, loggerAttr, h.groups, &record)

	h.option.LoggerFunc(wasiLevel(record.Level), logContext, message)

	return nil
}

// WithAttrs returns a new [WebassemblyHandler] whose attributes consists
// of h's attributes followed by attrs, nested under the currently open groups.
func (h *WebassemblyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	for i := len(h.groups) - 1; i >= 0; i-- {
		attrs = []slog.Attr{{Key: h.groups[i], Value: slog.GroupValue(attrs...)}}
	}
	return &WebassemblyHandler{
		option: h.option,
		attrs:  append(h.attrs[:len(h.attrs):len(h.attrs)], attrs...),
		groups: h.groups,
	}
}

// WithGroup returns a new [WebassemblyHandler] where the attributes are
// grouped under a common name.
func (h *WebassemblyHandler) WithGroup(name string) slog.Handler {
	// https://cs.opensource.google/go/x/exp/+/46b07846:slog/handler.go;l=247
	if name == "" {
		return h
	}

	return &WebassemblyHandler{
		option: h.option,
		attrs:  h.attrs,
		groups: append(h.groups[:len(h.groups):len(h.groups)], name),
	}
}

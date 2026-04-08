package trek

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"
)

type contextKey int

const (
	decisionKey contextKey = iota
	capCounterKey
)

// Handler wraps an slog.Handler to provide request-scoped log level elevation.
type Handler struct {
	inner       slog.Handler
	redactAttr  RedactAttrFunc
	redactEvent RedactEventFunc
}

// WrapHandler wraps an existing slog.Handler with Trek's request-scoped behavior.
func WrapHandler(h slog.Handler) *Handler {
	return &Handler{inner: h}
}

// WrapHandlerWithRedaction wraps an existing slog.Handler with Trek and redaction.
func WrapHandlerWithRedaction(h slog.Handler, redactAttr RedactAttrFunc, redactEvent RedactEventFunc) *Handler {
	return &Handler{
		inner:       h,
		redactAttr:  redactAttr,
		redactEvent: redactEvent,
	}
}

// Enabled reports whether the handler handles records at the given level.
// It checks the request context for a Trek decision and elevates accordingly.
func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	decision := DecisionFromContext(ctx)
	if decision == nil || !decision.Matched {
		return h.inner.Enabled(ctx, level)
	}

	minLevel := slogLevelFromTrek(decision.EffectiveLevel)
	if level >= minLevel {
		return true
	}

	return h.inner.Enabled(ctx, level)
}

// Handle handles the Record, applying redaction and cap enforcement.
func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	decision := DecisionFromContext(ctx)

	if decision != nil && decision.Matched {
		if !h.checkAndIncrementCap(ctx, decision, r.Level) {
			return nil
		}

		r = h.addTrekAttrs(r, decision)
	}

	if h.redactAttr != nil {
		r = h.redactRecord(r)
	}

	return h.inner.Handle(ctx, r)
}

// WithAttrs returns a new Handler with the given attributes.
func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{
		inner:       h.inner.WithAttrs(attrs),
		redactAttr:  h.redactAttr,
		redactEvent: h.redactEvent,
	}
}

// WithGroup returns a new Handler with the given group name.
func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{
		inner:       h.inner.WithGroup(name),
		redactAttr:  h.redactAttr,
		redactEvent: h.redactEvent,
	}
}

// checkAndIncrementCap checks if the cap has been reached and increments the counter.
// Returns true if the log should be emitted, false if cap reached.
func (h *Handler) checkAndIncrementCap(ctx context.Context, decision *Decision, level slog.Level) bool {
	if level < slog.LevelDebug {
		return true
	}

	if decision.Caps.MaxDebugEventsPerRequest <= 0 {
		return true
	}

	counter := capCounterFromContext(ctx)
	if counter == nil {
		return true
	}

	count := counter.Add(1)
	if int(count) > decision.Caps.MaxDebugEventsPerRequest {
		if int(count) == decision.Caps.MaxDebugEventsPerRequest+1 {
			// Record cap triggered metric
			GetMetrics().IncrCapsTriggered()

			capReachedRecord := slog.NewRecord(time.Now(), slog.LevelWarn, "TREK_CAP_REACHED", 0)
			capReachedRecord.AddAttrs(
				slog.String("trek.session_id", decision.SessionID),
				slog.Int("trek.cap", decision.Caps.MaxDebugEventsPerRequest),
			)
			_ = h.inner.Handle(ctx, capReachedRecord)
		}
		return false
	}

	return true
}

// addTrekAttrs adds Trek-specific attributes to a log record.
func (h *Handler) addTrekAttrs(r slog.Record, decision *Decision) slog.Record {
	r.AddAttrs(
		slog.Bool("trek.matched", true),
		slog.String("trek.session_id", decision.SessionID),
	)

	for k, v := range decision.Labels {
		r.AddAttrs(slog.String("trek.labels."+k, v))
	}

	return r
}

// EmitSessionMatchedMarker emits a diagnostic marker event when a session matches.
// This should be called once per request when a session is matched, typically by the middleware.
func EmitSessionMatchedMarker(ctx context.Context, logger *slog.Logger, decision *Decision) {
	if decision == nil || !decision.Matched {
		return
	}

	attrs := []slog.Attr{
		slog.String("trek.event", "session_matched"),
		slog.String("trek.session_id", decision.SessionID),
		slog.String("trek.level", string(decision.EffectiveLevel)),
		slog.String("trek.reason", string(decision.ReasonCode)),
	}

	for k, v := range decision.Labels {
		attrs = append(attrs, slog.String("trek.labels."+k, v))
	}

	// Create a record at debug level for the marker
	r := slog.NewRecord(time.Now(), slog.LevelDebug, "trek.session_matched", 0)
	r.AddAttrs(attrs...)

	// Get the underlying handler and emit directly
	if h, ok := logger.Handler().(*Handler); ok {
		_ = h.inner.Handle(ctx, r)
	}
}

// redactRecord applies redaction to all attributes in a record.
func (h *Handler) redactRecord(r slog.Record) slog.Record {
	var newAttrs []slog.Attr

	r.Attrs(func(a slog.Attr) bool {
		newVal, drop := h.redactAttr(a.Key, a.Value.Any())
		if !drop {
			newAttrs = append(newAttrs, slog.Any(a.Key, newVal))
		}
		return true
	})

	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	newRecord.AddAttrs(newAttrs...)

	return newRecord
}

// slogLevelFromTrek converts a Trek level to an slog.Level.
func slogLevelFromTrek(level Level) slog.Level {
	switch level {
	case LevelTrace:
		return slog.LevelDebug - 4
	case LevelDebug:
		return slog.LevelDebug
	case LevelInfo:
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

// DecisionFromContext retrieves the Trek decision from the context.
func DecisionFromContext(ctx context.Context) *Decision {
	if ctx == nil {
		return nil
	}
	v := ctx.Value(decisionKey)
	if v == nil {
		return nil
	}
	d, ok := v.(*Decision)
	if !ok {
		return nil
	}
	return d
}

// ContextWithDecision returns a new context with the Trek decision stored.
func ContextWithDecision(ctx context.Context, decision *Decision) context.Context {
	ctx = context.WithValue(ctx, decisionKey, decision)
	ctx = context.WithValue(ctx, capCounterKey, &atomic.Int64{})
	return ctx
}

// capCounterFromContext retrieves the cap counter from the context.
func capCounterFromContext(ctx context.Context) *atomic.Int64 {
	if ctx == nil {
		return nil
	}
	v := ctx.Value(capCounterKey)
	if v == nil {
		return nil
	}
	counter, ok := v.(*atomic.Int64)
	if !ok {
		return nil
	}
	return counter
}

// IsDebug returns true if the current request context has debug logging enabled.
// This is the "check pattern" for cases where the wrapper pattern isn't suitable.
func IsDebug(ctx context.Context) bool {
	decision := DecisionFromContext(ctx)
	return decision != nil && decision.Matched
}

// EffectiveLevel returns the effective log level for the current request.
func EffectiveLevel(ctx context.Context) Level {
	decision := DecisionFromContext(ctx)
	if decision == nil || !decision.Matched {
		return LevelInfo
	}
	return decision.EffectiveLevel
}

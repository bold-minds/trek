package trek

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestWrapHandler(t *testing.T) {
	inner := slog.NewTextHandler(nil, nil)
	handler := WrapHandler(inner)

	if handler == nil {
		t.Fatal("WrapHandler() should not return nil")
	}
	if handler.inner != inner {
		t.Error("WrapHandler() should store the inner handler")
	}
}

func TestWrapHandlerWithRedaction(t *testing.T) {
	inner := slog.NewTextHandler(nil, nil)
	redactAttr := func(key string, val any) (any, bool) {
		return val, false
	}
	redactEvent := func(msg string, attrs map[string]any) (map[string]any, bool) {
		return attrs, false
	}

	handler := WrapHandlerWithRedaction(inner, redactAttr, redactEvent)

	if handler == nil {
		t.Fatal("WrapHandlerWithRedaction() should not return nil")
	}
	if handler.redactAttr == nil {
		t.Error("WrapHandlerWithRedaction() should store redactAttr")
	}
	if handler.redactEvent == nil {
		t.Error("WrapHandlerWithRedaction() should store redactEvent")
	}
}

func TestDecisionFromContext_Nil(t *testing.T) {
	decision := DecisionFromContext(nil)
	if decision != nil {
		t.Error("DecisionFromContext(nil) should return nil")
	}
}

func TestDecisionFromContext_NoDecision(t *testing.T) {
	ctx := context.Background()
	decision := DecisionFromContext(ctx)
	if decision != nil {
		t.Error("DecisionFromContext() with no decision should return nil")
	}
}

func TestDecisionFromContext_WithDecision(t *testing.T) {
	ctx := context.Background()
	expected := &Decision{
		Matched:        true,
		SessionID:      "sess-123",
		EffectiveLevel: LevelDebug,
	}

	ctx = ContextWithDecision(ctx, expected)
	actual := DecisionFromContext(ctx)

	if actual == nil {
		t.Fatal("DecisionFromContext() should return the stored decision")
	}
	if actual.SessionID != expected.SessionID {
		t.Errorf("SessionID = %q, want %q", actual.SessionID, expected.SessionID)
	}
	if actual.EffectiveLevel != expected.EffectiveLevel {
		t.Errorf("EffectiveLevel = %q, want %q", actual.EffectiveLevel, expected.EffectiveLevel)
	}
}

func TestContextWithDecision(t *testing.T) {
	ctx := context.Background()
	decision := &Decision{
		Matched:   true,
		SessionID: "sess-456",
	}

	newCtx := ContextWithDecision(ctx, decision)

	if newCtx == ctx {
		t.Error("ContextWithDecision() should return a new context")
	}

	// Verify cap counter is also set
	counter := capCounterFromContext(newCtx)
	if counter == nil {
		t.Error("ContextWithDecision() should also set cap counter")
	}
}

func TestCapCounterFromContext_Nil(t *testing.T) {
	counter := capCounterFromContext(nil)
	if counter != nil {
		t.Error("capCounterFromContext(nil) should return nil")
	}
}

func TestCapCounterFromContext_NoCounter(t *testing.T) {
	ctx := context.Background()
	counter := capCounterFromContext(ctx)
	if counter != nil {
		t.Error("capCounterFromContext() with no counter should return nil")
	}
}

func TestCapCounterFromContext_WithCounter(t *testing.T) {
	ctx := context.Background()
	decision := &Decision{Matched: true}
	ctx = ContextWithDecision(ctx, decision)

	counter := capCounterFromContext(ctx)
	if counter == nil {
		t.Fatal("capCounterFromContext() should return counter")
	}

	// Test incrementing
	val := counter.Add(1)
	if val != 1 {
		t.Errorf("counter.Add(1) = %d, want 1", val)
	}

	val = counter.Add(1)
	if val != 2 {
		t.Errorf("counter.Add(1) second time = %d, want 2", val)
	}
}

func TestIsDebug_NoDecision(t *testing.T) {
	ctx := context.Background()
	if IsDebug(ctx) {
		t.Error("IsDebug() should return false when no decision")
	}
}

func TestIsDebug_NotMatched(t *testing.T) {
	ctx := context.Background()
	decision := &Decision{Matched: false}
	ctx = ContextWithDecision(ctx, decision)

	if IsDebug(ctx) {
		t.Error("IsDebug() should return false when not matched")
	}
}

func TestIsDebug_Matched(t *testing.T) {
	ctx := context.Background()
	decision := &Decision{Matched: true}
	ctx = ContextWithDecision(ctx, decision)

	if !IsDebug(ctx) {
		t.Error("IsDebug() should return true when matched")
	}
}

func TestEffectiveLevel_NoDecision(t *testing.T) {
	ctx := context.Background()
	level := EffectiveLevel(ctx)

	if level != LevelInfo {
		t.Errorf("EffectiveLevel() = %q, want %q", level, LevelInfo)
	}
}

func TestEffectiveLevel_NotMatched(t *testing.T) {
	ctx := context.Background()
	decision := &Decision{
		Matched:        false,
		EffectiveLevel: LevelDebug,
	}
	ctx = ContextWithDecision(ctx, decision)

	level := EffectiveLevel(ctx)
	if level != LevelInfo {
		t.Errorf("EffectiveLevel() = %q, want %q when not matched", level, LevelInfo)
	}
}

func TestEffectiveLevel_Matched(t *testing.T) {
	ctx := context.Background()
	decision := &Decision{
		Matched:        true,
		EffectiveLevel: LevelDebug,
	}
	ctx = ContextWithDecision(ctx, decision)

	level := EffectiveLevel(ctx)
	if level != LevelDebug {
		t.Errorf("EffectiveLevel() = %q, want %q", level, LevelDebug)
	}
}

func TestSlogLevelFromTrek(t *testing.T) {
	tests := []struct {
		level    Level
		expected slog.Level
	}{
		{LevelTrace, slog.LevelDebug - 4},
		{LevelDebug, slog.LevelDebug},
		{LevelInfo, slog.LevelInfo},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(string(tt.level), func(t *testing.T) {
			got := slogLevelFromTrek(tt.level)
			if got != tt.expected {
				t.Errorf("slogLevelFromTrek(%q) = %v, want %v", tt.level, got, tt.expected)
			}
		})
	}
}

func TestHandler_WithAttrs(t *testing.T) {
	inner := slog.NewTextHandler(nil, nil)
	handler := WrapHandler(inner)

	attrs := []slog.Attr{
		slog.String("key", "value"),
	}

	newHandler := handler.WithAttrs(attrs)

	if newHandler == nil {
		t.Fatal("WithAttrs() should not return nil")
	}

	// Should return a new Handler instance
	trekHandler, ok := newHandler.(*Handler)
	if !ok {
		t.Fatal("WithAttrs() should return a *Handler")
	}
	if trekHandler == handler {
		t.Error("WithAttrs() should return a new handler")
	}
}

func TestHandler_WithGroup(t *testing.T) {
	inner := slog.NewTextHandler(nil, nil)
	handler := WrapHandler(inner)

	newHandler := handler.WithGroup("mygroup")

	if newHandler == nil {
		t.Fatal("WithGroup() should not return nil")
	}

	trekHandler, ok := newHandler.(*Handler)
	if !ok {
		t.Fatal("WithGroup() should return a *Handler")
	}
	if trekHandler == handler {
		t.Error("WithGroup() should return a new handler")
	}
}

func TestEmitSessionMatchedMarker_NilDecision(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()

	// Should not panic
	EmitSessionMatchedMarker(ctx, logger, nil)
}

func TestEmitSessionMatchedMarker_NotMatched(t *testing.T) {
	ctx := context.Background()
	logger := slog.Default()
	decision := &Decision{Matched: false}

	// Should not panic and should be a no-op
	EmitSessionMatchedMarker(ctx, logger, decision)
}

func TestHandler_Enabled_NoDecision(t *testing.T) {
	inner := slog.NewTextHandler(nil, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	handler := WrapHandler(inner)
	ctx := context.Background()

	// Debug should be disabled without a decision
	if handler.Enabled(ctx, slog.LevelDebug) {
		t.Error("Enabled(Debug) should be false without decision")
	}

	// Info should be enabled
	if !handler.Enabled(ctx, slog.LevelInfo) {
		t.Error("Enabled(Info) should be true")
	}
}

func TestHandler_Enabled_WithMatchedDecision(t *testing.T) {
	inner := slog.NewTextHandler(nil, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	handler := WrapHandler(inner)
	ctx := context.Background()

	decision := &Decision{
		Matched:        true,
		EffectiveLevel: LevelDebug,
	}
	ctx = ContextWithDecision(ctx, decision)

	// Debug should be enabled with debug decision
	if !handler.Enabled(ctx, slog.LevelDebug) {
		t.Error("Enabled(Debug) should be true with debug decision")
	}
}

func TestHandler_addTrekAttrs(t *testing.T) {
	inner := slog.NewTextHandler(nil, nil)
	handler := WrapHandler(inner)

	record := slog.NewRecord(time.Now(), slog.LevelDebug, "test message", 0)
	decision := &Decision{
		Matched:   true,
		SessionID: "sess-test",
		Labels: map[string]string{
			"ticket": "TREK-123",
		},
	}

	newRecord := handler.addTrekAttrs(record, decision)

	// Verify attrs were added by checking attr count changed
	var attrCount int
	newRecord.Attrs(func(a slog.Attr) bool {
		attrCount++
		return true
	})

	if attrCount < 2 {
		t.Errorf("addTrekAttrs() should add at least 2 attrs, got %d", attrCount)
	}
}

package trek_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bold-minds/trek"
)

func TestCachePolling(t *testing.T) {
	var requestCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)

		resp := trek.ActiveSessionsResponse{
			Revision:   "rev1",
			ServerTime: time.Now(),
			Sessions: []trek.Session{
				{
					ID:        "sess_1",
					Selector:  trek.Selector{UserID: "u123"},
					Level:     trek.LevelDebug,
					ExpiresAt: time.Now().Add(10 * time.Minute),
					Labels:    map[string]string{"test": "true"},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := trek.NewClient(server.URL, "token", "org1", "prod")
	cache := trek.NewCache(client, "test-service", 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache.Start(ctx)

	time.Sleep(50 * time.Millisecond)

	sessions := cache.GetSessions()
	if len(sessions) != 1 {
		t.Errorf("expected 1 session, got %d", len(sessions))
	}

	if sessions[0].ID != "sess_1" {
		t.Errorf("expected sess_1, got %s", sessions[0].ID)
	}

	time.Sleep(150 * time.Millisecond)

	if requestCount.Load() < 2 {
		t.Errorf("expected at least 2 requests (polling), got %d", requestCount.Load())
	}

	cache.Stop()
}

func TestMiddlewareIntegration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := trek.ActiveSessionsResponse{
			Revision:   "rev1",
			ServerTime: time.Now(),
			Sessions: []trek.Session{
				{
					ID:        "sess_1",
					Selector:  trek.Selector{UserID: "u123"},
					Level:     trek.LevelDebug,
					ExpiresAt: time.Now().Add(10 * time.Minute),
					Labels:    map[string]string{},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := trek.NewClient(server.URL, "token", "org1", "prod")
	cache := trek.NewCache(client, "test-service", time.Hour)

	ctx := context.Background()
	cache.Refresh(ctx)

	extractor := trek.ContextExtractor{
		UserIDHeader:    "X-User-ID",
		RequestIDHeader: "X-Request-ID",
	}

	var capturedDecision *trek.Decision

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedDecision = trek.DecisionFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := trek.Middleware(cache, "test-service", extractor)
	wrappedHandler := middleware(handler)

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-User-ID", "u123")
	w := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(w, req)

	if capturedDecision == nil {
		t.Fatal("expected decision in context")
	}

	if !capturedDecision.Matched {
		t.Error("expected match")
	}

	if capturedDecision.SessionID != "sess_1" {
		t.Errorf("expected sess_1, got %s", capturedDecision.SessionID)
	}

	if capturedDecision.EffectiveLevel != trek.LevelDebug {
		t.Errorf("expected debug level, got %s", capturedDecision.EffectiveLevel)
	}
}

func TestHandlerLevelElevation(t *testing.T) {
	var logOutput []slog.Record

	innerHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})

	testHandler := &recordingHandler{inner: innerHandler, records: &logOutput}
	trekHandler := trek.WrapHandler(testHandler)

	logger := slog.New(trekHandler)

	decision := &trek.Decision{
		Matched:        true,
		SessionID:      "sess_1",
		EffectiveLevel: trek.LevelDebug,
		Labels:         map[string]string{},
		Caps:           trek.Caps{MaxDebugEventsPerRequest: 100},
	}
	ctx := trek.ContextWithDecision(context.Background(), decision)

	logger.DebugContext(ctx, "this should appear")

	if len(logOutput) != 1 {
		t.Errorf("expected 1 log record, got %d", len(logOutput))
	}
}

func TestHandlerNoElevationWithoutMatch(t *testing.T) {
	var logOutput []slog.Record

	testHandler := &recordingHandler{records: &logOutput, minLevel: slog.LevelInfo}
	trekHandler := trek.WrapHandler(testHandler)

	logger := slog.New(trekHandler)

	ctx := context.Background()

	logger.DebugContext(ctx, "this should not appear")

	if len(logOutput) != 0 {
		t.Errorf("expected 0 log records (no elevation), got %d", len(logOutput))
	}
}

func TestCapsEnforcement(t *testing.T) {
	var logOutput []slog.Record

	innerHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	testHandler := &recordingHandler{inner: innerHandler, records: &logOutput}
	trekHandler := trek.WrapHandler(testHandler)

	logger := slog.New(trekHandler)

	decision := &trek.Decision{
		Matched:        true,
		SessionID:      "sess_1",
		EffectiveLevel: trek.LevelDebug,
		Labels:         map[string]string{},
		Caps:           trek.Caps{MaxDebugEventsPerRequest: 3},
	}
	ctx := trek.ContextWithDecision(context.Background(), decision)

	for i := 0; i < 10; i++ {
		logger.DebugContext(ctx, "debug message", "i", i)
	}

	if len(logOutput) > 4 {
		t.Errorf("expected at most 4 records (3 + cap reached), got %d", len(logOutput))
	}
}

type recordingHandler struct {
	inner    slog.Handler
	records  *[]slog.Record
	minLevel slog.Level
}

func (h *recordingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.minLevel
}

func (h *recordingHandler) Handle(ctx context.Context, r slog.Record) error {
	*h.records = append(*h.records, r)
	return nil
}

func (h *recordingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *recordingHandler) WithGroup(name string) slog.Handler {
	return h
}

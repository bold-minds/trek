package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestNewNotifier(t *testing.T) {
	configs := []NotifyConfig{
		{Type: "slack", URL: "https://hooks.slack.com/test", Enabled: true},
		{Type: "webhook", URL: "https://example.com/webhook", Enabled: true},
	}

	notifier := NewNotifier(configs)

	if notifier == nil {
		t.Fatal("NewNotifier returned nil")
	}
	if notifier.httpClient == nil {
		t.Error("httpClient not initialized")
	}
	if len(notifier.configs) != len(configs) {
		t.Errorf("configs length = %d, want %d", len(notifier.configs), len(configs))
	}
}

func TestNotifier_NotifySessionCreated(t *testing.T) {
	var receivedRequests []struct {
		body    []byte
		headers http.Header
	}
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body []byte
		r.Body.Read(body)

		mu.Lock()
		receivedRequests = append(receivedRequests, struct {
			body    []byte
			headers http.Header
		}{body: body, headers: r.Header.Clone()})
		mu.Unlock()

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configs := []NotifyConfig{
		{Type: "slack", URL: server.URL, Enabled: true, OrgID: "org-1", EnvID: "dev"},
	}

	notifier := NewNotifier(configs)

	event := SessionCreatedEvent{
		SessionID: "sess-123",
		OrgID:     "org-1",
		EnvID:     "dev",
		Level:     "debug",
		Selector:  map[string]string{"user_id": "user-1"},
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedBy: "admin@example.com",
		Reason:    "Testing",
	}

	notifier.NotifySessionCreated(context.Background(), event)

	time.Sleep(100 * time.Millisecond)
}

func TestNotifier_NotifySessionRevoked(t *testing.T) {
	requestReceived := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived <- true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configs := []NotifyConfig{
		{Type: "slack", URL: server.URL, Enabled: true},
	}

	notifier := NewNotifier(configs)

	event := SessionRevokedEvent{
		SessionID: "sess-123",
		OrgID:     "org-1",
		EnvID:     "dev",
		RevokedBy: "admin@example.com",
	}

	notifier.NotifySessionRevoked(context.Background(), event)

	select {
	case <-requestReceived:
	case <-time.After(500 * time.Millisecond):
		t.Error("notification request not received")
	}
}

func TestNotifier_NotifySessionsExpired(t *testing.T) {
	t.Run("does not send notification for zero count", func(t *testing.T) {
		requestReceived := make(chan bool, 1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestReceived <- true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		configs := []NotifyConfig{
			{Type: "slack", URL: server.URL, Enabled: true},
		}

		notifier := NewNotifier(configs)
		notifier.NotifySessionsExpired(context.Background(), 0)

		select {
		case <-requestReceived:
			t.Error("should not send notification for zero count")
		case <-time.After(100 * time.Millisecond):
		}
	})

	t.Run("sends notification for non-zero count", func(t *testing.T) {
		requestReceived := make(chan bool, 1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestReceived <- true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		configs := []NotifyConfig{
			{Type: "slack", URL: server.URL, Enabled: true},
		}

		notifier := NewNotifier(configs)
		notifier.NotifySessionsExpired(context.Background(), 5)

		select {
		case <-requestReceived:
		case <-time.After(500 * time.Millisecond):
			t.Error("notification request not received")
		}
	})
}

func TestNotifier_NotifyPolicyUpdated(t *testing.T) {
	requestReceived := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestReceived <- true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configs := []NotifyConfig{
		{Type: "slack", URL: server.URL, Enabled: true},
	}

	notifier := NewNotifier(configs)

	event := PolicyUpdatedEvent{
		OrgID:     "org-1",
		EnvID:     "dev",
		UpdatedBy: "admin@example.com",
		Changes:   map[string]any{"max_ttl_seconds": 7200},
	}

	notifier.NotifyPolicyUpdated(context.Background(), event)

	select {
	case <-requestReceived:
	case <-time.After(500 * time.Millisecond):
		t.Error("notification request not received")
	}
}

func TestNotifier_ConfigFiltering(t *testing.T) {
	t.Run("filters by org ID", func(t *testing.T) {
		requestCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestCount++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		configs := []NotifyConfig{
			{Type: "slack", URL: server.URL, Enabled: true, OrgID: "org-1"},
			{Type: "slack", URL: server.URL, Enabled: true, OrgID: "org-2"},
		}

		notifier := NewNotifier(configs)

		event := SessionCreatedEvent{
			SessionID: "sess-123",
			OrgID:     "org-1",
			EnvID:     "dev",
		}

		notifier.NotifySessionCreated(context.Background(), event)

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		if requestCount != 1 {
			t.Errorf("requestCount = %d, want 1", requestCount)
		}
		mu.Unlock()
	})

	t.Run("filters by env ID", func(t *testing.T) {
		requestCount := 0
		var mu sync.Mutex

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestCount++
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		configs := []NotifyConfig{
			{Type: "slack", URL: server.URL, Enabled: true, EnvID: "dev"},
			{Type: "slack", URL: server.URL, Enabled: true, EnvID: "prod"},
		}

		notifier := NewNotifier(configs)

		event := SessionCreatedEvent{
			SessionID: "sess-123",
			OrgID:     "org-1",
			EnvID:     "dev",
		}

		notifier.NotifySessionCreated(context.Background(), event)

		time.Sleep(100 * time.Millisecond)

		mu.Lock()
		if requestCount != 1 {
			t.Errorf("requestCount = %d, want 1", requestCount)
		}
		mu.Unlock()
	})

	t.Run("skips disabled configs", func(t *testing.T) {
		requestReceived := make(chan bool, 1)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestReceived <- true
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		configs := []NotifyConfig{
			{Type: "slack", URL: server.URL, Enabled: false},
		}

		notifier := NewNotifier(configs)

		event := SessionCreatedEvent{
			SessionID: "sess-123",
			OrgID:     "org-1",
			EnvID:     "dev",
		}

		notifier.NotifySessionCreated(context.Background(), event)

		select {
		case <-requestReceived:
			t.Error("should not send notification when disabled")
		case <-time.After(100 * time.Millisecond):
		}
	})
}

func TestNotifier_WebhookFormat(t *testing.T) {
	var receivedPayload WebhookPayload
	payloadReceived := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q, want %q", r.Header.Get("Authorization"), "Bearer test-token")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedPayload); err != nil {
			t.Errorf("failed to decode payload: %v", err)
		}

		payloadReceived <- true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configs := []NotifyConfig{
		{Type: "webhook", URL: server.URL, Enabled: true, AuthHeader: "Bearer test-token"},
	}

	notifier := NewNotifier(configs)

	event := SessionCreatedEvent{
		SessionID: "sess-123",
		OrgID:     "org-1",
		EnvID:     "dev",
	}

	notifier.NotifySessionCreated(context.Background(), event)

	select {
	case <-payloadReceived:
		if receivedPayload.EventType != "session.created" {
			t.Errorf("EventType = %q, want %q", receivedPayload.EventType, "session.created")
		}
		if receivedPayload.Timestamp.IsZero() {
			t.Error("Timestamp should not be zero")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("webhook request not received")
	}
}

func TestNotifier_SlackFormat(t *testing.T) {
	var receivedMessage SlackMessage
	messageReceived := make(chan bool, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want %q", r.Header.Get("Content-Type"), "application/json")
		}

		if err := json.NewDecoder(r.Body).Decode(&receivedMessage); err != nil {
			t.Errorf("failed to decode message: %v", err)
		}

		messageReceived <- true
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	configs := []NotifyConfig{
		{Type: "slack", URL: server.URL, Enabled: true},
	}

	notifier := NewNotifier(configs)

	event := SessionCreatedEvent{
		SessionID: "sess-123",
		OrgID:     "org-1",
		EnvID:     "dev",
		Level:     "debug",
		CreatedBy: "admin@example.com",
		Reason:    "Testing",
		ExpiresAt: time.Now().Add(time.Hour),
	}

	notifier.NotifySessionCreated(context.Background(), event)

	select {
	case <-messageReceived:
		if receivedMessage.Text == "" {
			t.Error("Text should not be empty")
		}
		if len(receivedMessage.Attachments) == 0 {
			t.Error("Attachments should not be empty")
		}
	case <-time.After(500 * time.Millisecond):
		t.Error("slack request not received")
	}
}

func TestNotifier_ErrorHandling(t *testing.T) {
	t.Run("handles slack error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		configs := []NotifyConfig{
			{Type: "slack", URL: server.URL, Enabled: true},
		}

		notifier := NewNotifier(configs)

		event := SessionCreatedEvent{
			SessionID: "sess-123",
			OrgID:     "org-1",
			EnvID:     "dev",
		}

		notifier.NotifySessionCreated(context.Background(), event)
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("handles webhook error response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		configs := []NotifyConfig{
			{Type: "webhook", URL: server.URL, Enabled: true},
		}

		notifier := NewNotifier(configs)

		event := SessionCreatedEvent{
			SessionID: "sess-123",
			OrgID:     "org-1",
			EnvID:     "dev",
		}

		notifier.NotifySessionCreated(context.Background(), event)
		time.Sleep(100 * time.Millisecond)
	})

	t.Run("handles connection error", func(t *testing.T) {
		configs := []NotifyConfig{
			{Type: "slack", URL: "http://localhost:99999", Enabled: true},
		}

		notifier := NewNotifier(configs)

		event := SessionCreatedEvent{
			SessionID: "sess-123",
			OrgID:     "org-1",
			EnvID:     "dev",
		}

		notifier.NotifySessionCreated(context.Background(), event)
		time.Sleep(100 * time.Millisecond)
	})
}

func TestFormatSelector(t *testing.T) {
	tests := []struct {
		name     string
		selector map[string]string
		want     string
	}{
		{
			name:     "empty selector",
			selector: map[string]string{},
			want:     "(empty)",
		},
		{
			name:     "nil selector",
			selector: nil,
			want:     "(empty)",
		},
		{
			name:     "single field",
			selector: map[string]string{"user_id": "user-1"},
			want:     `{"user_id":"user-1"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSelector(tt.selector)
			if got != tt.want {
				t.Errorf("formatSelector() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatPolicyChanges(t *testing.T) {
	tests := []struct {
		name    string
		changes map[string]any
		want    string
	}{
		{
			name:    "empty changes",
			changes: map[string]any{},
			want:    "(no changes)",
		},
		{
			name:    "nil changes",
			changes: nil,
			want:    "(no changes)",
		},
		{
			name:    "with changes",
			changes: map[string]any{"max_ttl_seconds": 3600},
			want:    `{"max_ttl_seconds":3600}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatPolicyChanges(tt.changes)
			if got != tt.want {
				t.Errorf("formatPolicyChanges() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNotifyConfig(t *testing.T) {
	config := NotifyConfig{
		Type:       "slack",
		URL:        "https://hooks.slack.com/test",
		Enabled:    true,
		OrgID:      "org-1",
		EnvID:      "dev",
		AuthHeader: "Bearer token",
	}

	if config.Type != "slack" {
		t.Errorf("Type = %q, want %q", config.Type, "slack")
	}
	if !config.Enabled {
		t.Error("Enabled should be true")
	}
}

func TestSlackMessage(t *testing.T) {
	message := SlackMessage{
		Text: "Test message",
		Attachments: []SlackAttachment{
			{
				Color: "#36a64f",
				Fields: []SlackField{
					{Title: "Field 1", Value: "Value 1", Short: true},
				},
			},
		},
	}

	if message.Text != "Test message" {
		t.Errorf("Text = %q, want %q", message.Text, "Test message")
	}
	if len(message.Attachments) != 1 {
		t.Errorf("len(Attachments) = %d, want 1", len(message.Attachments))
	}
}

func TestWebhookPayload(t *testing.T) {
	payload := WebhookPayload{
		EventType: "session.created",
		Timestamp: time.Now(),
		Data:      map[string]string{"session_id": "sess-1"},
	}

	if payload.EventType != "session.created" {
		t.Errorf("EventType = %q, want %q", payload.EventType, "session.created")
	}
	if payload.Timestamp.IsZero() {
		t.Error("Timestamp should not be zero")
	}
}

func TestSessionCreatedEvent(t *testing.T) {
	event := SessionCreatedEvent{
		SessionID: "sess-1",
		OrgID:     "org-1",
		EnvID:     "dev",
		Level:     "debug",
		Selector:  map[string]string{"user_id": "user-1"},
		ExpiresAt: time.Now().Add(time.Hour),
		CreatedBy: "admin@example.com",
		Reason:    "Testing",
	}

	if event.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", event.SessionID, "sess-1")
	}
}

func TestSessionRevokedEvent(t *testing.T) {
	event := SessionRevokedEvent{
		SessionID: "sess-1",
		OrgID:     "org-1",
		EnvID:     "dev",
		RevokedBy: "admin@example.com",
	}

	if event.SessionID != "sess-1" {
		t.Errorf("SessionID = %q, want %q", event.SessionID, "sess-1")
	}
}

func TestPolicyUpdatedEvent(t *testing.T) {
	event := PolicyUpdatedEvent{
		OrgID:     "org-1",
		EnvID:     "dev",
		UpdatedBy: "admin@example.com",
		Changes:   map[string]any{"max_ttl_seconds": 7200},
	}

	if event.OrgID != "org-1" {
		t.Errorf("OrgID = %q, want %q", event.OrgID, "org-1")
	}
}

package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Notifier sends notifications via Slack and webhooks.
type Notifier struct {
	httpClient *http.Client
	configs    []NotifyConfig
}

// NotifyConfig holds notification configuration.
type NotifyConfig struct {
	Type       string // "slack" or "webhook"
	URL        string
	Enabled    bool
	OrgID      string
	EnvID      string
	AuthHeader string // For webhook auth
}

// NewNotifier creates a new notifier.
func NewNotifier(configs []NotifyConfig) *Notifier {
	return &Notifier{
		httpClient: &http.Client{Timeout: 10 * time.Second},
		configs:    configs,
	}
}

// NotifySessionCreated sends notification when a session is created.
func (n *Notifier) NotifySessionCreated(ctx context.Context, event SessionCreatedEvent) {
	message := SlackMessage{
		Text: fmt.Sprintf("🔍 Debug session created"),
		Attachments: []SlackAttachment{
			{
				Color: "#36a64f",
				Fields: []SlackField{
					{Title: "Session ID", Value: event.SessionID, Short: true},
					{Title: "Level", Value: event.Level, Short: true},
					{Title: "Created By", Value: event.CreatedBy, Short: true},
					{Title: "Expires", Value: event.ExpiresAt.Format(time.RFC3339), Short: true},
					{Title: "Reason", Value: event.Reason, Short: false},
					{Title: "Selector", Value: formatSelector(event.Selector), Short: false},
				},
			},
		},
	}

	n.send(ctx, "session.created", event.OrgID, event.EnvID, message, event)
}

// NotifySessionRevoked sends notification when a session is revoked.
func (n *Notifier) NotifySessionRevoked(ctx context.Context, event SessionRevokedEvent) {
	message := SlackMessage{
		Text: fmt.Sprintf("⏹️ Debug session revoked"),
		Attachments: []SlackAttachment{
			{
				Color: "#ff9900",
				Fields: []SlackField{
					{Title: "Session ID", Value: event.SessionID, Short: true},
					{Title: "Revoked By", Value: event.RevokedBy, Short: true},
				},
			},
		},
	}

	n.send(ctx, "session.revoked", event.OrgID, event.EnvID, message, event)
}

// NotifySessionsExpired sends notification when sessions expire.
func (n *Notifier) NotifySessionsExpired(ctx context.Context, count int) {
	if count == 0 {
		return
	}

	message := SlackMessage{
		Text: fmt.Sprintf("⏰ %d debug session(s) expired", count),
	}

	n.send(ctx, "sessions.expired", "", "", message, map[string]int{"count": count})
}

// NotifyPolicyUpdated sends notification when a policy is updated.
func (n *Notifier) NotifyPolicyUpdated(ctx context.Context, event PolicyUpdatedEvent) {
	message := SlackMessage{
		Text: fmt.Sprintf("📋 Policy updated"),
		Attachments: []SlackAttachment{
			{
				Color: "#0066cc",
				Fields: []SlackField{
					{Title: "Organization", Value: event.OrgID, Short: true},
					{Title: "Environment", Value: event.EnvID, Short: true},
					{Title: "Updated By", Value: event.UpdatedBy, Short: true},
					{Title: "Changes", Value: formatPolicyChanges(event.Changes), Short: false},
				},
			},
		},
	}

	n.send(ctx, "policy.updated", event.OrgID, event.EnvID, message, event)
}

// PolicyUpdatedEvent is emitted when a policy is updated.
type PolicyUpdatedEvent struct {
	OrgID     string         `json:"org_id"`
	EnvID     string         `json:"env_id"`
	UpdatedBy string         `json:"updated_by"`
	Changes   map[string]any `json:"changes"`
}

func formatPolicyChanges(changes map[string]any) string {
	if len(changes) == 0 {
		return "(no changes)"
	}
	b, _ := json.Marshal(changes)
	return string(b)
}

func (n *Notifier) send(ctx context.Context, eventType, orgID, envID string, slackMsg SlackMessage, webhookPayload any) {
	for _, cfg := range n.configs {
		if !cfg.Enabled {
			continue
		}

		if cfg.OrgID != "" && cfg.OrgID != orgID {
			continue
		}
		if cfg.EnvID != "" && cfg.EnvID != envID {
			continue
		}

		go func(cfg NotifyConfig) {
			var err error
			switch cfg.Type {
			case "slack":
				err = n.sendSlack(ctx, cfg.URL, slackMsg)
			case "webhook":
				err = n.sendWebhook(ctx, cfg.URL, cfg.AuthHeader, eventType, webhookPayload)
			}
			if err != nil {
				slog.Error("notification failed",
					"type", cfg.Type,
					"event", eventType,
					"error", err,
				)
			}
		}(cfg)
	}
}

func (n *Notifier) sendSlack(ctx context.Context, webhookURL string, message SlackMessage) error {
	body, err := json.Marshal(message)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhookURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack returned %d", resp.StatusCode)
	}

	return nil
}

func (n *Notifier) sendWebhook(ctx context.Context, url, authHeader, eventType string, payload any) error {
	webhookPayload := WebhookPayload{
		EventType: eventType,
		Timestamp: time.Now(),
		Data:      payload,
	}

	body, err := json.Marshal(webhookPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook returned %d", resp.StatusCode)
	}

	return nil
}

// SlackMessage represents a Slack webhook message.
type SlackMessage struct {
	Text        string            `json:"text"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment represents a Slack attachment.
type SlackAttachment struct {
	Color  string       `json:"color,omitempty"`
	Fields []SlackField `json:"fields,omitempty"`
}

// SlackField represents a Slack field.
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// WebhookPayload is the generic webhook payload.
type WebhookPayload struct {
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Data      any       `json:"data"`
}

// SessionCreatedEvent is emitted when a session is created.
type SessionCreatedEvent struct {
	SessionID string            `json:"session_id"`
	OrgID     string            `json:"org_id"`
	EnvID     string            `json:"env_id"`
	Level     string            `json:"level"`
	Selector  map[string]string `json:"selector"`
	ExpiresAt time.Time         `json:"expires_at"`
	CreatedBy string            `json:"created_by"`
	Reason    string            `json:"reason"`
}

// SessionRevokedEvent is emitted when a session is revoked.
type SessionRevokedEvent struct {
	SessionID string `json:"session_id"`
	OrgID     string `json:"org_id"`
	EnvID     string `json:"env_id"`
	RevokedBy string `json:"revoked_by"`
}

func formatSelector(sel map[string]string) string {
	if len(sel) == 0 {
		return "(empty)"
	}
	b, _ := json.Marshal(sel)
	return string(b)
}

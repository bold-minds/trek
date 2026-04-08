package trek

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// Client is an HTTP client for the Trek control plane API.
type Client struct {
	baseURL    string
	token      string
	orgID      string
	env        string
	httpClient *http.Client
}

// NewClient creates a new Trek API client with automatic retries.
func NewClient(baseURL, token, orgID, env string) *Client {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.RetryWaitMin = 100 * time.Millisecond
	retryClient.RetryWaitMax = 2 * time.Second
	retryClient.Logger = &leveledLogger{}
	retryClient.CheckRetry = retryPolicy
	retryClient.HTTPClient.Timeout = 10 * time.Second

	return &Client{
		baseURL:    baseURL,
		token:      token,
		orgID:      orgID,
		env:        env,
		httpClient: retryClient.StandardClient(),
	}
}

// NewClientWithHTTP creates a new Trek API client with a custom HTTP client.
// Note: Custom clients bypass the automatic retry logic.
func NewClientWithHTTP(baseURL, token, orgID, env string, httpClient *http.Client) *Client {
	return &Client{
		baseURL:    baseURL,
		token:      token,
		orgID:      orgID,
		env:        env,
		httpClient: httpClient,
	}
}

// retryPolicy determines if a request should be retried.
func retryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if err != nil {
		return true, nil
	}
	if resp.StatusCode == 0 || resp.StatusCode >= 500 || resp.StatusCode == 429 {
		return true, nil
	}
	return false, nil
}

// leveledLogger adapts slog to retryablehttp's LeveledLogger interface.
type leveledLogger struct{}

func (l *leveledLogger) Error(msg string, keysAndValues ...interface{}) {
	slog.Error(msg, keysAndValues...)
}

func (l *leveledLogger) Info(msg string, keysAndValues ...interface{}) {
	slog.Info(msg, keysAndValues...)
}

func (l *leveledLogger) Debug(msg string, keysAndValues ...interface{}) {
	slog.Debug(msg, keysAndValues...)
}

func (l *leveledLogger) Warn(msg string, keysAndValues ...interface{}) {
	slog.Warn(msg, keysAndValues...)
}

// GetActiveSessions fetches active sessions from the control plane.
// If revision matches current, returns nil (no update needed).
func (c *Client) GetActiveSessions(ctx context.Context, serviceName, currentRevision string) (*ActiveSessionsResponse, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/active-sessions", c.baseURL, c.orgID, c.env)

	params := url.Values{}
	if serviceName != "" {
		params.Set("service_name", serviceName)
	}
	if currentRevision != "" {
		params.Set("revision", currentRevision)
	}

	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotModified {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	var result ActiveSessionsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// CreateSession creates a new debug session.
func (c *Client) CreateSession(ctx context.Context, req CreateSessionRequest) (*CreateSessionResponse, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/sessions", c.baseURL, c.orgID, c.env)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
	}

	var result CreateSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// RevokeSession revokes an active debug session.
func (c *Client) RevokeSession(ctx context.Context, sessionID string) error {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/sessions/%s/revoke", c.baseURL, c.orgID, c.env, sessionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	return nil
}

// GetSession fetches a single session by ID.
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/sessions/%s", c.baseURL, c.orgID, c.env, sessionID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	var result Session
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ExtendSession extends the TTL of an active session.
func (c *Client) ExtendSession(ctx context.Context, sessionID string, additionalTTLSeconds int) (*Session, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/sessions/%s/extend", c.baseURL, c.orgID, c.env, sessionID)

	body, err := json.Marshal(map[string]int{"additional_ttl_seconds": additionalTTLSeconds})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
	}

	var result Session
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ListSessions lists sessions with optional status filter.
func (c *Client) ListSessions(ctx context.Context, status string) ([]Session, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/sessions", c.baseURL, c.orgID, c.env)

	if status != "" {
		endpoint += "?status=" + url.QueryEscape(status)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	var result struct {
		Sessions []Session `json:"sessions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Sessions, nil
}

// CreateToken creates a new service token.
func (c *Client) CreateToken(ctx context.Context, name string) (*TokenResponse, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/tokens", c.baseURL, c.orgID, c.env)

	body, err := json.Marshal(map[string]string{"name": name})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	var result TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// ListTokens lists service tokens.
func (c *Client) ListTokens(ctx context.Context) ([]TokenResponse, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/tokens", c.baseURL, c.orgID, c.env)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result struct {
		Tokens []TokenResponse `json:"tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Tokens, nil
}

// RevokeToken revokes a service token.
func (c *Client) RevokeToken(ctx context.Context, tokenID string) error {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/tokens/%s", c.baseURL, c.orgID, c.env, tokenID)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	return nil
}

// TokenResponse represents a token API response.
type TokenResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Token     string    `json:"token,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// CheckCaps checks and optionally increments global cap counters.
func (c *Client) CheckCaps(ctx context.Context, req CapsCheckRequest) (*CapsCheckResponse, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/caps/check", c.baseURL, c.orgID, c.env)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Authorization", "Bearer "+c.token)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB limit
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(respBody)}
	}

	var result CapsCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// IncrementCaps increments global cap counters (fire-and-forget friendly).
func (c *Client) IncrementCaps(ctx context.Context, sessionID, requestID string, maxPerSession, maxPerRequest int) (*CapsCheckResponse, error) {
	return c.CheckCaps(ctx, CapsCheckRequest{
		SessionID:        sessionID,
		RequestID:        requestID,
		MaxPerSession:    maxPerSession,
		MaxPerRequest:    maxPerRequest,
		IncrementSession: true,
		IncrementRequest: true,
	})
}

// CapsCheckRequest is the request for cap checking.
type CapsCheckRequest struct {
	SessionID        string `json:"session_id"`
	RequestID        string `json:"request_id"`
	MaxPerSession    int    `json:"max_per_session"`
	MaxPerRequest    int    `json:"max_per_request"`
	IncrementSession bool   `json:"increment_session"`
	IncrementRequest bool   `json:"increment_request"`
}

// CapsCheckResponse is the response from cap checking.
type CapsCheckResponse struct {
	Allowed           bool  `json:"allowed"`
	SessionCount      int64 `json:"session_count"`
	RequestCount      int64 `json:"request_count"`
	SessionCapReached bool  `json:"session_cap_reached"`
	RequestCapReached bool  `json:"request_cap_reached"`
}

// GetPolicy fetches the policy for the current org/env.
func (c *Client) GetPolicy(ctx context.Context) (*Policy, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/policies", c.baseURL, c.orgID, c.env)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result Policy
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// Policy represents org/env policy settings.
type Policy struct {
	ID                  string   `json:"id"`
	OrgID               string   `json:"org_id"`
	EnvID               string   `json:"env_id"`
	MaxTTLSeconds       int      `json:"max_ttl_seconds"`
	AllowEmptySelector  bool     `json:"allow_empty_selector"`
	AllowedSelectorKeys []string `json:"allowed_selector_keys,omitempty"`
	DefaultCaps         Caps     `json:"default_caps"`
	RequireReason       bool     `json:"require_reason"`
}

// ListAuditEvents fetches audit events for the current org/env.
func (c *Client) ListAuditEvents(ctx context.Context, cursor string) (*AuditEventsResponse, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs/%s/audit", c.baseURL, c.orgID, c.env)

	if cursor != "" {
		endpoint += "?cursor=" + url.QueryEscape(cursor)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result AuditEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// AuditEventsResponse is the response from the audit events endpoint.
type AuditEventsResponse struct {
	Events     []AuditEvent `json:"events"`
	NextCursor string       `json:"next_cursor,omitempty"`
}

// AuditEvent represents an audit log entry.
type AuditEvent struct {
	ID          string                 `json:"id"`
	OrgID       string                 `json:"org_id"`
	EnvID       string                 `json:"env_id"`
	ActorUserID string                 `json:"actor_user_id,omitempty"`
	Action      string                 `json:"action"`
	TargetType  string                 `json:"target_type"`
	TargetID    string                 `json:"target_id"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
}

// ListEnvironments fetches available environments for the org.
func (c *Client) ListEnvironments(ctx context.Context) ([]Environment, error) {
	endpoint := fmt.Sprintf("%s/orgs/%s/envs", c.baseURL, c.orgID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, &APIError{StatusCode: resp.StatusCode, Message: string(body)}
	}

	var result struct {
		Environments []Environment `json:"environments"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return result.Environments, nil
}

// Environment represents an environment.
type Environment struct {
	ID        string    `json:"id"`
	OrgID     string    `json:"org_id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// APIError represents an error response from the Trek API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("trek api error: status %d: %s", e.StatusCode, e.Message)
}

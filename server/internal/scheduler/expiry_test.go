package scheduler

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bold-minds/trek"
	"github.com/bold-minds/trek/server/internal/notify"
	"github.com/bold-minds/trek/server/internal/store"
)

type mockExpireStore struct {
	expireCount   atomic.Int32
	expiredReturn int
}

func (m *mockExpireStore) ExpireSessions(ctx context.Context) (int, error) {
	m.expireCount.Add(1)
	return m.expiredReturn, nil
}

// Implement other store methods as no-ops
func (m *mockExpireStore) CreateSession(ctx context.Context, session *store.Session) error {
	return nil
}
func (m *mockExpireStore) GetSession(ctx context.Context, orgID, envID, sessionID string) (*store.Session, error) {
	return nil, store.ErrNotFound
}
func (m *mockExpireStore) ListSessions(ctx context.Context, orgID, envID string, filter store.SessionFilter) ([]store.Session, error) {
	return nil, nil
}
func (m *mockExpireStore) GetActiveSessions(ctx context.Context, orgID, envID, serviceName string) ([]trek.Session, error) {
	return nil, nil
}
func (m *mockExpireStore) RevokeSession(ctx context.Context, orgID, envID, sessionID string) error {
	return nil
}
func (m *mockExpireStore) GetPolicy(ctx context.Context, orgID, envID string) (*store.Policy, error) {
	return nil, nil
}
func (m *mockExpireStore) SetPolicy(ctx context.Context, policy *store.Policy) error { return nil }
func (m *mockExpireStore) CreateAuditEvent(ctx context.Context, event *store.AuditEvent) error {
	return nil
}
func (m *mockExpireStore) ListAuditEvents(ctx context.Context, orgID, envID string, limit int, cursor string) ([]store.AuditEvent, string, error) {
	return nil, "", nil
}
func (m *mockExpireStore) ValidateToken(ctx context.Context, tokenHash string) (*store.ServiceToken, error) {
	return nil, store.ErrNotFound
}
func (m *mockExpireStore) ValidateTokenPlain(ctx context.Context, plainToken string) (*store.ServiceToken, error) {
	return nil, store.ErrNotFound
}
func (m *mockExpireStore) CreateToken(ctx context.Context, token *store.ServiceToken) error {
	return nil
}
func (m *mockExpireStore) RevokeToken(ctx context.Context, orgID, envID, tokenID string) error {
	return nil
}
func (m *mockExpireStore) ListTokens(ctx context.Context, orgID, envID string) ([]store.ServiceToken, error) {
	return nil, nil
}
func (m *mockExpireStore) GetOrg(ctx context.Context, orgID string) (*store.Org, error) {
	return nil, store.ErrNotFound
}
func (m *mockExpireStore) GetEnv(ctx context.Context, orgID, envID string) (*store.Env, error) {
	return nil, store.ErrNotFound
}
func (m *mockExpireStore) ListEnvs(ctx context.Context, orgID string) ([]store.Env, error) {
	return nil, nil
}
func (m *mockExpireStore) CreateEnv(ctx context.Context, env *store.Env) error      { return nil }
func (m *mockExpireStore) DeleteEnv(ctx context.Context, orgID, envID string) error { return nil }
func (m *mockExpireStore) ListUsers(ctx context.Context, orgID string) ([]store.User, error) {
	return nil, nil
}
func (m *mockExpireStore) CreateUser(ctx context.Context, user *store.User) error     { return nil }
func (m *mockExpireStore) DeleteUser(ctx context.Context, orgID, userID string) error { return nil }
func (m *mockExpireStore) ListRoles(ctx context.Context, orgID string) ([]store.Role, error) {
	return nil, nil
}
func (m *mockExpireStore) CreateRole(ctx context.Context, role *store.Role) error { return nil }
func (m *mockExpireStore) AssignRole(ctx context.Context, orgID, userID, roleID, envID string) error {
	return nil
}
func (m *mockExpireStore) RevokeRole(ctx context.Context, orgID, userID, roleID, envID string) error {
	return nil
}
func (m *mockExpireStore) GetUserRoles(ctx context.Context, orgID, userID string) ([]store.UserRole, error) {
	return nil, nil
}
func (m *mockExpireStore) GetUserPermissions(ctx context.Context, orgID, userID, envID string) ([]string, error) {
	return nil, nil
}
func (m *mockExpireStore) ListNotificationConfigs(ctx context.Context, orgID, envID string) ([]store.NotificationConfig, error) {
	return nil, nil
}
func (m *mockExpireStore) CreateNotificationConfig(ctx context.Context, config *store.NotificationConfig) error {
	return nil
}
func (m *mockExpireStore) UpdateNotificationConfig(ctx context.Context, orgID, configID string, config map[string]any, enabled *bool) error {
	return nil
}
func (m *mockExpireStore) DeleteNotificationConfig(ctx context.Context, orgID, configID string) error {
	return nil
}

func (m *mockExpireStore) Ping(ctx context.Context) error {
	return nil
}

func TestExpirySchedulerRunOnce(t *testing.T) {
	mockStore := &mockExpireStore{expiredReturn: 5}
	scheduler := NewExpiryScheduler(mockStore, nil, time.Second)

	count, err := scheduler.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if count != 5 {
		t.Errorf("expected 5 expired, got %d", count)
	}

	if mockStore.expireCount.Load() != 1 {
		t.Errorf("expected ExpireSessions called once, got %d", mockStore.expireCount.Load())
	}
}

func TestExpirySchedulerDefaultInterval(t *testing.T) {
	scheduler := NewExpiryScheduler(nil, nil, 0)

	if scheduler.interval != 30*time.Second {
		t.Errorf("expected default interval 30s, got %v", scheduler.interval)
	}
}

func TestExpirySchedulerStartStop(t *testing.T) {
	mockStore := &mockExpireStore{expiredReturn: 0}
	scheduler := NewExpiryScheduler(mockStore, nil, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go scheduler.Start(ctx)

	// Wait for a few sweeps
	time.Sleep(150 * time.Millisecond)

	scheduler.Stop()

	// Should have run at least 2 times
	count := mockStore.expireCount.Load()
	if count < 2 {
		t.Errorf("expected at least 2 sweeps, got %d", count)
	}
}

func TestExpirySchedulerWithNotifier(t *testing.T) {
	mockStore := &mockExpireStore{expiredReturn: 3}

	// Create a notifier (we can't easily test it sends, but we verify no panic)
	notifier := notify.NewNotifier(nil)

	scheduler := NewExpiryScheduler(mockStore, notifier, time.Second)

	count, err := scheduler.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	if count != 3 {
		t.Errorf("expected 3 expired, got %d", count)
	}
}

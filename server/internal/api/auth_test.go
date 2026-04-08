package api

import (
	"context"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/bold-minds/trek/server/internal/store"
	"github.com/gofiber/fiber/v2"
)

type mockStoreForAuth struct {
	store.Store
	validateTokenFn      func(ctx context.Context, token string) (*store.ServiceToken, error)
	getOrgFn             func(ctx context.Context, orgID string) (*store.Org, error)
	getEnvFn             func(ctx context.Context, orgID, envID string) (*store.Env, error)
	getUserPermissionsFn func(ctx context.Context, orgID, userID, envID string) ([]string, error)
}

func (m *mockStoreForAuth) ValidateTokenPlain(ctx context.Context, token string) (*store.ServiceToken, error) {
	if m.validateTokenFn != nil {
		return m.validateTokenFn(ctx, token)
	}
	return nil, store.ErrNotFound
}

func (m *mockStoreForAuth) GetOrg(ctx context.Context, orgID string) (*store.Org, error) {
	if m.getOrgFn != nil {
		return m.getOrgFn(ctx, orgID)
	}
	return nil, store.ErrNotFound
}

func (m *mockStoreForAuth) GetEnv(ctx context.Context, orgID, envID string) (*store.Env, error) {
	if m.getEnvFn != nil {
		return m.getEnvFn(ctx, orgID, envID)
	}
	return nil, store.ErrNotFound
}

func (m *mockStoreForAuth) GetUserPermissions(ctx context.Context, orgID, userID, envID string) ([]string, error) {
	if m.getUserPermissionsFn != nil {
		return m.getUserPermissionsFn(ctx, orgID, userID, envID)
	}
	return nil, nil
}

func TestNewAuthMiddleware(t *testing.T) {
	mockStore := &mockStoreForAuth{}
	middleware := NewAuthMiddleware(mockStore)

	if middleware == nil {
		t.Fatal("NewAuthMiddleware returned nil")
	}
	if middleware.store != mockStore {
		t.Error("store not set correctly")
	}
}

func TestAuthMiddleware_ServiceToken(t *testing.T) {
	tests := []struct {
		name           string
		authHeader     string
		orgParam       string
		envParam       string
		mockToken      *store.ServiceToken
		mockErr        error
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "missing authorization header",
			authHeader:     "",
			orgParam:       "org-1",
			expectedStatus: fiber.StatusUnauthorized,
			expectedCode:   "unauthorized",
		},
		{
			name:           "invalid authorization format - no Bearer",
			authHeader:     "Basic dXNlcjpwYXNz",
			orgParam:       "org-1",
			expectedStatus: fiber.StatusUnauthorized,
			expectedCode:   "unauthorized",
		},
		{
			name:           "invalid token - not found",
			authHeader:     "Bearer invalid-token",
			orgParam:       "org-1",
			mockErr:        store.ErrNotFound,
			expectedStatus: fiber.StatusUnauthorized,
			expectedCode:   "unauthorized",
		},
		{
			name:           "internal error on token validation",
			authHeader:     "Bearer some-token",
			orgParam:       "org-1",
			mockErr:        errors.New("database error"),
			expectedStatus: fiber.StatusInternalServerError,
			expectedCode:   "internal_error",
		},
		{
			name:       "token org mismatch",
			authHeader: "Bearer valid-token",
			orgParam:   "other-org",
			envParam:   "dev",
			mockToken: &store.ServiceToken{
				ID:    "token-1",
				OrgID: "org-1",
				EnvID: "dev",
			},
			expectedStatus: fiber.StatusForbidden,
			expectedCode:   "forbidden",
		},
		{
			name:       "token env mismatch",
			authHeader: "Bearer valid-token",
			orgParam:   "org-1",
			envParam:   "prod",
			mockToken: &store.ServiceToken{
				ID:    "token-1",
				OrgID: "org-1",
				EnvID: "dev",
			},
			expectedStatus: fiber.StatusForbidden,
			expectedCode:   "forbidden",
		},
		{
			name:       "valid token with matching org and env",
			authHeader: "Bearer valid-token",
			orgParam:   "org-1",
			envParam:   "dev",
			mockToken: &store.ServiceToken{
				ID:    "token-1",
				OrgID: "org-1",
				EnvID: "dev",
			},
			expectedStatus: fiber.StatusOK,
		},
		{
			name:       "valid token with matching org and no env param",
			authHeader: "Bearer valid-token",
			orgParam:   "org-1",
			envParam:   "",
			mockToken: &store.ServiceToken{
				ID:    "token-1",
				OrgID: "org-1",
				EnvID: "dev",
			},
			expectedStatus: fiber.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStoreForAuth{
				validateTokenFn: func(ctx context.Context, token string) (*store.ServiceToken, error) {
					if tt.mockErr != nil {
						return nil, tt.mockErr
					}
					return tt.mockToken, nil
				},
			}

			app := fiber.New()
			middleware := NewAuthMiddleware(mockStore)

			app.Get("/orgs/:org/envs/:env/test", middleware.ServiceToken, func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})
			app.Get("/orgs/:org/test", middleware.ServiceToken, func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			var path string
			if tt.envParam != "" {
				path = "/orgs/" + tt.orgParam + "/envs/" + tt.envParam + "/test"
			} else {
				path = "/orgs/" + tt.orgParam + "/test"
			}

			req := httptest.NewRequest("GET", path, nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d, body: %s", resp.StatusCode, tt.expectedStatus, body)
			}
		})
	}
}

func TestAuthMiddleware_RequireOrgEnv(t *testing.T) {
	tests := []struct {
		name           string
		orgParam       string
		envParam       string
		orgExists      bool
		envExists      bool
		orgErr         error
		envErr         error
		expectedStatus int
	}{
		{
			name:           "org not found",
			orgParam:       "nonexistent-org",
			envParam:       "dev",
			orgExists:      false,
			expectedStatus: fiber.StatusNotFound,
		},
		{
			name:           "org internal error",
			orgParam:       "org-1",
			envParam:       "dev",
			orgErr:         errors.New("database error"),
			expectedStatus: fiber.StatusInternalServerError,
		},
		{
			name:           "env not found",
			orgParam:       "org-1",
			envParam:       "nonexistent-env",
			orgExists:      true,
			envExists:      false,
			expectedStatus: fiber.StatusNotFound,
		},
		{
			name:           "env internal error",
			orgParam:       "org-1",
			envParam:       "dev",
			orgExists:      true,
			envErr:         errors.New("database error"),
			expectedStatus: fiber.StatusInternalServerError,
		},
		{
			name:           "org and env exist",
			orgParam:       "org-1",
			envParam:       "dev",
			orgExists:      true,
			envExists:      true,
			expectedStatus: fiber.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStore := &mockStoreForAuth{
				getOrgFn: func(ctx context.Context, orgID string) (*store.Org, error) {
					if tt.orgErr != nil {
						return nil, tt.orgErr
					}
					if !tt.orgExists {
						return nil, store.ErrNotFound
					}
					return &store.Org{ID: orgID, Name: "Test Org"}, nil
				},
				getEnvFn: func(ctx context.Context, orgID, envID string) (*store.Env, error) {
					if tt.envErr != nil {
						return nil, tt.envErr
					}
					if !tt.envExists {
						return nil, store.ErrNotFound
					}
					return &store.Env{ID: envID, OrgID: orgID, Name: envID}, nil
				},
			}

			app := fiber.New()
			middleware := NewAuthMiddleware(mockStore)

			app.Get("/orgs/:org/envs/:env/test", middleware.RequireOrgEnv, func(c *fiber.Ctx) error {
				return c.SendStatus(fiber.StatusOK)
			})

			path := "/orgs/" + tt.orgParam + "/envs/" + tt.envParam + "/test"
			req := httptest.NewRequest("GET", path, nil)

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test() error: %v", err)
			}

			if resp.StatusCode != tt.expectedStatus {
				body, _ := io.ReadAll(resp.Body)
				t.Errorf("status = %d, want %d, body: %s", resp.StatusCode, tt.expectedStatus, body)
			}
		})
	}
}

func TestTokenFromContext(t *testing.T) {
	t.Run("returns nil when no token in context", func(t *testing.T) {
		ctx := context.Background()
		token := TokenFromContext(ctx)
		if token != nil {
			t.Errorf("expected nil, got %v", token)
		}
	})

	t.Run("returns token when present in context", func(t *testing.T) {
		expectedToken := &store.ServiceToken{
			ID:    "token-1",
			OrgID: "org-1",
			EnvID: "dev",
		}
		ctx := context.WithValue(context.Background(), tokenContextKey{}, expectedToken)
		token := TokenFromContext(ctx)
		if token == nil {
			t.Fatal("expected token, got nil")
		}
		if token.ID != expectedToken.ID {
			t.Errorf("token.ID = %q, want %q", token.ID, expectedToken.ID)
		}
	})

	t.Run("returns nil when wrong type in context", func(t *testing.T) {
		ctx := context.WithValue(context.Background(), tokenContextKey{}, "not-a-token")
		token := TokenFromContext(ctx)
		if token != nil {
			t.Errorf("expected nil for wrong type, got %v", token)
		}
	})
}

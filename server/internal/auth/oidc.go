package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OIDCConfig holds OIDC provider configuration.
type OIDCConfig struct {
	IssuerURL     string
	ClientID      string
	ClientSecret  string
	RedirectURL   string
	Scopes        []string
	AllowedEmails []string
	AllowedDomain string
}

// OIDCProvider handles OIDC authentication.
type OIDCProvider struct {
	config     OIDCConfig
	httpClient *http.Client

	mu           sync.RWMutex
	jwksCache    *JWKSCache
	discoveryDoc *DiscoveryDocument
}

// DiscoveryDocument represents OIDC discovery metadata.
type DiscoveryDocument struct {
	Issuer                string   `json:"issuer"`
	AuthorizationEndpoint string   `json:"authorization_endpoint"`
	TokenEndpoint         string   `json:"token_endpoint"`
	UserinfoEndpoint      string   `json:"userinfo_endpoint"`
	JwksURI               string   `json:"jwks_uri"`
	ScopesSupported       []string `json:"scopes_supported"`
}

// JWKSCache caches JSON Web Key Sets.
type JWKSCache struct {
	Keys      []JWK
	ExpiresAt time.Time
}

// JWK represents a JSON Web Key.
type JWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Alg string `json:"alg"`
}

// IDTokenClaims represents claims from an ID token.
type IDTokenClaims struct {
	Issuer        string `json:"iss"`
	Subject       string `json:"sub"`
	Audience      string `json:"aud"`
	Expiry        int64  `json:"exp"`
	IssuedAt      int64  `json:"iat"`
	Nonce         string `json:"nonce"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// NewOIDCProvider creates a new OIDC provider.
func NewOIDCProvider(config OIDCConfig) (*OIDCProvider, error) {
	if config.IssuerURL == "" {
		return nil, fmt.Errorf("OIDC configuration invalid: issuer URL is required")
	}

	provider := &OIDCProvider{
		config: config,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	if err := provider.fetchDiscoveryDocument(context.Background()); err != nil {
		return nil, fmt.Errorf("fetch discovery document: %w", err)
	}

	return provider, nil
}

// fetchDiscoveryDocument fetches the OIDC discovery document.
func (p *OIDCProvider) fetchDiscoveryDocument(ctx context.Context) error {
	wellKnownURL := strings.TrimSuffix(p.config.IssuerURL, "/") + "/.well-known/openid-configuration"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnownURL, nil)
	if err != nil {
		return err
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("discovery endpoint returned %d", resp.StatusCode)
	}

	var doc DiscoveryDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}

	p.mu.Lock()
	p.discoveryDoc = &doc
	p.mu.Unlock()

	return nil
}

// GetAuthorizationURL returns the authorization URL for login.
func (p *OIDCProvider) GetAuthorizationURL(state, nonce string) (string, error) {
	p.mu.RLock()
	doc := p.discoveryDoc
	p.mu.RUnlock()

	if doc == nil {
		return "", fmt.Errorf("OIDC provider not initialized: discovery document not loaded")
	}

	scopes := p.config.Scopes
	if len(scopes) == 0 {
		scopes = []string{"openid", "email", "profile"}
	}

	params := fmt.Sprintf(
		"?client_id=%s&redirect_uri=%s&response_type=code&scope=%s&state=%s&nonce=%s",
		p.config.ClientID,
		p.config.RedirectURL,
		strings.Join(scopes, "+"),
		state,
		nonce,
	)

	return doc.AuthorizationEndpoint + params, nil
}

// ExchangeCode exchanges an authorization code for tokens.
func (p *OIDCProvider) ExchangeCode(ctx context.Context, code string) (*TokenResponse, error) {
	p.mu.RLock()
	doc := p.discoveryDoc
	p.mu.RUnlock()

	if doc == nil {
		return nil, fmt.Errorf("OIDC provider not initialized: discovery document not loaded")
	}

	body := fmt.Sprintf(
		"grant_type=authorization_code&code=%s&redirect_uri=%s&client_id=%s&client_secret=%s",
		code,
		p.config.RedirectURL,
		p.config.ClientID,
		p.config.ClientSecret,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, doc.TokenEndpoint, strings.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, err
	}

	return &tokenResp, nil
}

// TokenResponse represents the token endpoint response.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
}

// GetUserInfo fetches user info using the access token.
func (p *OIDCProvider) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	p.mu.RLock()
	doc := p.discoveryDoc
	p.mu.RUnlock()

	if doc == nil {
		return nil, fmt.Errorf("OIDC provider not initialized: discovery document not loaded")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, doc.UserinfoEndpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned %d", resp.StatusCode)
	}

	var userInfo UserInfo
	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return nil, err
	}

	return &userInfo, nil
}

// UserInfo represents user information from the userinfo endpoint.
type UserInfo struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// ValidateIDTokenNonce validates the nonce claim in an ID token.
// This is a simplified validation - in production, you should also verify the signature.
func (p *OIDCProvider) ValidateIDTokenNonce(idToken, expectedNonce string) error {
	// Parse the JWT payload (middle part)
	parts := strings.Split(idToken, ".")
	if len(parts) != 3 {
		return fmt.Errorf("ID token validation failed: invalid format, expected 3 parts but got %d", len(parts))
	}

	// Decode the payload
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}

	var claims IDTokenClaims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return fmt.Errorf("unmarshal claims: %w", err)
	}

	if claims.Nonce != expectedNonce {
		return fmt.Errorf("ID token validation failed: nonce mismatch")
	}

	return nil
}

// ValidateEmail checks if an email is allowed.
func (p *OIDCProvider) ValidateEmail(email string) bool {
	if len(p.config.AllowedEmails) > 0 {
		for _, allowed := range p.config.AllowedEmails {
			if strings.EqualFold(email, allowed) {
				return true
			}
		}
		return false
	}

	if p.config.AllowedDomain != "" {
		parts := strings.Split(email, "@")
		if len(parts) != 2 {
			return false
		}
		return strings.EqualFold(parts[1], p.config.AllowedDomain)
	}

	return true
}

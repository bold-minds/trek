package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication commands",
}

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Trek using Clerk",
	Long: `Authenticate with Trek using Clerk's device authorization flow.
This will open a browser for you to sign in, then store the token locally.`,
	RunE: runLogin,
}

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out and remove stored credentials",
	RunE:  runLogout,
}

var whoamiCmd = &cobra.Command{
	Use:   "whoami",
	Short: "Display current authentication status",
	RunE:  runWhoami,
}

func init() {
	rootCmd.AddCommand(authCmd)
	authCmd.AddCommand(loginCmd)
	authCmd.AddCommand(logoutCmd)
	authCmd.AddCommand(whoamiCmd)

	loginCmd.Flags().String("clerk-domain", "", "Clerk domain (e.g., clerk.example.com)")
	loginCmd.Flags().String("client-id", "", "Clerk OAuth client ID")
}

// DeviceAuthResponse is the response from Clerk's device authorization endpoint.
type DeviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// TokenResponse is the response from Clerk's token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
	Error        string `json:"error,omitempty"`
	ErrorDesc    string `json:"error_description,omitempty"`
}

// StoredCredentials represents saved authentication credentials.
type StoredCredentials struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Email        string    `json:"email,omitempty"`
}

func runLogin(cmd *cobra.Command, args []string) error {
	clerkDomain, _ := cmd.Flags().GetString("clerk-domain")
	clientID, _ := cmd.Flags().GetString("client-id")

	// Try to get from environment if not provided
	if clerkDomain == "" {
		clerkDomain = os.Getenv("TREK_CLERK_DOMAIN")
	}
	if clientID == "" {
		clientID = os.Getenv("TREK_CLERK_CLIENT_ID")
	}

	if clerkDomain == "" || clientID == "" {
		return fmt.Errorf("clerk-domain and client-id are required (set via flags or TREK_CLERK_DOMAIN/TREK_CLERK_CLIENT_ID env vars)")
	}

	ctx := context.Background()

	// Step 1: Request device authorization
	deviceAuth, err := requestDeviceAuthorization(ctx, clerkDomain, clientID)
	if err != nil {
		return fmt.Errorf("device authorization failed: %w", err)
	}

	// Step 2: Display instructions to user
	fmt.Println("\n🔐 Trek Authentication")
	fmt.Println("━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Printf("\nOpen this URL in your browser:\n\n  %s\n\n", deviceAuth.VerificationURIComplete)
	fmt.Printf("Or go to %s and enter code: %s\n\n", deviceAuth.VerificationURI, deviceAuth.UserCode)
	fmt.Println("Waiting for authentication...")

	// Step 3: Poll for token
	token, err := pollForToken(ctx, clerkDomain, clientID, deviceAuth)
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	// Step 4: Save credentials
	creds := StoredCredentials{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    time.Now().Add(time.Duration(token.ExpiresIn) * time.Second),
	}

	if err := saveCredentials(creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	fmt.Println("\n✅ Successfully authenticated!")
	return nil
}

func runLogout(cmd *cobra.Command, args []string) error {
	credPath := getCredentialsPath()
	if err := os.Remove(credPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove credentials: %w", err)
	}

	fmt.Println("✅ Logged out successfully")
	return nil
}

func runWhoami(cmd *cobra.Command, args []string) error {
	creds, err := loadCredentials()
	if err != nil {
		fmt.Println("Not authenticated. Run 'trek auth login' to authenticate.")
		return nil
	}

	if time.Now().After(creds.ExpiresAt) {
		fmt.Println("Session expired. Run 'trek auth login' to re-authenticate.")
		return nil
	}

	fmt.Printf("Authenticated\n")
	if creds.Email != "" {
		fmt.Printf("Email: %s\n", creds.Email)
	}
	fmt.Printf("Expires: %s\n", creds.ExpiresAt.Format(time.RFC3339))
	return nil
}

func requestDeviceAuthorization(ctx context.Context, clerkDomain, clientID string) (*DeviceAuthResponse, error) {
	endpoint := fmt.Sprintf("https://%s/oauth/device/code", clerkDomain)

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("scope", "openid email profile")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device auth request failed: %s", string(body))
	}

	var result DeviceAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func pollForToken(ctx context.Context, clerkDomain, clientID string, deviceAuth *DeviceAuthResponse) (*TokenResponse, error) {
	endpoint := fmt.Sprintf("https://%s/oauth/token", clerkDomain)

	interval := time.Duration(deviceAuth.Interval) * time.Second
	if interval < 5*time.Second {
		interval = 5 * time.Second
	}

	deadline := time.Now().Add(time.Duration(deviceAuth.ExpiresIn) * time.Second)

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}

		data := url.Values{}
		data.Set("client_id", clientID)
		data.Set("device_code", deviceAuth.DeviceCode)
		data.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(data.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		var result TokenResponse
		json.NewDecoder(resp.Body).Decode(&result)
		resp.Body.Close()

		switch result.Error {
		case "":
			return &result, nil
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "expired_token":
			return nil, fmt.Errorf("device code expired")
		case "access_denied":
			return nil, fmt.Errorf("access denied by user")
		default:
			return nil, fmt.Errorf("%s: %s", result.Error, result.ErrorDesc)
		}
	}

	return nil, fmt.Errorf("authentication timed out")
}

func getCredentialsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".trek", "credentials.json")
}

func saveCredentials(creds StoredCredentials) error {
	path := getCredentialsPath()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func loadCredentials() (*StoredCredentials, error) {
	path := getCredentialsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var creds StoredCredentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}

// GetAccessToken returns the current access token if valid, or empty string if not authenticated.
func GetAccessToken() string {
	creds, err := loadCredentials()
	if err != nil {
		return ""
	}

	if time.Now().After(creds.ExpiresAt) {
		return ""
	}

	return creds.AccessToken
}

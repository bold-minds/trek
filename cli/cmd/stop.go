package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	revokeSessionID string
	revokeYes       bool
)

var sessionRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke a debug session",
	Long: `Revoke an active debug session by ID.

Example:
  trek session revoke sess_abc123
  trek session revoke sess_abc123 --yes`,
	Args: cobra.MaximumNArgs(1),
	RunE: runRevoke,
}

func init() {
	sessionCmd.AddCommand(sessionRevokeCmd)

	sessionRevokeCmd.Flags().StringVar(&revokeSessionID, "session", "", "Session ID to revoke (alternative to positional arg)")
	sessionRevokeCmd.Flags().BoolVarP(&revokeYes, "yes", "y", false, "Skip confirmation prompt")
}

func runRevoke(cmd *cobra.Command, args []string) error {
	// Get session ID from positional arg or flag
	sessionID := revokeSessionID
	if len(args) > 0 {
		sessionID = args[0]
	}
	if sessionID == "" {
		return fmt.Errorf("session ID required\n  Usage: trek session revoke <session_id>\n  Example: trek session revoke sess_abc123")
	}

	// Interactive confirmation unless --yes is provided
	if !revokeYes {
		fmt.Printf("This will revoke debug session %s\n", sessionID)
		fmt.Print("Are you sure? [y/N]: ")
		var response string
		fmt.Scanln(&response)
		if response != "y" && response != "Y" && response != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := client.RevokeSession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to revoke session: %w", err)
	}

	fmt.Printf("Session %s revoked\n", sessionID)
	return nil
}

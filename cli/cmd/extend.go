package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	extendSessionID string
	extendTTL       time.Duration
)

var sessionExtendCmd = &cobra.Command{
	Use:   "extend <session_id>",
	Short: "Extend session TTL",
	Long: `Extend the TTL of an active debug session.

Examples:
  trek session extend sess_abc123 --ttl 30m
  trek session extend sess_abc123 --ttl 1h`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExtend,
}

func init() {
	sessionCmd.AddCommand(sessionExtendCmd)

	sessionExtendCmd.Flags().StringVar(&extendSessionID, "session", "", "Session ID (alternative to positional arg)")
	sessionExtendCmd.Flags().DurationVar(&extendTTL, "ttl", 15*time.Minute, "Additional TTL to add (e.g., 15m, 1h)")
}

func runExtend(cmd *cobra.Command, args []string) error {
	// Get session ID from positional arg or flag
	sessionID := extendSessionID
	if len(args) > 0 {
		sessionID = args[0]
	}
	if sessionID == "" {
		return fmt.Errorf("session ID required\n  Usage: trek session extend <session_id> --ttl <duration>\n  Example: trek session extend sess_abc123 --ttl 30m")
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.ExtendSession(ctx, sessionID, int(extendTTL.Seconds()))
	if err != nil {
		return fmt.Errorf("failed to extend session: %w", err)
	}

	if quietMode {
		fmt.Println(resp.ID)
		return nil
	}

	fmt.Printf("Session extended successfully\n")
	fmt.Printf("  ID:         %s\n", resp.ID)
	fmt.Printf("  New Expiry: %s\n", resp.ExpiresAt.Format(time.RFC3339))

	return nil
}

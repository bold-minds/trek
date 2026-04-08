package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var getSessionID string

var sessionGetCmd = &cobra.Command{
	Use:   "get <session_id>",
	Short: "Get session details",
	Long: `Get detailed information about a specific debug session.

Examples:
  trek session get sess_abc123
  trek session get sess_abc123 --output json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGet,
}

func init() {
	sessionCmd.AddCommand(sessionGetCmd)

	sessionGetCmd.Flags().StringVar(&getSessionID, "session", "", "Session ID (alternative to positional arg)")
}

func runGet(cmd *cobra.Command, args []string) error {
	// Get session ID from positional arg or flag
	sessionID := getSessionID
	if len(args) > 0 {
		sessionID = args[0]
	}
	if sessionID == "" {
		return fmt.Errorf("session ID required\n  Usage: trek session get <session_id>\n  Example: trek session get sess_abc123")
	}

	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session, err := client.GetSession(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	// Handle output format
	switch outputFmt {
	case "json":
		data, err := json.MarshalIndent(session, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal session: %w", err)
		}
		fmt.Println(string(data))
	case "yaml":
		data, err := yaml.Marshal(session)
		if err != nil {
			return fmt.Errorf("failed to marshal session: %w", err)
		}
		fmt.Print(string(data))
	default:
		// Table/human-readable format
		if quietMode {
			fmt.Println(session.ID)
			return nil
		}

		selectorStr := formatSelector(session.Selector)
		status := "active"
		if time.Now().After(session.ExpiresAt) {
			status = "expired"
		}

		fmt.Printf("Session Details\n")
		fmt.Println("----------------------------------------")
		fmt.Printf("  ID:         %s\n", session.ID)
		fmt.Printf("  Status:     %s\n", status)
		fmt.Printf("  Level:      %s\n", session.Level)
		fmt.Printf("  Selector:   %s\n", selectorStr)
		fmt.Printf("  Expires:    %s\n", session.ExpiresAt.Format(time.RFC3339))
		if len(session.Labels) > 0 {
			fmt.Printf("  Labels:\n")
			for k, v := range session.Labels {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
		if session.Caps.MaxDebugEventsPerRequest > 0 || session.Caps.MaxDebugEventsPerSession > 0 {
			fmt.Printf("  Caps:\n")
			if session.Caps.MaxDebugEventsPerRequest > 0 {
				fmt.Printf("    Max Events/Request: %d\n", session.Caps.MaxDebugEventsPerRequest)
			}
			if session.Caps.MaxDebugEventsPerSession > 0 {
				fmt.Printf("    Max Events/Session: %d\n", session.Caps.MaxDebugEventsPerSession)
			}
		}
	}

	return nil
}

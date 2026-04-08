package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/bold-minds/trek"
	"github.com/spf13/cobra"
)

var requestContext string

var inspectCmd = &cobra.Command{
	Use:   "inspect",
	Short: "Inspect how a request would be evaluated",
	Long: `Test how a request context would be evaluated against active sessions.
This runs the evaluator locally without making API calls (except to fetch sessions).

Example:
  trek inspect --request-context '{"user_id":"u123","route":"/api/orders"}'`,
	RunE: runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)

	inspectCmd.Flags().StringVar(&requestContext, "request-context", "", "Request context as JSON")
	inspectCmd.MarkFlagRequired("request-context")
}

func runInspect(cmd *cobra.Command, args []string) error {
	var ctx trek.RequestContext
	if err := json.Unmarshal([]byte(requestContext), &ctx); err != nil {
		return fmt.Errorf("invalid request context JSON: %w", err)
	}

	client, err := getClient()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Warning: Could not create client, using empty session list")
		decision := trek.Decide(time.Now(), "cli", ctx, nil)
		printDecision(decision)
		return nil
	}

	resp, err := client.GetActiveSessions(cmd.Context(), "cli", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Could not fetch sessions: %v\n", err)
		decision := trek.Decide(time.Now(), "cli", ctx, nil)
		printDecision(decision)
		return nil
	}

	decision := trek.Decide(time.Now(), "cli", ctx, resp.Sessions)
	printDecision(decision)

	return nil
}

func printDecision(d trek.Decision) {
	fmt.Printf("Decision:\n")
	fmt.Printf("  Matched:         %v\n", d.Matched)
	fmt.Printf("  Session ID:      %s\n", d.SessionID)
	fmt.Printf("  Effective Level: %s\n", d.EffectiveLevel)
	fmt.Printf("  Reason Code:     %s\n", d.ReasonCode)

	if len(d.Labels) > 0 {
		fmt.Printf("  Labels:\n")
		for k, v := range d.Labels {
			fmt.Printf("    %s: %s\n", k, v)
		}
	}
}

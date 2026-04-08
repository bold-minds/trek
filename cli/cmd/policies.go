package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var policiesCmd = &cobra.Command{
	Use:   "policies",
	Short: "Manage policies",
	Long:  `Commands for viewing and managing Trek policies.`,
}

var policiesGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get current policy settings",
	Long: `Display the current policy settings for the organization and environment.

Examples:
  trek policies get
  trek policies get --env prod`,
	RunE: runPoliciesGet,
}

func init() {
	rootCmd.AddCommand(policiesCmd)
	policiesCmd.AddCommand(policiesGetCmd)
}

func runPoliciesGet(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	policy, err := client.GetPolicy(ctx)
	if err != nil {
		return fmt.Errorf("failed to get policy: %w", err)
	}

	fmt.Printf("Policy for %s/%s\n", orgID, env)
	fmt.Println("------------------------------------------")
	fmt.Printf("  Max TTL:              %s\n", formatDuration(policy.MaxTTLSeconds))
	fmt.Printf("  Require Reason:       %v\n", policy.RequireReason)
	fmt.Printf("  Allow Empty Selector: %v\n", policy.AllowEmptySelector)

	if len(policy.AllowedSelectorKeys) > 0 {
		fmt.Printf("  Allowed Selector Keys: %v\n", policy.AllowedSelectorKeys)
	}

	if policy.DefaultCaps.MaxDebugEventsPerRequest > 0 || policy.DefaultCaps.MaxDebugEventsPerSession > 0 {
		fmt.Printf("  Default Caps:\n")
		if policy.DefaultCaps.MaxDebugEventsPerRequest > 0 {
			fmt.Printf("    Max Events/Request: %d\n", policy.DefaultCaps.MaxDebugEventsPerRequest)
		}
		if policy.DefaultCaps.MaxDebugEventsPerSession > 0 {
			fmt.Printf("    Max Events/Session: %d\n", policy.DefaultCaps.MaxDebugEventsPerSession)
		}
	}

	return nil
}

func formatDuration(seconds int) string {
	d := time.Duration(seconds) * time.Second
	if d >= time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	if d >= time.Minute {
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute"
		}
		return fmt.Sprintf("%d minutes", mins)
	}
	return fmt.Sprintf("%d seconds", seconds)
}

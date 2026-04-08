package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/bold-minds/trek"
	"github.com/spf13/cobra"
)

var (
	userID    string
	requestID string
	tenantID  string
	route     string
	ttl       time.Duration
	level     string
	reason    string
	labels    []string
)

var sessionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a debug session",
	Long: `Create a new debug session to enable targeted logging.

Examples:
  trek session create --user u123 --ttl 15m --level debug --reason "investigating order issue"
  trek session create --route "/api/orders*" --ttl 10m --level trace
  trek session create --tenant t456 --ttl 30m --level debug`,
	RunE: runCreate,
}

func init() {
	sessionCmd.AddCommand(sessionCreateCmd)

	sessionCreateCmd.Flags().StringVar(&userID, "user", "", "Target user ID")
	sessionCreateCmd.Flags().StringVar(&requestID, "request", "", "Target request ID")
	sessionCreateCmd.Flags().StringVar(&tenantID, "tenant", "", "Target tenant ID")
	sessionCreateCmd.Flags().StringVar(&route, "route", "", "Target route (supports * prefix matching)")
	sessionCreateCmd.Flags().DurationVar(&ttl, "ttl", 15*time.Minute, "Session TTL (e.g., 15m, 1h)")
	sessionCreateCmd.Flags().StringVar(&level, "level", "debug", "Log level (debug or trace)")
	sessionCreateCmd.Flags().StringVar(&reason, "reason", "", "Reason for enabling debug (required by policy)")
	sessionCreateCmd.Flags().StringArrayVar(&labels, "label", nil, "Labels in key=value format (can be repeated)")
}

func runCreate(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	selector := trek.Selector{
		UserID:    userID,
		RequestID: requestID,
		TenantID:  tenantID,
		Route:     route,
	}

	if trek.IsEmptySelector(selector) {
		return fmt.Errorf("at least one selector field required (--user, --request, --tenant, or --route)")
	}

	labelMap, err := parseLabels(labels)
	if err != nil {
		return err
	}

	req := trek.CreateSessionRequest{
		Selector:   selector,
		Level:      trek.Level(level),
		TTLSeconds: int(ttl.Seconds()),
		Reason:     reason,
		Labels:     labelMap,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := client.CreateSession(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	fmt.Printf("Session created successfully\n")
	fmt.Printf("  ID:         %s\n", resp.ID)
	fmt.Printf("  Status:     %s\n", resp.Status)
	fmt.Printf("  Expires:    %s\n", resp.ExpiresAt.Format(time.RFC3339))
	fmt.Printf("  Propagation: ≤10s (poll interval 5s)\n")

	return nil
}

func parseLabels(labels []string) (map[string]string, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	result := make(map[string]string, len(labels))
	for _, l := range labels {
		parts := strings.SplitN(l, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid label format %q: expected key=value", l)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

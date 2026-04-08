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
	statusFilter string
	watchMode    bool
)

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List debug sessions",
	Long: `List debug sessions with optional status filter.

Examples:
  trek session list
  trek session list --status active
  trek session list --watch`,
	RunE: runList,
}

func init() {
	sessionCmd.AddCommand(sessionListCmd)

	sessionListCmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status (active, revoked, expired)")
	sessionListCmd.Flags().BoolVar(&watchMode, "watch", false, "Watch for changes (refresh every 2s)")
}

func runList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	if watchMode {
		return runListWatch(cmd.Context(), client)
	}

	return listSessionsOnce(cmd.Context(), client)
}

func listSessionsOnce(ctx context.Context, client *trek.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	sessions, err := client.ListSessions(ctx, statusFilter)
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	printSessionTable(sessions)
	return nil
}

func runListWatch(ctx context.Context, client *trek.Client) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	fmt.Println("Watching sessions (Ctrl+C to stop)...")
	fmt.Println()

	for {
		fmt.Print("\033[H\033[2J")
		fmt.Printf("Sessions (updated %s)\n\n", time.Now().Format("15:04:05"))

		listCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		sessions, err := client.ListSessions(listCtx, statusFilter)
		cancel()

		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else {
			printSessionTable(sessions)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func printSessionTable(sessions []trek.Session) {
	if len(sessions) == 0 {
		fmt.Println("No sessions found")
		return
	}

	fmt.Printf("%-28s %-8s %-8s %-25s %s\n", "ID", "STATUS", "LEVEL", "EXPIRES", "SELECTOR")
	fmt.Println("--------------------------------------------------------------------------------------------")

	for _, s := range sessions {
		status := "active"
		if time.Now().After(s.ExpiresAt) {
			status = "expired"
		}

		selectorStr := formatSelector(s.Selector)
		fmt.Printf("%-28s %-8s %-8s %-25s %s\n",
			truncate(s.ID, 28),
			status,
			s.Level,
			s.ExpiresAt.Format("2006-01-02 15:04:05"),
			truncate(selectorStr, 30),
		)
	}
}

func formatSelector(s trek.Selector) string {
	var parts []string
	if s.UserID != "" {
		parts = append(parts, "user:"+s.UserID)
	}
	if s.TenantID != "" {
		parts = append(parts, "tenant:"+s.TenantID)
	}
	if s.RequestID != "" {
		parts = append(parts, "req:"+s.RequestID)
	}
	if s.Route != "" {
		parts = append(parts, "route:"+s.Route)
	}
	if len(s.Custom) > 0 {
		for k, v := range s.Custom {
			parts = append(parts, k+":"+v)
		}
	}
	if len(parts) == 0 {
		return "(empty)"
	}
	return strings.Join(parts, ", ")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

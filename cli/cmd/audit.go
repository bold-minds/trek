package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var (
	auditSince string
)

var auditCmd = &cobra.Command{
	Use:   "audit",
	Short: "View audit logs",
	Long:  `Commands for viewing Trek audit logs.`,
}

var auditListCmd = &cobra.Command{
	Use:   "list",
	Short: "List audit events",
	Long: `List audit events for the current organization and environment.

Examples:
  trek audit list
  trek audit list --since 30m
  trek audit list --since 1h`,
	RunE: runAuditList,
}

func init() {
	rootCmd.AddCommand(auditCmd)
	auditCmd.AddCommand(auditListCmd)

	auditListCmd.Flags().StringVar(&auditSince, "since", "", "Show events since duration (e.g., 30m, 1h, 24h)")
}

func runAuditList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var sinceTime time.Time
	if auditSince != "" {
		d, err := time.ParseDuration(auditSince)
		if err != nil {
			return fmt.Errorf("invalid --since duration: %w", err)
		}
		sinceTime = time.Now().Add(-d)
	}

	resp, err := client.ListAuditEvents(ctx, "")
	if err != nil {
		return fmt.Errorf("failed to list audit events: %w", err)
	}

	events := resp.Events
	if !sinceTime.IsZero() {
		var filtered []struct {
			ID          string
			Action      string
			TargetType  string
			TargetID    string
			ActorUserID string
			CreatedAt   time.Time
		}
		for _, e := range events {
			if e.CreatedAt.After(sinceTime) {
				filtered = append(filtered, struct {
					ID          string
					Action      string
					TargetType  string
					TargetID    string
					ActorUserID string
					CreatedAt   time.Time
				}{
					ID:          e.ID,
					Action:      e.Action,
					TargetType:  e.TargetType,
					TargetID:    e.TargetID,
					ActorUserID: e.ActorUserID,
					CreatedAt:   e.CreatedAt,
				})
			}
		}
		if len(filtered) == 0 {
			fmt.Printf("No audit events found since %s\n", sinceTime.Format(time.RFC3339))
			return nil
		}
		fmt.Printf("Audit events since %s\n", sinceTime.Format("15:04:05"))
		fmt.Println("------------------------------------------")
		fmt.Printf("%-20s %-20s %-15s %-28s %s\n", "TIME", "ACTION", "TARGET TYPE", "TARGET ID", "ACTOR")
		fmt.Println("------------------------------------------------------------------------------------------------------")
		for _, e := range filtered {
			fmt.Printf("%-20s %-20s %-15s %-28s %s\n",
				e.CreatedAt.Format("2006-01-02 15:04:05"),
				e.Action,
				e.TargetType,
				truncate(e.TargetID, 28),
				e.ActorUserID,
			)
		}
		return nil
	}

	if len(events) == 0 {
		fmt.Println("No audit events found")
		return nil
	}

	fmt.Printf("%-20s %-20s %-15s %-28s %s\n", "TIME", "ACTION", "TARGET TYPE", "TARGET ID", "ACTOR")
	fmt.Println("------------------------------------------------------------------------------------------------------")

	for _, e := range events {
		fmt.Printf("%-20s %-20s %-15s %-28s %s\n",
			e.CreatedAt.Format("2006-01-02 15:04:05"),
			e.Action,
			e.TargetType,
			truncate(e.TargetID, 28),
			e.ActorUserID,
		)
	}

	return nil
}

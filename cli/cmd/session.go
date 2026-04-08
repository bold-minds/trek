package cmd

import (
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:   "session",
	Short: "Manage debug sessions",
	Long:  `Commands for creating, listing, and revoking debug sessions.`,
}

func init() {
	rootCmd.AddCommand(sessionCmd)
}

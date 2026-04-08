package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Manage service tokens",
}

var tokensCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new service token",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, _ := cmd.Flags().GetString("name")
		if name == "" {
			return fmt.Errorf("--name is required")
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		resp, err := client.CreateToken(ctx, name)
		if err != nil {
			return fmt.Errorf("create token failed: %w", err)
		}

		fmt.Printf("Token created successfully!\n\n")
		fmt.Printf("  ID:    %s\n", resp.ID)
		fmt.Printf("  Name:  %s\n", resp.Name)
		fmt.Printf("  Token: %s\n\n", resp.Token)
		fmt.Printf("⚠️  Save this token now - it cannot be retrieved later!\n")

		return nil
	},
}

var tokensListCmd = &cobra.Command{
	Use:   "list",
	Short: "List service tokens",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		tokens, err := client.ListTokens(ctx)
		if err != nil {
			return fmt.Errorf("list tokens failed: %w", err)
		}

		if len(tokens) == 0 {
			fmt.Println("No tokens found")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "ID\tNAME\tCREATED")
		for _, t := range tokens {
			fmt.Fprintf(w, "%s\t%s\t%s\n",
				t.ID,
				t.Name,
				t.CreatedAt.Format("2006-01-02 15:04"),
			)
		}
		w.Flush()

		return nil
	},
}

var tokensRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke a service token",
	RunE: func(cmd *cobra.Command, args []string) error {
		tokenID, _ := cmd.Flags().GetString("id")
		if tokenID == "" {
			return fmt.Errorf("--id is required")
		}

		client, err := getClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := client.RevokeToken(ctx, tokenID); err != nil {
			return fmt.Errorf("revoke token failed: %w", err)
		}

		fmt.Printf("Token %s revoked\n", tokenID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tokensCmd)

	tokensCmd.AddCommand(tokensCreateCmd)
	tokensCreateCmd.Flags().String("name", "", "Token name")
	tokensCreateCmd.MarkFlagRequired("name")

	tokensCmd.AddCommand(tokensListCmd)

	tokensCmd.AddCommand(tokensRevokeCmd)
	tokensRevokeCmd.Flags().String("id", "", "Token ID to revoke")
	tokensRevokeCmd.MarkFlagRequired("id")
}

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environments",
	Long:  `Commands for managing Trek environments.`,
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available environments",
	Long:  `List all environments available in the organization.`,
	RunE:  runEnvList,
}

var envSwitchCmd = &cobra.Command{
	Use:   "switch <environment>",
	Short: "Switch to a different environment",
	Long: `Switch to a different environment for subsequent commands.

Examples:
  trek env switch prod
  trek env switch staging
  trek env switch dev`,
	Args: cobra.ExactArgs(1),
	RunE: runEnvSwitch,
}

func init() {
	rootCmd.AddCommand(envCmd)
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envSwitchCmd)
}

func runEnvList(cmd *cobra.Command, args []string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	envs, err := client.ListEnvironments(ctx)
	if err != nil {
		return fmt.Errorf("failed to list environments: %w", err)
	}

	if len(envs) == 0 {
		fmt.Println("No environments found")
		return nil
	}

	fmt.Printf("%-20s %-30s %s\n", "NAME", "ID", "CREATED")
	fmt.Println("----------------------------------------------------------------------")

	for _, e := range envs {
		marker := ""
		if e.Name == env {
			marker = " (current)"
		}
		fmt.Printf("%-20s %-30s %s%s\n",
			e.Name,
			e.ID,
			e.CreatedAt.Format("2006-01-02 15:04:05"),
			marker,
		)
	}

	return nil
}

func runEnvSwitch(cmd *cobra.Command, args []string) error {
	targetEnv := args[0]

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(home, ".trek", "config.yaml")

	var cfg map[string]interface{}
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = make(map[string]interface{})
		} else {
			return fmt.Errorf("failed to read config: %w", err)
		}
	} else {
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return fmt.Errorf("failed to parse config: %w", err)
		}
	}

	cfg["env"] = targetEnv

	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	newData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, newData, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	env = targetEnv

	fmt.Printf("Switched to environment: %s\n", targetEnv)
	return nil
}

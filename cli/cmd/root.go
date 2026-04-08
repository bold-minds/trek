package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/bold-minds/trek"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	cfgFile     string
	apiEndpoint string
	apiToken    string
	orgID       string
	env         string
	outputFmt   string
	quietMode   bool
	verboseMode bool
	noColor     bool
)

var rootCmd = &cobra.Command{
	Use:   "trek",
	Short: "Trek CLI - Targeted debug logging",
	Long: `Trek enables targeted, time-bounded debug logging.
Create debug sessions to increase logging verbosity for specific
users, requests, tenants, or routes without changing global log levels.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is ~/.trek/config.yaml)")
	rootCmd.PersistentFlags().StringVar(&apiEndpoint, "endpoint", "", "Trek API endpoint")
	rootCmd.PersistentFlags().StringVar(&apiToken, "token", "", "API token")
	rootCmd.PersistentFlags().StringVar(&orgID, "org", "", "Organization ID")
	rootCmd.PersistentFlags().StringVar(&env, "env", "", "Environment (dev/stage/prod)")
	rootCmd.PersistentFlags().StringVarP(&outputFmt, "output", "o", "table", "Output format: table, json, yaml")
	rootCmd.PersistentFlags().BoolVarP(&quietMode, "quiet", "q", false, "Only output IDs")
	rootCmd.PersistentFlags().BoolVarP(&verboseMode, "verbose", "v", false, "Verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
}

func initConfig() {
	if apiEndpoint == "" {
		apiEndpoint = os.Getenv("TREK_API_ENDPOINT")
	}
	if apiToken == "" {
		apiToken = os.Getenv("TREK_API_TOKEN")
	}
	if orgID == "" {
		orgID = os.Getenv("TREK_ORG_ID")
	}
	if env == "" {
		env = os.Getenv("TREK_ENV")
	}

	if cfgFile == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			cfgFile = filepath.Join(home, ".trek", "config.yaml")
		}
	}

	if cfgFile != "" {
		loadConfigFile(cfgFile)
	}
}

func loadConfigFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		// File not found is expected, other errors should be logged
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: failed to read config file %s: %v\n", path, err)
		}
		return
	}

	var cfg struct {
		Endpoint string `yaml:"endpoint"`
		Token    string `yaml:"token"`
		Org      string `yaml:"org"`
		Env      string `yaml:"env"`
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to parse config file %s: %v\n", path, err)
		return
	}

	if apiEndpoint == "" && cfg.Endpoint != "" {
		apiEndpoint = cfg.Endpoint
	}
	if apiToken == "" && cfg.Token != "" {
		apiToken = cfg.Token
	}
	if orgID == "" && cfg.Org != "" {
		orgID = cfg.Org
	}
	if env == "" && cfg.Env != "" {
		env = cfg.Env
	}
}

func getClient() (*trek.Client, error) {
	if apiEndpoint == "" {
		return nil, fmt.Errorf("API endpoint required (--endpoint or TREK_API_ENDPOINT)")
	}
	if apiToken == "" {
		return nil, fmt.Errorf("API token required (--token or TREK_API_TOKEN)")
	}
	if orgID == "" {
		return nil, fmt.Errorf("org ID required (--org or TREK_ORG_ID)")
	}
	if env == "" {
		return nil, fmt.Errorf("env required (--env or TREK_ENV)")
	}

	return trek.NewClient(apiEndpoint, apiToken, orgID, env), nil
}

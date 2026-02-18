package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/l3aro/go-context-query/internal/config"
	"github.com/l3aro/go-context-query/internal/healthcheck"
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run health checks on configuration and models",
	Long: `Checks the configuration and verifies that embedding models
are accessible and working properly.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, configPath, err := loadConfigWithPath()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		result, err := healthcheck.Check(cfg, configPath, configPath)
		if err != nil {
			return fmt.Errorf("health check failed: %w", err)
		}

		displayDoctorResult(result)

		hasError := result.WarmModel.Status == "error" ||
			(result.SearchModel.Status == "error" && result.SearchModel.Status != "inherited")

		if hasError {
			return fmt.Errorf("health check failed: one or more models are not accessible")
		}

		return nil
	},
}

func loadConfigWithPath() (*config.Config, string, error) {
	projectConfigPath := ".gcq/config.yaml"
	projectExists := fileExists(projectConfigPath)

	home, _ := os.UserHomeDir()
	globalConfigPath := ""
	if home != "" {
		globalConfigPath = filepath.Join(home, ".gcq", "config.yaml")
	}
	globalExists := fileExists(globalConfigPath)

	var effectivePath string
	if projectExists {
		effectivePath = projectConfigPath
	} else if globalExists {
		effectivePath = globalConfigPath
	} else {
		return nil, "", fmt.Errorf("no configuration found\n"+
			"Checked paths:\n"+
			"  - %s (project)\n"+
			"  - %s (global)\n"+
			"Run 'gcq init' to create a configuration file",
			projectConfigPath, globalConfigPath)
	}

	cfg, err := config.LoadFromFile(effectivePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load config from %s: %w", effectivePath, err)
	}

	return cfg, effectivePath, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func displayDoctorResult(result *healthcheck.HealthCheckResult) {
	fmt.Printf("Using config: %s (%s)\n\n", result.EffectivePath, result.EffectiveScope)

	fmt.Println("Warm Model:")
	fmt.Printf("  Provider: %s\n", result.WarmModel.Provider)
	fmt.Printf("  Model: %s\n", result.WarmModel.Model)
	if result.WarmModel.URL != "" {
		fmt.Printf("  URL: %s\n", result.WarmModel.URL)
	}
	printModelStatus(result.WarmModel.Status, result.WarmModel.Error)

	fmt.Println("\nSearch Model:")
	fmt.Printf("  Provider: %s\n", result.SearchModel.Provider)
	fmt.Printf("  Model: %s\n", result.SearchModel.Model)
	if result.SearchModel.URL != "" {
		fmt.Printf("  URL: %s\n", result.SearchModel.URL)
	}
	if result.SearchModel.Status == "inherited" {
		fmt.Printf("  Status: %s (inherited from warm)\n", formatStatusIcon(result.SearchModel.Status))
	} else {
		printModelStatus(result.SearchModel.Status, result.SearchModel.Error)
	}
}

func printModelStatus(status string, errMsg string) {
	icon := formatStatusIcon(status)
	fmt.Printf("  Status: %s %s\n", icon, status)
	if errMsg != "" && status == "error" {
		fmt.Printf("  Error: %s\n", errMsg)
	}
}

func formatStatusIcon(status string) string {
	switch status {
	case "ready":
		return "✓"
	case "downloading":
		return "◐"
	case "inherited":
		return "✓"
	case "error":
		return "✗"
	default:
		return "?"
	}
}

func init() {
	RootCmd.AddCommand(doctorCmd)
}

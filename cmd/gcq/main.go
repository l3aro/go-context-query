// Package main implements the go-context-query CLI (gcq).
// It provides commands for building semantic indexes, querying code context,
// and managing the daemon.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/l3aro/go-context-query/cmd/gcq/commands"
	"github.com/l3aro/go-context-query/internal/config"
	"github.com/l3aro/go-context-query/internal/daemon"
	"github.com/l3aro/go-context-query/pkg/embed"
	"github.com/l3aro/go-context-query/pkg/semantic"
	"github.com/spf13/cobra"
)

var (
	version   = "dev"
	buildTime = ""
)

func main() {
	// Add build command
	buildCmd := &cobra.Command{
		Use:   "build [flags]",
		Short: "Build semantic index",
		RunE: func(cmd *cobra.Command, args []string) error {
			projectPath, _ := cmd.Flags().GetString("project")
			providerType, _ := cmd.Flags().GetString("provider")
			modelName, _ := cmd.Flags().GetString("model")
			return runBuild(projectPath, providerType, modelName)
		},
	}
	buildCmd.Flags().String("project", ".", "Project directory to index")
	buildCmd.Flags().String("provider", "ollama", "Embedding provider (ollama or huggingface)")
	buildCmd.Flags().String("model", "", "Embedding model name")

	// Add start command
	startCmd := &cobra.Command{
		Use:   "start [flags]",
		Short: "Start daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			daemonPath, _ := cmd.Flags().GetString("daemon")
			socketPath, _ := cmd.Flags().GetString("socket")
			configPath, _ := cmd.Flags().GetString("config")
			verbose, _ := cmd.Flags().GetBool("v")
			background, _ := cmd.Flags().GetBool("d")
			return runStart(daemonPath, socketPath, configPath, verbose, background)
		},
	}
	startCmd.Flags().String("daemon", "", "Path to daemon binary")
	startCmd.Flags().String("socket", "", "Unix socket path")
	startCmd.Flags().String("config", "", "Config file path")
	startCmd.Flags().BoolP("v", "v", false, "Verbose logging")
	startCmd.Flags().BoolP("d", "d", false, "Run in background")

	// Add stop command
	stopCmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStop()
		},
	}

	// Add status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonOutput, _ := cmd.Flags().GetBool("json")
			return runStatus(jsonOutput)
		},
	}
	statusCmd.Flags().BoolP("json", "j", false, "Output as JSON")

	// Add all commands to root
	commands.RootCmd.AddCommand(buildCmd)
	commands.RootCmd.AddCommand(startCmd)
	commands.RootCmd.AddCommand(stopCmd)
	commands.RootCmd.AddCommand(statusCmd)

	commands.RootCmd.Flags().BoolP("version", "v", false, "Print version information")
	commands.RootCmd.SetVersionTemplate(`gcq version {{.Version}}
`)
	commands.RootCmd.Version = version

	if err := commands.RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runBuild(projectPath, providerType, modelName string) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	var provider embed.Provider

	switch providerType {
	case "ollama":
		model := modelName
		if model == "" {
			model = cfg.OllamaModel
		}
		provider, err = embed.NewOllamaProvider(&embed.Config{
			Model:    model,
			Endpoint: cfg.OllamaBaseURL,
			APIKey:   cfg.OllamaAPIKey,
		})
		if err != nil {
			return fmt.Errorf("creating Ollama provider: %w", err)
		}
	case "huggingface":
		model := modelName
		if model == "" {
			model = cfg.HFModel
		}
		provider, err = embed.NewHuggingFaceProvider(&embed.Config{
			Model:  model,
			APIKey: cfg.HFToken,
		})
		if err != nil {
			return fmt.Errorf("creating HuggingFace provider: %w", err)
		}
	default:
		return fmt.Errorf("unknown provider: %s (use 'ollama' or 'huggingface')", providerType)
	}

	return semantic.BuildIndex(projectPath, provider)
}

func runStart(daemonPath, socketPath, configPath string, verbose, background bool) error {
	opts := &daemon.StartOptions{
		DaemonPath:   daemonPath,
		SocketPath:   socketPath,
		ConfigPath:   configPath,
		Verbose:      verbose,
		WaitForReady: true,
		ReadyTimeout: 10 * time.Second,
		Background:   background,
	}

	result, err := daemon.Start(opts)
	if err != nil {
		return err
	}

	if !result.Success {
		if result.Error != "" {
			fmt.Printf("Failed to start daemon: %s\n", result.Error)
		}
		if result.PID > 0 {
			fmt.Printf("Daemon already running with PID %d\n", result.PID)
		}
		return nil
	}

	fmt.Printf("Daemon started with PID %d\n", result.PID)
	return nil
}

func runStop() error {
	result, err := daemon.Stop()
	if err != nil {
		return err
	}

	if !result.Success {
		if result.Error != "" {
			fmt.Printf("Failed to stop daemon: %s\n", result.Error)
		}
		return nil
	}

	fmt.Printf("Daemon stopped (PID: %d)\n", result.PID)
	return nil
}

func runStatus(jsonOutput bool) error {
	result, err := daemon.GetStatus()
	if err != nil {
		return err
	}

	if jsonOutput {
		data, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	if result.Error != "" {
		fmt.Printf("Status: %s\n", result.Status)
		fmt.Printf("Error: %s\n", result.Error)
		return nil
	}

	fmt.Printf("Status: %s\n", result.Status)
	if result.PID > 0 {
		fmt.Printf("PID: %d\n", result.PID)
	}
	if result.Version != "" {
		fmt.Printf("Version: %s\n", result.Version)
	}
	if !result.StartedAt.IsZero() {
		fmt.Printf("Started: %s\n", result.StartedAt.Format(time.RFC3339))
	}

	return nil
}

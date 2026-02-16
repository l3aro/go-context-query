package commands

import (
	"github.com/spf13/cobra"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "gcq",
	Short: "go-context-query - Semantic code indexing and analysis",
	Long: `go-context-query provides tools for analyzing code structure and gathering context.

Commands:
  tree        Display file tree structure
  structure   Show code structure (functions, classes, imports)
  extract     Full file analysis
  context     Get LLM-ready context from entry point
  calls       Build call graph for a project
  impact      Find callers of a function
  warm        Build semantic index for a project
  semantic    Semantic search over indexed code

Use "gcq [command] --help" for more information about a command.`,
}

// Execute adds all child commands to the root command and sets flags appropriately
func Execute() error {
	return RootCmd.Execute()
}

func init() {
	// Add subcommands
	RootCmd.AddCommand(treeCmd)
	RootCmd.AddCommand(structureCmd)
	RootCmd.AddCommand(extractCmd)
	RootCmd.AddCommand(contextCmd)
	RootCmd.AddCommand(callsCmd)
	RootCmd.AddCommand(impactCmd)
	RootCmd.AddCommand(warmCmd)
	RootCmd.AddCommand(semanticCmd)
}

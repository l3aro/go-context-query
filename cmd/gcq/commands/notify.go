package commands

import (
	"fmt"
	"os"

	"github.com/l3aro/go-context-query/pkg/dirty"
	"github.com/spf13/cobra"
)

// notifyCmd represents the notify command
var notifyCmd = &cobra.Command{
	Use:   "notify [path]",
	Short: "Mark a file as dirty for tracking",
	Long: `Marks a file as dirty by computing its content hash.
This is used to track which files have changed and need reprocessing.

Examples:
  gcq notify /path/to/file.go
  gcq notify ./src/main.go`,
	Args: cobra.RangeArgs(1, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		// Check if file exists
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return fmt.Errorf("file does not exist: %s", path)
			}
			return fmt.Errorf("stat file: %w", err)
		}

		if info.IsDir() {
			return fmt.Errorf("path must be a file, not a directory: %s", path)
		}

		// Load existing tracker or create new one
		tracker, err := dirty.NewFromCache()
		if err != nil {
			// If no cache exists, create new tracker
			tracker = dirty.New()
		}

		// Mark file as dirty
		if err := tracker.MarkDirty(path); err != nil {
			return fmt.Errorf("marking dirty: %w", err)
		}

		// Save the state
		if err := tracker.Save(); err != nil {
			return fmt.Errorf("saving dirty state: %w", err)
		}

		// Show dirty count
		count := tracker.Count()
		fmt.Printf("Marked %s as dirty\n", path)
		fmt.Printf("Total dirty files: %d\n", count)

		return nil
	},
}

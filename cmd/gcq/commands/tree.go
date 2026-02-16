package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/user/go-context-query/internal/scanner"
)

// TreeNode represents a node in the file tree for JSON output
type TreeNode struct {
	Name     string      `json:"name"`
	Path     string      `json:"path"`
	Type     string      `json:"type"` // "file" or "directory"
	Language string      `json:"language,omitempty"`
	Children []*TreeNode `json:"children,omitempty"`
}

// treeCmd represents the tree command
var treeCmd = &cobra.Command{
	Use:   "tree [path]",
	Short: "Display file tree structure",
	Long:  `Shows a tree view of the directory structure starting from the given path.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := "."
		if len(args) > 0 {
			path = args[0]
		}

		// Get absolute path
		absPath, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("getting absolute path: %w", err)
		}

		// Check if path exists
		info, err := os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("stat path: %w", err)
		}

		if !info.IsDir() {
			return fmt.Errorf("path is not a directory: %s", path)
		}

		// Scan directory
		sc := scanner.New(scanner.DefaultOptions())
		files, err := sc.Scan(absPath)
		if err != nil {
			return fmt.Errorf("scanning directory: %w", err)
		}

		// Get JSON flag
		jsonOutput, _ := cmd.Flags().GetBool("json")

		if jsonOutput {
			return outputTreeJSON(absPath, files)
		}
		return outputTreeText(absPath, files)
	},
}

func init() {
	treeCmd.Flags().BoolP("json", "j", false, "Output as JSON")
}

// buildTree builds a tree structure from file list
func buildTree(root string, files []scanner.FileInfo) *TreeNode {
	rootNode := &TreeNode{
		Name: filepath.Base(root),
		Path: root,
		Type: "directory",
	}

	// Create a map of directories
	dirs := make(map[string]*TreeNode)

	// Sort files by path
	sort.Slice(files, func(i, j int) bool {
		return files[i].Path < files[j].Path
	})

	for _, file := range files {
		parts := strings.Split(file.Path, "/")
		current := rootNode

		for i, part := range parts {
			isLast := i == len(parts)-1
			path := strings.Join(parts[:i+1], "/")

			if isLast {
				// Add file node
				child := &TreeNode{
					Name:     part,
					Path:     file.FullPath,
					Type:     "file",
					Language: file.Language,
				}
				current.Children = append(current.Children, child)
			} else {
				// Directory
				if child, ok := dirs[path]; ok {
					current = child
				} else {
					child := &TreeNode{
						Name:     part,
						Path:     filepath.Join(root, path),
						Type:     "directory",
						Children: []*TreeNode{},
					}
					current.Children = append(current.Children, child)
					dirs[path] = child
					current = child
				}
			}
		}
	}

	// Sort children (directories first, then files, alphabetically)
	sortTree(rootNode)

	return rootNode
}

// sortTree sorts tree nodes (directories first, then alphabetically)
func sortTree(node *TreeNode) {
	// Sort children
	sort.Slice(node.Children, func(i, j int) bool {
		if node.Children[i].Type != node.Children[j].Type {
			return node.Children[i].Type == "directory"
		}
		return node.Children[i].Name < node.Children[j].Name
	})

	// Recursively sort
	for _, child := range node.Children {
		if child.Type == "directory" {
			sortTree(child)
		}
	}
}

func outputTreeJSON(root string, files []scanner.FileInfo) error {
	tree := buildTree(root, files)
	data, err := json.MarshalIndent(tree, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func outputTreeText(root string, files []scanner.FileInfo) error {
	tree := buildTree(root, files)
	printTree(tree, "", true)
	return nil
}

func printTree(node *TreeNode, prefix string, isLast bool) {
	// Print current node
	connector := "└── "
	if !isLast {
		connector = "├── "
	}

	if node.Type == "file" {
		lang := ""
		if node.Language != "" {
			lang = " (" + node.Language + ")"
		}
		fmt.Printf("%s%s%s%s\n", prefix, connector, node.Name, lang)
	} else {
		if node.Name != "." {
			fmt.Printf("%s%s%s/\n", prefix, connector, node.Name)
		}
	}

	// Process children
	if len(node.Children) > 0 {
		newPrefix := prefix
		if node.Name != "." {
			newPrefix = prefix + connector + "    "
			// Replace connector with appropriate spacing
			newPrefix = strings.ReplaceAll(newPrefix, "└── ", "    ")
			newPrefix = strings.ReplaceAll(newPrefix, "├── ", "│   ")
		}

		for i, child := range node.Children {
			printTree(child, newPrefix, i == len(node.Children)-1)
		}
	}
}

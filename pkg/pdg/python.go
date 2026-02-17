package pdg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/l3aro/go-context-query/pkg/cfg"
	"github.com/l3aro/go-context-query/pkg/dfg"
)

// ExtractPDG extracts the Program Dependence Graph from a file for the specified function.
// It dispatches to the appropriate language-specific extractor based on file extension.
func ExtractPDG(filePath string, functionName string) (*PDGInfo, error) {
	ext := strings.ToLower(filepath.Ext(filePath))

	switch ext {
	case ".py":
		return extractPythonPDG(filePath, functionName)
	case ".go":
		return extractGoPDG(filePath, functionName)
	case ".ts", ".tsx":
		return extractTypeScriptPDG(filePath, functionName)
	case ".rs":
		return extractRustPDG(filePath, functionName)
	case ".java":
		return extractJavaPDG(filePath, functionName)
	case ".c":
		return extractCPDG(filePath, functionName)
	case ".cpp", ".cc":
		return extractCppPDG(filePath, functionName)
	case ".rb":
		return extractRubyPDG(filePath, functionName)
	case ".php":
		return extractPhpPDG(filePath, functionName)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", filePath)
	}
}

func extractPythonPDG(filePath string, functionName string) (*PDGInfo, error) {
	_, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	dfgInfo, err := dfg.ExtractDFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting DFG: %w", err)
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	pdgInfo := builder.Build()

	return pdgInfo, nil
}

func extractGoPDG(filePath string, functionName string) (*PDGInfo, error) {
	_, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", filePath, err)
	}

	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	dfgInfo, err := dfg.ExtractDFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting DFG: %w", err)
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	pdgInfo := builder.Build()

	return pdgInfo, nil
}

func extractTypeScriptPDG(filePath string, functionName string) (*PDGInfo, error) {
	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	dfgInfo, err := dfg.ExtractDFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting DFG: %w", err)
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	return builder.Build(), nil
}

func extractRustPDG(filePath string, functionName string) (*PDGInfo, error) {
	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	dfgInfo, err := dfg.ExtractDFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting DFG: %w", err)
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	return builder.Build(), nil
}

func extractJavaPDG(filePath string, functionName string) (*PDGInfo, error) {
	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	dfgInfo, err := dfg.ExtractDFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting DFG: %w", err)
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	return builder.Build(), nil
}

func extractCPDG(filePath string, functionName string) (*PDGInfo, error) {
	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	dfgInfo, err := dfg.ExtractDFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting DFG: %w", err)
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	return builder.Build(), nil
}

func extractCppPDG(filePath string, functionName string) (*PDGInfo, error) {
	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	dfgInfo, err := dfg.ExtractDFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting DFG: %w", err)
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	return builder.Build(), nil
}

func extractRubyPDG(filePath string, functionName string) (*PDGInfo, error) {
	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	dfgInfo, err := dfg.ExtractDFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting DFG: %w", err)
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	return builder.Build(), nil
}

func extractPhpPDG(filePath string, functionName string) (*PDGInfo, error) {
	cfgInfo, err := cfg.ExtractCFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting CFG: %w", err)
	}

	dfgInfo, err := dfg.ExtractDFG(filePath, functionName)
	if err != nil {
		return nil, fmt.Errorf("extracting DFG: %w", err)
	}

	builder := NewPDGBuilder(cfgInfo, dfgInfo)
	return builder.Build(), nil
}

package callgraph

import (
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

func TestPythonImportResolver_Resolve_FromImport(t *testing.T) {
	resolver := NewPythonImportResolver("/project")

	tests := []struct {
		name           string
		imp            types.Import
		fromFile       string
		wantCanonical  string
		wantLocal      string
		wantIsFrom     bool
		wantIsRelative bool
	}{
		{
			name: "from os import path",
			imp: types.Import{
				Module: "os",
				Names:  []string{"path"},
				IsFrom: true,
			},
			fromFile:       "/project/main.py",
			wantCanonical:  "os.path",
			wantLocal:      "path",
			wantIsFrom:     true,
			wantIsRelative: false,
		},
		{
			name: "from os.path import join",
			imp: types.Import{
				Module: "os.path",
				Names:  []string{"join"},
				IsFrom: true,
			},
			fromFile:       "/project/main.py",
			wantCanonical:  "os.path.join",
			wantLocal:      "join",
			wantIsFrom:     true,
			wantIsRelative: false,
		},
		{
			name: "from collections import defaultdict, OrderedDict",
			imp: types.Import{
				Module: "collections",
				Names:  []string{"defaultdict", "OrderedDict"},
				IsFrom: true,
			},
			fromFile:       "/project/main.py",
			wantCanonical:  "collections.defaultdict",
			wantLocal:      "defaultdict",
			wantIsFrom:     true,
			wantIsRelative: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := resolver.Resolve(tt.imp, tt.fromFile)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}

			if resolved.CanonicalName != tt.wantCanonical {
				t.Errorf("CanonicalName = %v, want %v", resolved.CanonicalName, tt.wantCanonical)
			}
			if resolved.LocalName != tt.wantLocal {
				t.Errorf("LocalName = %v, want %v", resolved.LocalName, tt.wantLocal)
			}
			if resolved.IsFrom != tt.wantIsFrom {
				t.Errorf("IsFrom = %v, want %v", resolved.IsFrom, tt.wantIsFrom)
			}
			if resolved.IsRelative != tt.wantIsRelative {
				t.Errorf("IsRelative = %v, want %v", resolved.IsRelative, tt.wantIsRelative)
			}
		})
	}
}

func TestPythonImportResolver_Resolve_RegularImport(t *testing.T) {
	resolver := NewPythonImportResolver("/project")

	tests := []struct {
		name          string
		imp           types.Import
		fromFile      string
		wantCanonical string
		wantLocal     string
		wantIsFrom    bool
	}{
		{
			name: "import os",
			imp: types.Import{
				Module: "os",
				Names:  []string{"os"},
				IsFrom: false,
			},
			fromFile:      "/project/main.py",
			wantCanonical: "os",
			wantLocal:     "os",
			wantIsFrom:    false,
		},
		{
			name: "import os.path",
			imp: types.Import{
				Module: "os.path",
				Names:  []string{"os.path"},
				IsFrom: false,
			},
			fromFile:      "/project/main.py",
			wantCanonical: "os.path",
			wantLocal:     "os.path",
			wantIsFrom:    false,
		},
		{
			name: "import numpy as np",
			imp: types.Import{
				Module: "numpy",
				Names:  []string{"np"},
				IsFrom: false,
			},
			fromFile:      "/project/main.py",
			wantCanonical: "numpy",
			wantLocal:     "np",
			wantIsFrom:    false,
		},
		{
			name: "import pandas as pd",
			imp: types.Import{
				Module: "pandas",
				Names:  []string{"pd"},
				IsFrom: false,
			},
			fromFile:      "/project/main.py",
			wantCanonical: "pandas",
			wantLocal:     "pd",
			wantIsFrom:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := resolver.Resolve(tt.imp, tt.fromFile)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}

			if resolved.CanonicalName != tt.wantCanonical {
				t.Errorf("CanonicalName = %v, want %v", resolved.CanonicalName, tt.wantCanonical)
			}
			if resolved.LocalName != tt.wantLocal {
				t.Errorf("LocalName = %v, want %v", resolved.LocalName, tt.wantLocal)
			}
			if resolved.IsFrom != tt.wantIsFrom {
				t.Errorf("IsFrom = %v, want %v", resolved.IsFrom, tt.wantIsFrom)
			}
		})
	}
}

func TestPythonImportResolver_Resolve_RelativeImport(t *testing.T) {
	resolver := NewPythonImportResolver("/project")

	tests := []struct {
		name           string
		imp            types.Import
		fromFile       string
		wantCanonical  string
		wantLocal      string
		wantIsRelative bool
		wantLevel      int
	}{
		{
			name: "from . import utils",
			imp: types.Import{
				Module: ".",
				Names:  []string{"utils"},
				IsFrom: true,
			},
			fromFile:       "/project/pkg/module.py",
			wantCanonical:  "pkg.utils",
			wantLocal:      "utils",
			wantIsRelative: true,
			wantLevel:      1,
		},
		{
			name: "from .utils import helper",
			imp: types.Import{
				Module: ".utils",
				Names:  []string{"helper"},
				IsFrom: true,
			},
			fromFile:       "/project/pkg/module.py",
			wantCanonical:  "pkg.utils.helper",
			wantLocal:      "helper",
			wantIsRelative: true,
			wantLevel:      1,
		},
		{
			name: "from .. import utils",
			imp: types.Import{
				Module: "..",
				Names:  []string{"utils"},
				IsFrom: true,
			},
			fromFile:       "/project/pkg/sub/module.py",
			wantCanonical:  "pkg.utils",
			wantLocal:      "utils",
			wantIsRelative: true,
			wantLevel:      2,
		},
		{
			name: "from ..utils import helper",
			imp: types.Import{
				Module: "..utils",
				Names:  []string{"helper"},
				IsFrom: true,
			},
			fromFile:       "/project/pkg/sub/module.py",
			wantCanonical:  "pkg.utils.helper",
			wantLocal:      "helper",
			wantIsRelative: true,
			wantLevel:      2,
		},
		{
			name: "from ... import config",
			imp: types.Import{
				Module: "...",
				Names:  []string{"config"},
				IsFrom: true,
			},
			fromFile:       "/project/a/b/c/module.py",
			wantCanonical:  "a.config",
			wantLocal:      "config",
			wantIsRelative: true,
			wantLevel:      3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolved, err := resolver.Resolve(tt.imp, tt.fromFile)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}

			if resolved.CanonicalName != tt.wantCanonical {
				t.Errorf("CanonicalName = %v, want %v", resolved.CanonicalName, tt.wantCanonical)
			}
			if resolved.LocalName != tt.wantLocal {
				t.Errorf("LocalName = %v, want %v", resolved.LocalName, tt.wantLocal)
			}
			if resolved.IsRelative != tt.wantIsRelative {
				t.Errorf("IsRelative = %v, want %v", resolved.IsRelative, tt.wantIsRelative)
			}
			if resolved.RelativeLevel != tt.wantLevel {
				t.Errorf("RelativeLevel = %v, want %v", resolved.RelativeLevel, tt.wantLevel)
			}
		})
	}
}

func TestPythonImportResolver_ResolveAll(t *testing.T) {
	resolver := NewPythonImportResolver("/project")

	imports := []types.Import{
		{
			Module: "os",
			Names:  []string{"os"},
			IsFrom: false,
		},
		{
			Module: "numpy",
			Names:  []string{"np"},
			IsFrom: false,
		},
		{
			Module: "collections",
			Names:  []string{"defaultdict", "OrderedDict"},
			IsFrom: true,
		},
		{
			Module: "os.path",
			Names:  []string{"join", "exists"},
			IsFrom: true,
		},
	}

	result, err := resolver.ResolveAll(imports, "/project/main.py")
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	// Check regular imports
	if canonical, ok := result["os"]; !ok || canonical != "os" {
		t.Errorf("Expected os -> os, got %v", canonical)
	}
	if canonical, ok := result["np"]; !ok || canonical != "numpy" {
		t.Errorf("Expected np -> numpy, got %v", canonical)
	}

	// Check from imports
	if canonical, ok := result["defaultdict"]; !ok || canonical != "collections.defaultdict" {
		t.Errorf("Expected defaultdict -> collections.defaultdict, got %v", canonical)
	}
	if canonical, ok := result["OrderedDict"]; !ok || canonical != "collections.OrderedDict" {
		t.Errorf("Expected OrderedDict -> collections.OrderedDict, got %v", canonical)
	}
	if canonical, ok := result["join"]; !ok || canonical != "os.path.join" {
		t.Errorf("Expected join -> os.path.join, got %v", canonical)
	}
	if canonical, ok := result["exists"]; !ok || canonical != "os.path.exists" {
		t.Errorf("Expected exists -> os.path.exists, got %v", canonical)
	}
}

func TestPythonImportResolver_ResolveAll_Relative(t *testing.T) {
	resolver := NewPythonImportResolver("/project")

	imports := []types.Import{
		{
			Module: ".",
			Names:  []string{"utils"},
			IsFrom: true,
		},
		{
			Module: ".helpers",
			Names:  []string{"helper_func"},
			IsFrom: true,
		},
		{
			Module: "..",
			Names:  []string{"config"},
			IsFrom: true,
		},
	}

	fromFile := "/project/pkg/sub/module.py"
	result, err := resolver.ResolveAll(imports, fromFile)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	if canonical, ok := result["utils"]; !ok || canonical != "pkg.sub.utils" {
		t.Errorf("Expected utils -> pkg.sub.utils, got %v", canonical)
	}
	if canonical, ok := result["helper_func"]; !ok || canonical != "pkg.sub.helpers.helper_func" {
		t.Errorf("Expected helper_func -> pkg.sub.helpers.helper_func, got %v", canonical)
	}
	if canonical, ok := result["config"]; !ok || canonical != "pkg.config" {
		t.Errorf("Expected config -> pkg.config, got %v", canonical)
	}
}

func TestPythonImportResolver_LookupCanonicalName(t *testing.T) {
	resolver := NewPythonImportResolver("/project")

	importMap := map[string]string{
		"np":          "numpy",
		"pd":          "pandas",
		"defaultdict": "collections.defaultdict",
		"join":        "os.path.join",
	}

	tests := []struct {
		localName     string
		wantCanonical string
		wantFound     bool
	}{
		{"np", "numpy", true},
		{"pd", "pandas", true},
		{"defaultdict", "collections.defaultdict", true},
		{"join", "os.path.join", true},
		{"notfound", "", false},
		{"os", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.localName, func(t *testing.T) {
			canonical, found := resolver.LookupCanonicalName(importMap, tt.localName)
			if found != tt.wantFound {
				t.Errorf("LookupCanonicalName() found = %v, want %v", found, tt.wantFound)
			}
			if canonical != tt.wantCanonical {
				t.Errorf("LookupCanonicalName() canonical = %v, want %v", canonical, tt.wantCanonical)
			}
		})
	}
}

func TestPythonImportResolver_ResolveQualifiedName(t *testing.T) {
	resolver := NewPythonImportResolver("/project")

	importMap := map[string]string{
		"np":    "numpy",
		"pd":    "pandas",
		"utils": "myproject.utils",
	}

	tests := []struct {
		qualifiedName string
		wantResult    string
	}{
		{"np.array", "numpy.array"},
		{"np.linalg.norm", "numpy.linalg.norm"},
		{"pd.DataFrame", "pandas.DataFrame"},
		{"utils.helper", "myproject.utils.helper"},
		{"os.path.join", "os.path.join"}, // not in import map
		{"something.else", "something.else"},
	}

	for _, tt := range tests {
		t.Run(tt.qualifiedName, func(t *testing.T) {
			result := resolver.ResolveQualifiedName(importMap, tt.qualifiedName)
			if result != tt.wantResult {
				t.Errorf("ResolveQualifiedName() = %v, want %v", result, tt.wantResult)
			}
		})
	}
}

func TestPythonImportResolver_ResolveAll_Wildcard(t *testing.T) {
	resolver := NewPythonImportResolver("/project")

	imports := []types.Import{
		{
			Module: "module",
			Names:  []string{"*"},
			IsFrom: true,
		},
		{
			Module: "os",
			Names:  []string{"path"},
			IsFrom: true,
		},
	}

	result, err := resolver.ResolveAll(imports, "/project/main.py")
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	// Wildcard imports should be skipped
	if _, ok := result["*"]; ok {
		t.Error("Wildcard import should not be in result")
	}

	// Non-wildcard imports should still be resolved
	if canonical, ok := result["path"]; !ok || canonical != "os.path" {
		t.Errorf("Expected path -> os.path, got %v", canonical)
	}
}

func TestPythonImportResolver_Resolve_InvalidRelative(t *testing.T) {
	resolver := NewPythonImportResolver("/project")

	imp := types.Import{
		Module: "..",
		Names:  []string{"utils"},
		IsFrom: true,
	}

	// Try to resolve from root level - should fail
	_, err := resolver.Resolve(imp, "/project/main.py")
	if err == nil {
		t.Error("Expected error for relative import beyond root, got nil")
	}
}

func TestPythonImportResolver_ResolveAll_MultipleFiles(t *testing.T) {
	rootDir := t.TempDir()
	resolver := NewPythonImportResolver(rootDir)

	// Create directory structure
	pkgDir := filepath.Join(rootDir, "pkg")
	subDir := filepath.Join(pkgDir, "sub")

	imports := []types.Import{
		{
			Module: "os",
			Names:  []string{"os"},
			IsFrom: false,
		},
		{
			Module: ".",
			Names:  []string{"utils"},
			IsFrom: true,
		},
	}

	fromFile := filepath.Join(subDir, "module.py")
	result, err := resolver.ResolveAll(imports, fromFile)
	if err != nil {
		t.Fatalf("ResolveAll() error = %v", err)
	}

	if canonical, ok := result["os"]; !ok || canonical != "os" {
		t.Errorf("Expected os -> os, got %v", canonical)
	}

	if canonical, ok := result["utils"]; !ok {
		t.Error("Expected utils to be in result")
	} else if canonical != "pkg.sub.utils" {
		t.Errorf("Expected utils -> pkg.sub.utils, got %v", canonical)
	}
}

func TestPythonImportResolver_Resolve_EmptyNames(t *testing.T) {
	resolver := NewPythonImportResolver("/project")

	imp := types.Import{
		Module: "os",
		Names:  []string{},
		IsFrom: false,
	}

	resolved, err := resolver.Resolve(imp, "/project/main.py")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Empty names should result in empty LocalName and OriginalName
	if resolved.LocalName != "" {
		t.Errorf("Expected empty LocalName, got %v", resolved.LocalName)
	}
	if resolved.OriginalName != "" {
		t.Errorf("Expected empty OriginalName, got %v", resolved.OriginalName)
	}
}

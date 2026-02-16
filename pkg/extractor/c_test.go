package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

func TestCExtractor(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		check func(*testing.T, *types.ModuleInfo)
	}{
		{
			name: "simple function",
			code: `
#include <stdio.h>

void hello() {
	printf("hello\n");
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				fn := m.Functions[0]
				if fn.Name != "hello" {
					t.Errorf("expected function name hello, got %s", fn.Name)
				}
				if fn.ReturnType != "void" {
					t.Errorf("expected return type void, got %s", fn.ReturnType)
				}
			},
		},
		{
			name: "function with params and return type",
			code: `
int add(int a, int b) {
	return a + b;
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				fn := m.Functions[0]
				if fn.Name != "add" {
					t.Errorf("expected function name add, got %s", fn.Name)
				}
				if fn.Params != "(int a, int b)" {
					t.Errorf("expected params '(int a, int b)', got %s", fn.Params)
				}
				if fn.ReturnType != "int" {
					t.Errorf("expected return type 'int', got %s", fn.ReturnType)
				}
			},
		},
		{
			name: "function returning pointer",
			code: `
char* get_name() {
	return "test";
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				fn := m.Functions[0]
				if fn.Name != "get_name" {
					t.Errorf("expected function name get_name, got %s", fn.Name)
				}
			},
		},
		{
			name: "struct definition",
			code: `
struct Point {
	int x;
	int y;
};
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Structs) != 1 {
					t.Errorf("expected 1 struct, got %d", len(m.Structs))
				}
				s := m.Structs[0]
				if s.Name != "Point" {
					t.Errorf("expected struct name Point, got %s", s.Name)
				}
				if len(s.Fields) < 2 {
					t.Errorf("expected at least 2 fields, got %d", len(s.Fields))
				}
			},
		},
		{
			name: "include directives",
			code: `
#include <stdio.h>
#include <stdlib.h>
#include "local.h"
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Imports) != 3 {
					t.Errorf("expected 3 includes, got %d", len(m.Imports))
				}
				// Check for stdio.h
				found := false
				for _, imp := range m.Imports {
					if imp.Module == "stdio.h" {
						found = true
						if !imp.IsFrom {
							t.Errorf("expected stdio.h to be marked as system include")
						}
						break
					}
				}
				if !found {
					t.Errorf("expected to find stdio.h include")
				}
				// Check for local.h
				foundLocal := false
				for _, imp := range m.Imports {
					if imp.Module == "local.h" {
						foundLocal = true
						break
					}
				}
				if !foundLocal {
					t.Errorf("expected to find local.h include")
				}
			},
		},
		{
			name: "typedef definition",
			code: `
typedef unsigned int uint32;
typedef struct Point Point;
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 2 {
					t.Errorf("expected 2 typedefs (as classes), got %d", len(m.Classes))
				}
			},
		},
		{
			name: "multiple functions",
			code: `
void func1() {}
int func2(int x) { return x; }
char* func3() { return NULL; }
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 3 {
					t.Errorf("expected 3 functions, got %d", len(m.Functions))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			cFile := filepath.Join(tmpDir, "test.c")

			if err := os.WriteFile(cFile, []byte(tt.code), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			extractor := NewCExtractor()
			m, err := extractor.Extract(cFile)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}

			tt.check(t, m)
		})
	}
}

func TestCExtractorLanguage(t *testing.T) {
	extractor := NewCExtractor()
	if extractor.Language() != C {
		t.Errorf("expected language C, got %s", extractor.Language())
	}
}

func TestCExtractorFileExtensions(t *testing.T) {
	extractor := NewCExtractor()
	exts := extractor.FileExtensions()
	if len(exts) != 2 {
		t.Errorf("expected 2 extensions, got %d", len(exts))
	}
	expected := []string{".c", ".h"}
	for i, ext := range expected {
		if exts[i] != ext {
			t.Errorf("expected extension %s, got %s", ext, exts[i])
		}
	}
}

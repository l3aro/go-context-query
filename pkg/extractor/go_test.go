package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

func TestGoExtractor(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		check func(*testing.T, *types.ModuleInfo)
	}{
		{
			name: "simple function",
			code: `package main

func Hello() string {
	return "hello"
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				if m.Functions[0].Name != "Hello" {
					t.Errorf("expected function name Hello, got %s", m.Functions[0].Name)
				}
			},
		},
		{
			name: "function with params and return type",
			code: `package main

func Add(a int, b int) int {
	return a + b
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				fn := m.Functions[0]
				if fn.Name != "Add" {
					t.Errorf("expected function name Add, got %s", fn.Name)
				}
				if fn.Params != "(a int, b int)" {
					t.Errorf("expected params '(a int, b int)', got %s", fn.Params)
				}
				if fn.ReturnType != "int" {
					t.Errorf("expected return type 'int', got %s", fn.ReturnType)
				}
			},
		},
		{
			name: "method with receiver",
			code: `package main

type MyInt int

func (m MyInt) Double() int {
	return int(m) * 2
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				fn := m.Functions[0]
				if fn.Name != "Double" {
					t.Errorf("expected method name Double, got %s", fn.Name)
				}
				if !fn.IsMethod {
					t.Errorf("expected IsMethod to be true")
				}
			},
		},
		{
			name: "struct definition",
			code: `package main

type Person struct {
	Name string
	Age  int
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Structs) != 1 {
					t.Errorf("expected 1 struct, got %d", len(m.Structs))
				}
				strct := m.Structs[0]
				if strct.Name != "Person" {
					t.Errorf("expected struct name Person, got %s", strct.Name)
				}
				if len(strct.Fields) == 0 {
					t.Errorf("expected fields, got empty")
				}
			},
		},
		{
			name: "interface definition",
			code: `package main

type Reader interface {
	Read(p []byte) (n int, err error)
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Interfaces) != 1 {
					t.Errorf("expected 1 interface, got %d", len(m.Interfaces))
				}
				iface := m.Interfaces[0]
				if iface.Name != "Reader" {
					t.Errorf("expected interface name Reader, got %s", iface.Name)
				}
				if len(iface.Methods) == 0 {
					t.Errorf("expected methods, got empty")
				}
			},
		},
		{
			name: "import statement",
			code: `package main

import "fmt"

func main() {
	fmt.Println("hello")
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Imports) != 1 {
					t.Errorf("expected 1 import, got %d", len(m.Imports))
				}
				if m.Imports[0].Module != "fmt" {
					t.Errorf("expected import 'fmt', got %s", m.Imports[0].Module)
				}
			},
		},
		{
			name: "grouped imports",
			code: `package main

import (
	"fmt"
	"os"
)

func main() {}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Imports) != 2 {
					t.Errorf("expected 2 imports, got %d", len(m.Imports))
				}
			},
		},
		{
			name: "multiple functions and methods",
			code: `package main

func TopLevelFunc() {}

type Container struct{}

func (c Container) Method1() {}

func (c Container) Method2() {}
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
			goFile := filepath.Join(tmpDir, "test.go")

			if err := os.WriteFile(goFile, []byte(tt.code), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			extractor := NewGoExtractor()
			m, err := extractor.Extract(goFile)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}

			tt.check(t, m)
		})
	}
}

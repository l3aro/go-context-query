package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

func TestRustExtractor(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		check func(*testing.T, *types.ModuleInfo)
	}{
		{
			name: "simple function",
			code: `fn hello() -> &'static str {
    "hello"
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				if m.Functions[0].Name != "hello" {
					t.Errorf("expected function name hello, got %s", m.Functions[0].Name)
				}
			},
		},
		{
			name: "function with params and return type",
			code: `fn add(a: i32, b: i32) -> i32 {
    a + b
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
				if fn.Params != "(a: i32, b: i32)" {
					t.Errorf("expected params '(a: i32, b: i32)', got %s", fn.Params)
				}
				if fn.ReturnType != "i32" {
					t.Errorf("expected return type 'i32', got %s", fn.ReturnType)
				}
			},
		},
		{
			name: "async function",
			code: `async fn fetch_data() -> Result<String, Error> {
    Ok(String::from("data"))
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				fn := m.Functions[0]
				if fn.Name != "fetch_data" {
					t.Errorf("expected function name fetch_data, got %s", fn.Name)
				}
				if !fn.IsAsync {
					t.Errorf("expected IsAsync to be true")
				}
			},
		},
		{
			name: "struct definition",
			code: `struct Person {
    name: String,
    age: u32,
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
			name: "generic struct",
			code: `struct Container<T> {
    value: T,
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Structs) != 1 {
					t.Errorf("expected 1 struct, got %d", len(m.Structs))
				}
				strct := m.Structs[0]
				if strct.Name != "Container<T>" {
					t.Errorf("expected struct name Container<T>, got %s", strct.Name)
				}
			},
		},
		{
			name: "trait definition",
			code: `trait Drawable {
    fn draw(&self);
    fn area(&self) -> f64;
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Traits) != 1 {
					t.Errorf("expected 1 trait, got %d", len(m.Traits))
				}
				trait := m.Traits[0]
				if trait.Name != "Drawable" {
					t.Errorf("expected trait name Drawable, got %s", trait.Name)
				}
				if len(trait.Methods) != 2 {
					t.Errorf("expected 2 methods in trait, got %d", len(trait.Methods))
				}
			},
		},
		{
			name: "impl block",
			code: `struct Circle {
    radius: f64,
}

impl Circle {
    fn new(radius: f64) -> Self {
        Circle { radius }
    }

    fn area(&self) -> f64 {
        3.14159 * self.radius * self.radius
    }
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Structs) != 1 {
					t.Errorf("expected 1 struct, got %d", len(m.Structs))
				}
				if len(m.Functions) != 2 {
					t.Errorf("expected 2 methods, got %d", len(m.Functions))
				}
				methodNames := make(map[string]bool)
				for _, fn := range m.Functions {
					methodNames[fn.Name] = true
					if !fn.IsMethod {
						t.Errorf("expected %s to be a method", fn.Name)
					}
				}
				if !methodNames["new"] {
					t.Errorf("expected method 'new' not found")
				}
				if !methodNames["area"] {
					t.Errorf("expected method 'area' not found")
				}
			},
		},
		{
			name: "use statement",
			code: `use std::collections::HashMap;

fn main() {
    let mut map = HashMap::new();
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Imports) != 1 {
					t.Errorf("expected 1 import, got %d", len(m.Imports))
				}
				if m.Imports[0].Module != "std::collections::HashMap" {
					t.Errorf("expected import 'std::collections::HashMap', got %s", m.Imports[0].Module)
				}
			},
		},
		{
			name: "grouped use statement",
			code: `use std::io::{self, Read, Write};

fn main() {}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Imports) != 1 {
					t.Errorf("expected 1 import, got %d", len(m.Imports))
				}
				if m.Imports[0].Module != "std::io" {
					t.Errorf("expected module 'std::io', got %s", m.Imports[0].Module)
				}
				if len(m.Imports[0].Names) != 3 {
					t.Errorf("expected 3 imported names, got %d", len(m.Imports[0].Names))
				}
			},
		},
		{
			name: "enum definition",
			code: `enum Color {
    Red,
    Green,
    Blue,
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Enums) != 1 {
					t.Errorf("expected 1 enum, got %d", len(m.Enums))
				}
				enumItem := m.Enums[0]
				if enumItem.Name != "Color" {
					t.Errorf("expected enum name Color, got %s", enumItem.Name)
				}
				if len(enumItem.Variants) != 3 {
					t.Errorf("expected 3 variants, got %d", len(enumItem.Variants))
				}
				variants := make(map[string]bool)
				for _, v := range enumItem.Variants {
					variants[v] = true
				}
				if !variants["Red"] {
					t.Errorf("expected variant 'Red' not found")
				}
				if !variants["Green"] {
					t.Errorf("expected variant 'Green' not found")
				}
				if !variants["Blue"] {
					t.Errorf("expected variant 'Blue' not found")
				}
			},
		},
		{
			name: "enum with data",
			code: `enum Message {
    Quit,
    Move { x: i32, y: i32 },
    Write(String),
    ChangeColor(i32, i32, i32),
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Enums) != 1 {
					t.Errorf("expected 1 enum, got %d", len(m.Enums))
				}
				enumItem := m.Enums[0]
				if enumItem.Name != "Message" {
					t.Errorf("expected enum name Message, got %s", enumItem.Name)
				}
				if len(enumItem.Variants) != 4 {
					t.Errorf("expected 4 variants, got %d", len(enumItem.Variants))
				}
			},
		},
		{
			name: "multiple functions",
			code: `fn first() {}

fn second() {}

fn third() {}
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
			rustFile := filepath.Join(tmpDir, "test.rs")

			if err := os.WriteFile(rustFile, []byte(tt.code), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			extractor := NewRustExtractor()
			m, err := extractor.Extract(rustFile)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}

			tt.check(t, m)
		})
	}
}

func TestRustExtractorLanguage(t *testing.T) {
	extractor := NewRustExtractor()
	if extractor.Language() != Rust {
		t.Errorf("expected language Rust, got %s", extractor.Language())
	}
}

func TestRustExtractorFileExtensions(t *testing.T) {
	extractor := &RustExtractor{}
	extensions := extractor.FileExtensions()
	if len(extensions) != 2 {
		t.Errorf("expected 2 file extensions, got %d", len(extensions))
	}
	if extensions[0] != ".rs" || extensions[1] != ".rlib" {
		t.Errorf("expected extensions [.rs .rlib], got %v", extensions)
	}
}

func TestRustExtractorExtractFromBytes(t *testing.T) {
	code := []byte(`fn test() -> i32 {
    42
}
`)

	extractor := NewRustExtractor()
	m, err := extractor.(*RustExtractor).ExtractFromBytes(code, "test.rs")
	if err != nil {
		t.Fatalf("ExtractFromBytes failed: %v", err)
	}

	if len(m.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(m.Functions))
	}

	if m.Functions[0].Name != "test" {
		t.Errorf("expected function name test, got %s", m.Functions[0].Name)
	}
}

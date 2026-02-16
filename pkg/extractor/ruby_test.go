package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

func TestRubyExtractor(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		check func(*testing.T, *types.ModuleInfo)
	}{
		{
			name: "simple method",
			code: `def hello
  "hello"
end
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
			name: "method with params",
			code: `def add(a, b)
  a + b
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				fn := m.Functions[0]
				if fn.Name != "add" {
					t.Errorf("expected function name add, got %s", fn.Name)
				}
				if fn.Params != "(a, b)" {
					t.Errorf("expected params '(a, b)', got %s", fn.Params)
				}
			},
		},
		{
			name: "simple class",
			code: `class Person
  def initialize(name)
    @name = name
  end

  def greet
    "Hello, #{@name}"
  end
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 1 {
					t.Errorf("expected 1 class, got %d", len(m.Classes))
				}
				class := m.Classes[0]
				if class.Name != "Person" {
					t.Errorf("expected class name Person, got %s", class.Name)
				}
				if len(class.Methods) != 2 {
					t.Errorf("expected 2 methods, got %d", len(class.Methods))
				}
			},
		},
		{
			name: "class with inheritance",
			code: `class Employee < Person
  def initialize(name, salary)
    super(name)
    @salary = salary
  end
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 1 {
					t.Errorf("expected 1 class, got %d", len(m.Classes))
				}
				class := m.Classes[0]
				if class.Name != "Employee" {
					t.Errorf("expected class name Employee, got %s", class.Name)
				}
				if len(class.Bases) != 1 {
					t.Errorf("expected 1 base class, got %d", len(class.Bases))
				}
				if class.Bases[0] != "Person" {
					t.Errorf("expected base class Person, got %s", class.Bases[0])
				}
			},
		},
		{
			name: "simple module",
			code: `module Greeter
  def greet(name)
    "Hello, #{name}"
  end
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Traits) != 1 {
					t.Errorf("expected 1 module (stored as trait), got %d", len(m.Traits))
				}
				mod := m.Traits[0]
				if mod.Name != "Greeter" {
					t.Errorf("expected module name Greeter, got %s", mod.Name)
				}
				if len(mod.Methods) != 1 {
					t.Errorf("expected 1 method in module, got %d", len(mod.Methods))
				}
			},
		},
		{
			name: "require statements",
			code: `require "json"
require "net/http"

require_relative "helpers"

class ApiClient
  def fetch
    # code
  end
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Imports) != 3 {
					t.Errorf("expected 3 imports, got %d", len(m.Imports))
				}
				modules := make(map[string]bool)
				for _, imp := range m.Imports {
					modules[imp.Module] = true
				}
				if !modules["json"] {
					t.Errorf("expected import 'json' not found")
				}
				if !modules["net/http"] {
					t.Errorf("expected import 'net/http' not found")
				}
				if !modules["helpers"] {
					t.Errorf("expected import 'helpers' not found")
				}
			},
		},
		{
			name: "class method (singleton)",
			code: `class Calculator
  def self.add(a, b)
    a + b
  end
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 1 {
					t.Errorf("expected 1 class, got %d", len(m.Classes))
				}
				class := m.Classes[0]
				if len(class.Methods) != 1 {
					t.Errorf("expected 1 method, got %d", len(class.Methods))
				}
				method := class.Methods[0]
				if method.Name != "self.add" {
					t.Errorf("expected method name 'self.add', got %s", method.Name)
				}
			},
		},
		{
			name: "multiple methods",
			code: `def first
  1
end

def second
  2
end

def third
  3
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 3 {
					t.Errorf("expected 3 functions, got %d", len(m.Functions))
				}
			},
		},
		{
			name: "method with block param",
			code: `def each_item(&block)
  items.each(&block)
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				fn := m.Functions[0]
				if fn.Name != "each_item" {
					t.Errorf("expected function name each_item, got %s", fn.Name)
				}
			},
		},
		{
			name: "namespaced class",
			code: `module Utils
  class Helper
    def help
      "helping"
    end
  end
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Traits) != 1 {
					t.Errorf("expected 1 module (stored as trait), got %d", len(m.Traits))
				}
			},
		},
		{
			name: "empty class",
			code: `class EmptyClass
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 1 {
					t.Errorf("expected 1 class, got %d", len(m.Classes))
				}
				class := m.Classes[0]
				if class.Name != "EmptyClass" {
					t.Errorf("expected class name EmptyClass, got %s", class.Name)
				}
				if len(class.Methods) != 0 {
					t.Errorf("expected 0 methods, got %d", len(class.Methods))
				}
			},
		},
		{
			name: "method with comment",
			code: `# This is a greeting method
def greet(name)
  "Hello, #{name}"
end
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 1 {
					t.Errorf("expected 1 function, got %d", len(m.Functions))
				}
				fn := m.Functions[0]
				if fn.Name != "greet" {
					t.Errorf("expected function name greet, got %s", fn.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			rubyFile := filepath.Join(tmpDir, "test.rb")

			if err := os.WriteFile(rubyFile, []byte(tt.code), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			extractor := NewRubyExtractor()
			m, err := extractor.Extract(rubyFile)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}

			tt.check(t, m)
		})
	}
}

func TestRubyExtractorLanguage(t *testing.T) {
	extractor := NewRubyExtractor()
	if extractor.Language() != Ruby {
		t.Errorf("expected language Ruby, got %s", extractor.Language())
	}
}

func TestRubyExtractorFileExtensions(t *testing.T) {
	extractor := &RubyExtractor{}
	extensions := extractor.FileExtensions()
	if len(extensions) != 3 {
		t.Errorf("expected 3 file extensions, got %d", len(extensions))
	}
	expected := map[string]bool{
		".rb":      false,
		".erb":     false,
		".gemspec": false,
	}
	for _, ext := range extensions {
		if _, ok := expected[ext]; ok {
			expected[ext] = true
		}
	}
	for ext, found := range expected {
		if !found {
			t.Errorf("expected extension %s not found", ext)
		}
	}
}

func TestRubyExtractorExtractFromBytes(t *testing.T) {
	code := []byte(`def test
  42
end
`)

	extractor := NewRubyExtractor()
	m, err := extractor.(*RubyExtractor).ExtractFromBytes(code, "test.rb")
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

func TestRubyExtractorComplexFile(t *testing.T) {
	code := `# A simple service class for user management
require "json"
require_relative "base_service"

module Services
  # Base class for all services
  class UserService
    def initialize(api_client)
      @api_client = api_client
    end

    def find_user(id)
      @api_client.get("/users/#{id}")
    end

    def self.create_admin(name)
      new(AdminApiClient.new).create(name: name, role: :admin)
    end
  end

  module Helpers
    def format_response(data)
      { success: true, data: data }
    end
  end
end
`

	tmpDir := t.TempDir()
	rubyFile := filepath.Join(tmpDir, "user_service.rb")

	if err := os.WriteFile(rubyFile, []byte(code), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	extractor := NewRubyExtractor()
	m, err := extractor.Extract(rubyFile)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(m.Imports) < 2 {
		t.Errorf("expected at least 2 imports, got %d", len(m.Imports))
	}

	if len(m.Traits) != 2 {
		t.Errorf("expected 2 modules (stored as traits), got %d", len(m.Traits))
	}

	foundServices := false
	foundHelpers := false
	for _, trait := range m.Traits {
		if trait.Name == "Services" {
			foundServices = true
		} else if trait.Name == "Helpers" {
			foundHelpers = true
		}
	}
	if !foundServices {
		t.Errorf("expected to find Services module")
	}
	if !foundHelpers {
		t.Errorf("expected to find Helpers module")
	}
}

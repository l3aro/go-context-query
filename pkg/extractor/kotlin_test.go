package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

func TestKotlinExtractor(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		check func(*testing.T, *types.ModuleInfo)
	}{
		{
			name: "simple class",
			code: `package com.example

class Hello {
    fun sayHello() {
        println("Hello")
    }
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 1 {
					t.Errorf("expected 1 class, got %d", len(m.Classes))
				}
				if m.Classes[0].Name != "Hello" {
					t.Errorf("expected class name Hello, got %s", m.Classes[0].Name)
				}
			},
		},
		{
			name: "class with methods",
			code: `package com.example

class Calculator {
    fun add(a: Int, b: Int): Int {
        return a + b
    }
    
    fun subtract(a: Int, b: Int): Int {
        return a - b
    }
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 1 {
					t.Errorf("expected 1 class, got %d", len(m.Classes))
				}
				class := m.Classes[0]
				if class.Name != "Calculator" {
					t.Errorf("expected class name Calculator, got %s", class.Name)
				}
				if len(class.Methods) != 2 {
					t.Errorf("expected 2 methods, got %d", len(class.Methods))
				}
			},
		},
		{
			name: "interface definition",
			code: `package com.example

interface Greeting {
    fun sayHello()
    fun getMessage(): String
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Interfaces) != 1 {
					t.Errorf("expected 1 interface, got %d", len(m.Interfaces))
				}
				iface := m.Interfaces[0]
				if iface.Name != "Greeting" {
					t.Errorf("expected interface name Greeting, got %s", iface.Name)
				}
				if len(iface.Methods) != 2 {
					t.Errorf("expected 2 interface methods, got %d", len(iface.Methods))
				}
			},
		},
		{
			name: "import statements",
			code: `package com.example

import java.util.List
import java.util.ArrayList

class Importer {
    private val items: List<String> = ArrayList()
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Imports) != 2 {
					t.Errorf("expected 2 imports, got %d", len(m.Imports))
				}
				foundList := false
				foundArrayList := false
				for _, imp := range m.Imports {
					if imp.Module == "java.util.List" {
						foundList = true
					}
					if imp.Module == "java.util.ArrayList" {
						foundArrayList = true
					}
				}
				if !foundList {
					t.Errorf("expected import java.util.List")
				}
				if !foundArrayList {
					t.Errorf("expected import java.util.ArrayList")
				}
			},
		},
		{
			name: "class with inheritance",
			code: `package com.example

class Dog : Animal, Pet {
    fun bark() {
        println("Woof!")
    }
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 1 {
					t.Errorf("expected 1 class, got %d", len(m.Classes))
				}
				class := m.Classes[0]
				if class.Name != "Dog" {
					t.Errorf("expected class name Dog, got %s", class.Name)
				}
			},
		},
		{
			name: "data class",
			code: `package com.example

data class User(val name: String, val age: Int)
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 1 {
					t.Errorf("expected 1 class (data class), got %d", len(m.Classes))
				}
				class := m.Classes[0]
				if class.Name != "User" {
					t.Errorf("expected class name User, got %s", class.Name)
				}
			},
		},
		{
			name: "object declaration",
			code: `package com.example

object Singleton {
    fun doSomething() {
        println("Doing something")
    }
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				foundSingleton := false
				for _, class := range m.Classes {
					if class.Name == "Singleton" {
						foundSingleton = true
						break
					}
				}
				if !foundSingleton {
					t.Errorf("expected object Singleton to be extracted")
				}
			},
		},
		{
			name: "top-level function",
			code: `package com.example

fun main() {
    println("Hello, World!")
}

fun add(a: Int, b: Int): Int = a + b
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Functions) != 2 {
					t.Errorf("expected 2 top-level functions, got %d", len(m.Functions))
				}
				foundMain := false
				foundAdd := false
				for _, fn := range m.Functions {
					if fn.Name == "main" {
						foundMain = true
					}
					if fn.Name == "add" {
						foundAdd = true
					}
				}
				if !foundMain {
					t.Errorf("expected function main")
				}
				if !foundAdd {
					t.Errorf("expected function add")
				}
			},
		},
		{
			name: "multiple classes",
			code: `package com.example

class First {
    fun method1() {}
}

class Second {
    fun method2() {}
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 2 {
					t.Errorf("expected 2 classes, got %d", len(m.Classes))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			kotlinFile := filepath.Join(tmpDir, "Test.kt")

			if err := os.WriteFile(kotlinFile, []byte(tt.code), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			extractor := NewKotlinExtractor()
			m, err := extractor.Extract(kotlinFile)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}

			tt.check(t, m)
		})
	}
}

func TestKotlinExtractorLanguage(t *testing.T) {
	extractor := NewKotlinExtractor()
	if extractor.Language() != Kotlin {
		t.Errorf("expected language Kotlin, got %s", extractor.Language())
	}
}

func TestKotlinExtractorFileExtensions(t *testing.T) {
	extractor := NewKotlinExtractor().(*KotlinExtractor)
	extensions := extractor.FileExtensions()
	if len(extensions) != 2 || extensions[0] != ".kt" || extensions[1] != ".kts" {
		t.Errorf("expected [.kt .kts], got %v", extensions)
	}
}

func TestKotlinExtractorExtractFromBytes(t *testing.T) {
	code := `package com.example

class TestClass {
    fun testMethod() {
        // test
    }
}
`
	extractor := NewKotlinExtractor().(*KotlinExtractor)
	m, err := extractor.ExtractFromBytes([]byte(code), "Test.kt")
	if err != nil {
		t.Fatalf("ExtractFromBytes failed: %v", err)
	}

	if len(m.Classes) != 1 {
		t.Errorf("expected 1 class, got %d", len(m.Classes))
	}
	if m.Classes[0].Name != "TestClass" {
		t.Errorf("expected class name TestClass, got %s", m.Classes[0].Name)
	}
}

func TestKotlinScriptFile(t *testing.T) {
	code := `fun main() {
    println("Hello from Kotlin Script!")
}
`
	tmpDir := t.TempDir()
	kotlinFile := filepath.Join(tmpDir, "test.kts")

	if err := os.WriteFile(kotlinFile, []byte(code), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	extractor := NewKotlinExtractor()
	m, err := extractor.Extract(kotlinFile)
	if err != nil {
		t.Fatalf("Extract failed: %v", err)
	}

	if len(m.Functions) != 1 {
		t.Errorf("expected 1 function, got %d", len(m.Functions))
	}
	if m.Functions[0].Name != "main" {
		t.Errorf("expected function name main, got %s", m.Functions[0].Name)
	}
}

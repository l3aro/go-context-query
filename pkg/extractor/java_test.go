package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

func TestJavaExtractor(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		check func(*testing.T, *types.ModuleInfo)
	}{
		{
			name: "simple class",
			code: `package com.example;

public class Hello {
    public void sayHello() {
        System.out.println("Hello");
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
			code: `package com.example;

public class Calculator {
    public int add(int a, int b) {
        return a + b;
    }
    
    public int subtract(int a, int b) {
        return a - b;
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
			code: `package com.example;

public interface Greeting {
    void sayHello();
    String getMessage();
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
			code: `package com.example;

import java.util.List;
import java.util.ArrayList;

public class Importer {
    private List<String> items = new ArrayList<>();
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
			code: `package com.example;

public class Dog extends Animal implements Pet {
    public void bark() {
        System.out.println("Woof!");
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
				// Should have at least 2 bases (Animal and Pet)
				if len(class.Bases) < 2 {
					t.Errorf("expected at least 2 base classes/interfaces, got %d", len(class.Bases))
				}
			},
		},
		{
			name: "constructor",
			code: `package com.example;

public class Person {
    private String name;
    
    public Person(String name) {
        this.name = name;
    }
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Classes) != 1 {
					t.Errorf("expected 1 class, got %d", len(m.Classes))
				}
				class := m.Classes[0]
				if len(class.Methods) != 1 {
					t.Errorf("expected 1 method (constructor), got %d", len(class.Methods))
				}
				method := class.Methods[0]
				if method.Name != "Person" {
					t.Errorf("expected constructor name Person, got %s", method.Name)
				}
			},
		},
		{
			name: "multiple classes",
			code: `package com.example;

class First {
    void method1() {}
}

class Second {
    void method2() {}
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
			javaFile := filepath.Join(tmpDir, "Test.java")

			if err := os.WriteFile(javaFile, []byte(tt.code), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			extractor := NewJavaExtractor()
			m, err := extractor.Extract(javaFile)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}

			tt.check(t, m)
		})
	}
}

func TestJavaExtractorLanguage(t *testing.T) {
	extractor := NewJavaExtractor()
	if extractor.Language() != Java {
		t.Errorf("expected language Java, got %s", extractor.Language())
	}
}

func TestJavaExtractorFileExtensions(t *testing.T) {
	extractor := NewJavaExtractor().(*JavaExtractor)
	extensions := extractor.FileExtensions()
	if len(extensions) != 1 || extensions[0] != ".java" {
		t.Errorf("expected [.java], got %v", extensions)
	}
}

func TestJavaExtractorExtractFromBytes(t *testing.T) {
	code := `package com.example;

public class TestClass {
    public void testMethod() {
        // test
    }
}
`
	extractor := NewJavaExtractor().(*JavaExtractor)
	m, err := extractor.ExtractFromBytes([]byte(code), "Test.java")
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

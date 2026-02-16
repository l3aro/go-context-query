package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

func TestCPPExtractor(t *testing.T) {
	tests := []struct {
		name  string
		code  string
		check func(*testing.T, *types.ModuleInfo)
	}{
		{
			name: "simple function",
			code: `
#include <iostream>

void hello() {
	std::cout << "hello" << std::endl;
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
			name: "class definition",
			code: `
class Point {
private:
	int x;
	int y;
public:
	Point(int x, int y);
	int getX() const;
	int getY() const;
};
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				found := false
				for _, c := range m.Classes {
					if c.Name == "Point" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find class Point")
				}
			},
		},
		{
			name: "class with inheritance",
			code: `
class Shape {
public:
	virtual double area() const = 0;
};

class Circle : public Shape {
private:
	double radius;
public:
	Circle(double r) : radius(r) {}
	double area() const override;
};
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				var foundCircle bool
				for _, c := range m.Classes {
					if c.Name == "Circle" {
						foundCircle = true
						if len(c.Bases) == 0 {
							t.Errorf("expected Circle to have base classes")
						}
						break
					}
				}
				if !foundCircle {
					t.Errorf("expected to find class Circle")
				}
			},
		},
		{
			name: "struct definition",
			code: `
struct Vector3D {
	double x;
	double y;
	double z;
	
	Vector3D(double x, double y, double z);
	double magnitude() const;
};
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				found := false
				for _, s := range m.Structs {
					if s.Name == "Vector3D" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find struct Vector3D")
				}
			},
		},
		{
			name: "include directives",
			code: `
#include <iostream>
#include <vector>
#include <string>
#include "myheader.hpp"
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				if len(m.Imports) != 4 {
					t.Errorf("expected 4 includes, got %d", len(m.Imports))
				}
				foundIostream := false
				foundMyHeader := false
				for _, imp := range m.Imports {
					if imp.Module == "iostream" {
						foundIostream = true
						if !imp.IsFrom {
							t.Errorf("expected iostream to be marked as system include")
						}
					}
					if imp.Module == "myheader.hpp" {
						foundMyHeader = true
					}
				}
				if !foundIostream {
					t.Errorf("expected to find iostream include")
				}
				if !foundMyHeader {
					t.Errorf("expected to find myheader.hpp include")
				}
			},
		},
		{
			name: "template function",
			code: `
template<typename T>
T max(T a, T b) {
	return (a > b) ? a : b;
}
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				found := false
				for _, c := range m.Classes {
					if c.Name == "max" && c.Docstring == "template" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find template function max")
				}
			},
		},
		{
			name: "template class",
			code: `
template<typename T>
class Stack {
private:
	std::vector<T> items;
public:
	void push(const T& item);
	T pop();
	bool empty() const;
};
`,
			check: func(t *testing.T, m *types.ModuleInfo) {
				found := false
				for _, c := range m.Classes {
					if c.Name == "Stack" {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected to find class Stack")
				}
			},
		},
		{
			name: "multiple functions",
			code: `
void func1() {}
int func2(int x) { return x; }
double func3(double x, double y) { return x + y; }
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
			cppFile := filepath.Join(tmpDir, "test.cpp")

			if err := os.WriteFile(cppFile, []byte(tt.code), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			extractor := NewCPPExtractor()
			m, err := extractor.Extract(cppFile)
			if err != nil {
				t.Fatalf("Extract failed: %v", err)
			}

			tt.check(t, m)
		})
	}
}

func TestCPPExtractorLanguage(t *testing.T) {
	extractor := NewCPPExtractor()
	if extractor.Language() != CPP {
		t.Errorf("expected language CPP, got %s", extractor.Language())
	}
}

func TestCPPExtractorFileExtensions(t *testing.T) {
	extractor := NewCPPExtractor()
	exts := extractor.FileExtensions()
	if len(exts) != 6 {
		t.Errorf("expected 6 extensions, got %d", len(exts))
	}
	expected := []string{".cpp", ".hpp", ".cc", ".hh", ".cxx", ".hxx"}
	for i, ext := range expected {
		if exts[i] != ext {
			t.Errorf("expected extension %s, got %s", ext, exts[i])
		}
	}
}

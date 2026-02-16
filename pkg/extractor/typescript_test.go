package extractor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/l3aro/go-context-query/pkg/types"
)

// TestTypeScriptExtractorAdvanced tests advanced extraction features
func TestTypeScriptExtractorAdvanced(t *testing.T) {
	tmpDir := t.TempDir()
	tsFile := filepath.Join(tmpDir, "advanced_test.ts")

	tsCode := `/**
 * Advanced test module with various TypeScript constructs.
 */

import { Component } from 'react';
import { useState, useEffect } from './hooks';
import * as Utils from './utils';
import DefaultExport, { NamedExport } from 'module';
import { foo as bar } from 'aliased';

// Function declarations
function simpleFunction(x: number, y: number): number {
    return x + y;
}

async function asyncFunction(): Promise<string> {
    return "hello";
}

function functionNoTypes(a, b) {
    return a + b;
}

// Arrow functions
const arrowFunction = (x: number): number => x * 2;

const arrowFunctionMulti = (a: string, b: string): string => {
    return a + b;
};

// Class declarations
class BaseClass {
    /** Base class constructor */
    constructor(public value: number) {}
    
    baseMethod(): number {
        return this.value;
    }
}

class ChildClass extends BaseClass {
    /** Child class method */
    childMethod(x: number): number {
        return x * 2;
    }
}

// Interface declarations
interface UserInterface {
    id: number;
    name: string;
    email?: string;
}

interface ExtendedUser extends UserInterface {
    createdAt: Date;
}

// Type declarations
type StringOrNumber = string | number;
type Callback = (data: string) => void;

// Enum declarations
enum Color {
    Red = "red",
    Green = "green",
    Blue = "blue"
}

// Export statements
export function exportedFunction(): void {
    console.log("exported");
}

export class ExportedClass {
    public method(): void {}
}

export interface ExportedInterface {
    prop: string;
}

export type ExportedType = string | number;
`

	err := os.WriteFile(tsFile, []byte(tsCode), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	extractor := NewTypeScriptExtractor()
	info, err := extractor.Extract(tsFile)
	if err != nil {
		t.Fatalf("Extract() failed: %v", err)
	}

	t.Run("FunctionExtraction", func(t *testing.T) {
		if len(info.Functions) == 0 {
			t.Fatal("Expected functions to be extracted")
		}

		funcMap := make(map[string]bool)
		for _, fn := range info.Functions {
			funcMap[fn.Name] = true
		}

		expectedFunctions := []string{
			"simpleFunction",
			"asyncFunction",
			"functionNoTypes",
			"exportedFunction",
		}

		for _, name := range expectedFunctions {
			if !funcMap[name] {
				t.Errorf("Expected function '%s' not found", name)
			}
		}
	})

	t.Run("AsyncFunctionDetection", func(t *testing.T) {
		var asyncFound bool
		for _, fn := range info.Functions {
			if fn.Name == "asyncFunction" && fn.IsAsync {
				asyncFound = true
			}
		}
		if !asyncFound {
			t.Error("asyncFunction should have IsAsync=true")
		}
	})

	t.Run("ClassExtraction", func(t *testing.T) {
		if len(info.Classes) == 0 {
			t.Fatal("Expected classes to be extracted")
		}

		classMap := make(map[string]bool)
		for _, cls := range info.Classes {
			classMap[cls.Name] = true
		}

		expectedClasses := []string{
			"BaseClass",
			"ChildClass",
			"ExportedClass",
		}

		for _, name := range expectedClasses {
			if !classMap[name] {
				t.Errorf("Expected class '%s' not found", name)
			}
		}
	})

	t.Run("MethodExtraction", func(t *testing.T) {
		classMap := make(map[string]types.Class)
		for i := range info.Classes {
			classMap[info.Classes[i].Name] = info.Classes[i]
		}

		if baseClass, ok := classMap["BaseClass"]; ok {
			if len(baseClass.Methods) == 0 {
				t.Error("BaseClass should have methods")
			}
		}
	})

	t.Run("InterfaceExtraction", func(t *testing.T) {
		if len(info.Interfaces) == 0 {
			t.Fatal("Expected interfaces to be extracted")
		}

		ifaceMap := make(map[string]bool)
		for _, iface := range info.Interfaces {
			ifaceMap[iface.Name] = true
		}

		expected := []string{"UserInterface", "ExtendedUser", "ExportedInterface"}
		for _, name := range expected {
			if !ifaceMap[name] {
				t.Errorf("Expected interface '%s' not found", name)
			}
		}
	})

	t.Run("EnumExtraction", func(t *testing.T) {
		if len(info.Enums) == 0 {
			t.Fatal("Expected enums to be extracted")
		}

		enumMap := make(map[string]bool)
		for _, enum := range info.Enums {
			enumMap[enum.Name] = true
		}

		if !enumMap["Color"] {
			t.Error("Expected enum 'Color' not found")
		}
	})

	t.Run("LineNumbers", func(t *testing.T) {
		for _, fn := range info.Functions {
			if fn.LineNumber == 0 {
				t.Errorf("Function %s should have a line number", fn.Name)
			}
		}

		for _, cls := range info.Classes {
			if cls.LineNumber == 0 {
				t.Errorf("Class %s should have a line number", cls.Name)
			}
		}
	})
}

// TestTypeScriptExtractorEdgeCases tests edge cases
func TestTypeScriptExtractorEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		code     string
		expected int
	}{
		{
			name:     "empty_file",
			code:     "",
			expected: 0,
		},
		{
			name:     "only_imports",
			code:     "import { x } from 'module';\nimport y from 'other';\n",
			expected: 0,
		},
		{
			name:     "class_only",
			code:     "class MyClass {\n    method() {}\n}\n",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tsFile := filepath.Join(tmpDir, tt.name+".ts")
			err := os.WriteFile(tsFile, []byte(tt.code), 0644)
			if err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			extractor := NewTypeScriptExtractor()
			info, err := extractor.Extract(tsFile)
			if err != nil {
				t.Fatalf("Extract() failed: %v", err)
			}

			if len(info.Functions) != tt.expected {
				t.Errorf("Expected %d functions, got %d", tt.expected, len(info.Functions))
			}
		})
	}
}

// TestExtractTypeScriptFromBytes tests extraction directly from bytes
func TestExtractTypeScriptFromBytes(t *testing.T) {
	tsCode := []byte(`
function hello(): string {
    return "hello";
}

class Greeter {
    greet(name: string): string {
        return "Hello, " + name + "!";
    }
}
`)

	extractor := NewTypeScriptExtractor().(*TypeScriptExtractor)
	info, err := extractor.ExtractFromBytes(tsCode, "test.ts")
	if err != nil {
		t.Fatalf("ExtractFromBytes() failed: %v", err)
	}

	if len(info.Functions) != 1 {
		t.Errorf("Expected 1 function, got %d", len(info.Functions))
	}

	if len(info.Classes) != 1 {
		t.Errorf("Expected 1 class, got %d", len(info.Classes))
	}

	if info.Path != "test.ts" {
		t.Errorf("Expected path 'test.ts', got '%s'", info.Path)
	}
}

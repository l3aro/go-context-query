package dfg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPythonScopeTracking(t *testing.T) {
	tests := []struct {
		name         string
		code         string
		funcName     string
		wantVars     []string
		wantCaptures bool
	}{
		{
			name: "lambda captures outer variable",
			code: `def test_func():
	x = 10
	f = lambda y: x + y
	return f`,
			funcName:     "test_func",
			wantVars:     []string{"x", "f", "y"},
			wantCaptures: true,
		},
		{
			name: "lambda parameter is local",
			code: `def test_func():
	f = lambda x: x * 2
	return f(5)`,
			funcName:     "test_func",
			wantVars:     []string{"f", "x"},
			wantCaptures: false,
		},
		{
			name: "list comprehension scope",
			code: `def test_func():
	outer = [1, 2, 3]
	result = [x * 2 for x in outer]
	return result`,
			funcName:     "test_func",
			wantVars:     []string{"outer", "result", "x"},
			wantCaptures: true,
		},
		{
			name: "nested lambda captures",
			code: `def test_func():
	outer = 1
	f = lambda x: lambda y: outer + x + y
	return f`,
			funcName:     "test_func",
			wantVars:     []string{"outer", "f", "x"},
			wantCaptures: true,
		},
		{
			name: "dict comprehension scope",
			code: `def test_func():
	keys = ['a', 'b']
	result = {k: len(k) for k in keys}
	return result`,
			funcName:     "test_func",
			wantVars:     []string{"keys", "result", "k"},
			wantCaptures: true,
		},
		{
			name: "generator expression scope",
			code: `def test_func():
	data = [1, 2, 3]
	gen = (x * 2 for x in data)
	return gen`,
			funcName:     "test_func",
			wantVars:     []string{"data", "gen", "x"},
			wantCaptures: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file with test code
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.py")
			if err := os.WriteFile(tmpFile, []byte(tt.code), 0644); err != nil {
				t.Fatalf("failed to write test file: %v", err)
			}

			dfg, err := extractPythonDFG(tmpFile, tt.funcName)
			if err != nil {
				t.Fatalf("extractPythonDFG failed: %v", err)
			}

			// Check that expected variables are tracked
			for _, varName := range tt.wantVars {
				found := false
				for _, ref := range dfg.VarRefs {
					if ref.Name == varName {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected variable %q to be tracked, but it wasn't found in refs", varName)
				}
			}

			// Check that scope stack is being used (by verifying we got refs)
			if len(dfg.VarRefs) == 0 {
				t.Error("expected some variable references, got none")
			}

			t.Logf("DFG for %s: %d refs, vars: %v", tt.name, len(dfg.VarRefs), getVarNames(dfg.VarRefs))
		})
	}
}

func TestPythonLambdaCaptures(t *testing.T) {
	code := `def outer_func():
	x = 10
	y = 20
	f = lambda a: x + y + a
	return f`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	dfg, err := extractPythonDFG(tmpFile, "outer_func")
	if err != nil {
		t.Fatalf("extractPythonDFG failed: %v", err)
	}

	varNames := getVarNames(dfg.VarRefs)

	expectedVars := []string{"x", "y", "f", "a"}
	for _, expected := range expectedVars {
		found := false
		for _, name := range varNames {
			if name == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected variable %q to be tracked", expected)
		}
	}

	xRefs := dfg.Variables["x"]
	if len(xRefs) == 0 {
		t.Error("expected x to have references")
	}

	hasDef := false
	hasUse := false
	for _, ref := range xRefs {
		if ref.RefType == RefTypeDefinition {
			hasDef = true
		}
		if ref.RefType == RefTypeUse {
			hasUse = true
		}
	}
	if !hasDef {
		t.Error("expected x to have a definition")
	}
	if !hasUse {
		t.Error("expected x to have uses (captured in closures)")
	}
}

func TestPythonComprehensionScope(t *testing.T) {
	code := `def test_func():
	# The loop variable in comprehension should be local to comprehension
	# and not leak to outer scope
	x = 1
	result = [x + i for i in range(10)]
	# x here refers to outer x, i is not accessible
	return x, result`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.py")
	if err := os.WriteFile(tmpFile, []byte(code), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	dfg, err := extractPythonDFG(tmpFile, "test_func")
	if err != nil {
		t.Fatalf("extractPythonDFG failed: %v", err)
	}

	// Should have: x (def), result (def), i (def in comprehension), x (use), i (use)
	varNames := getVarNames(dfg.VarRefs)

	if !contains(varNames, "x") {
		t.Error("expected x to be tracked")
	}
	if !contains(varNames, "result") {
		t.Error("expected result to be tracked")
	}
	if !contains(varNames, "i") {
		t.Error("expected i (comprehension variable) to be tracked")
	}

	t.Logf("Variables tracked: %v", varNames)
}

func getVarNames(refs []VarRef) []string {
	seen := make(map[string]bool)
	var result []string
	for _, ref := range refs {
		if !seen[ref.Name] {
			seen[ref.Name] = true
			result = append(result, ref.Name)
		}
	}
	return result
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.TrimSpace(s) == item {
			return true
		}
	}
	return false
}

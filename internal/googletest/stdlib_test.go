package googletest

import (
	"go/parser"
	gotoken "go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The emulator's own code uses only the Go standard library, so the
// yardstick shares no code with eventlog, which it measures; its tests use
// eventlog's service clients (D-081).
func TestEmulatorImportsOnlyTheStandardLibrary(t *testing.T) {
	paths, err := filepath.Glob("*.go")
	if err != nil || len(paths) == 0 {
		t.Fatalf("source files: %v", err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(gotoken.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, spec := range parsed.Imports {
			imported, _ := strconv.Unquote(spec.Path.Value)
			if first, _, _ := strings.Cut(imported, "/"); strings.Contains(first, ".") {
				t.Errorf("%s imports %s, which is not in the standard library", path, imported)
			}
		}
	}
}

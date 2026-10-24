package service

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestMacBundleFindsProjectAndAdjacentPythonEnvironments(t *testing.T) {
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module test"), 0o600); err != nil {
		t.Fatal(err)
	}
	adjacent := filepath.Join(project, "build", "bin")
	executable := filepath.Join(adjacent, "photo-backup-manager.app", "Contents", "MacOS", "photo-backup-manager")
	roots := aiPythonSearchRoots(t.TempDir(), executable, "darwin")
	if !slices.Contains(roots, project) || !slices.Contains(roots, adjacent) {
		t.Fatalf("roots = %v", roots)
	}
}

func TestMacFinderCanDiscoverHomebrewAndPythonOrgWithoutShellPath(t *testing.T) {
	paths := macSystemPythonPaths()
	for _, path := range []string{"/opt/homebrew/bin/python3.13", "/opt/homebrew/opt/python@3.13/bin/python3.13", "/Library/Frameworks/Python.framework/Versions/3.13/bin/python3.13"} {
		if !slices.Contains(paths, path) {
			t.Fatalf("missing %s", path)
		}
	}
}

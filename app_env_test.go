package main

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestMacBundleEnvIsFoundOutsideTheBundle(t *testing.T) {
	root := t.TempDir()
	executable := filepath.Join(root, "Photo Backup.app", "Contents", "MacOS", "photo-backup-manager")
	want := []string{filepath.Join(root, "Photo Backup.app", "Contents", "MacOS", ".env"), filepath.Join(root, ".env")}
	if paths := appEnvPaths(executable, "darwin"); !reflect.DeepEqual(paths, want) {
		t.Fatalf("paths = %v", paths)
	}
}

func TestEnvLookupDoesNotWalkUnrelatedParentDirectories(t *testing.T) {
	root := t.TempDir()
	for _, platform := range []string{"windows", "darwin"} {
		executable := filepath.Join(root, "folder", "photo-backup-manager")
		want := []string{filepath.Join(root, "folder", ".env")}
		if paths := appEnvPaths(executable, platform); !reflect.DeepEqual(paths, want) {
			t.Fatalf("%s paths = %v", platform, paths)
		}
	}
}

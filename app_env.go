package main

import (
	"path/filepath"
	"strings"
)

// Finder starts a .app independently of the shell's working directory.
// Keep the user's .env outside the signed bundle, next to the .app.
func appEnvPaths(executable, platform string) []string {
	directory := filepath.Dir(executable)
	paths := []string{filepath.Join(directory, ".env")}
	if platform == "darwin" && filepath.Base(directory) == "MacOS" && filepath.Base(filepath.Dir(directory)) == "Contents" {
		bundle := filepath.Dir(filepath.Dir(directory))
		if strings.HasSuffix(strings.ToLower(bundle), ".app") {
			paths = append(paths, filepath.Join(filepath.Dir(bundle), ".env"))
		}
	}
	return paths
}

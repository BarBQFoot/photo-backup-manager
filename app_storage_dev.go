//go:build !production

package main

// Keep the existing project DB when using wails dev and repository tools.
func configureAppStorage() error { return nil }

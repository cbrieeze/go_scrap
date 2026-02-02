package main

import (
	"os"
	"testing"
)

func TestRunTestConfigsFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/README.txt", []byte("ignore"), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	// Ensure flag parsing doesn't panic; avoid running network by using empty temp dir.
	runTestConfigs([]string{"--dir", dir, "--dry-run"})
}

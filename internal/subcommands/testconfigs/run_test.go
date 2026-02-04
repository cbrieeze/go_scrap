package testconfigs

import (
	"os"
	"testing"
)

func TestRunFlags(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/README.txt", []byte("ignore"), 0600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if err := Run([]string{"--dir", dir, "--dry-run"}); err != nil {
		t.Fatalf("run: %v", err)
	}
}

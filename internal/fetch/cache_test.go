package fetch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetCachePath(t *testing.T) {
	path := GetCachePath("https://example.com/docs")
	if path == "" {
		t.Fatal("expected non-empty cache path")
	}
	if filepath.Dir(path) != filepath.Join("artifacts", "cache") {
		t.Fatalf("unexpected cache dir: %s", filepath.Dir(path))
	}
	if !strings.HasSuffix(path, ".html") {
		t.Fatalf("expected html cache file, got %s", path)
	}
}

func TestSaveToCache(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "cache.html")
	content := "<html>cache</html>"

	if err := SaveToCache(path, content); err != nil {
		t.Fatalf("save cache failed: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cache failed: %v", err)
	}
	if string(data) != content {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

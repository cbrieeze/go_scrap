package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
)

func GetCachePath(urlStr string) string {
	h := sha256.Sum256([]byte(urlStr))
	name := hex.EncodeToString(h[:]) + ".html"
	return filepath.Join("output", "cache", name)
}

func SaveToCache(path string, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0600)
}

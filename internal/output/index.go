package output

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go_scrap/internal/parse"
)

type IndexRecord struct {
	ID            string `json:"id"`
	URL           string `json:"url"`
	SourceURL     string `json:"source_url"`
	Heading       string `json:"heading"`
	HeadingLevel  int    `json:"heading_level"`
	HeadingPath   string `json:"heading_path"`
	Content       string `json:"content"`
	TokenEstimate int    `json:"token_estimate"`
}

func WriteIndex(outDir, baseURL string, sections []parse.Section) (string, error) {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, "index.jsonl")
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Track hierarchy: level -> heading text
	hierarchy := make(map[int]string)

	for _, sec := range sections {
		// Update hierarchy
		hierarchy[sec.HeadingLevel] = sec.HeadingText
		// Clear deeper levels
		for k := range hierarchy {
			if k > sec.HeadingLevel {
				delete(hierarchy, k)
			}
		}

		// Build path string "Parent > Child"
		var pathParts []string
		for i := 1; i <= 6; i++ {
			if val, ok := hierarchy[i]; ok {
				pathParts = append(pathParts, val)
			}
		}
		headingPath := strings.Join(pathParts, " > ")

		// Stable ID: hash(baseURL + headingPath + headingID)
		idRaw := baseURL + "|" + headingPath + "|" + sec.HeadingID
		idHash := sha256.Sum256([]byte(idRaw))
		stableID := hex.EncodeToString(idHash[:])[:16]

		rec := IndexRecord{
			ID:            stableID,
			URL:           baseURL, // In a crawler, this would be the specific page URL
			SourceURL:     baseURL + "#" + sec.HeadingID,
			Heading:       sec.HeadingText,
			HeadingLevel:  sec.HeadingLevel,
			HeadingPath:   headingPath,
			Content:       strings.TrimSpace(sec.ContentHTML), // Storing HTML for now, could be MD
			TokenEstimate: len(sec.ContentHTML) / 4,           // Rough estimate
		}

		line, err := json.Marshal(rec)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to marshal index record %q: %v\n", rec.Heading, err)
			continue
		}
		if _, err := f.Write(line); err != nil {
			return "", err
		}
		if _, err := f.Write([]byte("\n")); err != nil {
			return "", err
		}
	}
	return path, nil
}

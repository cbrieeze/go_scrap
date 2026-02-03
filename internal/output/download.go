package output

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type downloadJob struct {
	AbsoluteURL string
	Filename    string
	LocalPath   string
	LocalRef    string
}

func Download(doc *goquery.Document, baseURL, outputDir, userAgent string) error {
	if doc == nil {
		return errors.New("nil document")
	}

	assetsDir := filepath.Join(outputDir, "assets")
	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return err
	}

	downloaded := make(map[string]string)

	doc.Find("img").Each(func(_ int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if !exists || src == "" {
			return
		}

		job, err := buildDownloadJob(src, baseURL, assetsDir)
		if err != nil || job == nil {
			return
		}

		if localName, ok := downloaded[job.AbsoluteURL]; ok {
			s.SetAttr("src", "assets/"+localName)
			return
		}

		if err := fetchAsset(job, userAgent); err == nil {
			downloaded[job.AbsoluteURL] = job.Filename
			s.SetAttr("src", job.LocalRef)
		}
	})

	return nil
}

func buildDownloadJob(src, baseURL, assetsDir string) (*downloadJob, error) {
	u, err := url.Parse(src)
	if err != nil {
		return nil, err
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	absURL := base.ResolveReference(u).String()

	ext := filepath.Ext(absURL)
	if idx := strings.Index(ext, "?"); idx != -1 {
		ext = ext[:idx]
	}
	if ext == "" {
		ext = ".jpg"
	}

	hash := sha256.Sum256([]byte(absURL))
	filename := hex.EncodeToString(hash[:])[:16] + ext
	localPath := filepath.Join(assetsDir, filename)

	return &downloadJob{
		AbsoluteURL: absURL,
		Filename:    filename,
		LocalPath:   localPath,
		LocalRef:    "assets/" + filename,
	}, nil
}

func fetchAsset(job *downloadJob, userAgent string) error {
	if job == nil {
		return fmt.Errorf("missing download job")
	}
	if _, err := os.Stat(job.LocalPath); err == nil {
		return nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", job.AbsoluteURL, nil)
	if err != nil {
		return err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	out, err := os.Create(job.LocalPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

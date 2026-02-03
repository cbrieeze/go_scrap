package fetch_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go_scrap/internal/fetch"
)

func TestFetch_StaticTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := fetch.Fetch(ctx, fetch.Options{URL: srv.URL, Mode: fetch.ModeStatic, Timeout: 10 * time.Millisecond})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestFetch_StaticSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res, err := fetch.Fetch(ctx, fetch.Options{URL: srv.URL, Mode: fetch.ModeStatic, Timeout: time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.HTML == "" {
		t.Fatal("expected html content")
	}
}

func TestFetch_MissingURL(t *testing.T) {
	_, err := fetch.Fetch(context.Background(), fetch.Options{})
	if err == nil {
		t.Fatal("expected error for missing url")
	}
}

func TestFetch_UnknownMode(t *testing.T) {
	_, err := fetch.Fetch(context.Background(), fetch.Options{URL: "https://example.com", Mode: fetch.Mode("bogus")})
	if err == nil {
		t.Fatal("expected error for unknown mode")
	}
}

func TestFetch_AutoUsesStatic(t *testing.T) {
	payload := "<html><body>" + strings.Repeat("x", 2100) + "<h1>Title</h1></body></html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	res, err := fetch.Fetch(ctx, fetch.Options{URL: srv.URL, Mode: fetch.ModeAuto, Timeout: time.Second})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FinalMode != fetch.ModeStatic || res.SourceInfo != "auto:static" {
		t.Fatalf("expected auto:static, got %+v", res)
	}
}

func TestFetch_StaticHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := fetch.Fetch(ctx, fetch.Options{URL: srv.URL, Mode: fetch.ModeStatic, Timeout: time.Second})
	if err == nil {
		t.Fatal("expected status error")
	}
}

func TestFetch_StaticUserAgent(t *testing.T) {
	wantUA := "test-agent"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != wantUA {
			t.Fatalf("expected User-Agent %q, got %q", wantUA, got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>ok</html>"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := fetch.Fetch(ctx, fetch.Options{URL: srv.URL, Mode: fetch.ModeStatic, Timeout: time.Second, UserAgent: wantUA})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

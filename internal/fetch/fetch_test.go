package fetch_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go_scrap/internal/fetch"
)

func TestFetch_StaticTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

package fetch

import (
	"context"
	"testing"
)

func TestFetchStatic_InvalidURL(t *testing.T) {
	_, err := fetchStatic(context.Background(), Options{URL: "://bad"})
	if err == nil {
		t.Fatal("expected error for invalid url")
	}
}

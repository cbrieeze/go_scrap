package tui

import "testing"

func TestParseInt(t *testing.T) {
	v, err := parseInt("42")
	if err != nil || v != 42 {
		t.Fatalf("expected 42, got %d (err=%v)", v, err)
	}

	if _, err := parseInt("bad"); err == nil {
		t.Fatalf("expected error for non-integer")
	}
}

func TestValidateIntString(t *testing.T) {
	v := validateIntString(0, 10)
	if err := v("5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := v("-1"); err == nil {
		t.Fatalf("expected error for below range")
	}
	if err := v("11"); err == nil {
		t.Fatalf("expected error for above range")
	}
}

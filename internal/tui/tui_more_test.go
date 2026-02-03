package tui

import "testing"

func TestParseInt_EmptyInput(t *testing.T) {
	if v, err := parseInt("  "); err == nil || v != 0 {
		t.Fatalf("expected error for empty input, got (%d,%v)", v, err)
	}
}

func TestParseFloat(t *testing.T) {
	if v, err := parseFloat("3.5"); err != nil || v != 3.5 {
		t.Fatalf("parseFloat: got (%f,%v)", v, err)
	}
	if v, err := parseFloat(" "); err != nil || v != 0 {
		t.Fatalf("parseFloat empty: got (%f,%v)", v, err)
	}
	if _, err := parseFloat("nope"); err == nil {
		t.Fatal("expected error for invalid float")
	}
}

func TestValidateIntString_TypeError(t *testing.T) {
	v := validateIntString(0, 10)
	if err := v("nope"); err == nil {
		t.Fatal("expected type error")
	}
}

func TestValidateFloatString(t *testing.T) {
	v := validateFloatString(0, 10)
	if err := v("5"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := v("11"); err == nil {
		t.Fatal("expected out of range error")
	}
	if err := v("nope"); err == nil {
		t.Fatal("expected type error")
	}
}

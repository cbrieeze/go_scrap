package cli

import "testing"

func TestStringFlag(t *testing.T) {
	var f stringFlag
	if err := f.Set("foo"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if f.Value != "foo" {
		t.Errorf("expected 'foo', got %q", f.Value)
	}
	if !f.WasSet {
		t.Error("expected WasSet to be true")
	}
	if s := f.String(); s != "foo" {
		t.Errorf("expected String()='foo', got %q", s)
	}
}

func TestIntFlag(t *testing.T) {
	var f intFlag
	if err := f.Set("42"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if f.Value != 42 {
		t.Errorf("expected 42, got %d", f.Value)
	}
	if !f.WasSet {
		t.Error("expected WasSet to be true")
	}
	if s := f.String(); s != "42" {
		t.Errorf("expected String()='42', got %q", s)
	}

	if err := f.Set("invalid"); err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestFloatFlag(t *testing.T) {
	var f floatFlag
	if err := f.Set("3.14"); err != nil {
		t.Fatalf("Set failed: %v", err)
	}
	if f.Value != 3.14 {
		t.Errorf("expected 3.14, got %f", f.Value)
	}
	if !f.WasSet {
		t.Error("expected WasSet to be true")
	}
	if s := f.String(); s != "3.14" {
		t.Errorf("expected String()='3.14', got %q", s)
	}

	if err := f.Set("invalid"); err == nil {
		t.Error("expected error for invalid input")
	}
}

func TestBoolFlag(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"true", true},
		{"1", true},
		{"yes", true},
		{"y", true},
		{"false", false},
		{"0", false},
		{"no", false},
		{"garbage", false},
	}

	for _, tt := range tests {
		var f boolFlag
		if err := f.Set(tt.input); err != nil {
			t.Errorf("Set(%q) failed: %v", tt.input, err)
		}
		if f.Value != tt.want {
			t.Errorf("Set(%q) got %v, want %v", tt.input, f.Value, tt.want)
		}
		if !f.WasSet {
			t.Errorf("Set(%q) did not set WasSet", tt.input)
		}
	}

	var f boolFlag
	if !f.IsBoolFlag() {
		t.Error("IsBoolFlag should return true")
	}
	f.Value = true
	if s := f.String(); s != "true" {
		t.Errorf("String() got %q, want 'true'", s)
	}
}

package cli

import (
	"fmt"
	"strings"
)

type stringFlag struct {
	Value  string
	WasSet bool
}

func (s *stringFlag) String() string { return s.Value }
func (s *stringFlag) Set(v string) error {
	s.Value = v
	s.WasSet = true
	return nil
}

type intFlag struct {
	Value  int
	WasSet bool
}

func (i *intFlag) String() string { return fmt.Sprintf("%d", i.Value) }
func (i *intFlag) Set(v string) error {
	var parsed int
	_, err := fmt.Sscanf(v, "%d", &parsed)
	if err != nil {
		return err
	}
	i.Value = parsed
	i.WasSet = true
	return nil
}

type floatFlag struct {
	Value  float64
	WasSet bool
}

func (f *floatFlag) String() string { return fmt.Sprintf("%g", f.Value) }
func (f *floatFlag) Set(v string) error {
	var parsed float64
	_, err := fmt.Sscanf(v, "%f", &parsed)
	if err != nil {
		return err
	}
	f.Value = parsed
	f.WasSet = true
	return nil
}

type boolFlag struct {
	Value  bool
	WasSet bool
}

func (b *boolFlag) String() string { return fmt.Sprintf("%t", b.Value) }
func (b *boolFlag) Set(v string) error {
	v = strings.ToLower(strings.TrimSpace(v))
	b.Value = v == "true" || v == "1" || v == "yes" || v == "y"
	b.WasSet = true
	return nil
}

func (b *boolFlag) IsBoolFlag() bool { return true }

type stringMapFlag struct {
	Values map[string]string
	WasSet bool
}

func (s *stringMapFlag) String() string {
	if len(s.Values) == 0 {
		return ""
	}
	parts := make([]string, 0, len(s.Values))
	for key, value := range s.Values {
		parts = append(parts, fmt.Sprintf("%s=%s", key, value))
	}
	return strings.Join(parts, ",")
}

func (s *stringMapFlag) Set(v string) error {
	key, value, ok := strings.Cut(v, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return fmt.Errorf("expected key=value, got %q", v)
	}
	if s.Values == nil {
		s.Values = make(map[string]string)
	}
	s.Values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	s.WasSet = true
	return nil
}

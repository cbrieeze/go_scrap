package scraperr

import (
	"fmt"
	"strings"
)

// ErrorType defines the category of the error for better handling/display.
type ErrorType string

const (
	TypeSelectorNotFound ErrorType = "SelectorNotFound"
	TypeEmptyContent     ErrorType = "EmptyContent"
	TypeTimeout          ErrorType = "Timeout"
	TypeNetwork          ErrorType = "Network"
)

// Error is a custom error type that holds context about the failure.
type Error struct {
	Type    ErrorType
	Message string
	Context map[string]string
	Cause   error
}

func (e *Error) Error() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[%s] %s", e.Type, e.Message))

	if len(e.Context) > 0 {
		sb.WriteString(" | context: {")
		var parts []string
		for k, v := range e.Context {
			parts = append(parts, fmt.Sprintf("%s=%q", k, v))
		}
		sb.WriteString(strings.Join(parts, ", "))
		sb.WriteString("}")
	}

	if e.Cause != nil {
		sb.WriteString(fmt.Sprintf(" | cause: %v", e.Cause))
	}
	return sb.String()
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// NewSelectorNotFound returns an error indicating a DOM selector matched nothing.
func NewSelectorNotFound(selector, url string) *Error {
	return &Error{
		Type:    TypeSelectorNotFound,
		Message: fmt.Sprintf("selector %q returned no elements", selector),
		Context: map[string]string{
			"selector": selector,
			"url":      url,
		},
	}
}

// NewEmptyContent returns an error indicating content extraction resulted in an empty string.
func NewEmptyContent(url, selector string) *Error {
	return &Error{
		Type:    TypeEmptyContent,
		Message: "extracted content was empty",
		Context: map[string]string{
			"url":      url,
			"selector": selector,
		},
	}
}

// NewTimeout returns an error indicating an operation exceeded its time limit.
func NewTimeout(operation, durationStr string, cause error) *Error {
	return &Error{
		Type:    TypeTimeout,
		Message: fmt.Sprintf("operation %q timed out after %s", operation, durationStr),
		Context: map[string]string{
			"operation": operation,
			"duration":  durationStr,
		},
		Cause: cause,
	}
}

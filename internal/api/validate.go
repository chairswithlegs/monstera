package api

import (
	"strconv"
	"time"
)

// MaxOffset is the upper bound for offset-based pagination across all API
// endpoints. Values above this cap are clamped to MaxOffset to prevent
// expensive database scans.
const MaxOffset = 10_000

// ClampOffset returns offset clamped to [0, MaxOffset].
func ClampOffset(offset int) int {
	if offset < 0 {
		return 0
	}
	if offset > MaxOffset {
		return MaxOffset
	}
	return offset
}

// ValidateRequiredField returns ErrUnprocessable if value is empty.
// Use for required request body fields (semantic validation → 422).
func ValidateRequiredField(value, fieldName string) error {
	if value == "" {
		return NewMissingRequiredFieldError(fieldName)
	}
	return nil
}

// ValidateOneOf returns ErrUnprocessable if value is not in the allowed slice.
func ValidateOneOf[T comparable](value T, allowed []T, fieldName string) error {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	return NewInvalidValueError(fieldName)
}

// ValidateRFC3339 parses raw as RFC3339 and returns the time, or ErrUnprocessable on parse failure.
func ValidateRFC3339(raw, fieldName string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, NewInvalidRFC3339Error(fieldName)
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, NewInvalidRFC3339Error(fieldName)
	}
	return t, nil
}

// ValidateRFC3339Optional parses *raw as RFC3339 if non-nil and non-empty; returns the time or nil, or ErrUnprocessable on parse failure.
func ValidateRFC3339Optional(raw *string, fieldName string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	t, err := ValidateRFC3339(*raw, fieldName)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ValidatePositiveInt parses raw as an integer. If raw is empty, returns defaultVal.
// Returns ErrUnprocessable if raw is non-empty and not a positive integer, or if the value exceeds max (in which case max is returned and no error).
func ValidatePositiveInt(raw, fieldName string, defaultVal, max int) (int, error) {
	if raw == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return 0, NewPositiveIntRequiredError(fieldName)
	}
	if n > max {
		return max, nil
	}
	return n, nil
}

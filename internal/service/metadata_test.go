package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncodeMetadata(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		m    map[string]string
		want string
	}{
		{
			name: "empty map",
			m:    map[string]string{},
			want: "{}",
		},
		{
			name: "nil map",
			m:    nil,
			want: "{}",
		},
		{
			name: "normal values",
			m:    map[string]string{"key": "value"},
			want: `{"key":"value"}`,
		},
		{
			name: "control characters stripped",
			m:    map[string]string{"k": "hello\x00world"},
			want: `{"k":"helloworld"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := string(encodeMetadata(tc.m))
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestSanitizeMetadataValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "normal string",
			in:   "hello world",
			want: "hello world",
		},
		{
			name: "truncated at max length",
			in:   strings.Repeat("a", 3000),
			want: strings.Repeat("a", maxMetadataValueLen),
		},
		{
			name: "control characters removed",
			in:   "hello\nworld\t!\x00",
			want: "helloworld!",
		},
		{
			name: "all invalid chars",
			in:   "\x00\x01\x02",
			want: "[invalid]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := sanitizeMetadataValue(tc.in)
			assert.Equal(t, tc.want, got)
		})
	}
}

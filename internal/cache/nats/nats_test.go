package nats

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"simple", "foo", "foo"},
		{"colons", "idempotency:acc1:key2", "idempotency.acc1.key2"},
		{"mixed", "httpsig:{abc123}", "httpsig..abc123."},
		{"already clean", "some-key_123.sub", "some-key_123.sub"},
		{"spaces and special", "a b@c#d", "a.b.c.d"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, sanitizeKey(tt.in))
		})
	}
}

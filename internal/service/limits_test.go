package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClampLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                      string
		limit, defaultLim, maxLim int
		want                      int
	}{
		{"zero uses default", 0, 20, 80, 20},
		{"negative uses default", -5, 20, 80, 20},
		{"within range unchanged", 30, 20, 80, 30},
		{"above max capped", 200, 20, 80, 80},
		{"exactly max", 80, 20, 80, 80},
		{"exactly one", 1, 20, 80, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ClampLimit(tt.limit, tt.defaultLim, tt.maxLim))
		})
	}
}

package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAccount_IsLocal(t *testing.T) {
	t.Parallel()

	remoteDomain := "example.com"
	tests := []struct {
		name       string
		account    Account
		wantLocal  bool
		wantRemote bool
	}{
		{
			name:       "local account (nil Domain)",
			account:    Account{Domain: nil},
			wantLocal:  true,
			wantRemote: false,
		},
		{
			name:       "remote account",
			account:    Account{Domain: &remoteDomain},
			wantLocal:  false,
			wantRemote: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantLocal, tt.account.IsLocal())
			assert.Equal(t, tt.wantRemote, tt.account.IsRemote())
		})
	}
}

func TestStatus_IsLocal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		status     Status
		wantLocal  bool
		wantRemote bool
	}{
		{
			name:       "local status",
			status:     Status{Local: true},
			wantLocal:  true,
			wantRemote: false,
		},
		{
			name:       "remote status",
			status:     Status{Local: false},
			wantLocal:  false,
			wantRemote: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.wantLocal, tt.status.IsLocal())
			assert.Equal(t, tt.wantRemote, tt.status.IsRemote())
		})
	}
}

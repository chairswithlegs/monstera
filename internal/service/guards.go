package service

import (
	"fmt"

	"github.com/chairswithlegs/monstera/internal/domain"
)

// requireLocal returns domain.ErrForbidden if local is false.
// Call at the top of methods that must only operate on local entities.
func requireLocal(local bool, method string) error {
	if !local {
		return fmt.Errorf("%s: %w", method, domain.ErrForbidden)
	}
	return nil
}

// requireRemote returns domain.ErrForbidden if local is true.
// Call at the top of methods that must only operate on remote entities.
func requireRemote(local bool, method string) error {
	if local {
		return fmt.Errorf("%s: %w", method, domain.ErrForbidden)
	}
	return nil
}

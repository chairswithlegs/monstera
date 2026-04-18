package domain

import "errors"

var (
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("conflict")
	ErrForbidden        = errors.New("forbidden")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrValidation       = errors.New("validation error")
	ErrRateLimited      = errors.New("rate limited")
	ErrGone             = errors.New("gone")
	ErrUnprocessable    = errors.New("unprocessable entity")
	ErrAccountSuspended = errors.New("account suspended")
	// ErrDeletionAlreadyRequested indicates an account already has a pending
	// deletion and the request is a no-op / conflict. Maps to HTTP 409.
	ErrDeletionAlreadyRequested = errors.New("deletion already requested")
)

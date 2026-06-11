package application

import "errors"

// Sentinel errors used to distinguish authorization failures from other errors.
var (
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("not found")
)

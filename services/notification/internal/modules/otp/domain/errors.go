package domain

import "errors"

// Typed errors so the REST layer can map each to a distinct HTTP status.
var (
	ErrOTPNotFound        = errors.New("otp request not found")
	ErrOTPExpired         = errors.New("otp code has expired")
	ErrOTPTooManyAttempts = errors.New("too many verification attempts")
	ErrOTPMismatch        = errors.New("otp code does not match")
)

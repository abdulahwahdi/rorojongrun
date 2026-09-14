package domain

import "time"

// MaxAttempts default max verify attempts before an OTP is rejected outright.
const MaxAttempts = 5

// DefaultExpiry is how long a requested OTP code remains valid.
const DefaultExpiry = 5 * time.Minute

// TemplateCodeOTPVerification is the single, fixed notification template
// code used for all OTP deliveries regardless of purpose — keeps the
// operator-editable template table small; purpose-specific copy is a
// {{.purpose}} variable inside the one template, not a new DB row per
// purpose.
const TemplateCodeOTPVerification = "otp_verification"

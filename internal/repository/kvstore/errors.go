package kvstore

import "errors"

// ErrOTPNotFound indicates that no unexpired one-time password remains to consume.
var ErrOTPNotFound = errors.New("kvstore: otp not found")

// ErrSessionNotFound indicates that a session token has no stored record.
var ErrSessionNotFound = errors.New("kvstore: session not found")

package kvstore

import "errors"

// ErrOTPNotFound indicates that no unexpired one-time password remains to consume.
var ErrOTPNotFound = errors.New("kvstore: otp not found")

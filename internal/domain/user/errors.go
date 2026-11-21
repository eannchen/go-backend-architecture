package user

import "errors"

// Repository outcomes are independent of a database driver.
var (
	ErrNotFound      = errors.New("user not found")
	ErrAlreadyExists = errors.New("user already exists")
)

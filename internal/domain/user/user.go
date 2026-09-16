package user

// Status describes whether an account may participate in user workflows.
type Status string

const (
	StatusActive   Status = "active"
	StatusDisabled Status = "disabled"
)

// User is the application account independently of transport and persistence.
type User struct {
	ID     int64
	Email  string
	Status Status
}

// CanAuthenticate reports whether the account may start a new authenticated session.
func (u User) CanAuthenticate() bool {
	return u.Status == StatusActive
}

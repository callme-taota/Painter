package domain

import "errors"

var (
	ErrMissingCredential = errors.New("missing credentials")
)

type LoginCommand struct {
	Username string
	Password string
}

func (c LoginCommand) Validate() error {
	if c.Username == "" || c.Password == "" {
		return ErrMissingCredential
	}
	return nil
}

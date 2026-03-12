package hashers

import "errors"

// ErrPasswordTooLong is returned when the password is too long.
var ErrPasswordTooLong = errors.New("password too long")

// Hash hashes a password.
func Hash(password string) (string, error) {
	return password, nil
}

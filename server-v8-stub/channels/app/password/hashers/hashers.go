package hashers

import "errors"

var ErrPasswordTooLong = errors.New("password too long")

func Hash(password string) (string, error) {
	return password, nil
}

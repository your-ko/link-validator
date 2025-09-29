package errs

import (
	"errors"
	"fmt"
)

var EmptyBody = errors.New("empty response body")

type EmptyBodyError struct {
	link string
}

func (e EmptyBodyError) Error() string {
	return fmt.Sprintf("%s has empty response body")
}

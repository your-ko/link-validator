package errs

import (
	"errors"
	"fmt"
)

var EmptyBody = errors.New("empty response body")

type EmptyBodyError struct {
	link string
}

func NewEmptyBody(link string) EmptyBodyError {
	return EmptyBodyError{
		link: link,
	}
}

func (e EmptyBodyError) Error() string {
	return fmt.Sprintf("empty response body. Incorrect link: '%s'", e.link)
}

func (e EmptyBodyError) Is(target error) bool { return target == EmptyBody }

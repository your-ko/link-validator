package errs

import (
	"errors"
)

// TODO: Not sure I need it

var ErrEmptyBody = errors.New("empty response body")

type EmptyBodyError struct {
	link string
}

func NewEmptyBody(link string) EmptyBodyError {
	return EmptyBodyError{
		link: link,
	}
}

func (e EmptyBodyError) Error() string {
	return ErrEmptyBody.Error()
}

func (e EmptyBodyError) Is(target error) bool { return target == ErrEmptyBody }

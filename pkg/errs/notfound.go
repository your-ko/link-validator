package errs

import (
	"errors"
)

var ErrNotFound = errors.New("not found")

func NewNotFound(link string) NotFoundError {
	return NotFoundError{
		link: link,
	}
}
func NewNotFoundMessage(message string) NotFoundError {
	return NotFoundError{
		message: message,
	}
}

type NotFoundError struct {
	link    string
	message string
}

func (e NotFoundError) Error() string {
	if e.message == "" {
		return ErrNotFound.Error()
	} else {
		return e.message
	}
}

func (e NotFoundError) Is(target error) bool { return target == ErrNotFound }

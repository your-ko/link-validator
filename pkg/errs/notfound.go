package errs

import (
	"errors"
	"fmt"
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
		return fmt.Sprintf("%s. Incorrect link: '%s'", ErrNotFound.Error(), e.link)
	} else {
		return e.message
	}
}

func (e NotFoundError) Is(target error) bool { return target == ErrNotFound }

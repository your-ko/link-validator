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

type NotFoundError struct {
	link string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("%s. Incorrect link: '%s'", ErrNotFound.Error(), e.link)
}

func (e NotFoundError) Is(target error) bool { return target == ErrNotFound }

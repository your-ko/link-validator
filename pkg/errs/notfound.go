package errs

import (
	"errors"
	"fmt"
)

var NotFound = errors.New("not found")

func NewNotFound(link string) NotFoundError {
	return NotFoundError{
		link: link,
	}
}

type NotFoundError struct {
	link string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("%s. Incorrect link: '%s'", NotFound.Error(), e.link)
}

func (e NotFoundError) Is(target error) bool { return target == NotFound }

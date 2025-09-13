package errs

import "errors"

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
	return e.link
}

func (e NotFoundError) Is(target error) bool { return target == NotFound }

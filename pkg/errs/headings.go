package errs

import (
	"errors"
	"fmt"
)

var ErrHeadingLinkToDir = errors.New("points to dir but contains a heading (./dir#blah)")
var ErrEmptyHeading = errors.New("empty heading (./file.md#)")

type HeadingLinkToDirError struct {
	Link string
}

func (e HeadingLinkToDirError) Error() string {
	return fmt.Sprintf("%s. Incorrect link: '%s'",
		ErrHeadingLinkToDir.Error(), e.Link)
}

func (e HeadingLinkToDirError) Is(target error) bool { return target == ErrHeadingLinkToDir }

func NewHeadingLinkToDir(link string) error {
	return HeadingLinkToDirError{Link: link}
}

type EmptyHeadingError struct {
	link string
}

func (e EmptyHeadingError) Error() string {
	return fmt.Sprintf("%s. Incorrect link: '%s'",
		ErrEmptyHeading.Error(), e.link)
}

func (e EmptyHeadingError) Is(target error) bool { return target == ErrEmptyHeading }

func NewEmptyHeadingError(link string) error {
	return EmptyHeadingError{link: link}
}

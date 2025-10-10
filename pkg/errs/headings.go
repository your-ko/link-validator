package errs

import (
	"errors"
	"fmt"
)

var HeadingLinkToDir = errors.New("points to dir but contains a heading (./file.md#blah)")
var EmptyHeading = errors.New("empty heading (./file.md#)")

type HeadingLinkToDirError struct {
	Link string
}

func (e HeadingLinkToDirError) Error() string {
	return fmt.Sprintf("%s. Incorrect link: '%s'",
		HeadingLinkToDir.Error(), e.Link)
}

func (e HeadingLinkToDirError) Is(target error) bool { return target == HeadingLinkToDir }

func NewHeadingLinkToDir(link string) error {
	return HeadingLinkToDirError{Link: link}
}

type EmptyHeadingError struct {
	link string
}

func (e EmptyHeadingError) Error() string {
	return fmt.Sprintf("%s. Incorrect link: '%s'",
		EmptyHeading.Error(), e.link)
}

func (e EmptyHeadingError) Is(target error) bool { return target == EmptyHeading }

func NewEmptyHeadingError(link string) error {
	return EmptyHeadingError{link: link}
}

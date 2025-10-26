package errs

import (
	"errors"
	"fmt"
)

var ErrAnchorLinkToDir = errors.New("points to dir but contains an anchor (./dir#blah)")
var ErrEmptyAnchor = errors.New("empty anchor (./file.md#)")

type AnchorLinkToDirError struct {
	Link string
}

func (e AnchorLinkToDirError) Error() string {
	return fmt.Sprintf("%s. Incorrect link: '%s'",
		ErrAnchorLinkToDir.Error(), e.Link)
}

func (e AnchorLinkToDirError) Is(target error) bool { return target == ErrAnchorLinkToDir }

func NewAnchorLinkToDir(link string) error {
	return AnchorLinkToDirError{Link: link}
}

type EmptyAnchorError struct {
	link string
}

func (e EmptyAnchorError) Error() string {
	return fmt.Sprintf("%s. Incorrect link: '%s'",
		ErrEmptyAnchor.Error(), e.link)
}

func (e EmptyAnchorError) Is(target error) bool { return target == ErrEmptyAnchor }

func NewEmptyAnchorError(link string) error {
	return EmptyAnchorError{link: link}
}

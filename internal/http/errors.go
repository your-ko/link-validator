package http

import "fmt"

type StatusCodeError struct {
	StatusCode int
	Link       string
}

func (e *StatusCodeError) Error() string {
	return fmt.Sprintf("statusCode %v for the link %s", e.StatusCode, e.Link)
}

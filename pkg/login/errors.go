package login

import (
	"errors"
	"fmt"
)

// ErrorKind identifies whether retrying a failed login can make progress.
type ErrorKind uint8

const (
	ErrorTransient ErrorKind = iota + 1
	ErrorAuthentication
	ErrorConfiguration
)

// Error carries a stable failure class and an optional gateway response code.
// Gateway response bodies are deliberately not copied into the error so that
// callers cannot accidentally write credentials or response data to logs.
type Error struct {
	Kind      ErrorKind
	Operation string
	Code      string
	Message   string
	Err       error
}

func (e *Error) Error() string {
	switch {
	case e.Code != "":
		return fmt.Sprintf("%s: %s (code %s)", e.Operation, e.Message, e.Code)
	case e.Err != nil:
		return fmt.Sprintf("%s: %s: %v", e.Operation, e.Message, e.Err)
	default:
		return fmt.Sprintf("%s: %s", e.Operation, e.Message)
	}
}

func (e *Error) Unwrap() error {
	return e.Err
}

// KindOf returns the declared failure class. Unknown errors are treated as
// transient so an unexpected local or gateway failure is not made permanent.
func KindOf(err error) ErrorKind {
	var classified *Error
	if errors.As(err, &classified) {
		return classified.Kind
	}
	return ErrorTransient
}

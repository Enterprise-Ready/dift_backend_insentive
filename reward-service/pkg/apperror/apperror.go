package apperror

import (
	"fmt"
	_ "github.com/PlatformCore/libpackage/core/errors"
)

type Error struct {
	Code string
	Err  error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return e.Code
	}
	return fmt.Sprintf("%s: %v", e.Code, e.Err)
}
func (e *Error) Unwrap() error { return e.Err }
func Wrap(code string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Err: err}
}
func New(code string) error { return &Error{Code: code} }

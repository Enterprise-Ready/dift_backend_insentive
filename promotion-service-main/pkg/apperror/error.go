package apperror

import "fmt"

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *Error) Error() string    { return e.Message }
func New(code, msg string) *Error { return &Error{Code: code, Message: msg} }
func Wrap(code string, err error) error {
	if err == nil {
		return nil
	}
	return &Error{Code: code, Message: fmt.Sprintf("%v", err)}
}

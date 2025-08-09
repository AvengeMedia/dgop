package errdefs

import (
	"fmt"

	"github.com/danielgtaylor/huma/v2"
)

// Huma response error
type ResponseError struct {
	Status  int      `json:"status"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

func (e *ResponseError) Error() string {
	return e.Message
}

func (e *ResponseError) GetStatus() int {
	return e.Status
}

var HumaErrorFunc = func(status int, message string, errs ...error) huma.StatusError {
	details := make([]string, len(errs))
	for i, err := range errs {
		details[i] = err.Error()
	}
	return &ResponseError{
		Status:  status,
		Message: message,
		Details: details,
	}
}

type ErrorType int

// Errors
var (
	// Permissions
	ErrInvalidInput = NewCustomError(ErrTypeInvalidInput, "")
	ErrNotFound     = NewCustomError(ErrTypeNotFound, "")
)

// More dynamic errors
const (
	ErrTypeInvalidInput ErrorType = iota
	ErrTypeNotFound
)

var errorTypeStrings = map[ErrorType]string{
	ErrTypeInvalidInput: "ErrInvalidInput",
	ErrTypeNotFound:     "ErrNotFound",
}

func (e ErrorType) String() string {
	if s, ok := errorTypeStrings[e]; ok {
		return s
	}
	return "ErrUnknown"
}

type CustomError struct {
	Type    ErrorType
	Message string
}

func (e *CustomError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type.String(), e.Message)
}

func (e *CustomError) Is(target error) bool {
	t, ok := target.(*CustomError)
	if !ok {
		return false
	}
	return e.Type == t.Type
}

func NewCustomError(t ErrorType, message string) *CustomError {
	return &CustomError{
		Type:    t,
		Message: message,
	}
}

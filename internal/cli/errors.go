package cli

import (
	"errors"
	"fmt"
)

const (
	ExitOK      = 0
	ExitGeneral = 1
	ExitUsage   = 2
	ExitConfig  = 3
	ExitAuth    = 4
	ExitAPI     = 5
	ExitNetwork = 6
)

type ExitError struct {
	Code    int
	Message string
	Err     error
}

func (e *ExitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return fmt.Sprintf("exit code %d", e.Code)
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

func exitError(code int, message string, err error) error {
	return &ExitError{Code: code, Message: message, Err: err}
}

func exitCode(err error) int {
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.Code
	}
	return ExitGeneral
}

func errorCode(err error) string {
	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		switch exitErr.Code {
		case ExitUsage:
			return "usage_error"
		case ExitConfig:
			return "config_error"
		case ExitAuth:
			return "auth_error"
		case ExitAPI:
			return "api_error"
		case ExitNetwork:
			return "network_error"
		}
	}
	return "error"
}

package usererr

import (
	"errors"
	"os"
	"strings"
)

type Kind string

const (
	KindInvalidInput Kind = "invalid_input"
	KindTimeout      Kind = "timeout"
	KindRateLimit    Kind = "rate_limit"
	KindAuth         Kind = "auth"
	KindNetwork      Kind = "network"
	KindUnavailable  Kind = "unavailable"
	KindConfig       Kind = "config"
	KindInternal     Kind = "internal"
)

type Error struct {
	Kind    Kind
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return strings.TrimSpace(e.Message)
	}
	if strings.TrimSpace(e.Message) == "" {
		return e.Cause.Error()
	}
	return strings.TrimSpace(e.Message) + ": " + e.Cause.Error()
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func New(kind Kind, message string) error {
	return &Error{
		Kind:    kind,
		Message: strings.TrimSpace(message),
	}
}

func Wrap(kind Kind, message string, cause error) error {
	if cause == nil {
		return New(kind, message)
	}
	return &Error{
		Kind:    kind,
		Message: strings.TrimSpace(message),
		Cause:   cause,
	}
}

func Message(err error) string {
	var userErr *Error
	if errors.As(err, &userErr) {
		return strings.TrimSpace(userErr.Message)
	}
	return ""
}

func OrDefault(err error, fallback string) string {
	if msg := Message(err); msg != "" {
		return msg
	}
	fallback = strings.TrimSpace(fallback)
	if fallback != "" {
		return fallback
	}
	return "Request failed. Please retry."
}

func WrapLocalStorage(message string, cause error) error {
	if msg := Message(cause); msg != "" {
		return cause
	}
	lower := strings.ToLower(strings.TrimSpace(cause.Error()))
	switch {
	case errors.Is(cause, os.ErrPermission), strings.Contains(lower, "permission denied"), strings.Contains(lower, "operation not permitted"), strings.Contains(lower, "read-only file system"):
		return Wrap(KindConfig, "The relay can't access local storage because of a filesystem permission problem. Check the configured directories and permissions.", cause)
	case strings.Contains(lower, "no space left on device"), strings.Contains(lower, "disk quota exceeded"):
		return Wrap(KindUnavailable, "The relay can't write to local storage because the disk is full.", cause)
	default:
		return Wrap(KindInternal, message, cause)
	}
}

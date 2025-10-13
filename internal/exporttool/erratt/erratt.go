package erratt

import (
	"errors"
	"fmt"
	"log/slog"
)

type Error struct {
	text       string
	wrappedErr error
	attrs      []any
}

// Error is an error type
var _ error = &Error{}

func New(text string, args ...any) *Error {
	return &Error{
		text:       text,
		wrappedErr: nil,
		attrs:      args,
	}
}

func (ea *Error) Error() string {
	return ea.text
}

func (ea *Error) Attrs() []any {
	return ea.attrs
}

func (ea *Error) Unwrap() error {
	return ea.wrappedErr
}

func (ea *Error) With(args ...any) *Error {
	return &Error{
		text:       ea.text,
		wrappedErr: ea.wrappedErr,
		attrs:      append(ea.attrs, args...),
	}
}

func Errorf(format string, a ...any) *Error {
	err := fmt.Errorf(format, a...)
	var wrappedErr error
	var attrs []any
	if unwerr, ok := err.(interface {
		Unwrap() error
	}); ok {
		wrappedErr = unwerr.Unwrap()
		if attrErr, ok := wrappedErr.(interface {
			Attrs() []any
		}); ok {
			attrs = attrErr.Attrs()
		}
	}
	return &Error{
		text:       err.Error(),
		wrappedErr: wrappedErr,
		attrs:      attrs,
	}
}

func SlogWith(err error, logger *slog.Logger) {
	ewa := &Error{}
	if errors.As(err, &ewa) {
		logger.Error(ewa.text, ewa.attrs...)
	}
	logger.Error(err.Error())
}

func Slog(err error) {
	SlogWith(err, slog.Default())
}

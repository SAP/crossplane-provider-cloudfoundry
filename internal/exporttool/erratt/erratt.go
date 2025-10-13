package erratt

import (
	"errors"
	"fmt"
	"log/slog"
)

type Error interface {
	error
	With(args ...any) Error
	Attrs() []any
	Unwrap() error
}

type errorWithAttrs struct {
	text       string
	wrappedErr error
	attrs      []any
}

// Error is an error type
var _ error = &errorWithAttrs{}
var _ Error = &errorWithAttrs{}

func New(text string, args ...any) Error {
	return &errorWithAttrs{
		text:       text,
		wrappedErr: nil,
		attrs:      args,
	}
}

func (ea *errorWithAttrs) Error() string {
	return ea.text
}

func (ea *errorWithAttrs) Attrs() []any {
	return ea.attrs
}

func (ea *errorWithAttrs) Unwrap() error {
	return ea.wrappedErr
}

func (ea *errorWithAttrs) With(args ...any) Error {
	return &errorWithAttrs{
		text:       ea.text,
		wrappedErr: ea.wrappedErr,
		attrs:      append(ea.attrs, args...),
	}
}

func Errorf(format string, a ...any) Error {
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
	return &errorWithAttrs{
		text:       err.Error(),
		wrappedErr: wrappedErr,
		attrs:      attrs,
	}
}

func SlogWith(err error, logger *slog.Logger) {
	ewa := &errorWithAttrs{}
	if errors.As(err, &ewa) {
		logger.Error(ewa.text, ewa.attrs...)
	}
	logger.Error(err.Error())
}

func Slog(err error) {
	SlogWith(err, slog.Default())
}

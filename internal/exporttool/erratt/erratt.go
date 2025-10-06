package erratt

import (
	"errors"
	"fmt"
	"log/slog"
)

type ErrorWithAttrs struct {
	text       string
	wrappedErr error
	attrs      []any
}

// ErroWithAttrs is an error type
var _ error = ErrorWithAttrs{}

func New(text string, args ...any) ErrorWithAttrs {
	return ErrorWithAttrs{
		text:       text,
		wrappedErr: nil,
		attrs:      args,
	}
}

func (ea ErrorWithAttrs) Error() string {
	return ea.text
}

func (ea ErrorWithAttrs) Attrs() []any {
	return ea.attrs
}

func (ea ErrorWithAttrs) Unwrap() error {
	return ea.wrappedErr
}

func (ea ErrorWithAttrs) With(args ...any) ErrorWithAttrs {
	return ErrorWithAttrs{
		text:       ea.text,
		wrappedErr: ea.wrappedErr,
		attrs:      append(ea.attrs, args...),
	}
}

func Errorf(format string, a ...any) ErrorWithAttrs {
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
	return ErrorWithAttrs{
		text:       err.Error(),
		wrappedErr: wrappedErr,
		attrs:      attrs,
	}
}

func SlogWith(err error, logger *slog.Logger) {
	ewa := ErrorWithAttrs{}
	if errors.As(err, &ewa) {
		logger.Error(ewa.text, ewa.attrs...)
	}
	logger.Error(err.Error())
}

func Slog(err error) {
	SlogWith(err, slog.Default())
}

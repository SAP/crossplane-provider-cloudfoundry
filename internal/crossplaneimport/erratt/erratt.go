package erratt

import (
	"errors"
	"fmt"
	"log/slog"
)

type ErrorWithAttrs struct {
	error
	attrs []any
}

func (ea ErrorWithAttrs) Wrap(msg string, attrs ...slog.Attr) ErrorWithAttrs {
	newEa := ErrorWithAttrs{
		error: fmt.Errorf("%s: %s", msg, ea.error.Error()),
		attrs: make([]any, len(ea.attrs)),
	}
	copy(newEa.attrs, ea.attrs)
	for i := range attrs {
		newEa.attrs = append(newEa.attrs, any(attrs[i]))
	}
	return newEa
}

func (ea ErrorWithAttrs) Attrs() []any {
	return ea.attrs
}

func ErrA(err error, attrs ...slog.Attr) ErrorWithAttrs {
	ea := ErrorWithAttrs{
		error: err,
		attrs: make([]any, len(attrs)),
	}
	for i := range attrs {
		ea.attrs[i] = attrs[i]
	}
	return ea
}

func S(msg string, attrs ...slog.Attr) ErrorWithAttrs {
	ea := ErrorWithAttrs{
		error: errors.New(msg),
		attrs: make([]any, len(attrs)),
	}
	for i := range attrs {
		ea.attrs[i] = attrs[i]
	}
	return ea
}

func Wrap(msg string, err any, attrs ...slog.Attr) ErrorWithAttrs {
	//nolint:errorlint
	switch e := err.(type) {
	case ErrorWithAttrs:
		return e.Wrap(msg, attrs...)
	case error:
		return S(msgerr(msg, e), attrs...)
	default:
		panic("Wrapping a non-error type")
	}
}

func msgerr(msg string, err error) string {
	errMsg := err.Error()
	if len(msg) > 0 {
		errMsg = fmt.Sprintf("%s: %+v", msg, err.Error())
	}
	return errMsg
}

func SLog(msg string, err error) {
	//nolint:errorlint
	switch e := err.(type) {
	case ErrorWithAttrs:
		slog.Error(msgerr(msg, err), e.Attrs()...)
	case error:
		if len(msg) > 0 {
			slog.Error(msg, "error", err)
		} else {
			slog.Error(err.Error())
		}
	default:
		panic("SLog applied on non-error")
	}
}

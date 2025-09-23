package erratt

import (
	"fmt"
	"log/slog"
)

type ErrorWithAttrs struct {
	text       string
	wrappedErr error
	// attrs      []slog.Attr
	attrs []any
}

// ErroWithAttrs is an error type
var _ error = ErrorWithAttrs{}

func New(text string, args ...any) ErrorWithAttrs {
	return ErrorWithAttrs{
		text:       text,
		wrappedErr: nil,
		// attrs:      argsToAttrSlice(args),
		attrs: args,
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
	switch tErr := err.(type) {
	case ErrorWithAttrs:
		// anyAttrs := make([]any, len(tErr.attrs))
		// for i := range tErr.attrs {
		// 	anyAttrs[i] = any(tErr.attrs[i])
		// }
		logger.Error(tErr.text, tErr.attrs...)
	case error:
		logger.Error(tErr.Error())
	}
}

func Slog(err error) {
	SlogWith(err, slog.Default())
}

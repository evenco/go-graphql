package gqlerrors

import (
	"errors"

	"golang.org/x/net/context"

	"github.com/evenco/go-graphql/language/location"
)

type ErrorFormatterFunc func(ctx context.Context, err error) FormattedError

type FormattedError struct {
	Message   string                    `json:"message"`
	Locations []location.SourceLocation `json:"locations,omitempty"`
	Details   interface{}               `json:"details,omitempty"`
}

func (g FormattedError) Error() string {
	return g.Message
}

var Formatter ErrorFormatterFunc

func NewFormattedError(ctx context.Context, message string) FormattedError {
	err := errors.New(message)
	return FormatError(ctx, err)
}

func FormatError(ctx context.Context, err error) FormattedError {
	switch err := err.(type) {
	case FormattedError:
		return err
	case Error:
		return FormattedError{
			Message:   err.Error(),
			Locations: err.Locations,
		}
	case *Error:
		return FormattedError{
			Message:   err.Error(),
			Locations: err.Locations,
		}
	}
	if Formatter != nil {
		return Formatter(ctx, err)
	}
	return FormattedError{
		Message:   err.Error(),
		Locations: []location.SourceLocation{},
	}
}

func FormatErrors(ctx context.Context, errs ...error) []FormattedError {
	formattedErrors := []FormattedError{}
	for _, err := range errs {
		formattedErrors = append(formattedErrors, FormatError(ctx, err))
	}
	return formattedErrors
}

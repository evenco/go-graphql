package graphql

import (
	"github.com/evenco/go-graphql/gqlerrors"
	"github.com/evenco/go-graphql/language/ast"
)

type ValidationResult struct {
	IsValid bool
	Errors  []gqlerrors.FormattedError
}

func ValidateDocument(schema Schema, ast *ast.Document) (vr ValidationResult) {
	vr.IsValid = true
	return vr
}

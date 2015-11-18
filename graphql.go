package graphql

import (
	"golang.org/x/net/context"

	"github.com/evenco/go-graphql/gqlerrors"
	"github.com/evenco/go-graphql/language/parser"
	"github.com/evenco/go-graphql/language/source"
)

type Params struct {
	Schema         Schema
	RequestString  string
	RootObject     map[string]interface{}
	VariableValues map[string]interface{}
	OperationName  string
}

func Graphql(ctx context.Context, p Params) *Result {
	source := source.NewSource(&source.Source{
		Body: p.RequestString,
		Name: "GraphQL request",
	})
	AST, err := parser.Parse(parser.ParseParams{Source: source})
	if err != nil {
		return &Result{
			Errors: gqlerrors.FormatErrors(err),
		}
	}
	validationResult := ValidateDocument(p.Schema, AST)

	if !validationResult.IsValid {
		return &Result{
			Errors: validationResult.Errors,
		}
	}

	return Execute(ctx, ExecuteParams{
		Schema:        p.Schema,
		Root:          p.RootObject,
		AST:           AST,
		OperationName: p.OperationName,
		Args:          p.VariableValues,
	})
}

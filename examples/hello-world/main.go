package main

import (
	"encoding/json"
	"fmt"
	"log"

	"golang.org/x/net/context"

	"github.com/evenco/go-graphql"
)

func main() {
	// Schema
	fields := graphql.FieldConfigMap{
		"hello": &graphql.FieldConfig{
			Type: graphql.String,
			Resolve: func(ctx context.Context, p graphql.GQLFRParams) interface{} {
				return "world"
			},
		},
	}
	rootQuery := graphql.ObjectConfig{Name: "RootQuery", Fields: fields}
	schemaConfig := graphql.SchemaConfig{Query: graphql.NewObject(rootQuery)}
	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		log.Fatalf("failed to create new schema, error: %v", err)
	}

	// Query
	query := `
		{
			hello
		}
	`
	params := graphql.Params{Schema: schema, RequestString: query}
	r := graphql.Graphql(context.Background(), params)
	if len(r.Errors) > 0 {
		log.Fatalf("failed to execute graphql operation, errors: %+v", r.Errors)
	}
	rJSON, _ := json.Marshal(r)
	fmt.Printf("%s \n", rJSON) // {“data”:{“hello”:”world”}}
}

package api

import "embed"

// JSONSchema schema sources
//
//go:embed all:jsonschema
var JSONSchema embed.FS

// GraphQLSchema schema sources (GraphQL is off in this service, see CLAUDE.md)
//
// //go:embed all:graphql
var GraphQLSchema embed.FS

// Templates holds the printable invoice / receipt HTML templates
//
//go:embed templates
var Templates embed.FS

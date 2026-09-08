package api

import "embed"

// JSONSchema schema sources
//
//go:embed all:jsonschema
var JSONSchema embed.FS

// GraphQLSchema schema sources
//
// //go:embed all:graphql
var GraphQLSchema embed.FS

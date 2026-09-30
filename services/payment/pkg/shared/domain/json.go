package domain

import "monorepo/globalshared/gormx"

// JSON is a jsonb column value, see gormx.JSON
type JSON = gormx.JSON

// NewJSON marshals v into a JSON value (nil becomes {}).
func NewJSON(v any) JSON { return gormx.NewJSON(v) }

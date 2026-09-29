package gormx

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSON is a jsonb column value. It marshals as raw JSON in API responses.
type JSON json.RawMessage

// NewJSON marshals v into a JSON value (nil becomes {}).
func NewJSON(v any) JSON {
	if v == nil {
		return JSON("{}")
	}
	b, err := json.Marshal(v)
	if err != nil {
		return JSON("{}")
	}
	return JSON(b)
}

// Decode unmarshals into v; an empty value decodes to the zero value of v.
func (j JSON) Decode(v any) error {
	if len(j) == 0 {
		return nil
	}
	return json.Unmarshal(j, v)
}

// Value implements driver.Valuer
func (j JSON) Value() (driver.Value, error) {
	if len(j) == 0 {
		return "{}", nil
	}
	return string(j), nil
}

// Scan implements sql.Scanner
func (j *JSON) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append((*j)[:0], v...)
	case string:
		*j = JSON(v)
	default:
		return errors.New("gormx.JSON: unsupported scan type")
	}
	return nil
}

// MarshalJSON implements json.Marshaler
func (j JSON) MarshalJSON() ([]byte, error) {
	if len(j) == 0 {
		return []byte("{}"), nil
	}
	return j, nil
}

// UnmarshalJSON implements json.Unmarshaler
func (j *JSON) UnmarshalJSON(b []byte) error {
	*j = append((*j)[:0], b...)
	return nil
}

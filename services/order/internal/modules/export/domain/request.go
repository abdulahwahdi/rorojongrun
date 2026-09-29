package domain

import "encoding/json"

// RequestCreateExport asks for a CSV export; Filter takes the query parameters of the matching list
// endpoint as a JSON object (GET /v1/orders for orders and order_lines, GET /v1/invoices for invoices)
type RequestCreateExport struct {
	Type   string          `json:"type"`
	Filter json.RawMessage `json:"filter"`
}

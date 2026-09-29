package workerhandler

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// parseEnvelope extracts what a gateway callback needs for verification from a Kafka message.
//
// The webhook ingress publishes {"headers": {...}, "body": <gateway payload>} where body is
// the gateway JSON object, or a JSON string when the exact bytes matter for a signature.
// A message without a "body" key is taken as the raw gateway payload itself, with the Kafka
// record headers standing in for the HTTP headers. Header names are lower-cased.
func parseEnvelope(value []byte, kafkaHeaders map[string]string) (headers map[string]string, body []byte) {
	headers = make(map[string]string, len(kafkaHeaders))
	for k, v := range kafkaHeaders {
		headers[strings.ToLower(k)] = v
	}

	var env struct {
		Headers map[string]any  `json:"headers"`
		Body    json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(value, &env); err != nil || len(env.Body) == 0 {
		return headers, value
	}
	for k, v := range env.Headers {
		switch t := v.(type) {
		case string:
			headers[strings.ToLower(k)] = t
		case nil:
		default:
			headers[strings.ToLower(k)] = fmt.Sprint(t)
		}
	}
	body = env.Body
	if body[0] == '"' {
		var s string
		if json.Unmarshal(body, &s) == nil {
			body = []byte(s)
		}
	}
	return headers, body
}

// sameTopics reports whether two topic lists hold the same topics, ignoring order
func sameTopics(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := append([]string(nil), a...), append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

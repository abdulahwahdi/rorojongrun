package workerhandler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_parseEnvelope(t *testing.T) {
	t.Run("envelope with object body", func(t *testing.T) {
		headers, body := parseEnvelope(
			[]byte(`{"headers":{"X-Callback-Token":"tok","x-num":5,"skipped":null},"body":{"external_id":"tx-1","status":"PAID"}}`),
			map[string]string{"Traceparent": "abc"})
		assert.Equal(t, "tok", headers["x-callback-token"], "names are lower-cased")
		assert.Equal(t, "5", headers["x-num"])
		assert.Equal(t, "abc", headers["traceparent"], "kafka headers are kept")
		assert.NotContains(t, headers, "skipped")
		assert.JSONEq(t, `{"external_id":"tx-1","status":"PAID"}`, string(body))
	})

	t.Run("body as a JSON string keeps the exact bytes", func(t *testing.T) {
		_, body := parseEnvelope([]byte(`{"headers":{},"body":"{\"a\": 1}"}`), nil)
		assert.Equal(t, `{"a": 1}`, string(body))
	})

	t.Run("a raw gateway payload with kafka headers as the http headers", func(t *testing.T) {
		raw := []byte(`{"order_id":"tx-1","transaction_status":"settlement"}`)
		headers, body := parseEnvelope(raw, map[string]string{"X-Callback-Token": "tok"})
		assert.Equal(t, raw, body)
		assert.Equal(t, "tok", headers["x-callback-token"])
	})

	t.Run("not json at all is passed through", func(t *testing.T) {
		headers, body := parseEnvelope([]byte("plain"), nil)
		assert.Equal(t, "plain", string(body))
		assert.Empty(t, headers)
	})

	t.Run("envelope headers win over kafka headers", func(t *testing.T) {
		headers, _ := parseEnvelope([]byte(`{"headers":{"x-a":"env"},"body":{}}`), map[string]string{"x-a": "kafka"})
		assert.Equal(t, "env", headers["x-a"])
	})
}

func Test_sameTopics(t *testing.T) {
	assert.True(t, sameTopics(nil, nil))
	assert.True(t, sameTopics([]string{"a", "b"}, []string{"b", "a"}), "order does not matter")
	assert.False(t, sameTopics([]string{"a"}, []string{"a", "b"}))
	assert.False(t, sameTopics([]string{"a", "b"}, []string{"a", "c"}))
}

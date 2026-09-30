package domain

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_PaymentEvent_Meta(t *testing.T) {
	var ev PaymentEvent
	assert.NoError(t, json.Unmarshal([]byte(`{"metadata":{"merchantId":"M1","outletId":7,"rate":1.5,"ok":true,"obj":{"a":1}}}`), &ev))
	assert.Equal(t, "M1", ev.Meta("merchantId"))
	assert.Equal(t, "7", ev.Meta("outletId"), "numbers are kept as their text")
	assert.Equal(t, "1.5", ev.Meta("rate"))
	assert.Equal(t, "true", ev.Meta("ok"))
	assert.Equal(t, "", ev.Meta("obj"), "objects are not a merchant id")
	assert.Equal(t, "", ev.Meta("missing"))
}

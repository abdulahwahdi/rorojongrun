package helper

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_FormatIDR(t *testing.T) {
	cases := map[int64]string{0: "Rp0", 999: "Rp999", 1000: "Rp1.000", 101000: "Rp101.000", 1250000: "Rp1.250.000", 1000000000: "Rp1.000.000.000", -5000: "-Rp5.000"}
	for in, want := range cases {
		assert.Equal(t, want, FormatIDR(in))
	}
}

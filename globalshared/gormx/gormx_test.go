package gormx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Like(t *testing.T) {
	assert.Equal(t, `%50\%\_off%`, Like("50%_off"), "wildcards in user input are escaped")
	assert.Equal(t, `%a\\b%`, Like(`a\b`))
}

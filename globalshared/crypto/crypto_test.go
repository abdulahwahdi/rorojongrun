package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEncryptDecrypt(t *testing.T) {
	enc, err := Encrypt("s", []byte("payload"))
	assert.NoError(t, err)
	dec, err := Decrypt("s", enc)
	assert.NoError(t, err)
	assert.Equal(t, "payload", string(dec))

	_, err = Decrypt("other", enc)
	assert.Error(t, err, "wrong secret")
	_, err = Decrypt("s", enc[:len(enc)-4]+"AAAA")
	assert.Error(t, err, "tampered ciphertext")
	_, err = Decrypt("s", "")
	assert.Error(t, err, "empty input")

	enc2, _ := Encrypt("s", []byte("payload"))
	assert.NotEqual(t, enc, enc2, "fresh nonce every time")
}

package helper

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Crypto(t *testing.T) {
	t.Run("secret hashing", func(t *testing.T) {
		h, err := HashSecret("pw")
		assert.NoError(t, err)
		assert.True(t, CheckSecret(h, "pw"))
		assert.False(t, CheckSecret(h, "PW"))
		assert.False(t, CheckSecret("", ""), "an empty hash must never match")
	})

	t.Run("aes-gcm round trip, wrong secret and tampering fail", func(t *testing.T) {
		enc, err := Encrypt("s", []byte("payload"))
		assert.NoError(t, err)
		dec, err := Decrypt("s", enc)
		assert.NoError(t, err)
		assert.Equal(t, "payload", string(dec))
		_, err = Decrypt("other", enc)
		assert.Error(t, err)
		_, err = Decrypt("s", enc[:len(enc)-4]+"AAAA")
		assert.Error(t, err)
		_, err = Decrypt("s", "")
		assert.Error(t, err)
		enc2, _ := Encrypt("s", []byte("payload"))
		assert.NotEqual(t, enc, enc2, "fresh nonce every time")
	})

	t.Run("rsa pem round trip", func(t *testing.T) {
		k, err := GenerateRSAKey()
		assert.NoError(t, err)
		priv, _ := PrivateKeyToPEM(k)
		pub, _ := PublicKeyToPEM(&k.PublicKey)
		k2, err := PrivateKeyFromPEM(priv)
		assert.NoError(t, err)
		assert.True(t, k.Equal(k2))
		p2, err := PublicKeyFromPEM(pub)
		assert.NoError(t, err)
		assert.True(t, k.PublicKey.Equal(p2))
		_, err = PrivateKeyFromPEM([]byte("junk"))
		assert.Error(t, err)
		_, err = PublicKeyFromPEM(priv)
		assert.Error(t, err)
	})

	t.Run("tokens", func(t *testing.T) {
		assert.NotEqual(t, RandomToken(32), RandomToken(32))
		assert.Equal(t, SHA256Hex("a"), SHA256Hex("a"))
		assert.Len(t, SHA256Hex("a"), 64)
	})
}

func Test_Errors(t *testing.T) {
	cases := map[int]error{404: NewNotFound("x"), 409: NewConflict("x"), 400: NewInvalid("x"), 403: NewForbidden("x"), 401: NewUnauthorized("x"), 423: NewLocked("x")}
	for status, err := range cases {
		assert.Equal(t, status, HTTPStatus(err))
		assert.Equal(t, status, HTTPStatus(fmt.Errorf("wrapped: %w", err)), "survives wrapping")
	}
	assert.Equal(t, 500, HTTPStatus(errors.New("boom")))
	assert.Equal(t, "x", NewInvalid("x").Error())
}

func Test_PermissionAllows(t *testing.T) {
	g := [][2]string{{"order", "cancel"}, {"user", "*"}}
	assert.True(t, PermissionAllows(g, "order", "cancel"))
	assert.False(t, PermissionAllows(g, "order", "refund"))
	assert.True(t, PermissionAllows(g, "user", "anything"))
	assert.False(t, PermissionAllows(g, "kitchen", "anything"))
	assert.True(t, PermissionAllows([][2]string{{"*", "*"}}, "x", "y"))
	assert.True(t, PermissionAllows([][2]string{{"*", "cancel"}}, "any", "cancel"))
	assert.False(t, PermissionAllows([][2]string{{"*", "cancel"}}, "any", "other"))
	assert.False(t, PermissionAllows(nil, "x", "y"))
}

func Test_Like_and_StrPtr(t *testing.T) {
	assert.Equal(t, `%50\%\_off%`, Like("50%_off"), "wildcards in user input are escaped")
	assert.Nil(t, StrPtr(""))
	assert.Equal(t, "a", *StrPtr("a"))
	assert.Equal(t, "", StrVal(nil))
	assert.Equal(t, "a", StrVal(StrPtr("a")))
}

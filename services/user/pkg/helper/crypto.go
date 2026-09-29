package helper

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// HashSecret bcrypt-hashes a password / client secret
func HashSecret(secret string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	return string(h), err
}

// CheckSecret compares a plaintext secret with its bcrypt hash
func CheckSecret(hash, secret string) bool {
	return hash != "" && bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}

// RandomToken returns n random bytes as url-safe base64
func RandomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// SHA256Hex hex digest, used for refresh token storage
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// GenerateRSAKey generates a 2048 bit signing key
func GenerateRSAKey() (*rsa.PrivateKey, error) { return rsa.GenerateKey(rand.Reader, 2048) }

// PrivateKeyToPEM PKCS#8 PEM
func PrivateKeyToPEM(k *rsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

// PrivateKeyFromPEM parses PKCS#8 PEM
func PrivateKeyFromPEM(b []byte) (*rsa.PrivateKey, error) {
	blk, _ := pem.Decode(b)
	if blk == nil {
		return nil, errors.New("invalid private key pem")
	}
	k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an rsa key")
	}
	return rk, nil
}

// PublicKeyToPEM PKIX PEM
func PublicKeyToPEM(k *rsa.PublicKey) ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(k)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// PublicKeyFromPEM parses PKIX PEM
func PublicKeyFromPEM(b []byte) (*rsa.PublicKey, error) {
	blk, _ := pem.Decode(b)
	if blk == nil {
		return nil, errors.New("invalid public key pem")
	}
	k, err := x509.ParsePKIXPublicKey(blk.Bytes)
	if err != nil {
		return nil, err
	}
	rk, ok := k.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an rsa key")
	}
	return rk, nil
}

func gcmFor(secret string) (cipher.AEAD, error) {
	key := sha256.Sum256([]byte(secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// Encrypt AES-256-GCM, returns base64(nonce||ciphertext)
func Encrypt(secret string, plaintext []byte) (string, error) {
	gcm, err := gcmFor(secret)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, plaintext, nil)), nil
}

// Decrypt reverses Encrypt
func Decrypt(secret, encoded string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	gcm, err := gcmFor(secret)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	return gcm.Open(nil, raw[:gcm.NonceSize()], raw[gcm.NonceSize():], nil)
}

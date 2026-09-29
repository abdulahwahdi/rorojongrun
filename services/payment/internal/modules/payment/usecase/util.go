package usecase

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"

	"monorepo/services/payment/internal/modules/gateway/provider"
	"monorepo/services/payment/pkg/shared"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/google/uuid"
)

func newUUID() string { return uuid.NewString() }

// newToken is the checkout credential: 32 random bytes
func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// cash codes avoid look-alike characters so a cashier can read them out
const cashAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func newCashCode() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	for i := range b {
		b[i] = cashAlphabet[int(b[i])%len(cashAlphabet)]
	}
	return string(b)
}

func defaultProviders(row shareddomain.Gateway, requireEnabled bool) (provider.Provider, provider.Config, error) {
	secret := shared.GetEnv().GatewayEncryptionSecret
	if requireEnabled {
		return provider.Resolve(row, secret)
	}
	cfg, err := provider.BuildConfig(row, secret)
	if err != nil {
		return nil, cfg, err
	}
	p, err := provider.New(row.Code)
	return p, cfg, err
}

func isValidUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

func decodeMap(j shareddomain.JSON) map[string]any {
	m := map[string]any{}
	_ = j.Decode(&m)
	return m
}

// sensitiveHeaders are stored encrypted in callback logs: they authenticate the callback
var sensitiveHeaders = map[string]bool{"x-callback-token": true, "x-mock-token": true, "authorization": true}

const sealedPrefix = "enc:"

func isSealed(v string) bool { return strings.HasPrefix(v, sealedPrefix) }

func jsonBytes(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// jakarta is the display zone of customer emails
var jakarta = func() *time.Location {
	if l, err := time.LoadLocation("Asia/Jakarta"); err == nil {
		return l
	}
	return time.FixedZone("WIB", 7*3600)
}()

package provider

import (
	"context"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"monorepo/globalshared/crypto"
	shareddomain "monorepo/services/payment/pkg/shared/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func chargeReq(method MethodInfo) ChargeRequest {
	return ChargeRequest{
		TransactionID: "tx-1", Amount: 10500, Currency: "IDR", Method: method, Description: "Order #1",
		Customer:  shareddomain.Customer{Name: "Budi", Email: "budi@example.com"},
		Items:     []shareddomain.Item{{Name: "Nasi", Price: 10000, Quantity: 1}, {Name: "Fee", Price: 500, Quantity: 1}},
		ExpiresAt: time.Now().Add(30 * time.Minute),
	}
}

func Test_Midtrans_CreateCharge(t *testing.T) {
	var gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		assert.Equal(t, "/v2/charge", r.URL.Path)
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"status_code":"201","transaction_id":"mid-1","va_numbers":[{"bank":"bca","va_number":"1234567"}]}`))
	}))
	defer srv.Close()

	cfg := Config{Code: "midtrans", Settings: map[string]any{"baseUrl": srv.URL}, Credentials: map[string]string{"serverKey": "SB-key"}}
	res, err := (&Midtrans{client: srv.Client()}).CreateCharge(context.Background(), cfg, chargeReq(MethodInfo{Code: "bca_va", Type: "virtual_account", Channel: "bca"}))
	require.NoError(t, err)

	assert.Equal(t, "Basic "+base64.StdEncoding.EncodeToString([]byte("SB-key:")), gotAuth)
	assert.Equal(t, "bank_transfer", gotBody["payment_type"])
	assert.Equal(t, "tx-1", gotBody["transaction_details"].(map[string]any)["order_id"])
	assert.EqualValues(t, 10500, gotBody["transaction_details"].(map[string]any)["gross_amount"])
	assert.Equal(t, "1234567", res.Instruction["vaNumber"])
	assert.Equal(t, "mid-1", res.ExternalRef)

	t.Run("gateway rejection in body", func(t *testing.T) {
		bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"status_code":"406","status_message":"duplicate order_id"}`))
		}))
		defer bad.Close()
		cfg.Settings["baseUrl"] = bad.URL
		_, err := (&Midtrans{client: bad.Client()}).CreateCharge(context.Background(), cfg, chargeReq(MethodInfo{Type: "virtual_account", Channel: "bca"}))
		assert.ErrorContains(t, err, "duplicate order_id")
	})

	t.Run("items that do not add up are collapsed to one line", func(t *testing.T) {
		req := chargeReq(MethodInfo{})
		req.Items = []shareddomain.Item{{Name: "x", Price: 1, Quantity: 1}}
		items := reconcileItems(req)
		require.Len(t, items, 1)
		assert.EqualValues(t, 10500, items[0].Price)
	})

	t.Run("missing server key", func(t *testing.T) {
		_, err := (&Midtrans{}).CreateCharge(context.Background(), Config{}, chargeReq(MethodInfo{}))
		assert.Error(t, err)
	})
}

func midtransCallback(orderID, code, gross, status, serverKey string) []byte {
	sum := sha512.Sum512([]byte(orderID + code + gross + serverKey))
	b, _ := json.Marshal(map[string]string{
		"order_id": orderID, "status_code": code, "gross_amount": gross,
		"signature_key": hex.EncodeToString(sum[:]), "transaction_status": status,
	})
	return b
}

func Test_Midtrans_ParseCallback(t *testing.T) {
	cfg := Config{Credentials: map[string]string{"serverKey": "SB-key"}}
	m := &Midtrans{}

	cases := map[string]string{
		"settlement": CallbackPaid, "expire": CallbackExpired, "deny": CallbackFailed,
		"cancel": CallbackFailed, "pending": CallbackIgnored, "refund": CallbackIgnored,
	}
	for status, want := range cases {
		res, err := m.ParseCallback(context.Background(), cfg, nil, midtransCallback("tx-1", "200", "10500.00", status, "SB-key"))
		require.NoError(t, err, status)
		assert.Equal(t, want, res.Status, status)
		assert.Equal(t, "tx-1", res.TransactionID)
		assert.EqualValues(t, 10500, res.Amount)
	}

	t.Run("wrong signature", func(t *testing.T) {
		_, err := m.ParseCallback(context.Background(), cfg, nil, midtransCallback("tx-1", "200", "10500.00", "settlement", "other-key"))
		assert.ErrorIs(t, err, ErrInvalidCallback)
	})
	t.Run("tampered amount", func(t *testing.T) {
		body := midtransCallback("tx-1", "200", "10500.00", "settlement", "SB-key")
		var m2 map[string]string
		_ = json.Unmarshal(body, &m2)
		m2["gross_amount"] = "1.00"
		body, _ = json.Marshal(m2)
		_, err := m.ParseCallback(context.Background(), cfg, nil, body)
		assert.ErrorIs(t, err, ErrInvalidCallback)
	})
	t.Run("capture with challenge fraud status is not paid", func(t *testing.T) {
		var m2 map[string]string
		_ = json.Unmarshal(midtransCallback("tx-1", "200", "10500.00", "capture", "SB-key"), &m2)
		m2["fraud_status"] = "challenge"
		body, _ := json.Marshal(m2)
		res, err := m.ParseCallback(context.Background(), cfg, nil, body)
		require.NoError(t, err)
		assert.Equal(t, CallbackIgnored, res.Status)
	})
	t.Run("garbage body", func(t *testing.T) {
		_, err := m.ParseCallback(context.Background(), cfg, nil, []byte("nope"))
		assert.ErrorIs(t, err, ErrInvalidCallback)
	})
}

func Test_Midtrans_Snap(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/snap/v1/transactions", r.URL.Path)
		_, _ = w.Write([]byte(`{"token":"t","redirect_url":"https://pay.example/t"}`))
	}))
	defer srv.Close()
	cfg := Config{Settings: map[string]any{"snapBaseUrl": srv.URL}, Credentials: map[string]string{"serverKey": "k"}}
	res, err := (&Midtrans{client: srv.Client()}).CreateCharge(context.Background(), cfg, chargeReq(MethodInfo{Code: "credit_card", Type: "card", Channel: "credit_card"}))
	require.NoError(t, err)
	assert.Equal(t, "redirect", res.Instruction["type"])
	assert.Equal(t, "https://pay.example/t", res.Instruction["url"])
}

func Test_Xendit(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/invoices/inv-1/expire!" {
			w.WriteHeader(http.StatusOK)
			return
		}
		assert.Equal(t, "/v2/invoices", r.URL.Path)
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		_, _ = w.Write([]byte(`{"id":"inv-1","invoice_url":"https://checkout.xendit.co/web/inv-1"}`))
	}))
	defer srv.Close()
	cfg := Config{Settings: map[string]any{"baseUrl": srv.URL}, Credentials: map[string]string{"secretKey": "xnd", "callbackToken": "tok"}}
	x := &Xendit{client: srv.Client()}

	res, err := x.CreateCharge(context.Background(), cfg, chargeReq(MethodInfo{Code: "xendit_invoice", Channel: "invoice"}))
	require.NoError(t, err)
	assert.Equal(t, "tx-1", gotBody["external_id"])
	assert.NotContains(t, gotBody, "payment_methods", "invoice channel lets the customer pick")
	assert.Equal(t, "inv-1", res.ExternalRef)
	assert.Equal(t, "https://checkout.xendit.co/web/inv-1", res.Instruction["url"])

	_, err = x.CreateCharge(context.Background(), cfg, chargeReq(MethodInfo{Code: "bca", Channel: "bca"}))
	require.NoError(t, err)
	assert.Equal(t, []any{"BCA"}, gotBody["payment_methods"])

	assert.NoError(t, x.Cancel(context.Background(), cfg, "tx-1", "inv-1"))

	t.Run("callback", func(t *testing.T) {
		body := []byte(`{"external_id":"tx-1","status":"PAID","amount":10500,"paid_amount":10500,"paid_at":"2026-09-25T10:00:00Z"}`)
		got, err := x.ParseCallback(context.Background(), cfg, map[string]string{"x-callback-token": "tok"}, body)
		require.NoError(t, err)
		assert.Equal(t, CallbackPaid, got.Status)
		assert.EqualValues(t, 10500, got.Amount)

		got, err = x.ParseCallback(context.Background(), cfg, map[string]string{"x-callback-token": "tok"}, []byte(`{"external_id":"tx-1","status":"EXPIRED"}`))
		require.NoError(t, err)
		assert.Equal(t, CallbackExpired, got.Status)

		_, err = x.ParseCallback(context.Background(), cfg, map[string]string{"x-callback-token": "bad"}, body)
		assert.ErrorIs(t, err, ErrInvalidCallback)
		_, err = x.ParseCallback(context.Background(), cfg, nil, body)
		assert.ErrorIs(t, err, ErrInvalidCallback)
	})
}

func Test_Mock(t *testing.T) {
	m := &Mock{}
	cfg := Config{Environment: "sandbox", Credentials: map[string]string{"callbackToken": "tok"}}

	res, err := m.CreateCharge(context.Background(), cfg, chargeReq(MethodInfo{Channel: "va"}))
	require.NoError(t, err)
	res2, _ := m.CreateCharge(context.Background(), cfg, chargeReq(MethodInfo{Channel: "va"}))
	assert.Equal(t, res.Instruction["vaNumber"], res2.Instruction["vaNumber"], "deterministic per transaction")
	assert.Len(t, res.Instruction["vaNumber"], 12)

	got, err := m.ParseCallback(context.Background(), cfg, map[string]string{"x-mock-token": "tok"}, []byte(`{"transactionId":"tx-1","status":"paid","amount":10500}`))
	require.NoError(t, err)
	assert.Equal(t, CallbackPaid, got.Status)

	_, err = m.ParseCallback(context.Background(), cfg, nil, []byte(`{"transactionId":"tx-1","status":"paid"}`))
	assert.ErrorIs(t, err, ErrInvalidCallback)

	prod := Config{Environment: "production"}
	_, err = m.CreateCharge(context.Background(), prod, chargeReq(MethodInfo{}))
	assert.Error(t, err)
}

func Test_Resolve(t *testing.T) {
	creds, _ := crypto.Encrypt("secret", []byte(`{"serverKey":"k"}`))
	row := shareddomain.Gateway{Code: "midtrans", IsEnabled: true, Environment: "sandbox", CredentialsEnc: creds, Settings: shareddomain.NewJSON(map[string]any{"baseUrl": "http://x"})}

	p, cfg, err := Resolve(row, "secret")
	require.NoError(t, err)
	assert.Equal(t, "midtrans", p.Code())
	assert.Equal(t, "k", cfg.Credentials["serverKey"])
	assert.Equal(t, "http://x", cfg.BaseURL("a", "b"))

	_, _, err = Resolve(row, "wrong-secret")
	assert.ErrorContains(t, err, "cannot be decrypted")

	disabled := row
	disabled.IsEnabled = false
	_, _, err = Resolve(disabled, "secret")
	assert.ErrorContains(t, err, "disabled")

	nocreds := row
	nocreds.CredentialsEnc = ""
	_, _, err = Resolve(nocreds, "secret")
	assert.ErrorContains(t, err, "no credentials")

	mock := shareddomain.Gateway{Code: "mock", IsEnabled: true, Environment: "sandbox"}
	_, _, err = Resolve(mock, "secret")
	assert.NoError(t, err, "mock needs no credentials")

	_, err = New("nope")
	assert.Error(t, err)
}

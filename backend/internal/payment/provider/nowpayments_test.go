//go:build unit

package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/payment"
	"github.com/stretchr/testify/require"
)

func TestNewNowPaymentsValidatesConfig(t *testing.T) {
	t.Parallel()

	// Missing required keys
	_, err := NewNowPayments("1", map[string]string{})
	require.ErrorContains(t, err, "apiKey")

	_, err = NewNowPayments("1", map[string]string{"apiKey": "key"})
	require.ErrorContains(t, err, "ipnSecret")

	// Happy path
	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":    "key",
		"ipnSecret": "secret",
	})
	require.NoError(t, err)
	require.Equal(t, payment.TypeNowPayments, prov.ProviderKey())
	require.Equal(t, []payment.PaymentType{payment.TypeNowPayments}, prov.SupportedTypes())
	require.Equal(t, "NOWPayments", prov.Name())
	require.Equal(t, "USDT", prov.config["currency"])
	require.Equal(t, "https://api.nowpayments.io", prov.config["apiBase"])
}

func TestNowPaymentsUsesCustomConfig(t *testing.T) {
	t.Parallel()

	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":      "key",
		"ipnSecret":   "secret",
		"apiBase":     "https://custom.example.com",
		"currency":    "BTC",
		"payCurrency": "btc",
	})
	require.NoError(t, err)
	require.Equal(t, "BTC", prov.MerchantIdentityMetadata()["currency"])
	require.Equal(t, "BTC", prov.currency())
	require.Equal(t, "btc", prov.payCurrency())
}

func TestNowPaymentsCurrencyDefaults(t *testing.T) {
	t.Parallel()

	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":    "key",
		"ipnSecret": "secret",
	})
	require.NoError(t, err)
	// NewNowPayments defaults currency to "USDT" per its config comment
	require.Equal(t, "USDT", prov.currency())
	require.Equal(t, "usdt", prov.payCurrency())
}

func TestNowPaymentsNilReceiverSafety(t *testing.T) {
	t.Parallel()

	var n *NowPayments
	require.Nil(t, n.MerchantIdentityMetadata())
	require.Equal(t, payment.DefaultPaymentCurrency, n.currency())
	require.Equal(t, "usdt", n.payCurrency())
}

func TestNowPaymentsMapPaymentStatus(t *testing.T) {
	t.Parallel()

	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":    "key",
		"ipnSecret": "secret",
	})
	require.NoError(t, err)

	tests := []struct {
		input    string
		expected string
	}{
		{"waiting", payment.ProviderStatusPending},
		{"confirming", payment.ProviderStatusPending},
		{"exchanging", payment.ProviderStatusPending},
		{"sending", payment.ProviderStatusPending},
		{"finished", payment.ProviderStatusPaid},
		{"failed", payment.ProviderStatusFailed},
		{"refunded", payment.ProviderStatusRefunded},
		{"unknown_status", payment.ProviderStatusPending},
		{"", payment.ProviderStatusPending},
	}

	for _, tt := range tests {
		result := prov.mapPaymentStatus(tt.input)
		require.Equal(t, tt.expected, result, "mapPaymentStatus(%q)", tt.input)
	}
}

func TestNowPaymentsVerifyIPNSignature(t *testing.T) {
	t.Parallel()

	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":    "key",
		"ipnSecret": "test-secret",
	})
	require.NoError(t, err)

	// Generate a known-good signature using the sorted-key algorithm from the API docs
	body := `{"payment_id":"123","payment_status":"finished"}`
	expectedSig := signNowPaymentsBody(body, "test-secret")

	require.True(t, prov.verifyIPNSignature(body, expectedSig))

	// Invalid signature
	require.False(t, prov.verifyIPNSignature(body, "invalid-signature"))

	// Empty signature
	require.False(t, prov.verifyIPNSignature(body, ""))
}

func TestNowPaymentsCreatePayment(t *testing.T) {
	t.Parallel()

	var (
		requestCount int
		requestMu    sync.Mutex
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/estimate":
			require.Equal(t, "key", r.Header.Get("x-api-key"))
			_, _ = w.Write([]byte(`{"estimated_amount":"100.50"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/payment":
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			var req map[string]interface{}
			require.NoError(t, json.Unmarshal(body, &req))
			require.Equal(t, "100.00", req["price_amount"])
			require.Equal(t, "USDT", req["price_currency"])
			require.Equal(t, "usdt", req["pay_currency"])
			require.Equal(t, "100.50", req["pay_amount"])
			require.Equal(t, "order_123", req["order_id"])
			require.Equal(t, "https://notify.example.com", req["ipn_callback_url"])
			requestMu.Lock()
			requestCount++
			requestMu.Unlock()
			_, _ = w.Write([]byte(`{"payment_id":"np_123","payment_status":"waiting","pay_address":"0xABC","pay_currency":"usdt","price_amount":100.00,"price_currency":"USDT","pay_amount":100.50,"order_id":"order_123","created_at":"2026-05-27T00:00:00Z"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":    "key",
		"ipnSecret": "secret",
		"apiBase":   server.URL,
	})
	require.NoError(t, err)
	prov.httpClient = server.Client()

	resp, err := prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID:   "order_123",
		Amount:    "100.00",
		NotifyURL: "https://notify.example.com",
	})
	require.NoError(t, err)
	require.Equal(t, "np_123", resp.TradeNo)
	require.Equal(t, "0xABC", resp.QRCode)
	require.Equal(t, "USDT", resp.Currency)
	require.Equal(t, "usdt", resp.PayCurrency)
	require.Equal(t, "100.5", resp.PayAmount) // FormatFloat(100.50, 'f', -1, 64) → "100.5"
	require.Equal(t, payment.CreatePaymentResultOrderCreated, resp.ResultType)
	require.Equal(t, 1, requestCount)
}

func TestNowPaymentsQueryOrder(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/payment/np_123", r.URL.Path)
		require.Equal(t, "GET", r.Method)
		_, _ = w.Write([]byte(`{"payment_id":"np_123","payment_status":"finished","pay_address":"0xABC","price_amount":100.00,"price_currency":"CNY","pay_currency":"usdttrc20","actually_paid":100.50,"pay_amount":100.50,"order_id":"order_123","created_at":"2026-05-27T00:00:00Z","updated_at":"2026-05-27T00:05:00Z"}`))
	}))
	defer server.Close()

	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":    "key",
		"ipnSecret": "secret",
		"apiBase":   server.URL,
	})
	require.NoError(t, err)
	prov.httpClient = server.Client()

	resp, err := prov.QueryOrder(context.Background(), "np_123")
	require.NoError(t, err)
	require.Equal(t, "np_123", resp.TradeNo)
	require.Equal(t, payment.ProviderStatusPaid, resp.Status)
	require.Equal(t, 100.50, resp.Amount)
	require.Equal(t, "2026-05-27T00:05:00Z", resp.PaidAt)
}

func signNowPaymentsBody(body, secret string) string {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(body), &params); err != nil {
		return ""
	}
	sorted, _ := json.Marshal(sortObject(params))
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write(sorted)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestNowPaymentsVerifyNotification(t *testing.T) {
	t.Parallel()

	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":    "key",
		"ipnSecret": "test-secret",
	})
	require.NoError(t, err)

	body := `{"payment_id":"np_123","payment_status":"finished","pay_address":"0xABC","actually_paid":100.50,"order_id":"order_123","updated_at":"2026-05-27T00:05:00Z"}`
	sig := signNowPaymentsBody(body, "test-secret")

	notification, err := prov.VerifyNotification(context.Background(), body, map[string]string{
		"x-nowpayments-sig": sig,
	})
	require.NoError(t, err)
	require.NotNil(t, notification)
	require.Equal(t, "np_123", notification.TradeNo)
	require.Equal(t, "order_123", notification.OrderID)
	require.Equal(t, 100.50, notification.Amount)
	require.Equal(t, payment.ProviderStatusSuccess, notification.Status)

	// Test failed status
	failedBody := `{"payment_id":"np_456","payment_status":"failed","actually_paid":0,"order_id":"order_456","updated_at":"2026-05-27T00:05:00Z"}`
	failedSig := signNowPaymentsBody(failedBody, "test-secret")
	failedNotif, err := prov.VerifyNotification(context.Background(), failedBody, map[string]string{
		"x-nowpayments-sig": failedSig,
	})
	require.NoError(t, err)
	require.Equal(t, payment.ProviderStatusFailed, failedNotif.Status)

	// Missing signature header
	_, err = prov.VerifyNotification(context.Background(), body, map[string]string{})
	require.ErrorContains(t, err, "missing signature")

	// Invalid signature
	_, err = prov.VerifyNotification(context.Background(), body, map[string]string{
		"x-nowpayments-sig": "bad",
	})
	require.ErrorContains(t, err, "invalid signature")
}

func TestNowPaymentsRefund(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/payment/np_123/refund", r.URL.Path)
		require.Equal(t, "POST", r.Method)
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &req))
		require.Equal(t, "np_123", req["payment_id"])
		require.Equal(t, "50.00", req["amount"])
		_, _ = w.Write([]byte(`{"refund_id":"ref_123","status":"success"}`))
	}))
	defer server.Close()

	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":    "key",
		"ipnSecret": "secret",
		"apiBase":   server.URL,
	})
	require.NoError(t, err)
	prov.httpClient = server.Client()

	resp, err := prov.Refund(context.Background(), payment.RefundRequest{
		TradeNo: "np_123",
		Amount:  "50.00",
	})
	require.NoError(t, err)
	require.Equal(t, "ref_123", resp.RefundID)
	require.Equal(t, payment.ProviderStatusSuccess, resp.Status)
}

func TestNowPaymentsCreatePaymentEstimateError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/estimate" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"internal error"}`))
		}
	}))
	defer server.Close()

	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":    "key",
		"ipnSecret": "secret",
		"apiBase":   server.URL,
	})
	require.NoError(t, err)
	prov.httpClient = server.Client()

	_, err = prov.CreatePayment(context.Background(), payment.CreatePaymentRequest{
		OrderID: "order_123",
		Amount:  "100.00",
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "nowpayments estimate")
}

func TestNowPaymentsMerchantIdentityMetadata(t *testing.T) {
	t.Parallel()

	prov, err := NewNowPayments("1", map[string]string{
		"apiKey":    "key",
		"ipnSecret": "secret",
		"currency":  "ETH",
	})
	require.NoError(t, err)
	meta := prov.MerchantIdentityMetadata()
	require.Equal(t, map[string]string{"currency": "ETH"}, meta)
}
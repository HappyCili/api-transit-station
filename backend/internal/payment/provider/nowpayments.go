package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/payment"
)

// flexibleString handles JSON values that can be either a string or a number,
// converting both to a Go string. NowPayments API returns some fields as strings
// in one endpoint and as numbers in another.
type flexibleString string

func (s *flexibleString) UnmarshalJSON(data []byte) error {
	if len(data) == 0 {
		return nil
	}
	if data[0] == '"' {
		var v string
		if err := json.Unmarshal(data, &v); err != nil {
			return err
		}
		*s = flexibleString(v)
		return nil
	}
	var v json.Number
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	*s = flexibleString(string(v))
	return nil
}

// NowPayments constants.
const (
	nowpaymentsHTTPTimeout       = 15 * time.Second
	nowpaymentsAPIVersion        = "v1"
	nowpaymentsIPNHeader         = "x-nowpayments-sig"
	nowpaymentsIPNStatusFinished = "finished"
	nowpaymentsIPNStatusFailed   = "failed"
	nowpaymentsIPNStatusRefunded = "refunded"
	maxNowPaymentsResponseSize   = 1 << 20 // 1MB

	nowpaymentsCurrenciesCacheTTL = 4 * time.Hour
	nowpaymentsCurrenciesCacheDir = "./data"
	nowpaymentsCurrenciesCacheFile = "nowpayments_currencies.json"
)

// NowPayments implements payment.Provider for NOWPayments crypto gateway.
type NowPayments struct {
	instanceID string
	config     map[string]string
	httpClient *http.Client
}

// NewNowPayments creates a new NOWPayments provider instance.
// config keys: apiKey, ipnSecret, apiBase, notifyUrl, currency (default: USDT)
func NewNowPayments(instanceID string, config map[string]string) (*NowPayments, error) {
	for _, k := range []string{"apiKey", "ipnSecret"} {
		if strings.TrimSpace(config[k]) == "" {
			return nil, fmt.Errorf("nowpayments config missing required key: %s", k)
		}
	}
	cfg := make(map[string]string, len(config))
	for k, v := range config {
		cfg[k] = v
	}
	if cfg["apiBase"] == "" {
		cfg["apiBase"] = "https://api.nowpayments.io"
	}
	if cfg["currency"] == "" {
		cfg["currency"] = "USDT"
	}
	return &NowPayments{
		instanceID: instanceID,
		config:     cfg,
		httpClient: &http.Client{Timeout: nowpaymentsHTTPTimeout},
	}, nil
}

func (n *NowPayments) Name() string        { return "NOWPayments" }
func (n *NowPayments) ProviderKey() string { return payment.TypeNowPayments }
func (n *NowPayments) SupportedTypes() []payment.PaymentType {
	return []payment.PaymentType{payment.TypeNowPayments}
}

func (n *NowPayments) MerchantIdentityMetadata() map[string]string {
	if n == nil {
		return nil
	}
	return map[string]string{"currency": n.currency()}
}

func (n *NowPayments) currency() string {
	if n == nil {
		return payment.DefaultPaymentCurrency
	}
	if c := strings.TrimSpace(n.config["currency"]); c != "" {
		return strings.ToLower(c)
	}
	return payment.DefaultPaymentCurrency
}

// payCurrency returns the cryptocurrency ticker to receive payment in.
// It reads the "payCurrency" config value directly as a simple ticker (e.g. "btc", "eth", "usdt").
func (n *NowPayments) payCurrency() string {
	if n == nil {
		return "usdt"
	}
	if c := strings.TrimSpace(n.config["payCurrency"]); c != "" {
		return strings.ToLower(c)
	}
	return "usdt"
}

// CreatePayment initiates a payment via NOWPayments.
// It first creates a payment, then generates a payment URL for the frontend.
func (n *NowPayments) CreatePayment(ctx context.Context, req payment.CreatePaymentRequest) (*payment.CreatePaymentResponse, error) {
	payCurrency := n.payCurrency()
	priceCurrency := n.currency()
	slog.Info("nowpayments create payment start",
		"provider_instance", n.instanceID,
		"order_id", req.OrderID,
		"amount", req.Amount,
		"price_currency", priceCurrency,
		"pay_currency", payCurrency,
	)

	// Step 1: Estimate price to get the exact crypto amount
	estimateURL := fmt.Sprintf("%s?amount=%s&currency_from=%s&currency_to=%s",
		n.apiURL("/v1/estimate"),
		url.QueryEscape(req.Amount),
		url.QueryEscape(priceCurrency),
		url.QueryEscape(payCurrency),
	)
	estimateBody, err := n.getJSON(ctx, estimateURL)
	if err != nil {
		slog.Error("nowpayments estimate failed",
			"provider_instance", n.instanceID,
			"order_id", req.OrderID,
			"error", err,
		)
		return nil, fmt.Errorf("nowpayments estimate: %w", err)
	}
	var estimateResp nowPaymentsEstimateResponse
	if err := json.Unmarshal(estimateBody, &estimateResp); err != nil {
		slog.Error("nowpayments parse estimate failed",
			"provider_instance", n.instanceID,
			"order_id", req.OrderID,
			"error", err,
		)
		return nil, fmt.Errorf("nowpayments parse estimate: %w", err)
	}
	if estimateResp.EstimatedAmount == "" {
		slog.Error("nowpayments estimate returned empty amount",
			"provider_instance", n.instanceID,
			"order_id", req.OrderID,
		)
		return nil, fmt.Errorf("nowpayments estimate: empty estimated amount")
	}
	slog.Info("nowpayments estimate success",
		"provider_instance", n.instanceID,
		"order_id", req.OrderID,
		"estimated_amount", estimateResp.EstimatedAmount,
	)

	// Step 2: Create the payment
	notifyURL := n.resolveNotifyURL(req)
	payReq := nowPaymentsCreateRequest{
		PriceAmount:      req.Amount,
		PriceCurrency:    priceCurrency,
		PayCurrency:      payCurrency,
		PayAmount:        estimateResp.EstimatedAmount,
		IPNCallbackURL:   notifyURL,
		OrderID:          req.OrderID,
		OrderDescription: req.Subject,
	}
	body, err := n.postJSON(ctx, n.apiURL("/v1/payment"), payReq)
	if err != nil {
		slog.Error("nowpayments create payment failed",
			"provider_instance", n.instanceID,
			"order_id", req.OrderID,
			"error", err,
		)
		return nil, fmt.Errorf("nowpayments create: %w", err)
	}
	var createResp nowPaymentsCreateResponse
	if err := json.Unmarshal(body, &createResp); err != nil {
		slog.Error("nowpayments parse create response failed",
			"provider_instance", n.instanceID,
			"order_id", req.OrderID,
			"error", err,
		)
		return nil, fmt.Errorf("nowpayments parse create: %w", err)
	}
	if createResp.PaymentID == "" {
		slog.Error("nowpayments create response missing payment_id",
			"provider_instance", n.instanceID,
			"order_id", req.OrderID,
			"response_body", string(body),
		)
		return nil, fmt.Errorf("nowpayments create: empty payment_id")
	}
	slog.Info("nowpayments create payment success",
		"provider_instance", n.instanceID,
		"order_id", req.OrderID,
		"payment_id", createResp.PaymentID,
		"pay_address", createResp.PayAddress,
		"pay_amount", createResp.PayAmount,
		"pay_currency", createResp.PayCurrency,
		"price_currency", createResp.PriceCurrency,
	)

	var payAmountStr string
	if createResp.PayAmount > 0 {
		payAmountStr = strconv.FormatFloat(createResp.PayAmount, 'f', -1, 64)
	}
	return &payment.CreatePaymentResponse{
		TradeNo:    string(createResp.PaymentID),
		QRCode:     createResp.PayAddress,
		Currency:   createResp.PriceCurrency,
		PayCurrency: createResp.PayCurrency,
		PayAmount:  payAmountStr,
		ResultType: payment.CreatePaymentResultOrderCreated,
	}, nil
}

// QueryOrder queries the payment status from NOWPayments.
func (n *NowPayments) QueryOrder(ctx context.Context, tradeNo string) (*payment.QueryOrderResponse, error) {
	slog.Debug("nowpayments query order",
		"provider_instance", n.instanceID,
		"trade_no", tradeNo,
	)
	body, err := n.getJSON(ctx, n.apiURL("/v1/payment/"+tradeNo))
	if err != nil {
		slog.Error("nowpayments query failed",
			"provider_instance", n.instanceID,
			"trade_no", tradeNo,
			"error", err,
		)
		return nil, fmt.Errorf("nowpayments query: %w", err)
	}
	var resp nowPaymentsStatusResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		slog.Error("nowpayments parse query response failed",
			"provider_instance", n.instanceID,
			"trade_no", tradeNo,
			"error", err,
		)
		return nil, fmt.Errorf("nowpayments parse query: %w", err)
	}
	status := n.mapPaymentStatus(resp.PaymentStatus)
	slog.Info("nowpayments query result",
		"provider_instance", n.instanceID,
		"trade_no", tradeNo,
		"payment_status", resp.PaymentStatus,
		"mapped_status", status,
		"actually_paid", resp.ActuallyPaid,
		"pay_amount", resp.PayAmount,
	)
	return &payment.QueryOrderResponse{
		TradeNo: tradeNo,
		Status:  status,
		Amount:  resp.ActuallyPaid,
		PaidAt:  resp.UpdatedAt,
		Metadata: n.MerchantIdentityMetadata(),
	}, nil
}

// VerifyNotification parses and verifies a NOWPayments IPN webhook callback.
func (n *NowPayments) VerifyNotification(_ context.Context, rawBody string, headers map[string]string) (*payment.PaymentNotification, error) {
	signature := headers[nowpaymentsIPNHeader]
	if signature == "" {
		slog.Warn("nowpayments IPN missing signature header",
			"provider_instance", n.instanceID,
		)
		return nil, fmt.Errorf("nowpayments IPN: missing signature header")
	}
	if !n.verifyIPNSignature(rawBody, signature) {
		slog.Warn("nowpayments IPN invalid signature",
			"provider_instance", n.instanceID,
		)
		return nil, fmt.Errorf("nowpayments IPN: invalid signature")
	}
	var ipn nowPaymentsIPNNotification
	if err := json.Unmarshal([]byte(rawBody), &ipn); err != nil {
		slog.Error("nowpayments parse IPN failed",
			"provider_instance", n.instanceID,
			"error", err,
		)
		return nil, fmt.Errorf("nowpayments parse IPN: %w", err)
	}
	status := payment.ProviderStatusFailed
	switch ipn.PaymentStatus {
	case nowpaymentsIPNStatusFinished:
		status = payment.ProviderStatusSuccess
	case nowpaymentsIPNStatusRefunded:
		status = payment.ProviderStatusRefunded
	}
	slog.Info("nowpayments IPN received",
		"provider_instance", n.instanceID,
		"payment_id", string(ipn.PaymentID),
		"order_id", string(ipn.OrderID),
		"payment_status", ipn.PaymentStatus,
		"mapped_status", status,
		"actually_paid", ipn.ActuallyPaid,
		"pay_amount", ipn.PayAmount,
	)
	return &payment.PaymentNotification{
		TradeNo:  string(ipn.PaymentID),
		OrderID:  string(ipn.OrderID),
		Amount:   ipn.ActuallyPaid,
		Status:   status,
		RawData:  rawBody,
		Metadata: n.MerchantIdentityMetadata(),
	}, nil
}

// Refund requests a refund from NOWPayments (if supported by merchant tier).
func (n *NowPayments) Refund(ctx context.Context, req payment.RefundRequest) (*payment.RefundResponse, error) {
	slog.Info("nowpayments refund requested",
		"provider_instance", n.instanceID,
		"order_id", req.OrderID,
		"trade_no", req.TradeNo,
		"amount", req.Amount,
	)
	refundReq := nowPaymentsRefundRequest{
		PaymentID: req.TradeNo,
		Amount:    req.Amount,
	}
	body, err := n.postJSON(ctx, n.apiURL("/v1/payment/"+req.TradeNo+"/refund"), refundReq)
	if err != nil {
		slog.Error("nowpayments refund failed",
			"provider_instance", n.instanceID,
			"order_id", req.OrderID,
			"trade_no", req.TradeNo,
			"error", err,
		)
		return nil, fmt.Errorf("nowpayments refund: %w", err)
	}
	var refundResp nowPaymentsRefundResponse
	if err := json.Unmarshal(body, &refundResp); err != nil {
		slog.Error("nowpayments parse refund response failed",
			"provider_instance", n.instanceID,
			"order_id", req.OrderID,
			"trade_no", req.TradeNo,
			"error", err,
		)
		return nil, fmt.Errorf("nowpayments parse refund: %w", err)
	}
	slog.Info("nowpayments refund success",
		"provider_instance", n.instanceID,
		"order_id", req.OrderID,
		"trade_no", req.TradeNo,
		"refund_id", refundResp.RefundID,
	)
	return &payment.RefundResponse{
		RefundID: refundResp.RefundID,
		Status:   payment.ProviderStatusSuccess,
	}, nil
}

// --- Internal helpers ---

// NowPaymentsCurrency represents a currency from the NowPayments API.
type NowPaymentsCurrency struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
}

// GetCurrencies fetches all available pay_currency tickers from NowPayments,
// caching the result to disk for nowpaymentsCurrenciesCacheTTL (4 hours).
// /v1/currencies returns: { "currencies": ["btc", "eth", "usdttrc20", ...] }
func (n *NowPayments) GetCurrencies(ctx context.Context) ([]NowPaymentsCurrency, error) {
	cached, err := n.readCachedCurrencies()
	if err == nil {
		return cached, nil
	}
	slog.Debug("nowpayments currencies cache miss, fetching from API",
		"provider_instance", n.instanceID,
		"error", err,
	)

	body, err := n.getJSON(ctx, n.apiURL(nowpaymentsAPIVersion+"/currencies"))
	if err != nil {
		return nil, fmt.Errorf("nowpayments get currencies: %w", err)
	}
	var resp struct {
		Currencies []string `json:"currencies"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("nowpayments unmarshal currencies: %w", err)
	}
	out := make([]NowPaymentsCurrency, len(resp.Currencies))
	for i, t := range resp.Currencies {
		out[i] = NowPaymentsCurrency{Ticker: t, Name: t}
	}

	if writeErr := n.writeCachedCurrencies(out); writeErr != nil {
		slog.Warn("nowpayments currencies cache write failed",
			"provider_instance", n.instanceID,
			"error", writeErr,
		)
	}
	return out, nil
}

func (n *NowPayments) cachedCurrencyPath() string {
	return filepath.Join(nowpaymentsCurrenciesCacheDir, n.instanceID+"_"+nowpaymentsCurrenciesCacheFile)
}

func (n *NowPayments) readCachedCurrencies() ([]NowPaymentsCurrency, error) {
	p := n.cachedCurrencyPath()
	fi, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if time.Since(fi.ModTime()) > nowpaymentsCurrenciesCacheTTL {
		return nil, fmt.Errorf("cache expired")
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var out []NowPaymentsCurrency
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	slog.Debug("nowpayments currencies cache hit", "provider_instance", n.instanceID, "file", p)
	return out, nil
}

func (n *NowPayments) writeCachedCurrencies(currencies []NowPaymentsCurrency) error {
	p := n.cachedCurrencyPath()
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	raw, err := json.Marshal(currencies)
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, raw, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

func (n *NowPayments) apiURL(path string) string {
	base := strings.TrimRight(n.config["apiBase"], "/")
	if base == "" {
		base = "https://api.nowpayments.io"
	}
	return base + "/" + strings.TrimLeft(path, "/")
}

func (n *NowPayments) resolveNotifyURL(req payment.CreatePaymentRequest) string {
	if req.NotifyURL != "" {
		return req.NotifyURL
	}
	return n.config["notifyUrl"]
}

func (n *NowPayments) mapPaymentStatus(status string) string {
	switch status {
	case "waiting":
		return payment.ProviderStatusPending
	case "confirming", "exchanging", "sending":
		return payment.ProviderStatusPending
	case nowpaymentsIPNStatusFinished:
		return payment.ProviderStatusPaid
	case nowpaymentsIPNStatusFailed:
		return payment.ProviderStatusFailed
	case nowpaymentsIPNStatusRefunded:
		return payment.ProviderStatusRefunded
	default:
		return payment.ProviderStatusPending
	}
}

// verifyIPNSignature verifies the HMAC-SHA512 signature on an IPN callback.
// Per the NowPayments API docs, the signature is computed on the JSON body
// with keys sorted alphabetically: JSON.stringify(params, Object.keys(params).sort())
func (n *NowPayments) verifyIPNSignature(rawBody string, signature string) bool {
	var params map[string]interface{}
	if err := json.Unmarshal([]byte(rawBody), &params); err != nil {
		return false
	}
	sortedBody, err := json.Marshal(sortObject(params))
	if err != nil {
		return false
	}
	mac := hmac.New(sha512.New, []byte(n.config["ipnSecret"]))
	mac.Write(sortedBody)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// sortObject recursively sorts a map's keys and returns a new map/slice with
// sorted keys, matching NowPayments' JSON.stringify(params, Object.keys(params).sort()).
func sortObject(v interface{}) interface{} {
	switch obj := v.(type) {
	case map[string]interface{}:
		keys := make([]string, 0, len(obj))
		for k := range obj {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		sorted := make(map[string]interface{}, len(obj))
		for _, k := range keys {
			sorted[k] = sortObject(obj[k])
		}
		return sorted
	case []interface{}:
		arr := make([]interface{}, len(obj))
		for i, v := range obj {
			arr[i] = sortObject(v)
		}
		return arr
	default:
		return v
	}
}

// --- HTTP helpers ---

func (n *NowPayments) postJSON(ctx context.Context, url string, payload any) ([]byte, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		slog.Error("nowpayments post marshal failed", "error", err, "provider_instance", n.instanceID)
		return nil, err
	}
	slog.Info("nowpayments post request",
		"provider_instance", n.instanceID,
		"url", url,
		"body", string(data),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", n.config["apiKey"])
	start := time.Now()
	resp, err := n.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		slog.Error("nowpayments post failed",
			"provider_instance", n.instanceID,
			"url", url,
			"latency_ms", latency.Milliseconds(),
			"error", err,
		)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxNowPaymentsResponseSize))
	if err != nil {
		return nil, err
	}
	slog.Info("nowpayments post response",
		"provider_instance", n.instanceID,
		"url", url,
		"status", resp.StatusCode,
		"latency_ms", latency.Milliseconds(),
		"body", string(body),
	)
	if resp.StatusCode >= 400 {
		slog.Warn("nowpayments post returned error status",
			"provider_instance", n.instanceID,
			"url", url,
			"status", resp.StatusCode,
			"latency_ms", latency.Milliseconds(),
			"body", string(body),
		)
		return nil, fmt.Errorf("nowpayments HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (n *NowPayments) getJSON(ctx context.Context, url string) ([]byte, error) {
	slog.Info("nowpayments get request",
		"provider_instance", n.instanceID,
		"url", url,
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", n.config["apiKey"])
	start := time.Now()
	resp, err := n.httpClient.Do(req)
	latency := time.Since(start)
	if err != nil {
		slog.Error("nowpayments get failed",
			"provider_instance", n.instanceID,
			"url", url,
			"latency_ms", latency.Milliseconds(),
			"error", err,
		)
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxNowPaymentsResponseSize))
	if err != nil {
		return nil, err
	}
	slog.Info("nowpayments get response",
		"provider_instance", n.instanceID,
		"url", url,
		"status", resp.StatusCode,
		"latency_ms", latency.Milliseconds(),
		"body", string(body),
	)
	if resp.StatusCode >= 400 {
		slog.Warn("nowpayments get returned error status",
			"provider_instance", n.instanceID,
			"url", url,
			"status", resp.StatusCode,
			"latency_ms", latency.Milliseconds(),
			"body", string(body),
		)
		return nil, fmt.Errorf("nowpayments HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// --- Data structures ---

type nowPaymentsEstimateRequest struct {
	Amount        string `json:"amount"`
	CurrencyFrom  string `json:"currency_from"`
	CurrencyTo    string `json:"currency_to"`
}

type nowPaymentsEstimateResponse struct {
	EstimatedAmount string `json:"estimated_amount"`
}

type nowPaymentsCreateRequest struct {
	PriceAmount      string `json:"price_amount"`
	PriceCurrency    string `json:"price_currency"`
	PayCurrency      string `json:"pay_currency"`
	PayAmount        string `json:"pay_amount,omitempty"`
	IPNCallbackURL   string `json:"ipn_callback_url"`
	OrderID          string `json:"order_id"`
	OrderDescription string `json:"order_description,omitempty"`
}

type nowPaymentsCreateResponse struct {
	PaymentID   flexibleString `json:"payment_id"`
	PaymentStatus string       `json:"payment_status"`
	PayAddress  string         `json:"pay_address"`
	PriceAmount float64        `json:"price_amount"`
	PriceCurrency string       `json:"price_currency"`
	PayCurrency string         `json:"pay_currency"`
	PayAmount   float64        `json:"pay_amount"`
	OrderID     flexibleString `json:"order_id"`
	CreatedAt   string         `json:"created_at"`
}

type nowPaymentsStatusResponse struct {
	PaymentID     flexibleString `json:"payment_id"`
	PaymentStatus string         `json:"payment_status"`
	PayAddress    string         `json:"pay_address"`
	PriceAmount   float64        `json:"price_amount"`
	PriceCurrency string         `json:"price_currency"`
	PayCurrency   string         `json:"pay_currency"`
	ActuallyPaid  float64        `json:"actually_paid"`
	PayAmount     float64        `json:"pay_amount"`
	OrderID       flexibleString `json:"order_id"`
	CreatedAt     string         `json:"created_at"`
	UpdatedAt     string         `json:"updated_at"`
}

type nowPaymentsIPNNotification struct {
	PaymentID     flexibleString `json:"payment_id"`
	PaymentStatus string         `json:"payment_status"`
	PayAddress    string         `json:"pay_address"`
	PriceAmount   float64        `json:"price_amount"`
	PriceCurrency string         `json:"price_currency"`
	PayCurrency   string         `json:"pay_currency"`
	ActuallyPaid  float64        `json:"actually_paid"`
	PayAmount     float64        `json:"pay_amount"`
	OrderID       flexibleString `json:"order_id"`
	OrderDescription string      `json:"order_description"`
	PurchaseID    flexibleString `json:"purchase_id"`
	UpdatedAt     string         `json:"updated_at"`
	CreatedAt     string         `json:"created_at"`
}

type nowPaymentsRefundRequest struct {
	PaymentID string `json:"payment_id"`
	Amount    string `json:"amount,omitempty"`
}

type nowPaymentsRefundResponse struct {
	RefundID string `json:"refund_id"`
	Status   string `json:"status"`
}
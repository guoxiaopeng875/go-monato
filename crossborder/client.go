// Package crossborder provides a Go client for the Monato Cross-Border API (LastMile).
//
// It supports cross-border money transfers between Mexico (MXN/SPEI),
// United States (USD), Europe/SEPA (EUR), and global crypto networks (USDC, USDT).
//
// Supported flow types:
//   - Cross-Ramp: fiat↔fiat between bank accounts (MXN↔USD, MXN↔EUR)
//   - Off-Ramp:   crypto→fiat into Mexican bank accounts (USDC/USDT → MXN)
//   - On-Ramp:    fiat→crypto from Mexico (MXN → USDC/USDT)
//
// Docs: https://docs.monato.com/products/lastmile
package crossborder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	// BaseURLProduction is the live Cross-Border API endpoint.
	BaseURLProduction = "https://lastmile.monato.com"
	// BaseURLStaging is the sandbox/staging endpoint for development and testing.
	BaseURLStaging = "https://lastmile.stg.monato.com"

	defaultBaseURL = BaseURLProduction
	defaultTimeout = 30 * time.Second
)

// Supported currencies.
const (
	CurrencyMXN  = "MXN"
	CurrencyUSD  = "USD"
	CurrencyEUR  = "EUR"
	CurrencyUSDC = "USDC"
	CurrencyUSDT = "USDT"
)

// Supported blockchain networks for crypto wallets.
const (
	BlockchainPolygon = "POL"
	BlockchainSolana  = "SOL"
)

// Transaction states.
const (
	StateCreated      = "created"
	StatePending      = "pending"
	StateWaitingFunds = "waiting_funds"
	StateFunded       = "funded"
	StateProcessing   = "processing"
	StateCompleted    = "completed"
	StateFailed       = "failed"
	StateCancelled    = "cancelled"
)

// Client is the Monato Cross-Border API client.
type Client struct {
	baseURL    string
	httpClient *http.Client

	// credentials (user + password auth)
	user     string
	password string

	// token management
	mu          sync.RWMutex
	token       string
	tokenExpiry time.Time

	// Sub-services
	Banks        *BankService
	BankAccounts *BankAccountService
	Quotes       *QuoteService
	Transactions *TransactionService
	Webhooks     *WebhookService
}

// Config holds the configuration for creating a Cross-Border client.
type Config struct {
	// User is your Monato account email address.
	User string
	// Password is your Monato account password.
	Password string
	// BaseURL overrides the default API base URL.
	// Use BaseURLStaging for sandbox testing.
	BaseURL string
	// Timeout sets the HTTP client timeout (default: 30s).
	Timeout time.Duration
	// HTTPClient allows providing a custom *http.Client.
	HTTPClient *http.Client
}

// New creates a new Monato Cross-Border API client and authenticates immediately.
//
//	client, err := crossborder.New(ctx, crossborder.Config{
//	    User:     "you@example.com",
//	    Password: "Password@123",
//	    BaseURL:  crossborder.BaseURLStaging, // sandbox
//	})
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.User == "" || cfg.Password == "" {
		return nil, fmt.Errorf("crossborder: User and Password are required")
	}

	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	c := &Client{
		baseURL:    baseURL,
		user:       cfg.User,
		password:   cfg.Password,
		httpClient: httpClient,
	}

	c.Banks = &BankService{client: c}
	c.BankAccounts = &BankAccountService{client: c}
	c.Quotes = &QuoteService{client: c}
	c.Transactions = &TransactionService{client: c}
	c.Webhooks = &WebhookService{client: c}

	if err := c.refreshToken(ctx); err != nil {
		return nil, fmt.Errorf("crossborder: initial authentication failed: %w", err)
	}

	return c, nil
}

// --- Authentication: POST /api/v1/access_token ---

type accessTokenRequest struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

type accessTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"` // seconds (typically 3600)
	ExpiresAt   string `json:"expires_at"` // ISO 8601 e.g. "2025-10-07T10:24:55-06:00"
}

func (c *Client) refreshToken(ctx context.Context) error {
	payload := accessTokenRequest{
		User:     c.user,
		Password: c.password,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("crossborder/auth: marshal error: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1/access_token", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("crossborder/auth: request creation error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("crossborder/auth: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Body: string(b)}
	}

	var tokenResp accessTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("crossborder/auth: decode error: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return fmt.Errorf("crossborder/auth: received empty token")
	}

	// Parse expiry
	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	if tokenResp.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, tokenResp.ExpiresAt); err == nil {
			expiry = t
		}
	}

	c.mu.Lock()
	c.token = tokenResp.AccessToken
	c.tokenExpiry = expiry
	c.mu.Unlock()

	return nil
}

func (c *Client) getToken(ctx context.Context) (string, error) {
	c.mu.RLock()
	token := c.token
	expiry := c.tokenExpiry
	c.mu.RUnlock()

	if token == "" || time.Now().Add(60*time.Second).After(expiry) {
		if err := c.refreshToken(ctx); err != nil {
			return "", err
		}
		c.mu.RLock()
		token = c.token
		c.mu.RUnlock()
	}
	return token, nil
}

// --- HTTP helpers ---

// APIError represents an error response from the Cross-Border API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("crossborder API error: status=%d body=%s", e.StatusCode, e.Body)
}

// APIResponse wraps the standard Monato response envelope.
type APIResponse struct {
	Status string          `json:"status"` // "success" or "failed"
	Data   json.RawMessage `json:"data"`
	Errors json.RawMessage `json:"errors,omitempty"`
}

// Fees represents transaction fee information.
type Fees struct {
	FixedFee    float64 `json:"fixed_fee"`
	FeeCurrency string  `json:"fee_currency"` // MXN, USD, EUR
}

// CryptoWallet represents a cryptocurrency wallet in transaction responses.
type CryptoWallet struct {
	Address          string `json:"address"`
	BlockchainSymbol string `json:"blockchain_symbol"` // POL, SOL
	TokenSymbol      string `json:"token_symbol"`      // USDC, USDT
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("crossborder: marshal error: %w", err)
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("crossborder: request creation error: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crossborder: request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(b)}
	}

	return resp, nil
}

// doRequestWithIdempotency executes an authenticated request with an Idempotency-Key header.
func (c *Client) doRequestWithIdempotency(ctx context.Context, method, path string, body interface{}, idempotencyKey string) (*http.Response, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("crossborder: marshal error: %w", err)
		}
		reqBody = bytes.NewBuffer(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("crossborder: request creation error: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("crossborder: request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(b)}
	}

	return resp, nil
}

// decodeData decodes the Monato API response envelope and extracts .data into v.
func decodeData(resp *http.Response, v interface{}) error {
	defer resp.Body.Close()
	var envelope APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("crossborder: decode error: %w", err)
	}
	if envelope.Status != "success" {
		return fmt.Errorf("crossborder: API returned status=%s errors=%s", envelope.Status, string(envelope.Errors))
	}
	if err := json.Unmarshal(envelope.Data, v); err != nil {
		return fmt.Errorf("crossborder: decode data error: %w", err)
	}
	return nil
}

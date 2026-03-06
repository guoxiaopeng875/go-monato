package crossborder

import (
	"context"
	"fmt"
	"net/http"
)

// TransactionService handles cross-border money transfers.
//
// Supported flows:
//   - fiat↔fiat:   MXN↔USD, MXN↔EUR (via bank accounts)
//   - fiat→crypto: MXN → USDC/USDT (bank account → crypto wallet)
//   - crypto→fiat: USDC/USDT → MXN (crypto wallet → bank account)
type TransactionService struct {
	client *Client
}

// Transaction represents a cross-border transfer.
// Note: amounts and rate are strings to preserve decimal precision.
type Transaction struct {
	ID                  string        `json:"id"`
	SourceCurrency      string        `json:"source_currency"`         // MXN, USD, EUR, USDC, USDT
	TargetCurrency      string        `json:"target_currency"`         // MXN, USD, EUR, USDC, USDT
	SourceAmount        string        `json:"source_amount"`           // string for precision
	TargetAmount        string        `json:"target_amount"`           // string for precision
	State               string        `json:"state"`                   // created, pending, processing, completed, failed, cancelled
	Rate                string        `json:"rate"`                    // exchange rate as string
	SourceBankAccountID *string       `json:"source_bank_account_id"`  // nullable (null if crypto source)
	TargetBankAccountID *string       `json:"target_bank_account_id"`  // nullable (null if crypto target)
	MemoMessage         *string       `json:"memo_message"`            // reference for USD Wire / EUR SEPA
	Fees                *Fees         `json:"fees,omitempty"`
	TargetCryptoWallet  *CryptoWallet `json:"target_crypto_wallet"`    // nullable (set for fiat→crypto)
	SourceCryptoWallet  *CryptoWallet `json:"source_crypto_wallet"`    // nullable (set for crypto→fiat)
}

// CreateTransactionRequest initiates a cross-border transfer.
//
// For fiat→fiat or crypto→fiat: provide TargetBankAccountID.
// For fiat→crypto: provide TargetCryptoWallet instead.
//
// Include an Idempotency-Key via the idempotencyKey parameter to ensure safe retries.
// Same key + same body → returns original transaction.
// Same key + different body → returns HTTP 409.
type CreateTransactionRequest struct {
	// SourceCurrency: MXN, USD, EUR, USDC, USDT
	SourceCurrency string `json:"sourceCurrency,omitempty"`
	// TargetCurrency: MXN, USD, EUR, USDC, USDT
	TargetCurrency string `json:"targetCurrency,omitempty"`
	// SourceAmount in source currency.
	SourceAmount float64 `json:"sourceAmount,omitempty"`
	// TargetAmount (optional, calculated if not provided).
	TargetAmount float64 `json:"targetAmount,omitempty"`
	// TargetBankAccountID: existing bank account UUID for fiat recipients.
	TargetBankAccountID string `json:"targetBankAccountId,omitempty"`
	// TargetCryptoWallet: for fiat→crypto, specify the destination wallet.
	TargetCryptoWallet *CreateCryptoWallet `json:"targetCryptoWallet,omitempty"`
	// CustomerID: your internal customer identifier.
	CustomerID string `json:"customerId,omitempty"`
}

// CreateCryptoWallet specifies a crypto wallet destination in a transaction request.
type CreateCryptoWallet struct {
	// Address is the on-chain wallet address.
	Address string `json:"address"`
	// BlockchainSymbol: "POL" (Polygon) or "SOL" (Solana).
	BlockchainSymbol string `json:"blockchain_symbol"`
	// TokenSymbol: "USDC" or "USDT".
	TokenSymbol string `json:"token_symbol"`
}

// Create initiates a cross-border transfer.
//
// Fiat → Crypto example (MXN → USDC on Polygon):
//
//	txn, err := client.Transactions.Create(ctx, crossborder.CreateTransactionRequest{
//	    SourceCurrency: crossborder.CurrencyMXN,
//	    TargetCurrency: crossborder.CurrencyUSDC,
//	    SourceAmount:   10000,
//	    TargetCryptoWallet: &crossborder.CreateCryptoWallet{
//	        Address:          "0xabc123...",
//	        BlockchainSymbol: crossborder.BlockchainPolygon,
//	        TokenSymbol:      crossborder.CurrencyUSDC,
//	    },
//	    CustomerID: "user-12345",
//	}, "unique-idempotency-key")
//
// Crypto → Fiat example (USDT → MXN):
//
//	txn, err := client.Transactions.Create(ctx, crossborder.CreateTransactionRequest{
//	    SourceCurrency:      crossborder.CurrencyUSDT,
//	    TargetCurrency:      crossborder.CurrencyMXN,
//	    SourceAmount:        500,
//	    TargetBankAccountID: mxnAccountID,
//	    CustomerID:          "user-12345",
//	}, "unique-idempotency-key")
//
// Endpoint: POST /api/v1/transactions
func (s *TransactionService) Create(ctx context.Context, req CreateTransactionRequest, idempotencyKey string) (*Transaction, error) {
	resp, err := s.client.doRequestWithIdempotency(ctx, http.MethodPost, "/api/v1/transactions", req, idempotencyKey)
	if err != nil {
		return nil, err
	}
	var txn Transaction
	return &txn, decodeData(resp, &txn)
}

// Get retrieves a transaction by its UUID.
// Use this to poll for status updates: created → pending → processing → completed.
//
// Endpoint: GET /api/v1/transactions/{id}
func (s *TransactionService) Get(ctx context.Context, id string) (*Transaction, error) {
	path := fmt.Sprintf("/api/v1/transactions/%s", id)
	resp, err := s.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var txn Transaction
	return &txn, decodeData(resp, &txn)
}

// List returns all transactions for the authenticated user.
//
// Endpoint: GET /api/v1/transactions
func (s *TransactionService) List(ctx context.Context) ([]Transaction, error) {
	resp, err := s.client.doRequest(ctx, http.MethodGet, "/api/v1/transactions", nil)
	if err != nil {
		return nil, err
	}
	var txns []Transaction
	return txns, decodeData(resp, &txns)
}

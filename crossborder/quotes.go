package crossborder

import (
	"context"
	"fmt"
	"net/http"
)

// QuoteService generates real-time exchange rate quotes before executing transfers.
// Quotes lock in a rate for a limited window, allowing users to preview the
// conversion and fees before committing.
type QuoteService struct {
	client *Client
}

// Quote represents an exchange rate quote from the API.
// Note: amounts and rate are returned as strings to preserve decimal precision.
type Quote struct {
	QuoteID        string `json:"quote_id"`
	SourceCurrency string `json:"source_currency"` // MXN, USD, EUR, USDC, USDT
	TargetCurrency string `json:"target_currency"` // MXN, USD, EUR, USDC, USDT
	SourceAmount   string `json:"source_amount"`   // string for precision e.g. "1000.1234"
	TargetAmount   string `json:"target_amount"`   // string for precision e.g. "17443.2869"
	Rate           string `json:"rate"`            // exchange rate as string e.g. "17.44850400269681"
	Fees           *Fees  `json:"fees,omitempty"`
}

// CreateQuoteRequest generates a new exchange rate quote.
//
// Provide either SourceAmount OR TargetAmount (not both):
//   - SourceAmount: "I want to send X MXN, how much USDC will I get?"
//   - TargetAmount: "I want to receive Y USDC, how much MXN do I need?"
type CreateQuoteRequest struct {
	// SourceCurrency: MXN, USD, EUR, USDC, USDT
	SourceCurrency string `json:"sourceCurrency"`
	// TargetCurrency: MXN, USD, EUR, USDC, USDT
	TargetCurrency string `json:"targetCurrency"`
	// SourceAmount: amount in source currency to convert.
	SourceAmount float64 `json:"sourceAmount,omitempty"`
	// TargetAmount: desired amount in target currency.
	TargetAmount float64 `json:"targetAmount,omitempty"`
	// BlockchainSymbol: required for crypto quotes. "POL" (Polygon) or "SOL" (Solana).
	BlockchainSymbol string `json:"blockchain_symbol,omitempty"`
}

// Create generates a new exchange rate quote.
//
//	// Example: How much USDC (Polygon) do I get for 10,000 MXN?
//	quote, err := client.Quotes.Create(ctx, crossborder.CreateQuoteRequest{
//	    SourceCurrency:   crossborder.CurrencyMXN,
//	    TargetCurrency:   crossborder.CurrencyUSDC,
//	    SourceAmount:     10000,
//	    BlockchainSymbol: crossborder.BlockchainPolygon,
//	})
//	// quote.TargetAmount = "490.12" (string), quote.Rate = "0.049012..."
//
// Endpoint: POST /api/v1/quotes
func (s *QuoteService) Create(ctx context.Context, req CreateQuoteRequest) (*Quote, error) {
	resp, err := s.client.doRequest(ctx, http.MethodPost, "/api/v1/quotes", req)
	if err != nil {
		return nil, err
	}
	var quote Quote
	return &quote, decodeData(resp, &quote)
}

// Get retrieves a quote by its ID.
//
// Endpoint: GET /api/v1/quotes/{id}
func (s *QuoteService) Get(ctx context.Context, id string) (*Quote, error) {
	path := fmt.Sprintf("/api/v1/quotes/%s", id)
	resp, err := s.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var quote Quote
	return &quote, decodeData(resp, &quote)
}

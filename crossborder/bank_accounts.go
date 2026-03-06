package crossborder

import (
	"context"
	"fmt"
	"net/http"
)

// BankAccountService manages recipient bank accounts (MXN, USD, EUR)
// for cross-border transfers.
//
// Note: Crypto wallets (USDC/USDT) are specified directly in the
// transaction request, not registered as bank accounts.
type BankAccountService struct {
	client *Client
}

// Address represents a physical address in API responses (snake_case).
type Address struct {
	Street     string `json:"street"`
	City       string `json:"city"`
	State      string `json:"state"`
	PostalCode string `json:"postal_code"`
	Country    string `json:"country,omitempty"`
}

// BankAccount represents a registered recipient bank account.
type BankAccount struct {
	ID                string   `json:"id"`
	Currency          string   `json:"currency"`           // MXN, USD, EUR
	AccountHolder     string   `json:"account_holder"`
	CountryCode       string   `json:"country_code"`
	AccountHolderType string   `json:"account_holder_type"` // INDIVIDUAL, BUSINESS
	FirstName         string   `json:"first_name"`
	LastName          string   `json:"last_name"`
	BankName          string   `json:"bank_name"`
	BIC               *string  `json:"bic"`           // SWIFT code (nullable)
	IBAN              *string  `json:"iban"`           // (nullable)
	CLABENumber       *string  `json:"clabe_number"`   // (nullable)
	BankID            *string  `json:"bank_id"`        // (nullable)
	RFC               *string  `json:"rfc"`            // Mexican tax ID (nullable)
	AccountNumber     *string  `json:"account_number"` // (nullable)
	RoutingNumber     *string  `json:"routing_number"` // (nullable)
	Address           *Address `json:"address"`        // (nullable)
}

// RequestAddress represents a physical address in API requests (camelCase).
type RequestAddress struct {
	StreetLine1 string `json:"streetLine1"`
	City        string `json:"city"`
	State       string `json:"state"`
	PostalCode  string `json:"postalCode"`
	Country     string `json:"country"`
}

// CreateBankAccountRequest creates a new recipient bank account.
//
// Required fields depend on currency:
//   - MXN: clabeNumber, bankId, rfc (optional)
//   - USD: accountNumber, routingNumber, address
//   - EUR: iban, bic (optional), address
//
// accountHolderType = "INDIVIDUAL" requires firstName + lastName.
// accountHolderType = "BUSINESS" requires accountHolder (company name).
type CreateBankAccountRequest struct {
	Currency          string          `json:"currency"`                    // MXN, USD, EUR
	CountryCode       string          `json:"countryCode"`                 // ISO country code
	AccountHolderType string          `json:"accountHolderType"`           // INDIVIDUAL or BUSINESS
	FirstName         string          `json:"firstName,omitempty"`         // required if INDIVIDUAL
	LastName          string          `json:"lastName,omitempty"`          // required if INDIVIDUAL
	AccountHolder     string          `json:"accountHolder,omitempty"`     // required if BUSINESS (company name)
	CLABENumber       string          `json:"clabeNumber,omitempty"`       // 18-digit CLABE (MXN)
	BankID            string          `json:"bankId,omitempty"`            // bank UUID from /api/v1/banks (MXN)
	RFC               string          `json:"rfc,omitempty"`               // Mexican tax ID
	AccountNumber     string          `json:"accountNumber,omitempty"`     // bank account number (USD)
	RoutingNumber     string          `json:"routingNumber,omitempty"`     // WIRE routing number (USD)
	BankName          string          `json:"bankName,omitempty"`          // bank name (USD/EUR)
	IBAN              string          `json:"iban,omitempty"`              // IBAN (EUR)
	BIC               string          `json:"bic,omitempty"`               // SWIFT/BIC code (EUR)
	Address           *RequestAddress `json:"address,omitempty"`           // required for USD and EUR
}

// Create registers a new recipient bank account.
//
// Endpoint: POST /api/v1/bank_accounts
func (s *BankAccountService) Create(ctx context.Context, req CreateBankAccountRequest) (*BankAccount, error) {
	resp, err := s.client.doRequest(ctx, http.MethodPost, "/api/v1/bank_accounts", req)
	if err != nil {
		return nil, err
	}
	var account BankAccount
	return &account, decodeData(resp, &account)
}

// Get retrieves a bank account by its UUID.
//
// Endpoint: GET /api/v1/bank_accounts/{id}
func (s *BankAccountService) Get(ctx context.Context, id string) (*BankAccount, error) {
	path := fmt.Sprintf("/api/v1/bank_accounts/%s", id)
	resp, err := s.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var account BankAccount
	return &account, decodeData(resp, &account)
}

// List returns all bank accounts for the authenticated user.
//
// Endpoint: GET /api/v1/bank_accounts
func (s *BankAccountService) List(ctx context.Context) ([]BankAccount, error) {
	resp, err := s.client.doRequest(ctx, http.MethodGet, "/api/v1/bank_accounts", nil)
	if err != nil {
		return nil, err
	}
	var accounts []BankAccount
	return accounts, decodeData(resp, &accounts)
}

// --- Convenience constructors ---

// NewMXNAccount creates a request for a Mexican bank account (SPEI).
// Get the bankID from client.Banks.List().
func NewMXNAccount(firstName, lastName, clabe, bankID string) CreateBankAccountRequest {
	return CreateBankAccountRequest{
		Currency:          CurrencyMXN,
		CountryCode:       "MX",
		AccountHolderType: "INDIVIDUAL",
		FirstName:         firstName,
		LastName:          lastName,
		CLABENumber:       clabe,
		BankID:            bankID,
	}
}

// NewUSDAccount creates a request for a US bank account (Wire).
func NewUSDAccount(firstName, lastName, accountNumber, routingNumber string, addr RequestAddress) CreateBankAccountRequest {
	return CreateBankAccountRequest{
		Currency:          CurrencyUSD,
		CountryCode:       "US",
		AccountHolderType: "INDIVIDUAL",
		FirstName:         firstName,
		LastName:          lastName,
		AccountNumber:     accountNumber,
		RoutingNumber:     routingNumber,
		Address:           &addr,
	}
}

// NewEURAccount creates a request for a European SEPA bank account.
func NewEURAccount(firstName, lastName, iban, bic, bankName string, addr RequestAddress) CreateBankAccountRequest {
	return CreateBankAccountRequest{
		Currency:          CurrencyEUR,
		CountryCode:       addr.Country,
		AccountHolderType: "INDIVIDUAL",
		FirstName:         firstName,
		LastName:          lastName,
		IBAN:              iban,
		BIC:               bic,
		BankName:          bankName,
		Address:           &addr,
	}
}

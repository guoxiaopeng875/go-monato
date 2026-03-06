package crossborder

import (
	"context"
	"fmt"
	"net/http"
)

// BankService provides access to the Mexican bank catalog.
// Use this to look up bank IDs required when creating MXN bank accounts.
type BankService struct {
	client *Client
}

// Bank represents a Mexican bank in the catalog.
type Bank struct {
	ID         string `json:"id"`          // UUID
	Name       string `json:"name"`        // e.g. "BBVA MEXICO"
	BankCode   string `json:"bank_code"`   // 5-digit code e.g. "40012"
	BankStatus string `json:"bank_status"` // "active" or "inactive"
}

// List retrieves the complete catalog of Mexican banks.
// Use bank IDs from this list when creating MXN recipient accounts.
//
// Endpoint: GET /api/v1/banks
func (s *BankService) List(ctx context.Context) ([]Bank, error) {
	resp, err := s.client.doRequest(ctx, http.MethodGet, "/api/v1/banks", nil)
	if err != nil {
		return nil, err
	}
	var banks []Bank
	return banks, decodeData(resp, &banks)
}

// Get retrieves a specific bank by its UUID or numeric bank code.
//
// Endpoint: GET /api/v1/banks/{id}
func (s *BankService) Get(ctx context.Context, id string) (*Bank, error) {
	path := fmt.Sprintf("/api/v1/banks/%s", id)
	resp, err := s.client.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var bank Bank
	return &bank, decodeData(resp, &bank)
}

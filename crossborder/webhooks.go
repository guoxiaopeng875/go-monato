package crossborder

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"
)

// WebhookService manages webhook endpoints for real-time transaction notifications.
//
// Transaction lifecycle events: CREATED → WAITING_FUNDS → FUNDED → PROCESSING → COMPLETED / FAILED
//
// Sandbox tip: create a transaction with targetAmount = 1001.01 to trigger
// the full notification sequence (CREATED, WAITING_FUNDS, FUNDED, PROCESSING, COMPLETED).
type WebhookService struct {
	client *Client
}

// Webhook represents a registered webhook endpoint.
type Webhook struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Secret string `json:"secret"`
	Active bool   `json:"active"`
}

// CreateWebhookRequest registers a new webhook endpoint.
type CreateWebhookRequest struct {
	// URL is the HTTPS endpoint where events will be delivered.
	URL string `json:"url"`
	// Secret is used to sign requests via HMAC SHA256 for verification.
	Secret string `json:"secret"`
}

// WebhookNotification represents the payload delivered to your webhook endpoint.
type WebhookNotification struct {
	ID        string              `json:"id"`        // unique message ID (use for deduplication)
	Type      string              `json:"type"`      // "TRANSACTION"
	Timestamp string              `json:"timestamp"` // ISO 8601
	Body      WebhookNotificationBody `json:"body"`
}

// WebhookNotificationBody contains the event details.
type WebhookNotificationBody struct {
	Event         string `json:"event"`          // CREATED, WAITING_FUNDS, FUNDED, PROCESSING, COMPLETED, FAILED
	TransactionID string `json:"transaction_id"`
}

// Create registers a new webhook endpoint.
// Events will be delivered immediately after creation.
//
// Endpoint: POST /api/v1/webhooks
func (s *WebhookService) Create(ctx context.Context, req CreateWebhookRequest) (*Webhook, error) {
	resp, err := s.client.doRequest(ctx, http.MethodPost, "/api/v1/webhooks", req)
	if err != nil {
		return nil, err
	}
	var webhook Webhook
	return &webhook, decodeData(resp, &webhook)
}

// List retrieves all registered webhook endpoints.
//
// Endpoint: GET /api/v1/webhooks
func (s *WebhookService) List(ctx context.Context) ([]Webhook, error) {
	resp, err := s.client.doRequest(ctx, http.MethodGet, "/api/v1/webhooks", nil)
	if err != nil {
		return nil, err
	}
	var webhooks []Webhook
	return webhooks, decodeData(resp, &webhooks)
}

// Delete removes a webhook endpoint. No further events will be delivered.
//
// Endpoint: DELETE /api/v1/webhooks/{id}
func (s *WebhookService) Delete(ctx context.Context, id string) error {
	path := fmt.Sprintf("/api/v1/webhooks/%s", id)
	resp, err := s.client.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// --- Webhook Signature Verification ---

// VerifySignature verifies the HMAC SHA256 signature of a webhook request.
//
// Extract these from the incoming HTTP request:
//   - signature: X-Webhook-Signature header
//   - rawBody: the raw request body bytes (before JSON parsing)
//   - secret: your webhook secret (from webhook registration)
//
// Returns true if the signature is valid.
//
//	func webhookHandler(w http.ResponseWriter, r *http.Request) {
//	    body, _ := io.ReadAll(r.Body)
//	    sig := r.Header.Get("X-Webhook-Signature")
//	    if !crossborder.VerifySignature(sig, body, "your-webhook-secret") {
//	        http.Error(w, "invalid signature", http.StatusUnauthorized)
//	        return
//	    }
//	    // process the notification...
//	}
func VerifySignature(signature string, rawBody []byte, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(rawBody)
	expected := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1
}

// VerifyTimestamp checks that the webhook timestamp is within the allowed window.
// Recommended: reject requests older than 5 minutes to prevent replay attacks.
//
//	ts := r.Header.Get("X-Webhook-Timestamp")
//	if !crossborder.VerifyTimestamp(ts, 5*time.Minute) {
//	    http.Error(w, "request too old", http.StatusRequestTimeout)
//	    return
//	}
func VerifyTimestamp(timestamp string, maxAge time.Duration) bool {
	t, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return false
	}
	return time.Since(t) <= maxAge
}

package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/ponti/instant-payment-gateway/internal/model"
)

// WebhookService dispatches webhook notifications asynchronously.
type WebhookService struct {
	url    string
	client *http.Client
}

func NewWebhookService(url string) *WebhookService {
	return &WebhookService{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Dispatch sends a webhook payload asynchronously.
func (w *WebhookService) Dispatch(payload model.WebhookPayload) {
	if w.url == "" {
		return
	}
	go func() {
		body, err := json.Marshal(payload)
		if err != nil {
			log.Error().Err(err).Msg("failed to marshal webhook payload")
			return
		}

		resp, err := w.client.Post(w.url, "application/json", bytes.NewReader(body))
		if err != nil {
			log.Error().Err(err).Str("url", w.url).Msg("webhook delivery failed")
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode >= 400 {
			log.Warn().Int("status", resp.StatusCode).Str("url", w.url).Msg("webhook endpoint returned error")
		} else {
			log.Info().Str("event", payload.Event).Msg("webhook delivered")
		}
	}()
}

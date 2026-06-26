package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/buktio/buktio/internal/authz"
)

// Object event names delivered to webhooks.
const (
	EventObjectCreated = "object.created"
	EventObjectDeleted = "object.deleted"
)

var validWebhookEvents = map[string]bool{
	EventObjectCreated: true,
	EventObjectDeleted: true,
}

// webhookClient delivers webhook callbacks. Short timeout — a slow receiver must
// not tie up a delivery worker.
var webhookClient = &http.Client{Timeout: 10 * time.Second}

// WebhookDTO is a subscription as the API returns it (the secret is never echoed).
type WebhookDTO struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	HasSecret bool      `json:"has_secret"`
	CreatedAt time.Time `json:"created_at"`
}

// ListWebhooks returns the bucket's webhook subscriptions.
func (s *Services) ListWebhooks(ctx context.Context, bucketID string) ([]WebhookDTO, error) {
	if _, _, err := s.bucketProvider(ctx, bucketID); err != nil {
		return nil, err
	}
	rows, err := s.Store.ListWebhooksByBucket(ctx, bucketID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]WebhookDTO, 0, len(rows))
	for _, w := range rows {
		out = append(out, WebhookDTO{
			ID: w.ID, URL: w.URL, Events: splitEvents(w.Events),
			Enabled: w.Enabled, HasSecret: w.Secret != "", CreatedAt: w.CreatedAt,
		})
	}
	return out, nil
}

// CreateWebhook adds a subscription. Managing webhooks is an update on the bucket
// (owner/admin under Pro RBAC; everyone under the OSS permit-all authorizer).
func (s *Services) CreateWebhook(ctx context.Context, bucketID, url string, events []string, secret string) (*WebhookDTO, error) {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBucket}); err != nil {
		return nil, err
	}
	if _, _, err := s.bucketProvider(ctx, bucketID); err != nil {
		return nil, err
	}
	url = strings.TrimSpace(url)
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, validationErr("url must be an http(s) URL")
	}
	clean := make([]string, 0, len(events))
	for _, e := range events {
		if !validWebhookEvents[e] {
			return nil, validationErr("unknown event: " + e)
		}
		clean = append(clean, e)
	}
	if len(clean) == 0 {
		return nil, validationErr("select at least one event")
	}
	w, err := s.Store.CreateWebhook(ctx, s.tenant(ctx).OrgID, bucketID, url, secret, strings.Join(clean, ","))
	if err != nil {
		return nil, mapRepoErr(err)
	}
	s.audit(ctx, "webhook.create", "bucket", bucketID, map[string]any{"url": url, "events": clean})
	return &WebhookDTO{
		ID: w.ID, URL: w.URL, Events: clean, Enabled: w.Enabled,
		HasSecret: w.Secret != "", CreatedAt: w.CreatedAt,
	}, nil
}

// DeleteWebhook removes a subscription.
func (s *Services) DeleteWebhook(ctx context.Context, bucketID, id string) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBucket}); err != nil {
		return err
	}
	if err := s.Store.DeleteWebhook(ctx, id, s.tenant(ctx).OrgID); err != nil {
		return mapRepoErr(err)
	}
	s.audit(ctx, "webhook.delete", "bucket", bucketID, map[string]any{"id": id})
	return nil
}

// webhookPayload is the JSON body POSTed to subscribers.
type webhookPayload struct {
	Event    string    `json:"event"`
	BucketID string    `json:"bucket_id"`
	Key      string    `json:"key"`
	Time     time.Time `json:"time"`
}

// fireWebhook dispatches an object event to matching subscriptions. It is
// fire-and-forget: the subscription lookup and HTTP delivery run on background
// goroutines so the object operation isn't blocked or failed by a slow/broken
// receiver. Best-effort (not durable across a restart).
func (s *Services) fireWebhook(bucketID, event, key string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		subs, err := s.Store.ListEnabledWebhooksByBucket(ctx, bucketID)
		cancel()
		if err != nil || len(subs) == 0 {
			return
		}
		body, err := json.Marshal(webhookPayload{Event: event, BucketID: bucketID, Key: key, Time: time.Now().UTC()})
		if err != nil {
			return
		}
		for _, sub := range subs {
			if !eventMatches(sub.Events, event) {
				continue
			}
			go s.deliverWebhook(sub.URL, sub.Secret, event, body)
		}
	}()
}

// deliverWebhook POSTs the payload with up to 3 attempts and exponential backoff.
// When a secret is set, the body is HMAC-SHA256 signed in X-Buktio-Signature.
func (s *Services) deliverWebhook(url, secret, event string, body []byte) {
	var sig string
	if secret != "" {
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		sig = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}
	backoff := time.Second
	for attempt := 1; attempt <= 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			cancel()
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "buktio-webhook")
		req.Header.Set("X-Buktio-Event", event)
		if sig != "" {
			req.Header.Set("X-Buktio-Signature", sig)
		}
		resp, err := webhookClient.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			cancel()
			if resp.StatusCode < 300 {
				return
			}
		} else {
			cancel()
		}
		if attempt < 3 {
			time.Sleep(backoff)
			backoff *= 3
		}
	}
	if s.Logger != nil {
		s.Logger.Warn("webhook delivery failed after retries", slog.String("url", url), slog.String("event", event))
	}
}

func splitEvents(csv string) []string {
	if csv == "" {
		return []string{}
	}
	return strings.Split(csv, ",")
}

func eventMatches(csv, event string) bool {
	for _, e := range strings.Split(csv, ",") {
		if e == event {
			return true
		}
	}
	return false
}

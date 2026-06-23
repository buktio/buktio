// Package garage implements the StorageProvider interface against Garage,
// composing two sub-clients: the Admin API v2 client (port 3903, bearer token)
// in this file, and an S3 client (port 3900, SigV4) in s3_client.go.
//
// NOTE: request/response field names below follow the Garage Admin API v2 docs
// and must be confirmed against the live garage-admin-v2.json before relying on
// them against a real cluster (the plan flags this explicitly). The HTTP plumbing
// and bootstrap logic are exercised by unit tests using an httptest fake.
package garage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AdminClient talks to the Garage Admin API v2 (default :3903). All calls use
// flat v2 paths (e.g. POST /v2/CreateBucket) authenticated with a bearer token.
type AdminClient struct {
	baseURL string
	token   string
	http    *http.Client
}

// NewAdminClient builds an Admin API v2 client.
func NewAdminClient(baseURL, token string) *AdminClient {
	return &AdminClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// Health performs the cheap, unauthenticated liveness probe: GET /health returns
// 200 when the cluster has quorum, 503 otherwise.
func (c *AdminClient) Health(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("garage admin /health: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("garage admin /health: status %d", resp.StatusCode)
	}
	return nil
}

// doJSON performs an authenticated v2 call: it marshals reqBody (if non-nil),
// sets the bearer token, and decodes a JSON response into respOut (if non-nil).
func (c *AdminClient) doJSON(ctx context.Context, method, path string, reqBody, respOut any) error {
	var body io.Reader
	if reqBody != nil {
		b, err := json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("garage admin: marshal %s: %w", path, err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("garage admin %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return &AdminError{Method: method, Path: path, Status: resp.StatusCode, Body: string(snippet)}
	}
	if respOut != nil {
		if err := json.NewDecoder(resp.Body).Decode(respOut); err != nil {
			return fmt.Errorf("garage admin %s %s: decode: %w", method, path, err)
		}
	}
	return nil
}

// AdminError is a non-2xx Garage admin response.
type AdminError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *AdminError) Error() string {
	return fmt.Sprintf("garage admin %s %s: status %d: %s", e.Method, e.Path, e.Status, e.Body)
}

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// GarageMetrics proxies the storage engine's Prometheus /metrics (cluster/node-
// global ops metrics only — never billing). Authenticated with the metrics token.
func (s *Services) GarageMetrics(ctx context.Context) ([]byte, error) {
	if s.GarageAdminURL == "" {
		return nil, storageUnavailableErr("no storage admin endpoint configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.GarageAdminURL+"/metrics", nil)
	if err != nil {
		return nil, storageUnavailableErr(err.Error())
	}
	if s.MetricsToken != "" {
		req.Header.Set("Authorization", "Bearer "+s.MetricsToken)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, storageUnavailableErr("metrics unreachable: " + err.Error())
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, storageUnavailableErr(fmt.Sprintf("metrics status %d", resp.StatusCode))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 5<<20))
}

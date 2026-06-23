package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"
)

// apiRequest calls the buktio API with a Bearer PAT (BUKTIO_TOKEN). Cluster
// management is an authenticated operation, so a token is required.
func apiRequest(method, path string, body any) ([]byte, error) {
	base := os.Getenv("BUKTIO_API_URL")
	if base == "" {
		base = "http://localhost:8080"
	}
	token := os.Getenv("BUKTIO_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("set BUKTIO_TOKEN to a buktio API token (PAT) to use cluster commands")
	}
	var rdr io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, base+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API %s %s: %s", method, path, string(data))
	}
	return data, nil
}

func clusterCmd() *cobra.Command {
	c := &cobra.Command{Use: "cluster", Short: "Manage storage backends"}
	c.AddCommand(clusterListCmd(), clusterAddCmd())
	return c
}

func clusterListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List storage backends",
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := apiRequest(http.MethodGet, "/api/v1/clusters", nil)
			if err != nil {
				return err
			}
			var out struct {
				Data []struct {
					Name      string `json:"name"`
					Provider  string `json:"provider"`
					Mode      string `json:"mode"`
					Status    string `json:"status"`
					IsPrimary bool   `json:"is_primary"`
				} `json:"data"`
			}
			if err := json.Unmarshal(data, &out); err != nil {
				return err
			}
			fmt.Printf("%-20s %-12s %-10s %-10s %s\n", "NAME", "PROVIDER", "MODE", "STATUS", "PRIMARY")
			for _, c := range out.Data {
				primary := ""
				if c.IsPrimary {
					primary = "yes"
				}
				fmt.Printf("%-20s %-12s %-10s %-10s %s\n", c.Name, c.Provider, c.Mode, c.Status, primary)
			}
			return nil
		},
	}
}

func clusterAddCmd() *cobra.Command {
	var name, provider, endpoint, region, accessKey, secret string
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Connect an external generic-S3 backend (R2/AWS S3/B2/SeaweedFS/Ceph)",
		RunE: func(_ *cobra.Command, _ []string) error {
			body := map[string]string{
				"name": name, "provider": provider, "s3_endpoint": endpoint,
				"s3_region": region, "access_key_id": accessKey, "secret_access_key": secret,
			}
			data, err := apiRequest(http.MethodPost, "/api/v1/clusters", body)
			if err != nil {
				return err
			}
			var c struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			_ = json.Unmarshal(data, &c)
			fmt.Printf("Added backend %q (%s)\n", c.Name, c.ID)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&name, "name", "", "display name")
	f.StringVar(&provider, "provider", "", "aws_s3|r2|b2|seaweedfs|ceph_rgw")
	f.StringVar(&endpoint, "endpoint", "", "S3 endpoint URL")
	f.StringVar(&region, "region", "", "S3 region (default per provider)")
	f.StringVar(&accessKey, "access-key", "", "access key id")
	f.StringVar(&secret, "secret", "", "secret access key")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("provider")
	_ = cmd.MarkFlagRequired("access-key")
	_ = cmd.MarkFlagRequired("secret")
	return cmd
}

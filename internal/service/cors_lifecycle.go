package service

import (
	"context"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/storage"
)

// CORSRuleDTO is a bucket CORS rule as the API exchanges it.
type CORSRuleDTO struct {
	AllowedOrigins []string `json:"allowed_origins"`
	AllowedMethods []string `json:"allowed_methods"`
	AllowedHeaders []string `json:"allowed_headers"`
	ExposeHeaders  []string `json:"expose_headers"`
	MaxAgeSeconds  int      `json:"max_age_seconds"`
}

// LifecycleRuleDTO is a bucket lifecycle rule (Garage: expiry + abort-incomplete-MPU).
type LifecycleRuleDTO struct {
	ID                     string `json:"id,omitempty"`
	Prefix                 string `json:"prefix"`
	Enabled                bool   `json:"enabled"`
	ExpireDays             int    `json:"expire_days"`
	AbortIncompleteMPUDays int    `json:"abort_incomplete_mpu_days"`
}

// resolveAlias returns the bucket's S3 alias and the provider for the cluster it
// lives on (routing CORS/lifecycle ops to the correct backend).
func (s *Services) resolveAlias(ctx context.Context, bucketID string) (string, storage.StorageProvider, error) {
	b, serr := s.loadBucket(ctx, bucketID) // tenant-scoped
	if serr != nil {
		return "", nil, serr
	}
	prov, perr := s.providerForBucket(ctx, b)
	if perr != nil {
		return "", nil, storageUnavailableErr("cannot reach the bucket's cluster: " + perr.Error())
	}
	return b.GarageGlobalAlias, prov, nil
}

// --- CORS ---

func (s *Services) GetBucketCORS(ctx context.Context, bucketID string) ([]CORSRuleDTO, error) {
	alias, prov, err := s.resolveAlias(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	rules, gerr := prov.GetCORS(ctx, alias)
	if gerr != nil {
		return nil, mapStorageErr(gerr)
	}
	out := make([]CORSRuleDTO, 0, len(rules))
	for _, r := range rules {
		out = append(out, CORSRuleDTO{
			AllowedOrigins: r.AllowedOrigins, AllowedMethods: r.AllowedMethods,
			AllowedHeaders: r.AllowedHeaders, ExposeHeaders: r.ExposeHeaders, MaxAgeSeconds: r.MaxAgeSeconds,
		})
	}
	return out, nil
}

func (s *Services) SetBucketCORS(ctx context.Context, bucketID string, rules []CORSRuleDTO) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBucket, ID: bucketID}); err != nil {
		return err
	}
	alias, prov, err := s.resolveAlias(ctx, bucketID)
	if err != nil {
		return err
	}
	srules := make([]storage.CORSRule, 0, len(rules))
	for _, r := range rules {
		if len(r.AllowedOrigins) == 0 || len(r.AllowedMethods) == 0 {
			return validationErr("each CORS rule needs at least one allowed origin and method")
		}
		srules = append(srules, storage.CORSRule{
			AllowedOrigins: r.AllowedOrigins, AllowedMethods: r.AllowedMethods,
			AllowedHeaders: r.AllowedHeaders, ExposeHeaders: r.ExposeHeaders, MaxAgeSeconds: r.MaxAgeSeconds,
		})
	}
	if gerr := prov.SetCORS(ctx, alias, srules); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "bucket.set_cors", "bucket", bucketID, map[string]any{"rules": len(srules)})
	return nil
}

func (s *Services) DeleteBucketCORS(ctx context.Context, bucketID string) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBucket, ID: bucketID}); err != nil {
		return err
	}
	alias, prov, err := s.resolveAlias(ctx, bucketID)
	if err != nil {
		return err
	}
	if gerr := prov.DeleteCORS(ctx, alias); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "bucket.delete_cors", "bucket", bucketID, nil)
	return nil
}

// --- Lifecycle ---

func (s *Services) GetBucketLifecycle(ctx context.Context, bucketID string) ([]LifecycleRuleDTO, error) {
	alias, prov, err := s.resolveAlias(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	rules, gerr := prov.GetLifecycle(ctx, alias)
	if gerr != nil {
		return nil, mapStorageErr(gerr)
	}
	out := make([]LifecycleRuleDTO, 0, len(rules))
	for _, r := range rules {
		out = append(out, LifecycleRuleDTO{
			ID: r.ID, Prefix: r.Prefix, Enabled: r.Enabled,
			ExpireDays: r.ExpireDays, AbortIncompleteMPUDays: r.AbortIncompleteMPUDays,
		})
	}
	return out, nil
}

func (s *Services) SetBucketLifecycle(ctx context.Context, bucketID string, rules []LifecycleRuleDTO) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBucket, ID: bucketID}); err != nil {
		return err
	}
	alias, prov, err := s.resolveAlias(ctx, bucketID)
	if err != nil {
		return err
	}
	srules := make([]storage.LifecycleRule, 0, len(rules))
	for _, r := range rules {
		if r.ExpireDays <= 0 && r.AbortIncompleteMPUDays <= 0 {
			return validationErr("each lifecycle rule needs an expiry or abort-incomplete-MPU day count")
		}
		srules = append(srules, storage.LifecycleRule{
			ID: r.ID, Prefix: r.Prefix, Enabled: r.Enabled,
			ExpireDays: r.ExpireDays, AbortIncompleteMPUDays: r.AbortIncompleteMPUDays,
		})
	}
	if gerr := prov.SetLifecycle(ctx, alias, srules); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "bucket.set_lifecycle", "bucket", bucketID, map[string]any{"rules": len(srules)})
	return nil
}

func (s *Services) DeleteBucketLifecycle(ctx context.Context, bucketID string) error {
	if err := s.authorize(ctx, authz.ActionUpdate, authz.Target{Kind: authz.ResourceBucket, ID: bucketID}); err != nil {
		return err
	}
	alias, prov, err := s.resolveAlias(ctx, bucketID)
	if err != nil {
		return err
	}
	if gerr := prov.DeleteLifecycle(ctx, alias); gerr != nil {
		return mapStorageErr(gerr)
	}
	s.audit(ctx, "bucket.delete_lifecycle", "bucket", bucketID, nil)
	return nil
}

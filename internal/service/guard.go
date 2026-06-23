package service

import (
	"context"
	"net/http"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/entitlements"
	"github.com/buktio/buktio/internal/repository"
)

// authorize checks whether the request's subject may perform action on target.
// In OSS s.Authz is PermitAll, so this is a no-op; paid editions inject an
// enforcing authorizer. Returns a 403 *Error on deny.
func (s *Services) authorize(ctx context.Context, action authz.Action, target authz.Target) *Error {
	subj, _ := authz.SubjectFrom(ctx)
	if d := s.Authz.Can(subj, action, target); !d.Allowed {
		return &Error{Code: "forbidden", Message: forbiddenMsg(d.Reason), HTTP: http.StatusForbidden}
	}
	return nil
}

func forbiddenMsg(reason string) string {
	if reason != "" {
		return reason
	}
	return "you do not have permission to perform this action"
}

// loadBucket loads a bucket by id scoped to the active tenant's project. A bucket
// in another tenant returns not-found (no cross-tenant existence leak). This is the
// choke point that prevents cross-org IDOR on every by-id bucket operation.
func (s *Services) loadBucket(ctx context.Context, id string) (*repository.Bucket, *Error) {
	b, err := s.Store.GetBucket(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if b.ProjectID != s.tenant(ctx).ProjectID {
		return nil, notFoundErr()
	}
	return b, nil
}

// quotaGuard rejects an operation that would push the active org over its storage
// ceiling. The ceiling is the tighter of the per-org column (organizations.
// quota_max_bytes) and the license entitlement (ResourceStorageGB). addBytes is
// the bytes the operation will add (0 for bucket-create — which then only gates
// when the org is already at/over the cap). It is a soft, buktio-enforced check on
// the create + API-proxied-upload paths; Garage's per-bucket maxSize is the hard,
// write-time enforcer that also covers presigned uploads. Returns a 402 *Error.
func (s *Services) quotaGuard(ctx context.Context, addBytes int64) *Error {
	orgID := s.tenant(ctx).OrgID
	if orgID == "" {
		return nil
	}
	capBytes := s.orgStorageCap(ctx, orgID)
	if capBytes < 0 {
		return nil // unlimited
	}
	used, _, _ := s.Store.OrgUsageTotals(ctx, orgID)
	if used+addBytes > capBytes {
		return &Error{
			Code:    "quota_exceeded",
			Message: "organization storage quota exceeded",
			HTTP:    http.StatusPaymentRequired,
		}
	}
	return nil
}

// orgStorageCap returns the effective org storage ceiling in bytes, or -1 for
// unlimited. It is the minimum of the per-org column and the license limit.
func (s *Services) orgStorageCap(ctx context.Context, orgID string) int64 {
	capBytes := int64(-1)
	if st, err := s.Store.GetOrgStatus(ctx, orgID); err == nil && st.QuotaMaxBytes != nil {
		capBytes = *st.QuotaMaxBytes
	}
	if lim := s.Ent.Limit(entitlements.ResourceStorageGB, entitlements.TenantID(orgID)); !lim.Unlimited {
		licBytes := lim.Max * 1024 * 1024 * 1024
		if capBytes < 0 || licBytes < capBytes {
			capBytes = licBytes
		}
	}
	return capBytes
}

// loadKey loads an access key by id scoped to the active tenant's project.
func (s *Services) loadKey(ctx context.Context, id string) (*repository.AccessKey, *Error) {
	k, err := s.Store.GetAccessKey(ctx, id)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	if k.ProjectID != s.tenant(ctx).ProjectID {
		return nil, notFoundErr()
	}
	return k, nil
}

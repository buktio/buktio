package garage

import (
	"time"

	"github.com/buktio/buktio/internal/storage"
)

// bucketFromInfo maps a Garage BucketInfoResponse to a storage.Bucket.
func bucketFromInfo(b *BucketInfoResponse) *storage.Bucket {
	out := &storage.Bucket{
		ID:            b.ID,
		GlobalAliases: b.GlobalAliases,
		Visibility:    storage.VisibilityPrivate,
		Quota: storage.Quota{
			MaxSizeBytes: b.Quotas.MaxSize,
			MaxObjects:   b.Quotas.MaxObjects,
		},
	}
	if b.WebsiteAccess {
		out.Visibility = storage.VisibilityPublic
		out.Website.Enabled = true
		if b.WebsiteConfig != nil {
			out.Website.IndexDocument = b.WebsiteConfig.IndexDocument
			out.Website.ErrorDocument = b.WebsiteConfig.ErrorDocument
		}
	}
	return out
}

// usageFromInfo maps a Garage BucketInfoResponse to a storage.BucketUsage,
// stamping the (eventually-consistent) capture time.
func usageFromInfo(b *BucketInfoResponse, capturedAt time.Time) *storage.BucketUsage {
	return &storage.BucketUsage{
		ObjectCount:                b.Objects,
		BytesUsed:                  b.Bytes,
		UnfinishedUploads:          b.UnfinishedUploads,
		UnfinishedMultipartUploads: b.UnfinishedMultipartUploads,
		UnfinishedMultipartParts:   b.UnfinishedMultipartUploadParts,
		UnfinishedMultipartBytes:   b.UnfinishedMultipartUploadBytes,
		QuotaMaxSize:               b.Quotas.MaxSize,
		QuotaMaxObjects:            b.Quotas.MaxObjects,
		CapturedAt:                 capturedAt,
	}
}

// keyPermsFromStorage maps storage.Permissions to Garage's read/write/owner bitmask.
func keyPermsFromStorage(p storage.Permissions) KeyPermissions {
	return KeyPermissions{Read: p.Read, Write: p.Write, Owner: p.Owner}
}

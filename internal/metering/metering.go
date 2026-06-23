// Package metering is the billing/usage-event seam.
//
// The Go API emits lifecycle and usage events to a MeteringSink. In OSS the sink
// is a no-op (no phone-home). The Hosted edition swaps in a sink that forwards to
// the billing pipeline. The usage_snapshots table is already provider-tagged and
// billing-grade, so enabling metering later needs no schema change.
package metering

import (
	"context"
	"time"
)

// EventType identifies a meterable lifecycle/usage event.
type EventType string

const (
	EventBucketCreated  EventType = "bucket.created"
	EventBucketDeleted  EventType = "bucket.deleted"
	EventKeyCreated     EventType = "key.created"
	EventKeyRevoked     EventType = "key.revoked"
	EventUsageSnapshot  EventType = "usage.snapshot"
	EventObjectUploaded EventType = "object.uploaded"
)

// Event is a single metering record.
type Event struct {
	Type     EventType
	TenantID string
	// Subject identifies the related resource (bucket id, key id, ...).
	Subject string
	// Quantity carries numeric payloads (e.g. bytes) when relevant.
	Quantity int64
	At       time.Time
	// Attributes carries provider-tagged extra dimensions.
	Attributes map[string]string
}

// Sink receives metering events. Implementations must be safe for concurrent use.
type Sink interface {
	Emit(ctx context.Context, ev Event) error
}

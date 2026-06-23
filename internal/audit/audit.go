// Package audit is the SIEM-forwarding seam. Every audit event written to the
// database is also fanned out to a Sink. In OSS the sink is a no-op (no
// phone-home); the Enterprise edition swaps in an ee/audit forwarder that ships
// events to a webhook / syslog / Splunk HEC. The database remains the source of
// record (and is tamper-evident via the hash chain); the sink is best-effort.
package audit

import "time"

// Event is a single audit record handed to a Sink.
type Event struct {
	OrgID       string         `json:"org_id,omitempty"`
	ActorUserID string         `json:"actor_user_id,omitempty"`
	ActorType   string         `json:"actor_type"`
	Action      string         `json:"action"`
	TargetType  string         `json:"target_type,omitempty"`
	TargetID    string         `json:"target_id,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	At          time.Time      `json:"at"`
}

// Sink receives audit events. Implementations must be safe for concurrent use and
// must not block the request path (buffer/drop or fan out asynchronously).
type Sink interface {
	Emit(Event)
}

// NoOp is the OSS sink: it discards events (no external forwarding).
type NoOp struct{}

func (NoOp) Emit(Event) {}

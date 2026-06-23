package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/buktio/buktio/internal/authz"
	"github.com/buktio/buktio/internal/repository"
)

// DashboardDTO is the dashboard aggregate.
type DashboardDTO struct {
	Cluster    ClusterHealthDTO `json:"cluster"`
	Capacity   CapacityDTO      `json:"capacity"`
	Totals     TotalsDTO        `json:"totals"`
	Versions   VersionsDTO      `json:"versions"`
	Alerts     []AlertDTO       `json:"alerts"`
	S3Endpoint string           `json:"s3_endpoint"`
}

type ClusterHealthDTO struct {
	Status     string `json:"status"`
	Mode       string `json:"mode"`
	NodesTotal int    `json:"nodes_total"`
	NodesOK    int    `json:"nodes_ok"`
}

type CapacityDTO struct {
	DiskTotalBytes int64   `json:"disk_total_bytes"`
	DiskAvailBytes int64   `json:"disk_avail_bytes"`
	UsedPct        float64 `json:"used_pct"`
}

type TotalsDTO struct {
	Buckets    int   `json:"buckets"`
	AccessKeys int   `json:"access_keys"`
	Objects    int64 `json:"objects"`
	BytesUsed  int64 `json:"bytes_used"`
}

type VersionsDTO struct {
	Buktio        string `json:"buktio"`
	StorageEngine string `json:"storage_engine"`
}

type AlertDTO struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Dashboard builds the dashboard aggregate (best-effort on cluster calls).
func (s *Services) Dashboard(ctx context.Context) (*DashboardDTO, error) {
	d := &DashboardDTO{
		S3Endpoint: s.S3PublicEndpoint,
		Versions:   VersionsDTO{Buktio: s.Version, StorageEngine: s.GarageVersion},
		Cluster:    ClusterHealthDTO{Status: "unavailable", Mode: s.ClusterMode},
		Alerts:     []AlertDTO{},
	}

	if h, err := s.Provider.GetClusterHealth(ctx); err == nil {
		d.Cluster = ClusterHealthDTO{Status: h.Status, Mode: s.ClusterMode, NodesTotal: h.KnownNodes, NodesOK: h.StorageNodesOK}
	}

	if st, err := s.Provider.GetClusterStatus(ctx); err == nil {
		var used, avail int64
		for _, n := range st.Nodes {
			used += n.DiskUsed
			avail += n.DiskAvail
		}
		total := used + avail
		d.Capacity = CapacityDTO{DiskTotalBytes: total, DiskAvailBytes: avail}
		if total > 0 {
			d.Capacity.UsedPct = float64(used) / float64(total) * 100
		}
	}

	buckets, _ := s.Store.CountBuckets(ctx, s.tenant(ctx).ProjectID)
	keys, _ := s.Store.CountAccessKeys(ctx, s.tenant(ctx).ProjectID)
	bytesUsed, objects, _ := s.Store.ProjectUsageTotals(ctx, s.tenant(ctx).ProjectID)
	d.Totals = TotalsDTO{Buckets: buckets, AccessKeys: keys, Objects: objects, BytesUsed: bytesUsed}

	d.Alerts = append(d.Alerts, AlertDTO{
		Level: "info", Code: "single_node_no_redundancy",
		Message: "Single-node deployment has no built-in data redundancy. Configure disk-level backups.",
	})
	return d, nil
}

// AuditDTO is an audit entry as the API returns it.
type AuditDTO struct {
	ID          int64          `json:"id"`
	Action      string         `json:"action"`
	ActorType   string         `json:"actor_type"`
	ActorUserID string         `json:"actor_user_id,omitempty"`
	TargetType  string         `json:"target_type"`
	TargetID    string         `json:"target_id"`
	Metadata    map[string]any `json:"metadata"`
	CreatedAt   time.Time      `json:"created_at"`
}

// AuditFilterInput parameterizes audit listing/export (Pro advanced audit).
type AuditFilterInput struct {
	From       time.Time
	To         time.Time
	Actor      string
	Action     string
	TargetType string
	Limit      int
	Offset     int
}

func (s *Services) auditEntries(ctx context.Context, in AuditFilterInput) ([]AuditDTO, error) {
	if err := s.authorize(ctx, authz.ActionRead, authz.Target{Kind: authz.ResourceAudit}); err != nil {
		return nil, err
	}
	// Validate the actor filter is a UUID so the ::uuid cast can't 500 on junk input.
	if in.Actor != "" {
		if _, perr := uuid.Parse(in.Actor); perr != nil {
			return nil, validationErr("actor must be a valid user id")
		}
	}
	entries, err := s.Store.ListAuditFiltered(ctx, repository.AuditFilter{
		OrgID: s.tenant(ctx).OrgID, // always scope to the active tenant
		Actor: in.Actor, Action: in.Action, TargetType: in.TargetType,
		From: in.From, To: in.To, Limit: in.Limit, Offset: in.Offset,
	})
	if err != nil {
		return nil, mapRepoErr(err)
	}
	out := make([]AuditDTO, 0, len(entries))
	for _, e := range entries {
		out = append(out, AuditDTO{
			ID: e.ID, Action: e.Action, ActorType: e.ActorType, ActorUserID: e.ActorUserID,
			TargetType: e.TargetType, TargetID: e.TargetID,
			Metadata: e.Metadata, CreatedAt: e.CreatedAt,
		})
	}
	return out, nil
}

// ListAudit returns filtered, org-scoped audit events.
func (s *Services) ListAudit(ctx context.Context, in AuditFilterInput) ([]AuditDTO, error) {
	return s.auditEntries(ctx, in)
}

// AuditVerifyDTO reports the audit hash-chain integrity check.
type AuditVerifyDTO struct {
	Checked   int    `json:"checked"`
	OK        bool   `json:"ok"`
	BrokenID  int64  `json:"broken_id,omitempty"`
	BrokenWhy string `json:"broken_why,omitempty"`
}

// VerifyAuditChain checks the tamper-evident hash chain for the active org. Read
// is RBAC-gated like the audit log itself.
func (s *Services) VerifyAuditChain(ctx context.Context) (*AuditVerifyDTO, error) {
	if err := s.authorize(ctx, authz.ActionRead, authz.Target{Kind: authz.ResourceAudit}); err != nil {
		return nil, err
	}
	res, err := s.Store.VerifyAuditChain(ctx, s.tenant(ctx).OrgID)
	if err != nil {
		return nil, mapRepoErr(err)
	}
	return &AuditVerifyDTO{Checked: res.Checked, OK: res.OK, BrokenID: res.BrokenID, BrokenWhy: res.BrokenWhy}, nil
}

// ExportAudit returns the filtered audit log as CSV or JSON (Pro advanced audit).
func (s *Services) ExportAudit(ctx context.Context, in AuditFilterInput, format string) ([]byte, string, error) {
	if in.Limit <= 0 {
		in.Limit = 5000 // export pulls a larger window
	}
	rows, err := s.auditEntries(ctx, in)
	if err != nil {
		return nil, "", err
	}
	if format == "json" {
		b, _ := json.Marshal(rows)
		return b, "application/json", nil
	}
	var buf bytes.Buffer
	cw := csv.NewWriter(&buf)
	_ = cw.Write([]string{"id", "created_at", "actor_type", "actor_user_id", "action", "target_type", "target_id"})
	for _, e := range rows {
		_ = cw.Write([]string{
			strconv.FormatInt(e.ID, 10), e.CreatedAt.UTC().Format(time.RFC3339),
			e.ActorType, e.ActorUserID, e.Action, e.TargetType, e.TargetID,
		})
	}
	cw.Flush()
	return buf.Bytes(), "text/csv", nil
}

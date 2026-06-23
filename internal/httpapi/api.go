package httpapi

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/buktio/buktio/internal/service"
)

// apiHandlers holds the dependencies for the /api/v1 product endpoints.
type apiHandlers struct {
	svc    *service.Services
	logger *slog.Logger
}

// register mounts the product endpoints on the given router.
func (h *apiHandlers) register(r chi.Router) {
	r.Route("/buckets", func(r chi.Router) {
		r.Get("/", h.listBuckets)
		r.Post("/", h.createBucket)
		r.Get("/{id}", h.getBucket)
		r.Patch("/{id}", h.patchBucket)
		r.Delete("/{id}", h.deleteBucket)
		r.Put("/{id}/access", h.setBucketAccess)
		r.Get("/{id}/usage", h.getBucketUsage)

		// CORS + lifecycle (v1.1).
		r.Get("/{id}/cors", h.getBucketCORS)
		r.Put("/{id}/cors", h.setBucketCORS)
		r.Delete("/{id}/cors", h.deleteBucketCORS)
		r.Get("/{id}/lifecycle", h.getBucketLifecycle)
		r.Put("/{id}/lifecycle", h.setBucketLifecycle)
		r.Delete("/{id}/lifecycle", h.deleteBucketLifecycle)

		// Object browser (M7).
		r.Get("/{id}/objects", h.listObjects)
		r.Delete("/{id}/objects", h.deleteObjects)
		r.Put("/{id}/objects/content", h.uploadObject)   // API-proxied upload
		r.Get("/{id}/objects/content", h.downloadObject) // API-proxied download
		r.Post("/{id}/objects/presign", h.presignObject) // direct (when S3 is reachable)
		r.Post("/{id}/objects/copy", h.copyObject)       // v1.1
		r.Post("/{id}/objects/move", h.moveObject)       // v1.1 (rename/move)

		// Trash (v1.1).
		r.Get("/{id}/trash", h.listTrash)
		r.Post("/{id}/trash/{trashId}/restore", h.restoreTrash)
		r.Delete("/{id}/trash/{trashId}", h.purgeTrash)
	})

	r.Route("/access-keys", func(r chi.Router) {
		r.Get("/", h.listKeys)
		r.Post("/", h.createKey)
		r.Get("/{id}", h.getKey)
		r.Delete("/{id}", h.deleteKey)
		r.Post("/{id}/grants", h.grantKey)
		r.Delete("/{id}/grants/{bucketId}", h.revokeKey)
	})

	r.Route("/api-tokens", func(r chi.Router) {
		r.Get("/", h.listAPITokens)
		r.Post("/", h.createAPIToken)
		r.Delete("/{id}", h.revokeAPIToken)
	})

	r.Route("/clusters", func(r chi.Router) {
		r.Get("/", h.listClusters)
		r.Post("/", h.addCluster)
		r.Get("/{id}", h.getCluster)
		r.Delete("/{id}", h.removeCluster)

		// Multi-node management (Garage clusters only; generic-S3 → 422).
		r.Get("/{id}/nodes", h.listNodes)
		r.Post("/{id}/nodes", h.addNode)
		r.Delete("/{id}/nodes/{nodeId}", h.removeNode)
		r.Get("/{id}/layout", h.getLayout)
		r.Post("/{id}/layout/preview", h.previewLayout)
		r.Post("/{id}/layout/revert", h.revertLayout)
	})

	r.Route("/members", func(r chi.Router) {
		r.Get("/", h.listMembers)
		r.Post("/invite", h.inviteMember)
		r.Patch("/{userId}", h.changeMemberRole)
		r.Delete("/{userId}", h.removeMember)
	})

	r.Get("/dashboard", h.dashboard)
	r.Get("/usage/traffic", h.trafficUsage)
	r.Get("/audit", h.listAudit)
	r.Get("/audit/export", h.exportAudit)
	r.Get("/audit/verify", h.verifyAudit) // tamper-evidence check (Enterprise)
	r.Get("/docs/snippets", h.docsSnippets)
	r.Get("/system/garage-metrics", h.garageMetrics) // ops metrics proxy

	r.Route("/system/backups", func(r chi.Router) {
		r.Get("/", h.listBackups)
		r.Post("/", h.createBackup)
		r.Route("/schedules", func(r chi.Router) {
			r.Get("/", h.listBackupSchedules)
			r.Post("/", h.createBackupSchedule)
			r.Patch("/{id}", h.updateBackupSchedule)
			r.Delete("/{id}", h.deleteBackupSchedule)
		})
		r.Get("/{id}", h.getBackup)
	})
	r.Get("/system/reconcile", h.reconcileReport)

	// Platform-admin tenant lifecycle (Enterprise). Guarded inside the service by
	// requirePlatformAdmin, so a non-operator session gets 403.
	r.Route("/orgs/{orgId}", func(r chi.Router) {
		r.Get("/status", h.getOrgStatus)
		r.Post("/suspend", h.suspendOrg)
		r.Post("/resume", h.resumeOrg)
		r.Put("/quota", h.setOrgQuota)
		r.Get("/clusters", h.listOrgClusters)
		r.Post("/clusters", h.assignOrgCluster)
		r.Delete("/clusters/{clusterId}", h.unassignOrgCluster)
		r.Post("/tenant-cluster", h.assignTenantCluster) // Hosted: pooled/dedicated provisioning
	})

	// SCIM provisioning-token management (the active org's owner/platform admin).
	// The SCIM protocol itself is served at /scim/v2 by the ee handler.
	r.Route("/scim-tokens", func(r chi.Router) {
		r.Get("/", h.listSCIMTokens)
		r.Post("/", h.createSCIMToken)
		r.Delete("/{id}", h.revokeSCIMToken)
	})

	// ABAC policies (owner/platform admin). Enforced only under the ee authorizer.
	r.Route("/policies", func(r chi.Router) {
		r.Get("/", h.listPolicies)
		r.Post("/", h.createPolicy)
		r.Patch("/{id}", h.setPolicyEnabled)
		r.Delete("/{id}", h.deletePolicy)
	})

	// White-label branding for the active org (read any member; write owner).
	r.Get("/branding", h.getBranding)
	r.Put("/branding", h.setBranding)

	// Usage-based billing (Hosted). Status readable by members; setup owner/admin.
	r.Get("/billing", h.billingStatus)
	r.Post("/billing/setup", h.setupBilling)
	r.Post("/billing/report", h.triggerBillingReport) // platform admin: run a reporting pass

	// S3-to-S3 import jobs (Hosted onboarding).
	r.Route("/migrations", func(r chi.Router) {
		r.Get("/", h.listMigrations)
		r.Post("/", h.startMigration)
		r.Get("/{id}", h.getMigration)
		r.Post("/{id}/cancel", h.cancelMigration)
	})
}

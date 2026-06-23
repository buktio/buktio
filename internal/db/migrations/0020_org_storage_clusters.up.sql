-- 0020: per-org cluster assignment (Enterprise add-on / Hosted foundation).
--
-- Maps an org to one or more storage clusters and marks one as its default, so an
-- org's new buckets land on its own (possibly dedicated) backend instead of the
-- shared primary. With no row, an org falls back to the primary cluster — so OSS
-- and single-cluster deployments are unchanged.
CREATE TABLE org_storage_clusters (
    id                 uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id             uuid        NOT NULL REFERENCES organizations(id)   ON DELETE CASCADE,
    storage_cluster_id uuid        NOT NULL REFERENCES storage_clusters(id) ON DELETE CASCADE,
    is_default         boolean     NOT NULL DEFAULT false,
    created_at         timestamptz NOT NULL DEFAULT now()
);

-- An org maps each cluster at most once.
CREATE UNIQUE INDEX uq_org_cluster ON org_storage_clusters (org_id, storage_cluster_id);
-- At most one default cluster per org.
CREATE UNIQUE INDEX uq_org_default_cluster ON org_storage_clusters (org_id) WHERE is_default;

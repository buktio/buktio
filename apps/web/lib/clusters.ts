/**
 * Shared, presentation-only helpers for storage backends (clusters). No styling
 * here — just value/label mapping reused across the clusters page, the buckets
 * list backend badge, and capability gating in bucket detail.
 */

import type { Cluster, ClusterCapabilities, ClusterProvider } from "@/lib/api";

/** Human label for a storage backend provider. */
export function providerLabel(provider: ClusterProvider): string {
  switch (provider) {
    case "garage":
      return "Garage";
    case "aws_s3":
      return "AWS S3";
    case "r2":
      return "Cloudflare R2";
    case "b2":
      return "Backblaze B2";
    case "seaweedfs":
      return "SeaweedFS";
    case "ceph_rgw":
      return "Ceph RGW";
    default:
      return provider;
  }
}

/** Providers that can be added through the UI (Garage is never offered). */
export const ADDABLE_PROVIDERS: { value: ClusterProvider; label: string }[] = [
  { value: "aws_s3", label: "AWS S3" },
  { value: "r2", label: "Cloudflare R2" },
  { value: "b2", label: "Backblaze B2" },
  { value: "seaweedfs", label: "SeaweedFS" },
  { value: "ceph_rgw", label: "Ceph RGW" },
];

/** s3_endpoint is required for every addable provider except AWS S3. */
export function endpointRequired(provider: ClusterProvider): boolean {
  return provider !== "aws_s3";
}

/** Default region used when the field is left blank, per provider. */
export function defaultRegion(provider: ClusterProvider): string {
  return provider === "r2" ? "auto" : "us-east-1";
}

/** Resolve the cluster a bucket lives on. Empty/absent cluster_id = primary. */
export function clusterForBucket(
  clusters: Cluster[],
  clusterId: string | undefined,
): Cluster | undefined {
  if (clusterId) {
    return clusters.find((c) => c.id === clusterId);
  }
  return clusters.find((c) => c.is_primary);
}

/** Capability rows used by the matrix, in a readable order. */
export const CAPABILITY_ROWS: { key: keyof ClusterCapabilities; label: string }[] = [
  { key: "manages_keys", label: "Access keys" },
  { key: "manages_quota", label: "Quota" },
  { key: "has_cluster_health", label: "Cluster health" },
  { key: "public_website", label: "Public website" },
  { key: "bucket_cors", label: "CORS" },
  { key: "lifecycle_expiry", label: "Lifecycle" },
  { key: "object_versioning", label: "Versioning" },
  { key: "per_object_public_acl", label: "Per-object ACL" },
];

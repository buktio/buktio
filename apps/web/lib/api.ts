/**
 * Typed fetch client for the buktio Go API. All endpoints live under /api/v1 and
 * are same-origin (next.config rewrites /api/* to the Go server). Cookies are set
 * by the API; mutations require the CSRF token from the (non-HttpOnly) buktio_csrf
 * cookie sent back as the X-CSRF-Token header.
 */

const API_BASE = "/api/v1";

/** Error thrown for any non-2xx response, carrying the API error envelope. */
export class ApiError extends Error {
  code: string;
  status: number;
  constructor(message: string, code: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.code = code;
    this.status = status;
  }
}

/** Read a cookie value by name from document.cookie (browser-only). */
function readCookie(name: string): string | null {
  if (typeof document === "undefined") return null;
  const match = document.cookie.match(
    new RegExp("(?:^|; )" + name.replace(/([.$?*|{}()[\]\\/+^])/g, "\\$1") + "=([^;]*)"),
  );
  return match ? decodeURIComponent(match[1]) : null;
}

type Method = "GET" | "POST" | "PUT" | "PATCH" | "DELETE";

async function request<T>(method: Method, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  const isMutation = method !== "GET";

  if (body !== undefined) {
    headers["Content-Type"] = "application/json";
  }
  if (isMutation) {
    const csrf = readCookie("buktio_csrf");
    if (csrf) headers["X-CSRF-Token"] = csrf;
  }

  const res = await fetch(`${API_BASE}${path}`, {
    method,
    headers,
    credentials: "same-origin",
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (res.status === 204) {
    return undefined as T;
  }

  let payload: unknown = null;
  const text = await res.text();
  if (text) {
    try {
      payload = JSON.parse(text);
    } catch {
      payload = text;
    }
  }

  if (!res.ok) {
    const env = payload as { error?: { code?: string; message?: string } } | null;
    const code = env?.error?.code ?? "request_failed";
    const message = env?.error?.message ?? `Request failed (${res.status})`;
    throw new ApiError(message, code, res.status);
  }

  return payload as T;
}

export function apiGet<T>(path: string): Promise<T> {
  return request<T>("GET", path);
}

export function apiSend<T>(method: Exclude<Method, "GET">, path: string, body?: unknown): Promise<T> {
  return request<T>(method, path, body);
}

/**
 * apiUpload streams a File straight to the API (which proxies it to storage with
 * the system key). Works behind any reverse proxy, unlike presigned URLs.
 */
export async function apiUpload(path: string, file: File): Promise<void> {
  const headers: Record<string, string> = {
    "Content-Type": file.type || "application/octet-stream",
  };
  const csrf = readCookie("buktio_csrf");
  if (csrf) headers["X-CSRF-Token"] = csrf;

  const res = await fetch(`${API_BASE}${path}`, {
    method: "PUT",
    headers,
    credentials: "same-origin",
    body: file,
  });
  if (!res.ok && res.status !== 204) {
    let message = `Upload failed (${res.status})`;
    try {
      const env = (await res.json()) as { error?: { message?: string; code?: string } };
      if (env?.error?.message) message = env.error.message;
      throw new ApiError(message, env?.error?.code ?? "upload_failed", res.status);
    } catch (e) {
      if (e instanceof ApiError) throw e;
      throw new ApiError(message, "upload_failed", res.status);
    }
  }
}

/** objectContentUrl returns the same-origin GET URL that streams an object out
 * (the browser sends the session cookie automatically). */
export function objectContentUrl(bucketId: string, key: string): string {
  return `${API_BASE}/buckets/${bucketId}/objects/content?key=${encodeURIComponent(key)}`;
}

/** objectUploadPath returns the PUT path for apiUpload. */
export function objectUploadPath(bucketId: string, key: string): string {
  return `/buckets/${bucketId}/objects/content?key=${encodeURIComponent(key)}`;
}

/* ------------------------------------------------------------------ */
/* v1.1 — upload progress, SSE-C, presign, metrics                    */
/* ------------------------------------------------------------------ */

/** Generate 32 random bytes encoded as base64 — used as an SSE-C key. */
export function random32B64(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  let binary = "";
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary);
}

/**
 * apiUploadProgress streams a File to the API via XMLHttpRequest so upload
 * progress can be reported. Optionally attaches an SSE-C key header. Resolves on
 * 2xx, rejects with ApiError otherwise.
 */
export function apiUploadProgress(
  path: string,
  file: File,
  opts?: { ssecKeyB64?: string; onProgress?: (pct: number) => void },
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", `${API_BASE}${path}`);
    xhr.withCredentials = true;
    xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");
    const csrf = readCookie("buktio_csrf");
    if (csrf) xhr.setRequestHeader("X-CSRF-Token", csrf);
    if (opts?.ssecKeyB64) xhr.setRequestHeader("X-Buktio-SSEC-Key", opts.ssecKeyB64);

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && opts?.onProgress) {
        opts.onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) {
        resolve();
        return;
      }
      let message = `Upload failed (${xhr.status})`;
      let code = "upload_failed";
      try {
        const env = JSON.parse(xhr.responseText) as {
          error?: { message?: string; code?: string };
        };
        if (env?.error?.message) message = env.error.message;
        if (env?.error?.code) code = env.error.code;
      } catch {
        // keep defaults
      }
      reject(new ApiError(message, code, xhr.status));
    };
    xhr.onerror = () => reject(new ApiError("Network error during upload", "network_error", 0));
    xhr.send(file);
  });
}

/** apiUploadSSEC uploads a file encrypted with a client-supplied SSE-C key. */
export function apiUploadSSEC(
  path: string,
  file: File,
  keyB64: string,
  onProgress?: (pct: number) => void,
): Promise<void> {
  return apiUploadProgress(path, file, { ssecKeyB64: keyB64, onProgress });
}

/**
 * apiDownloadSSEC fetches an SSE-C-encrypted object, supplying the decryption key
 * via header, and returns it as a Blob (the caller turns it into an object URL).
 */
export async function apiDownloadSSEC(
  bucketId: string,
  key: string,
  keyB64: string,
): Promise<Blob> {
  const res = await fetch(objectContentUrl(bucketId, key), {
    credentials: "same-origin",
    headers: { "X-Buktio-SSEC-Key": keyB64 },
  });
  if (!res.ok) {
    let message = `Download failed (${res.status})`;
    let code = "download_failed";
    try {
      const env = (await res.json()) as { error?: { message?: string; code?: string } };
      if (env?.error?.message) message = env.error.message;
      if (env?.error?.code) code = env.error.code;
    } catch {
      // keep defaults
    }
    throw new ApiError(message, code, res.status);
  }
  return res.blob();
}

/** presignUpload requests a presigned PUT URL for large direct-to-storage uploads. */
export async function presignUpload(
  bucketId: string,
  key: string,
  contentType: string,
): Promise<PresignResponse> {
  return apiSend<PresignResponse>("POST", `/buckets/${bucketId}/objects/presign`, {
    operation: "put",
    key,
    content_type: contentType || "application/octet-stream",
    expires_in: 900,
  } satisfies PresignBody);
}

/**
 * uploadPresigned PUTs a file directly to the (presigned) storage URL using
 * XMLHttpRequest for progress. No cookies or CSRF are sent to the presigned URL.
 */
export function uploadPresigned(
  url: string,
  file: File,
  onProgress?: (pct: number) => void,
): Promise<void> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("PUT", url);
    xhr.setRequestHeader("Content-Type", file.type || "application/octet-stream");
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress(Math.round((e.loaded / e.total) * 100));
      }
    };
    xhr.onload = () => {
      if (xhr.status >= 200 && xhr.status < 300) resolve();
      else reject(new ApiError(`Upload failed (${xhr.status})`, "upload_failed", xhr.status));
    };
    xhr.onerror = () => reject(new ApiError("Network error during upload", "network_error", 0));
    xhr.send(file);
  });
}

/** Threshold (bytes) above which uploads use the presigned PUT path. */
export const PRESIGN_THRESHOLD_BYTES = 8 * 1024 * 1024;

/** fetchGarageMetrics returns the raw Prometheus exposition text. */
export async function fetchGarageMetrics(): Promise<string> {
  const res = await fetch(`${API_BASE}/system/garage-metrics`, {
    credentials: "same-origin",
  });
  if (!res.ok) {
    throw new ApiError(`Failed to load metrics (${res.status})`, "metrics_failed", res.status);
  }
  return res.text();
}

/* ------------------------------------------------------------------ */
/* v1.1 — CORS                                                        */
/* ------------------------------------------------------------------ */

export interface CorsRule {
  allowed_origins: string[];
  allowed_methods: string[];
  allowed_headers: string[];
  expose_headers: string[];
  max_age_seconds: number;
}

export type MigrationJob = {
  id: string;
  source_bucket: string;
  dest_bucket_id: string;
  status: string;
  copied_objects: number;
  copied_bytes: number;
  error?: string;
};

export type StartMigrationInput = {
  source_endpoint: string;
  source_region?: string;
  source_bucket: string;
  access_key_id: string;
  secret_access_key: string;
  dest_bucket_id: string;
};

export function startMigration(input: StartMigrationInput): Promise<MigrationJob> {
  return apiSend<MigrationJob>("POST", "/migrations", input);
}

export function listMigrations(): Promise<{ migrations: MigrationJob[] }> {
  return apiGet<{ migrations: MigrationJob[] }>("/migrations");
}

export function getMigration(id: string): Promise<MigrationJob> {
  return apiGet<MigrationJob>(`/migrations/${id}`);
}

export function cancelMigration(id: string): Promise<void> {
  return apiSend<void>("POST", `/migrations/${id}/cancel`);
}

export type BillingStatus = {
  enabled: boolean;
  status?: string;
  storage_bytes: number;
  egress_bytes: number;
  requests: number;
  customer_set: boolean;
  period_start?: string;
};

export function getBilling(): Promise<BillingStatus> {
  return apiGet<BillingStatus>("/billing");
}

export function setupBilling(email: string): Promise<void> {
  return apiSend<void>("POST", "/billing/setup", { email });
}

/* ABAC policies (Enterprise) */
export type Policy = {
  id: string;
  name: string;
  template: string;
  config: Record<string, string>;
  enabled: boolean;
  roles: string[];
  created_at: string;
};

export function listPolicies(): Promise<{ policies: Policy[] }> {
  return apiGet<{ policies: Policy[] }>("/policies");
}

export function createPolicy(input: {
  name: string;
  template: string;
  config: Record<string, string>;
  roles: string[];
}): Promise<{ id: string }> {
  return apiSend<{ id: string }>("POST", "/policies", input);
}

export function setPolicyEnabled(id: string, enabled: boolean): Promise<void> {
  return apiSend<void>("PATCH", `/policies/${id}`, { enabled });
}

export function deletePolicy(id: string): Promise<void> {
  return apiSend<void>("DELETE", `/policies/${id}`);
}

/* SCIM provisioning tokens (Enterprise) */
export type ScimToken = {
  id: string;
  name: string;
  last_four?: string;
  created_at: string;
  last_used_at?: string;
};

export type CreatedScimToken = { id: string; token: string };

export function listScimTokens(): Promise<{ tokens: ScimToken[] }> {
  return apiGet<{ tokens: ScimToken[] }>("/scim-tokens");
}

export function createScimToken(name: string): Promise<CreatedScimToken> {
  return apiSend<CreatedScimToken>("POST", "/scim-tokens", { name });
}

export function revokeScimToken(id: string): Promise<void> {
  return apiSend<void>("DELETE", `/scim-tokens/${id}`);
}

export type SignupResult = {
  user_id: string;
  verification_sent: boolean;
  verification_token?: string; // dev only
};

// signup creates an org + unverified owner (Hosted self-serve). Public.
export function signup(email: string, password: string, orgName: string): Promise<SignupResult> {
  return apiSend<SignupResult>("POST", "/signup", { email, password, org_name: orgName });
}

// verifySignup consumes the email-verification token and logs the user in.
export function verifySignup(token: string): Promise<AuthResponse> {
  return apiSend<AuthResponse>("POST", "/signup/verify", { token });
}

export function resendVerification(email: string): Promise<void> {
  return apiSend<void>("POST", "/signup/resend", { email });
}

export type Branding = {
  display_name?: string;
  logo_url?: string;
  primary_color?: string;
  email_from?: string;
  custom_domain?: string;
};

// getBranding returns the active org's white-label settings (Enterprise). Empty
// object when unset or unlicensed.
export function getBranding(): Promise<Branding> {
  return apiGet<Branding>("/branding");
}

export function setBranding(b: Branding): Promise<void> {
  return apiSend<void>("PUT", "/branding", b);
}

export function getBucketCors(bucketId: string): Promise<ListEnvelope<CorsRule>> {
  return apiGet<ListEnvelope<CorsRule>>(`/buckets/${bucketId}/cors`);
}

export function setBucketCors(bucketId: string, rules: CorsRule[]): Promise<void> {
  return apiSend<void>("PUT", `/buckets/${bucketId}/cors`, { rules });
}

export function deleteBucketCors(bucketId: string): Promise<void> {
  return apiSend<void>("DELETE", `/buckets/${bucketId}/cors`);
}

/* ------------------------------------------------------------------ */
/* v1.1 — Lifecycle                                                   */
/* ------------------------------------------------------------------ */

export interface LifecycleRule {
  id: string;
  prefix: string;
  enabled: boolean;
  expire_days: number;
  abort_incomplete_mpu_days: number;
}

export function getBucketLifecycle(bucketId: string): Promise<ListEnvelope<LifecycleRule>> {
  return apiGet<ListEnvelope<LifecycleRule>>(`/buckets/${bucketId}/lifecycle`);
}

export function setBucketLifecycle(
  bucketId: string,
  rules: Omit<LifecycleRule, "id">[],
): Promise<void> {
  return apiSend<void>("PUT", `/buckets/${bucketId}/lifecycle`, { rules });
}

export function deleteBucketLifecycle(bucketId: string): Promise<void> {
  return apiSend<void>("DELETE", `/buckets/${bucketId}/lifecycle`);
}

/* ------------------------------------------------------------------ */
/* v1.1 — Object copy / move                                          */
/* ------------------------------------------------------------------ */

export function copyObject(bucketId: string, srcKey: string, dstKey: string): Promise<void> {
  return apiSend<void>("POST", `/buckets/${bucketId}/objects/copy`, {
    src_key: srcKey,
    dst_key: dstKey,
  });
}

export function moveObject(bucketId: string, srcKey: string, dstKey: string): Promise<void> {
  return apiSend<void>("POST", `/buckets/${bucketId}/objects/move`, {
    src_key: srcKey,
    dst_key: dstKey,
  });
}

/* ------------------------------------------------------------------ */
/* v1.1 — Trash                                                       */
/* ------------------------------------------------------------------ */

export interface TrashItem {
  id: string;
  key: string;
  size_bytes: number;
  deleted_at: string;
  purge_after: string;
}

export function listTrash(bucketId: string): Promise<ListEnvelope<TrashItem>> {
  return apiGet<ListEnvelope<TrashItem>>(`/buckets/${bucketId}/trash`);
}

export function restoreTrash(bucketId: string, trashId: string): Promise<void> {
  return apiSend<void>("POST", `/buckets/${bucketId}/trash/${trashId}/restore`);
}

export function purgeTrash(bucketId: string, trashId: string): Promise<void> {
  return apiSend<void>("DELETE", `/buckets/${bucketId}/trash/${trashId}`);
}

/* ------------------------------------------------------------------ */
/* v1.1 — API tokens (PATs)                                           */
/* ------------------------------------------------------------------ */

export interface ApiToken {
  id: string;
  name: string;
  secret_last_four: string;
  scopes: string[];
  expires_at: string | null;
  last_used_at: string | null;
  created_at: string;
}

export interface CreatedApiToken extends ApiToken {
  token: string;
  secret_shown_once: boolean;
}

export function listApiTokens(): Promise<ListEnvelope<ApiToken>> {
  return apiGet<ListEnvelope<ApiToken>>("/api-tokens");
}

export function createApiToken(name: string, expiresInDays: number): Promise<CreatedApiToken> {
  return apiSend<CreatedApiToken>("POST", "/api-tokens", {
    name,
    expires_in_days: expiresInDays,
  });
}

export function revokeApiToken(id: string): Promise<void> {
  return apiSend<void>("DELETE", `/api-tokens/${id}`);
}

/* ------------------------------------------------------------------ */
/* v2 — Clusters (storage backends)                                   */
/* ------------------------------------------------------------------ */

export type ClusterProvider =
  | "garage"
  | "aws_s3"
  | "r2"
  | "b2"
  | "seaweedfs"
  | "ceph_rgw"
  | string;

export type ClusterMode = "managed" | "external" | string;

export interface ClusterCapabilities {
  object_versioning: boolean;
  per_object_public_acl: boolean;
  bucket_cors: boolean;
  lifecycle_expiry: boolean;
  manages_keys: boolean;
  manages_quota: boolean;
  has_cluster_health: boolean;
  public_website: boolean;
}

export interface Cluster {
  id: string;
  name: string;
  provider: ClusterProvider;
  mode: ClusterMode;
  s3_endpoint: string;
  s3_region: string;
  status: string;
  is_primary: boolean;
  bucket_count: number;
  capabilities: ClusterCapabilities;
}

export interface AddClusterBody {
  name: string;
  provider: ClusterProvider;
  s3_endpoint: string;
  s3_region: string;
  access_key_id: string;
  secret_access_key: string;
  public_endpoint?: string;
}

/** listClusters returns every configured storage backend. */
export function listClusters(): Promise<ListEnvelope<Cluster>> {
  return apiGet<ListEnvelope<Cluster>>("/clusters");
}

/** getCluster fetches a single storage backend by id. */
export function getCluster(id: string): Promise<Cluster> {
  return apiGet<Cluster>(`/clusters/${id}`);
}

/** addCluster verifies and registers an external storage backend. */
export function addCluster(input: AddClusterBody): Promise<Cluster> {
  return apiSend<Cluster>("POST", "/clusters", input);
}

/** removeCluster deletes a backend (409 if it is primary or still has buckets). */
export function removeCluster(id: string): Promise<void> {
  return apiSend<void>("DELETE", `/clusters/${id}`);
}

/* ------------------------------------------------------------------ */
/* v2 — Cluster nodes & layout (Garage only)                          */
/* ------------------------------------------------------------------ */

export type ClusterNodeRole = "storage" | "gateway" | "";

export interface ClusterNode {
  id: string;
  hostname: string;
  addr: string;
  zone: string;
  role: ClusterNodeRole;
  is_up: boolean;
  draining: boolean;
  capacity_bytes: number | null;
  disk_total_bytes: number;
  disk_avail_bytes: number;
}

export interface AddNodeBody {
  node_id: string;
  /** "<node_id>@host:port" to connect a new peer first; optional. */
  peer?: string;
  zone?: string;
  /** >0 marks a storage node; 0 marks a gateway node. */
  capacity_bytes: number;
}

export interface LayoutRole {
  id: string;
  zone: string;
  capacity: number | null;
  tags: string[];
}

export interface ClusterLayout {
  version: number;
  roles: LayoutRole[];
  staged_role_changes: LayoutRole[];
}

export interface LayoutPreview {
  message: string[];
}

/** listNodes returns the cluster's node topology (Garage only). */
export function listNodes(id: string): Promise<ListEnvelope<ClusterNode>> {
  return apiGet<ListEnvelope<ClusterNode>>(`/clusters/${id}/nodes`);
}

/** addNode connects/assigns a node and applies the layout (202 accepted). */
export function addNode(id: string, body: AddNodeBody): Promise<void> {
  return apiSend<void>("POST", `/clusters/${id}/nodes`, body);
}

/** removeNode drains and removes a node from the cluster (202 accepted). */
export function removeNode(id: string, nodeId: string): Promise<void> {
  return apiSend<void>("DELETE", `/clusters/${id}/nodes/${nodeId}`);
}

/** getLayout returns the applied roles plus any staged role changes. */
export function getLayout(id: string): Promise<ClusterLayout> {
  return apiGet<ClusterLayout>(`/clusters/${id}/layout`);
}

/** previewLayout returns a human-readable preview of staged changes. */
export function previewLayout(id: string): Promise<LayoutPreview> {
  return apiSend<LayoutPreview>("POST", `/clusters/${id}/layout/preview`);
}

/** revertLayout drops pending staged changes (204 no content). */
export function revertLayout(id: string): Promise<void> {
  return apiSend<void>("POST", `/clusters/${id}/layout/revert`);
}

/* ------------------------------------------------------------------ */
/* v2 — Backups & reconcile                                           */
/* ------------------------------------------------------------------ */

export type BackupStatus = "pending" | "running" | "succeeded" | "failed" | string;

export interface BackupJob {
  id: string;
  kind: string;
  status: BackupStatus;
  path: string;
  size_bytes: number;
  error?: string;
  created_at: string;
  finished_at?: string;
}

export interface ReconcileReport {
  buckets_only_in_db: string[] | null;
  buckets_only_in_backend: string[] | null;
  in_sync: boolean;
}

/** listBackups returns recent backup jobs, newest first. */
export function listBackups(): Promise<ListEnvelope<BackupJob>> {
  return apiGet<ListEnvelope<BackupJob>>("/system/backups");
}

/** createBackup kicks off a metadata + config backup (202, status "pending"). */
export function createBackup(): Promise<BackupJob> {
  return apiSend<BackupJob>("POST", "/system/backups");
}

/** getBackup fetches a single backup job by id (used to poll for completion). */
export function getBackup(id: string): Promise<BackupJob> {
  return apiGet<BackupJob>(`/system/backups/${id}`);
}

/** reconcileReport compares the control-plane DB against the storage backend. */
export function reconcileReport(): Promise<ReconcileReport> {
  return apiGet<ReconcileReport>("/system/reconcile");
}

/* ------------------------------------------------------------------ */
/* v2 — Traffic (per access key)                                      */
/* ------------------------------------------------------------------ */

export interface TrafficRow {
  access_key_id: string;
  key_name: string;
  requests: number;
  bytes_in: number;
  bytes_out: number;
}

/** trafficUsage returns per-key request/byte counters over the given window. */
export function trafficUsage(hours = 24): Promise<ListEnvelope<TrafficRow>> {
  return apiGet<ListEnvelope<TrafficRow>>(`/usage/traffic?hours=${hours}`);
}

/* ------------------------------------------------------------------ */
/* DTO types                                                          */
/* ------------------------------------------------------------------ */

export type Role = "owner" | "admin" | "member" | "viewer" | string;

export interface User {
  id: string;
  email: string;
  full_name: string;
  role: Role;
}

/** Pro feature flags returned by /auth/me. In the OSS build all are true. */
export interface Features {
  rbac_enforced: boolean;
  multi_user: boolean;
  sso: boolean;
  scheduled_backups: boolean;
  advanced_audit: boolean;
  white_label: boolean;
  scim: boolean;
  self_serve_signup: boolean;
  billing: boolean;
  sso_configured: boolean;
}

export interface SetupStatus {
  initialized: boolean;
}

export interface AuthResponse {
  user: User;
  /** Present on /auth/me; absent on login/setup responses. */
  features?: Features;
}

/** Me is the authenticated session (/auth/me): the user plus Pro feature flags. */
export interface Me {
  user: User;
  features: Features;
}

/** authMe fetches the current session including Pro feature flags. */
export function authMe(): Promise<Me> {
  return apiGet<Me>("/auth/me");
}

export interface AuthMethods {
  password: boolean;
  sso: boolean;
}

/** authMethods reports which sign-in methods the instance exposes (public). */
export function authMethods(): Promise<AuthMethods> {
  return apiGet<AuthMethods>("/auth/methods");
}

/* ------------------------------------------------------------------ */
/* Pro — Members & invitations                                        */
/* ------------------------------------------------------------------ */

export interface Member {
  user_id: string;
  email: string;
  full_name: string;
  role: Role;
  created_at: string;
}

export interface Invitation {
  id: string;
  email: string;
  role: Role;
  expires_at: string;
}

export interface MembersResponse {
  members: Member[];
  invitations: Invitation[];
}

export interface CreatedInvitation extends Invitation {
  /** One-time accept URL — no email is sent, so this must be shown to copy. */
  link: string;
}

/** listMembers returns the team roster plus pending invitations. */
export function listMembers(): Promise<MembersResponse> {
  return apiGet<MembersResponse>("/members");
}

/** inviteMember creates a pending invitation and returns its one-time accept link. */
export function inviteMember(email: string, role: Role): Promise<CreatedInvitation> {
  return apiSend<CreatedInvitation>("POST", "/members/invite", { email, role });
}

/** changeMemberRole updates a member's role (409 if it would remove the last owner). */
export function changeMemberRole(userId: string, role: Role): Promise<void> {
  return apiSend<void>("PATCH", `/members/${userId}`, { role });
}

/** removeMember deletes a member (409 if it would remove the last owner). */
export function removeMember(userId: string): Promise<void> {
  return apiSend<void>("DELETE", `/members/${userId}`);
}

export interface AcceptInviteBody {
  token: string;
  password: string;
  full_name: string;
}

/** acceptInvite consumes an invite token, sets a password, and starts a session. */
export function acceptInvite(body: AcceptInviteBody): Promise<AuthResponse> {
  return apiSend<AuthResponse>("POST", "/auth/accept-invite", body);
}

/* ------------------------------------------------------------------ */
/* Pro — Advanced audit (filters + export)                            */
/* ------------------------------------------------------------------ */

export interface AuditFilter {
  /** RFC3339 timestamps. */
  from?: string;
  to?: string;
  actor?: string;
  action?: string;
  target?: string;
  limit?: number;
  offset?: number;
}

/** auditQueryString builds the shared query string for the audit list + export. */
function auditQueryString(filter: AuditFilter): string {
  const params = new URLSearchParams();
  if (filter.from) params.set("from", filter.from);
  if (filter.to) params.set("to", filter.to);
  if (filter.actor) params.set("actor", filter.actor);
  if (filter.action) params.set("action", filter.action);
  if (filter.target) params.set("target", filter.target);
  if (filter.limit !== undefined) params.set("limit", String(filter.limit));
  if (filter.offset !== undefined) params.set("offset", String(filter.offset));
  return params.toString();
}

/** listAudit fetches audit events filtered by the given criteria. */
export function listAudit(filter: AuditFilter = {}): Promise<ListEnvelope<AuditEvent>> {
  const qs = auditQueryString(filter);
  return apiGet<ListEnvelope<AuditEvent>>(`/audit${qs ? `?${qs}` : ""}`);
}

/**
 * auditExportUrl builds the same-origin URL for downloading the filtered audit log.
 * GET needs no CSRF; the browser sends the session cookie automatically.
 */
export function auditExportUrl(filter: AuditFilter, format: "csv" | "json"): string {
  const params = new URLSearchParams(auditQueryString(filter));
  params.set("format", format);
  return `${API_BASE}/audit/export?${params.toString()}`;
}

/* ------------------------------------------------------------------ */
/* Pro — Scheduled backups                                            */
/* ------------------------------------------------------------------ */

export interface BackupSchedule {
  id: string;
  enabled: boolean;
  interval_minutes: number;
  retention_count: number;
  offsite_enabled: boolean;
  next_run_at: string | null;
  last_run_at: string | null;
}

export interface BackupScheduleBody {
  interval_minutes: number;
  retention_count: number;
  offsite_enabled: boolean;
}

export interface PatchBackupScheduleBody extends BackupScheduleBody {
  enabled: boolean;
}

/** listBackupSchedules returns all configured backup schedules. */
export function listBackupSchedules(): Promise<ListEnvelope<BackupSchedule>> {
  return apiGet<ListEnvelope<BackupSchedule>>("/system/backups/schedules");
}

/** createBackupSchedule registers a new recurring backup schedule (201). */
export function createBackupSchedule(body: BackupScheduleBody): Promise<BackupSchedule> {
  return apiSend<BackupSchedule>("POST", "/system/backups/schedules", body);
}

/** updateBackupSchedule patches an existing schedule (204). */
export function updateBackupSchedule(id: string, body: PatchBackupScheduleBody): Promise<void> {
  return apiSend<void>("PATCH", `/system/backups/schedules/${id}`, body);
}

/** deleteBackupSchedule removes a schedule (204). */
export function deleteBackupSchedule(id: string): Promise<void> {
  return apiSend<void>("DELETE", `/system/backups/schedules/${id}`);
}

export type BucketVisibility = "private" | "public_website" | string;

export interface BucketQuota {
  max_bytes: number | null;
  max_objects: number | null;
}

export interface BucketUsage {
  bytes_used: number;
  objects: number;
}

export interface Bucket {
  id: string;
  name: string;
  garage_id: string;
  /** ID of the storage backend (cluster) this bucket lives on. Empty/absent = primary. */
  cluster_id?: string;
  visibility: BucketVisibility;
  website_enabled: boolean;
  quota: BucketQuota;
  usage: BucketUsage;
  created_at: string;
  /** Public website URL — present only when website_enabled. */
  public_url?: string;
}

export interface ListEnvelope<T> {
  data: T[];
}

export interface CreateBucketBody {
  name: string;
  quota_max_bytes?: number | null;
  quota_max_objects?: number | null;
  /** Target storage backend. Empty/omitted creates the bucket on the primary cluster. */
  cluster_id?: string;
}

export interface PatchBucketBody {
  quota_max_bytes: number | null;
  quota_max_objects: number | null;
}

export interface BucketAccessBody {
  public: boolean;
  index_document?: string;
  error_document?: string;
}

export interface S3Object {
  type: "object";
  key: string;
  size_bytes: number;
  etag: string;
  last_modified: string;
}

export interface ObjectsResponse {
  data: S3Object[];
  common_prefixes: string[];
  next_cursor?: string;
  is_truncated: boolean;
}

export interface PresignBody {
  operation: "get" | "put";
  key: string;
  expires_in?: number;
  content_type?: string;
}

export interface PresignResponse {
  url: string;
  method: string;
}

export interface Grant {
  bucket_id: string;
  bucket_name?: string;
  read: boolean;
  write: boolean;
  owner: boolean;
}

export interface AccessKey {
  id: string;
  name: string;
  access_key_id: string;
  can_create_bucket: boolean;
  secret_last_four: string;
  grants: Grant[];
  created_at: string;
}

export interface CreateAccessKeyBody {
  name: string;
  allow_create_bucket: boolean;
  grants: { bucket_id: string; read: boolean; write: boolean; owner: boolean }[];
}

export interface CreatedAccessKey extends AccessKey {
  secret_access_key: string;
  secret_shown_once: boolean;
}

export interface GrantBody {
  bucket_id: string;
  read: boolean;
  write: boolean;
  owner: boolean;
}

export interface Dashboard {
  cluster: {
    status: string;
    nodes_total: number;
    nodes_ok: number;
    /** Storage deployment mode — managed (buktio-run) or external. */
    mode?: "managed" | "external" | string;
  };
  capacity: {
    disk_total_bytes: number;
    disk_avail_bytes: number;
    used_pct: number;
  };
  totals: {
    buckets: number;
    access_keys: number;
    objects: number;
    bytes_used: number;
  };
  versions: {
    buktio: string;
    storage_engine: string;
  };
  alerts: { level: string; code: string; message: string }[];
  s3_endpoint: string;
}

export interface AuditEvent {
  id: string;
  action: string;
  actor_type: string;
  actor_user_id?: string;
  target_type: string;
  target_id: string;
  metadata: Record<string, unknown> | null;
  created_at: string;
}

export interface DocsSnippets {
  endpoint: string;
  region: string;
  bucket: string;
  access_key_id: string;
  addressing: string;
  signature: string;
  snippets: {
    aws_cli: string;
    rclone: string;
    boto3: string;
    node_sdk: string;
    restic: string;
  };
}

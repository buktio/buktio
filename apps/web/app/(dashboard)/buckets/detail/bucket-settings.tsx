"use client";

import * as React from "react";
import { Loader2, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  apiSend,
  ApiError,
  getBucketLifecycle,
  setBucketLifecycle,
  deleteBucketLifecycle,
  type Bucket,
  type ClusterCapabilities,
  type LifecycleRule,
} from "@/lib/api";
import { formatBytes, formatNumber } from "@/lib/format";
import { CopyButton } from "@/components/copy-button";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";

interface DraftLifecycle {
  prefix: string;
  enabled: boolean;
  expire_days: string;
  abort_incomplete_mpu_days: string;
}

function toLifecycleDraft(r: LifecycleRule): DraftLifecycle {
  return {
    prefix: r.prefix,
    enabled: r.enabled,
    expire_days: r.expire_days > 0 ? String(r.expire_days) : "",
    abort_incomplete_mpu_days:
      r.abort_incomplete_mpu_days > 0 ? String(r.abort_incomplete_mpu_days) : "",
  };
}

function LifecycleSection({ bucketId }: { bucketId: string }) {
  const [rules, setRules] = React.useState<DraftLifecycle[] | null>(null);
  const [saving, setSaving] = React.useState(false);
  const [clearing, setClearing] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await getBucketLifecycle(bucketId);
      setRules(res.data.map(toLifecycleDraft));
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load lifecycle rules");
      setRules([]);
    }
  }, [bucketId]);

  React.useEffect(() => {
    load();
  }, [load]);

  function update(idx: number, patch: Partial<DraftLifecycle>) {
    setRules((rs) => (rs ? rs.map((r, i) => (i === idx ? { ...r, ...patch } : r)) : rs));
  }

  function add() {
    setRules((rs) => [
      ...(rs ?? []),
      { prefix: "", enabled: true, expire_days: "", abort_incomplete_mpu_days: "" },
    ]);
  }

  function remove(idx: number) {
    setRules((rs) => (rs ? rs.filter((_, i) => i !== idx) : rs));
  }

  async function save() {
    if (!rules) return;
    const payload = rules.map((r) => {
      const expire = Number(r.expire_days);
      const abort = Number(r.abort_incomplete_mpu_days);
      return {
        prefix: r.prefix.trim(),
        enabled: r.enabled,
        expire_days: Number.isFinite(expire) && expire > 0 ? Math.round(expire) : 0,
        abort_incomplete_mpu_days:
          Number.isFinite(abort) && abort > 0 ? Math.round(abort) : 0,
      };
    });
    if (payload.some((r) => r.expire_days <= 0 && r.abort_incomplete_mpu_days <= 0)) {
      toast.error("Each rule needs an expiry or abort-incomplete-upload value greater than 0");
      return;
    }
    setSaving(true);
    try {
      await setBucketLifecycle(bucketId, payload);
      toast.success("Lifecycle rules saved");
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save lifecycle rules");
    } finally {
      setSaving(false);
    }
  }

  async function clear() {
    setClearing(true);
    try {
      await deleteBucketLifecycle(bucketId);
      toast.success("Lifecycle rules cleared");
      setRules([]);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to clear lifecycle rules");
    } finally {
      setClearing(false);
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>Lifecycle rules</CardTitle>
        <CardDescription>
          Automatically expire objects and abort stalled multipart uploads. Garage supports only
          expiry and abort-incomplete-multipart — no storage-class transitions.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {!rules ? (
          <Skeleton className="h-24 w-full" />
        ) : rules.length === 0 ? (
          <div className="text-muted-foreground rounded-lg border border-dashed p-6 text-center text-sm">
            No lifecycle rules. Objects are kept until deleted manually.
          </div>
        ) : (
          rules.map((r, i) => (
            <div key={i} className="flex flex-col gap-4 rounded-lg border p-4">
              <div className="flex items-center justify-between gap-3">
                <span className="text-sm font-medium">Rule {i + 1}</span>
                <div className="flex items-center gap-3">
                  <Label htmlFor={`lc-enabled-${i}`} className="text-muted-foreground text-xs">
                    Enabled
                  </Label>
                  <Switch
                    id={`lc-enabled-${i}`}
                    checked={r.enabled}
                    onCheckedChange={(c) => update(i, { enabled: c })}
                  />
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-8"
                    onClick={() => remove(i)}
                  >
                    <Trash2 className="size-4" />
                    <span className="sr-only">Remove rule</span>
                  </Button>
                </div>
              </div>
              <div className="grid gap-4 sm:grid-cols-3">
                <div className="grid gap-2">
                  <Label htmlFor={`lc-prefix-${i}`}>Prefix</Label>
                  <Input
                    id={`lc-prefix-${i}`}
                    value={r.prefix}
                    onChange={(e) => update(i, { prefix: e.target.value })}
                    placeholder="logs/"
                    className="font-mono text-xs"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor={`lc-expire-${i}`}>Expire after (days)</Label>
                  <Input
                    id={`lc-expire-${i}`}
                    type="number"
                    min="0"
                    value={r.expire_days}
                    onChange={(e) => update(i, { expire_days: e.target.value })}
                    placeholder="0 = off"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor={`lc-abort-${i}`}>Abort incomplete MPU (days)</Label>
                  <Input
                    id={`lc-abort-${i}`}
                    type="number"
                    min="0"
                    value={r.abort_incomplete_mpu_days}
                    onChange={(e) => update(i, { abort_incomplete_mpu_days: e.target.value })}
                    placeholder="0 = off"
                  />
                </div>
              </div>
            </div>
          ))
        )}
        <div>
          <Button type="button" variant="secondary" size="sm" onClick={add}>
            <Plus className="size-4" />
            Add rule
          </Button>
        </div>
      </CardContent>
      <CardFooter className="flex justify-between gap-2">
        <Button onClick={save} disabled={saving}>
          {saving && <Loader2 className="size-4 animate-spin" />}
          Save lifecycle rules
        </Button>
        <Button variant="outline" onClick={clear} disabled={clearing}>
          {clearing && <Loader2 className="size-4 animate-spin" />}
          Clear all
        </Button>
      </CardFooter>
    </Card>
  );
}

function bytesToGib(bytes: number | null): string {
  if (bytes === null || bytes <= 0) return "";
  return String(+(bytes / (1024 * 1024 * 1024)).toFixed(2));
}

function gibToBytes(value: string): number | null {
  const n = Number(value);
  if (!value.trim() || Number.isNaN(n) || n <= 0) return null;
  return Math.round(n * 1024 * 1024 * 1024);
}

/** Small shadcn-styled note shown where a control is disabled by capability. */
function UnsupportedNote() {
  return (
    <p className="text-muted-foreground text-xs">Not supported on this backend.</p>
  );
}

export function BucketSettings({
  bucket,
  onUpdated,
  capabilities,
}: {
  bucket: Bucket;
  onUpdated: (b: Bucket) => void;
  capabilities?: ClusterCapabilities;
}) {
  // When capabilities are unknown (e.g. primary cluster not resolved yet), allow
  // everything — gating only narrows what a known-limited backend exposes.
  const canWebsite = capabilities ? capabilities.public_website : true;
  const canQuota = capabilities ? capabilities.manages_quota : true;
  const canLifecycle = capabilities ? capabilities.lifecycle_expiry : true;
  // Access / website state
  const [isPublic, setIsPublic] = React.useState(bucket.visibility === "public_website");
  const [indexDoc, setIndexDoc] = React.useState("index.html");
  const [errorDoc, setErrorDoc] = React.useState("error.html");
  const [savingAccess, setSavingAccess] = React.useState(false);

  // Quota state
  const [quotaGib, setQuotaGib] = React.useState(bytesToGib(bucket.quota.max_bytes));
  const [quotaObjects, setQuotaObjects] = React.useState(
    bucket.quota.max_objects ? String(bucket.quota.max_objects) : "",
  );
  const [savingQuota, setSavingQuota] = React.useState(false);

  async function saveAccess() {
    setSavingAccess(true);
    try {
      const updated = await apiSend<Bucket>("PUT", `/buckets/${bucket.id}/access`, {
        public: isPublic,
        index_document: isPublic ? indexDoc : undefined,
        error_document: isPublic ? errorDoc : undefined,
      });
      onUpdated(updated);
      toast.success("Access settings saved");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save access settings");
    } finally {
      setSavingAccess(false);
    }
  }

  async function saveQuota() {
    setSavingQuota(true);
    try {
      const updated = await apiSend<Bucket>("PATCH", `/buckets/${bucket.id}`, {
        quota_max_bytes: gibToBytes(quotaGib),
        quota_max_objects:
          quotaObjects.trim() && Number(quotaObjects) > 0 ? Number(quotaObjects) : null,
      });
      onUpdated(updated);
      toast.success("Quota saved");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save quota");
    } finally {
      setSavingQuota(false);
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle>Public access &amp; website</CardTitle>
          <CardDescription>
            Make this bucket publicly readable and serve it as a static website.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="flex items-center justify-between rounded-lg border p-4">
            <div className="flex flex-col gap-0.5">
              <Label htmlFor="public-switch" className="text-sm font-medium">
                Public website
              </Label>
              <span className="text-muted-foreground text-xs">
                When enabled, objects are served publicly over HTTP.
              </span>
              {!canWebsite && <UnsupportedNote />}
            </div>
            <Switch
              id="public-switch"
              checked={isPublic}
              onCheckedChange={setIsPublic}
              disabled={!canWebsite}
            />
          </div>

          {isPublic && canWebsite && (
            <div className="grid gap-4 sm:grid-cols-2">
              <div className="grid gap-2">
                <Label htmlFor="index-doc">Index document</Label>
                <Input
                  id="index-doc"
                  value={indexDoc}
                  onChange={(e) => setIndexDoc(e.target.value)}
                  placeholder="index.html"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="error-doc">Error document</Label>
                <Input
                  id="error-doc"
                  value={errorDoc}
                  onChange={(e) => setErrorDoc(e.target.value)}
                  placeholder="error.html"
                />
              </div>
            </div>
          )}

          {bucket.website_enabled && bucket.public_url && (
            <div className="flex items-center justify-between gap-2 rounded-lg border p-3">
              <div className="flex min-w-0 flex-col">
                <span className="text-muted-foreground text-xs">Public URL</span>
                <a
                  href={bucket.public_url}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-primary truncate font-mono text-sm hover:underline"
                >
                  {bucket.public_url}
                </a>
              </div>
              <CopyButton value={bucket.public_url} label="Copy public URL" />
            </div>
          )}
        </CardContent>
        <CardFooter>
          <Button onClick={saveAccess} disabled={savingAccess || !canWebsite}>
            {savingAccess && <Loader2 className="size-4 animate-spin" />}
            Save access settings
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Quota</CardTitle>
          <CardDescription>
            Limit storage and object count. Leave blank for unlimited.
          </CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <div className="grid gap-4 sm:grid-cols-2">
            <div className="grid gap-2">
              <Label htmlFor="quota-bytes">Max size (GiB)</Label>
              <Input
                id="quota-bytes"
                type="number"
                min="0"
                step="0.1"
                value={quotaGib}
                onChange={(e) => setQuotaGib(e.target.value)}
                placeholder="Unlimited"
                disabled={!canQuota}
              />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="quota-objects">Max objects</Label>
              <Input
                id="quota-objects"
                type="number"
                min="0"
                step="1"
                value={quotaObjects}
                onChange={(e) => setQuotaObjects(e.target.value)}
                placeholder="Unlimited"
                disabled={!canQuota}
              />
            </div>
          </div>
          {!canQuota && <UnsupportedNote />}
          {bucket.quota.max_bytes && bucket.quota.max_bytes > 0 ? (
            <div className="flex flex-col gap-1.5">
              <Progress
                value={Math.min(100, (bucket.usage.bytes_used / bucket.quota.max_bytes) * 100)}
              />
              <p className="text-muted-foreground text-xs">
                {formatBytes(bucket.usage.bytes_used)} of {formatBytes(bucket.quota.max_bytes)} (
                {((bucket.usage.bytes_used / bucket.quota.max_bytes) * 100).toFixed(0)}%) across{" "}
                {formatNumber(bucket.usage.objects)} objects.
              </p>
            </div>
          ) : (
            <p className="text-muted-foreground text-xs">
              Currently using {formatBytes(bucket.usage.bytes_used)} across{" "}
              {formatNumber(bucket.usage.objects)} objects.
            </p>
          )}
        </CardContent>
        <CardFooter>
          <Button onClick={saveQuota} disabled={savingQuota || !canQuota}>
            {savingQuota && <Loader2 className="size-4 animate-spin" />}
            Save quota
          </Button>
        </CardFooter>
      </Card>

      {canLifecycle && <LifecycleSection bucketId={bucket.id} />}
    </div>
  );
}

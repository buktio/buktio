"use client";

import * as React from "react";
import { ArrowLeftRight, Ban, Loader2, Plus, RefreshCw } from "lucide-react";
import { toast } from "sonner";

import {
  apiGet,
  cancelMigration,
  listMigrations,
  startMigration,
  ApiError,
  type Bucket,
  type ListEnvelope,
  type MigrationJob,
} from "@/lib/api";
import { PageHeader } from "@/app/(dashboard)/page-header";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";

const ACTIVE_STATUSES = new Set(["pending", "running"]);

function isActive(status: string): boolean {
  return ACTIVE_STATUSES.has(status);
}

function formatCopied(job: MigrationJob): string {
  const mib = job.copied_bytes / 1024 ** 2;
  return `${job.copied_objects} obj · ${mib.toFixed(1)} MiB`;
}

export default function MigrationsPage() {
  const [migrations, setMigrations] = React.useState<MigrationJob[] | null>(null);
  const [buckets, setBuckets] = React.useState<Bucket[]>([]);
  const [loadError, setLoadError] = React.useState<string | null>(null);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [creating, setCreating] = React.useState(false);
  const [refreshing, setRefreshing] = React.useState(false);

  const [sourceEndpoint, setSourceEndpoint] = React.useState("");
  const [sourceRegion, setSourceRegion] = React.useState("us-east-1");
  const [sourceBucket, setSourceBucket] = React.useState("");
  const [accessKeyId, setAccessKeyId] = React.useState("");
  const [secretAccessKey, setSecretAccessKey] = React.useState("");
  const [destBucketId, setDestBucketId] = React.useState("");

  const [cancelling, setCancelling] = React.useState<MigrationJob | null>(null);
  const [cancelPending, setCancelPending] = React.useState(false);

  const reloadMigrations = React.useCallback(async () => {
    const res = await listMigrations();
    setMigrations(res.migrations);
  }, []);

  const load = React.useCallback(async () => {
    setLoadError(null);
    try {
      const [migres, bucketRes] = await Promise.all([
        listMigrations(),
        apiGet<ListEnvelope<Bucket>>("/buckets"),
      ]);
      setMigrations(migres.migrations);
      setBuckets(bucketRes.data);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to load migrations";
      setLoadError(message);
      setMigrations([]);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  const hasActive = migrations !== null && migrations.some((m) => isActive(m.status));

  // Auto-refresh while any job is pending/running.
  React.useEffect(() => {
    if (!hasActive) return;
    const interval = setInterval(() => {
      reloadMigrations().catch((err) => {
        toast.error(err instanceof ApiError ? err.message : "Failed to refresh migrations");
      });
    }, 3000);
    return () => clearInterval(interval);
  }, [hasActive, reloadMigrations]);

  const bucketName = React.useCallback(
    (id: string) => buckets.find((b) => b.id === id)?.name ?? id,
    [buckets],
  );

  async function onRefresh() {
    setRefreshing(true);
    try {
      await reloadMigrations();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to refresh migrations");
    } finally {
      setRefreshing(false);
    }
  }

  function resetForm() {
    setSourceEndpoint("");
    setSourceRegion("us-east-1");
    setSourceBucket("");
    setAccessKeyId("");
    setSecretAccessKey("");
    setDestBucketId("");
  }

  const canSubmit =
    sourceEndpoint.trim() !== "" &&
    sourceBucket.trim() !== "" &&
    accessKeyId.trim() !== "" &&
    secretAccessKey.trim() !== "" &&
    destBucketId !== "";

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    if (!canSubmit) return;
    setCreating(true);
    try {
      await startMigration({
        source_endpoint: sourceEndpoint.trim(),
        source_region: sourceRegion.trim() || undefined,
        source_bucket: sourceBucket.trim(),
        access_key_id: accessKeyId.trim(),
        secret_access_key: secretAccessKey,
        dest_bucket_id: destBucketId,
      });
      toast.success("Migration started");
      setCreateOpen(false);
      resetForm();
      await reloadMigrations();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to start migration");
    } finally {
      setCreating(false);
    }
  }

  async function onCancel() {
    if (!cancelling) return;
    setCancelPending(true);
    try {
      await cancelMigration(cancelling.id);
      toast.success("Migration cancelled");
      setCancelling(null);
      await reloadMigrations();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to cancel migration");
    } finally {
      setCancelPending(false);
    }
  }

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Migrations" }]}
        actions={
          <>
            <Button
              size="sm"
              variant="outline"
              onClick={onRefresh}
              disabled={refreshing || migrations === null}
            >
              <RefreshCw className={`size-4${refreshing ? " animate-spin" : ""}`} />
              Refresh
            </Button>
            <Button size="sm" onClick={() => setCreateOpen(true)}>
              <Plus className="size-4" />
              New migration
            </Button>
          </>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
        {migrations === null ? (
          <Skeleton className="h-64 w-full" />
        ) : loadError ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <ArrowLeftRight className="size-4" /> Migrations unavailable
              </CardTitle>
              <CardDescription>{loadError}</CardDescription>
            </CardHeader>
          </Card>
        ) : migrations.length === 0 ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <ArrowLeftRight className="size-4" /> No migrations yet
              </CardTitle>
              <CardDescription>
                Copy every object from any S3-compatible source (AWS S3, R2, MinIO, …) into a
                buktio bucket. Resumable; the source secret is encrypted at rest.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="size-4" />
                New migration
              </Button>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Migrations</CardTitle>
              <CardDescription>
                Copy every object from any S3-compatible source (AWS S3, R2, MinIO, …) into a
                buktio bucket. Resumable; the source secret is encrypted at rest.
              </CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Source bucket</TableHead>
                    <TableHead>Destination</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Copied</TableHead>
                    <TableHead className="w-12" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {migrations.map((m) => {
                    const failed = m.status === "failed";
                    return (
                      <TableRow key={m.id}>
                        <TableCell className="font-medium">{m.source_bucket}</TableCell>
                        <TableCell className="text-muted-foreground">
                          {bucketName(m.dest_bucket_id)}
                        </TableCell>
                        <TableCell>
                          {failed && m.error ? (
                            <Tooltip>
                              <TooltipTrigger asChild>
                                <Badge variant="destructive" className="font-normal">
                                  {m.status}
                                </Badge>
                              </TooltipTrigger>
                              <TooltipContent>{m.error}</TooltipContent>
                            </Tooltip>
                          ) : (
                            <Badge
                              variant={failed ? "destructive" : "secondary"}
                              className="font-normal"
                            >
                              {m.status}
                            </Badge>
                          )}
                        </TableCell>
                        <TableCell className="text-muted-foreground whitespace-nowrap">
                          {formatCopied(m)}
                        </TableCell>
                        <TableCell>
                          {isActive(m.status) && (
                            <Button
                              variant="ghost"
                              size="icon"
                              className="size-8"
                              onClick={() => setCancelling(m)}
                            >
                              <Ban className="size-4" />
                              <span className="sr-only">Cancel migration</span>
                            </Button>
                          )}
                        </TableCell>
                      </TableRow>
                    );
                  })}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}
      </div>

      {/* New migration dialog */}
      <Dialog
        open={createOpen}
        onOpenChange={(o) => {
          setCreateOpen(o);
          if (!o) resetForm();
        }}
      >
        <DialogContent className="sm:max-w-md">
          <form onSubmit={onCreate}>
            <DialogHeader>
              <DialogTitle>New migration</DialogTitle>
              <DialogDescription>
                Copy every object from any S3-compatible source into a buktio bucket. The source
                secret is encrypted at rest.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="source-endpoint">Source endpoint</Label>
                <Input
                  id="source-endpoint"
                  value={sourceEndpoint}
                  onChange={(e) => setSourceEndpoint(e.target.value)}
                  placeholder="https://s3.amazonaws.com"
                  autoComplete="off"
                  required
                  autoFocus
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="source-region">Source region</Label>
                <Input
                  id="source-region"
                  value={sourceRegion}
                  onChange={(e) => setSourceRegion(e.target.value)}
                  placeholder="us-east-1"
                  autoComplete="off"
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="source-bucket">Source bucket</Label>
                <Input
                  id="source-bucket"
                  value={sourceBucket}
                  onChange={(e) => setSourceBucket(e.target.value)}
                  placeholder="my-source-bucket"
                  autoComplete="off"
                  required
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="access-key-id">Access key ID</Label>
                <Input
                  id="access-key-id"
                  value={accessKeyId}
                  onChange={(e) => setAccessKeyId(e.target.value)}
                  autoComplete="off"
                  required
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="secret-access-key">Secret access key</Label>
                <Input
                  id="secret-access-key"
                  type="password"
                  value={secretAccessKey}
                  onChange={(e) => setSecretAccessKey(e.target.value)}
                  autoComplete="off"
                  required
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="dest-bucket">Destination bucket</Label>
                <Select value={destBucketId} onValueChange={setDestBucketId}>
                  <SelectTrigger id="dest-bucket">
                    <SelectValue placeholder="Select a bucket" />
                  </SelectTrigger>
                  <SelectContent>
                    {buckets.map((b) => (
                      <SelectItem key={b.id} value={b.id}>
                        {b.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={creating || !canSubmit}>
                {creating && <Loader2 className="size-4 animate-spin" />}
                Start migration
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Cancel confirmation */}
      <AlertDialog open={!!cancelling} onOpenChange={(o) => !o && setCancelling(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Cancel this migration?</AlertDialogTitle>
            <AlertDialogDescription>
              Copying from “{cancelling?.source_bucket}” will stop. Objects already copied remain in
              the destination bucket. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={cancelPending}>Keep running</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                onCancel();
              }}
              disabled={cancelPending}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {cancelPending && <Loader2 className="size-4 animate-spin" />}
              Cancel migration
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

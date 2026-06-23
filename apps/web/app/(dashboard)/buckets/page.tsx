"use client";

import * as React from "react";
import Link from "next/link";
import { Database, Loader2, MoreHorizontal, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  apiGet,
  apiSend,
  ApiError,
  listClusters,
  type Bucket,
  type Cluster,
  type CreateBucketBody,
  type ListEnvelope,
} from "@/lib/api";
import { clusterForBucket, providerLabel } from "@/lib/clusters";
import { formatBytes, formatDateShort, formatNumber } from "@/lib/format";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { VisibilityBadge } from "@/components/visibility-badge";
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu";
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

function gibToBytes(value: string): number | null {
  const n = Number(value);
  if (!value.trim() || Number.isNaN(n) || n <= 0) return null;
  return Math.round(n * 1024 * 1024 * 1024);
}

export default function BucketsPage() {
  const [buckets, setBuckets] = React.useState<Bucket[] | null>(null);
  const [clusters, setClusters] = React.useState<Cluster[]>([]);
  const [createOpen, setCreateOpen] = React.useState(false);
  const [creating, setCreating] = React.useState(false);
  const [name, setName] = React.useState("");
  const [quotaGib, setQuotaGib] = React.useState("");
  const [quotaObjects, setQuotaObjects] = React.useState("");
  const [clusterId, setClusterId] = React.useState("");
  const [deleting, setDeleting] = React.useState<Bucket | null>(null);
  const [deletePending, setDeletePending] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await apiGet<ListEnvelope<Bucket>>("/buckets");
      setBuckets(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load buckets");
      setBuckets([]);
    }
  }, []);

  React.useEffect(() => {
    load();
    listClusters()
      .then((res) => {
        setClusters(res.data);
        const primary = res.data.find((c) => c.is_primary);
        if (primary) setClusterId(primary.id);
      })
      .catch(() => {});
  }, [load]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setCreating(true);
    const primary = clusters.find((c) => c.is_primary);
    const body: CreateBucketBody = {
      name: name.trim(),
      quota_max_bytes: gibToBytes(quotaGib),
      quota_max_objects:
        quotaObjects.trim() && Number(quotaObjects) > 0 ? Number(quotaObjects) : null,
      // Omit when targeting the primary cluster (empty = primary on the backend).
      cluster_id: clusterId && clusterId !== primary?.id ? clusterId : undefined,
    };
    try {
      await apiSend<Bucket>("POST", "/buckets", body);
      toast.success(`Bucket "${body.name}" created`);
      setCreateOpen(false);
      setName("");
      setQuotaGib("");
      setQuotaObjects("");
      setClusterId(primary?.id ?? "");
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create bucket");
    } finally {
      setCreating(false);
    }
  }

  async function onDelete() {
    if (!deleting) return;
    setDeletePending(true);
    try {
      await apiSend<void>("DELETE", `/buckets/${deleting.id}`);
      toast.success(`Bucket "${deleting.name}" deleted`);
      setDeleting(null);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete bucket");
    } finally {
      setDeletePending(false);
    }
  }

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Buckets" }]}
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="size-4" />
            Create bucket
          </Button>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
        {buckets === null ? (
          <Skeleton className="h-64 w-full" />
        ) : buckets.length === 0 ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Database className="size-4" /> No buckets yet
              </CardTitle>
              <CardDescription>
                Create your first bucket to start storing objects.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="size-4" />
                Create bucket
              </Button>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Visibility</TableHead>
                    <TableHead>Backend</TableHead>
                    <TableHead className="text-right">Objects</TableHead>
                    <TableHead className="text-right">Size</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead className="w-12" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {buckets.map((b) => (
                    <TableRow key={b.id}>
                      <TableCell className="font-medium">
                        <Link href={`/buckets/${b.id}`} className="hover:underline">
                          {b.name}
                        </Link>
                      </TableCell>
                      <TableCell>
                        <VisibilityBadge visibility={b.visibility} />
                      </TableCell>
                      <TableCell>
                        {(() => {
                          const c = clusterForBucket(clusters, b.cluster_id);
                          return (
                            <Badge variant="outline" className="font-normal">
                              {c ? providerLabel(c.provider) : "—"}
                            </Badge>
                          );
                        })()}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatNumber(b.usage.objects)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatBytes(b.usage.bytes_used)}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatDateShort(b.created_at)}
                      </TableCell>
                      <TableCell>
                        <DropdownMenu>
                          <DropdownMenuTrigger asChild>
                            <Button variant="ghost" size="icon" className="size-8">
                              <MoreHorizontal className="size-4" />
                              <span className="sr-only">Actions</span>
                            </Button>
                          </DropdownMenuTrigger>
                          <DropdownMenuContent align="end">
                            <DropdownMenuItem asChild>
                              <Link href={`/buckets/${b.id}`}>Open</Link>
                            </DropdownMenuItem>
                            <DropdownMenuSeparator />
                            <DropdownMenuItem
                              variant="destructive"
                              onSelect={(e) => {
                                e.preventDefault();
                                setDeleting(b);
                              }}
                            >
                              <Trash2 className="size-4" />
                              Delete
                            </DropdownMenuItem>
                          </DropdownMenuContent>
                        </DropdownMenu>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Create bucket dialog */}
      <Dialog open={createOpen} onOpenChange={setCreateOpen}>
        <DialogContent>
          <form onSubmit={onCreate}>
            <DialogHeader>
              <DialogTitle>Create bucket</DialogTitle>
              <DialogDescription>
                Bucket names must be DNS-compatible (lowercase letters, digits, and hyphens).
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="bucket-name">Name</Label>
                <Input
                  id="bucket-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="my-bucket"
                  required
                  autoFocus
                />
              </div>
              {clusters.length > 1 && (
                <div className="grid gap-2">
                  <Label htmlFor="bucket-cluster">Storage backend</Label>
                  <Select value={clusterId} onValueChange={setClusterId}>
                    <SelectTrigger id="bucket-cluster">
                      <SelectValue placeholder="Primary cluster" />
                    </SelectTrigger>
                    <SelectContent>
                      {clusters.map((c) => (
                        <SelectItem key={c.id} value={c.id}>
                          {c.name} · {providerLabel(c.provider)}
                          {c.is_primary ? " (primary)" : ""}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              )}
              <div className="grid grid-cols-2 gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="quota-gib">Max size (GiB)</Label>
                  <Input
                    id="quota-gib"
                    type="number"
                    min="0"
                    step="0.1"
                    value={quotaGib}
                    onChange={(e) => setQuotaGib(e.target.value)}
                    placeholder="Unlimited"
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
                  />
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={creating || !name.trim()}>
                {creating && <Loader2 className="size-4 animate-spin" />}
                Create
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <AlertDialog open={!!deleting} onOpenChange={(o) => !o && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete bucket “{deleting?.name}”?</AlertDialogTitle>
            <AlertDialogDescription>
              This permanently empties and removes the bucket along with all of its objects.
              This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletePending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                onDelete();
              }}
              disabled={deletePending}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {deletePending && <Loader2 className="size-4 animate-spin" />}
              Delete
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

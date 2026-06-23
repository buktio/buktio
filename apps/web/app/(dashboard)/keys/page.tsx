"use client";

import * as React from "react";
import { KeyRound, Loader2, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  apiGet,
  apiSend,
  ApiError,
  listClusters,
  type AccessKey,
  type Bucket,
  type Cluster,
  type CreateAccessKeyBody,
  type CreatedAccessKey,
  type ListEnvelope,
} from "@/lib/api";
import { clusterForBucket } from "@/lib/clusters";
import { formatDateShort } from "@/lib/format";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { SecretDialog } from "@/app/(dashboard)/keys/secret-dialog";
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
import { Checkbox } from "@/components/ui/checkbox";
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
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

interface DraftGrant {
  bucket_id: string;
  read: boolean;
  write: boolean;
  owner: boolean;
}

function grantLabel(g: { read: boolean; write: boolean; owner: boolean }): string {
  if (g.owner) return "owner";
  const parts = [g.read && "read", g.write && "write"].filter(Boolean);
  return parts.length ? parts.join("+") : "none";
}

export default function KeysPage() {
  const [keys, setKeys] = React.useState<AccessKey[] | null>(null);
  const [buckets, setBuckets] = React.useState<Bucket[]>([]);
  const [clusters, setClusters] = React.useState<Cluster[]>([]);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [creating, setCreating] = React.useState(false);
  const [name, setName] = React.useState("");
  const [allowCreate, setAllowCreate] = React.useState(false);
  const [grants, setGrants] = React.useState<DraftGrant[]>([]);
  const [grantBucket, setGrantBucket] = React.useState("");

  const [createdSecret, setCreatedSecret] = React.useState<CreatedAccessKey | null>(null);
  const [deleting, setDeleting] = React.useState<AccessKey | null>(null);
  const [deletePending, setDeletePending] = React.useState(false);

  const loadKeys = React.useCallback(async () => {
    try {
      const res = await apiGet<ListEnvelope<AccessKey>>("/access-keys");
      setKeys(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load access keys");
      setKeys([]);
    }
  }, []);

  React.useEffect(() => {
    loadKeys();
    apiGet<ListEnvelope<Bucket>>("/buckets")
      .then((res) => setBuckets(res.data))
      .catch(() => {});
    listClusters()
      .then((res) => setClusters(res.data))
      .catch(() => {});
  }, [loadKeys]);

  // Only buckets on a backend that manages its own access keys can be granted.
  function bucketSupportsKeys(b: Bucket): boolean {
    const c = clusterForBucket(clusters, b.cluster_id);
    return c ? c.capabilities.manages_keys : true;
  }

  function bucketName(id: string): string {
    return buckets.find((b) => b.id === id)?.name ?? id;
  }

  function addGrant() {
    if (!grantBucket) return;
    if (grants.some((g) => g.bucket_id === grantBucket)) {
      toast.error("That bucket already has a grant");
      return;
    }
    setGrants((g) => [...g, { bucket_id: grantBucket, read: true, write: false, owner: false }]);
    setGrantBucket("");
  }

  function updateGrant(bucketId: string, patch: Partial<DraftGrant>) {
    setGrants((gs) =>
      gs.map((g) => (g.bucket_id === bucketId ? { ...g, ...patch } : g)),
    );
  }

  function removeGrant(bucketId: string) {
    setGrants((gs) => gs.filter((g) => g.bucket_id !== bucketId));
  }

  function resetForm() {
    setName("");
    setAllowCreate(false);
    setGrants([]);
    setGrantBucket("");
  }

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setCreating(true);
    const body: CreateAccessKeyBody = {
      name: name.trim(),
      allow_create_bucket: allowCreate,
      grants: grants.map((g) => ({
        bucket_id: g.bucket_id,
        read: g.owner || g.read,
        write: g.owner || g.write,
        owner: g.owner,
      })),
    };
    try {
      const created = await apiSend<CreatedAccessKey>("POST", "/access-keys", body);
      setCreateOpen(false);
      resetForm();
      setCreatedSecret(created);
      await loadKeys();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create access key");
    } finally {
      setCreating(false);
    }
  }

  async function onDelete() {
    if (!deleting) return;
    setDeletePending(true);
    try {
      await apiSend<void>("DELETE", `/access-keys/${deleting.id}`);
      toast.success(`Access key "${deleting.name}" deleted`);
      setDeleting(null);
      await loadKeys();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete access key");
    } finally {
      setDeletePending(false);
    }
  }

  const availableBuckets = buckets.filter(
    (b) => !grants.some((g) => g.bucket_id === b.id) && bucketSupportsKeys(b),
  );

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Access Keys" }]}
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="size-4" />
            Create key
          </Button>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
        {keys === null ? (
          <Skeleton className="h-64 w-full" />
        ) : keys.length === 0 ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <KeyRound className="size-4" /> No access keys yet
              </CardTitle>
              <CardDescription>
                Create an S3 access key to grant programmatic access to your buckets.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="size-4" />
                Create key
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
                    <TableHead>Access key ID</TableHead>
                    <TableHead>Grants</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead className="w-12" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {keys.map((k) => (
                    <TableRow key={k.id}>
                      <TableCell className="font-medium">
                        {k.name}
                        {k.can_create_bucket && (
                          <Badge variant="outline" className="ml-2 font-normal">
                            can create buckets
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="font-mono text-xs">{k.access_key_id}</TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {k.grants.length === 0 ? (
                            <span className="text-muted-foreground text-xs">No grants</span>
                          ) : (
                            k.grants.map((g) => (
                              <Badge key={g.bucket_id} variant="secondary" className="font-normal">
                                {g.bucket_name ?? bucketName(g.bucket_id)}: {grantLabel(g)}
                              </Badge>
                            ))
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {formatDateShort(k.created_at)}
                      </TableCell>
                      <TableCell>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          onClick={() => setDeleting(k)}
                        >
                          <Trash2 className="size-4" />
                          <span className="sr-only">Delete</span>
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Create key dialog */}
      <Dialog
        open={createOpen}
        onOpenChange={(o) => {
          setCreateOpen(o);
          if (!o) resetForm();
        }}
      >
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
          <form onSubmit={onCreate}>
            <DialogHeader>
              <DialogTitle>Create access key</DialogTitle>
              <DialogDescription>
                The secret is shown only once after creation — store it securely.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="key-name">Name</Label>
                <Input
                  id="key-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="ci-deploy"
                  required
                  autoFocus
                />
              </div>

              <div className="flex items-center justify-between rounded-lg border p-3">
                <div className="flex flex-col gap-0.5">
                  <Label htmlFor="allow-create" className="text-sm font-medium">
                    Allow creating buckets
                  </Label>
                  <span className="text-muted-foreground text-xs">
                    Lets this key create new buckets via the S3 API.
                  </span>
                </div>
                <Switch id="allow-create" checked={allowCreate} onCheckedChange={setAllowCreate} />
              </div>

              <div className="flex flex-col gap-2">
                <Label>Bucket grants</Label>
                <div className="flex gap-2">
                  <Select value={grantBucket} onValueChange={setGrantBucket}>
                    <SelectTrigger className="flex-1">
                      <SelectValue placeholder="Choose a bucket…" />
                    </SelectTrigger>
                    <SelectContent>
                      {availableBuckets.length === 0 ? (
                        <SelectItem value="__none" disabled>
                          No more buckets
                        </SelectItem>
                      ) : (
                        availableBuckets.map((b) => (
                          <SelectItem key={b.id} value={b.id}>
                            {b.name}
                          </SelectItem>
                        ))
                      )}
                    </SelectContent>
                  </Select>
                  <Button type="button" variant="secondary" onClick={addGrant} disabled={!grantBucket}>
                    Add
                  </Button>
                </div>

                {grants.length > 0 && (
                  <div className="flex flex-col gap-2 rounded-lg border p-2">
                    {grants.map((g) => (
                      <div
                        key={g.bucket_id}
                        className="flex flex-wrap items-center justify-between gap-3 rounded-md px-2 py-1.5"
                      >
                        <span className="text-sm font-medium">{bucketName(g.bucket_id)}</span>
                        <div className="flex items-center gap-4">
                          <label className="flex items-center gap-1.5 text-sm">
                            <Checkbox
                              checked={g.owner || g.read}
                              disabled={g.owner}
                              onCheckedChange={(c) => updateGrant(g.bucket_id, { read: c === true })}
                            />
                            Read
                          </label>
                          <label className="flex items-center gap-1.5 text-sm">
                            <Checkbox
                              checked={g.owner || g.write}
                              disabled={g.owner}
                              onCheckedChange={(c) => updateGrant(g.bucket_id, { write: c === true })}
                            />
                            Write
                          </label>
                          <label className="flex items-center gap-1.5 text-sm">
                            <Checkbox
                              checked={g.owner}
                              onCheckedChange={(c) => updateGrant(g.bucket_id, { owner: c === true })}
                            />
                            Owner
                          </label>
                          <Button
                            type="button"
                            variant="ghost"
                            size="icon"
                            className="size-7"
                            onClick={() => removeGrant(g.bucket_id)}
                          >
                            <Trash2 className="size-3.5" />
                            <span className="sr-only">Remove grant</span>
                          </Button>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={creating || !name.trim()}>
                {creating && <Loader2 className="size-4 animate-spin" />}
                Create key
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Secret-once dialog */}
      <SecretDialog
        created={createdSecret}
        onClose={() => setCreatedSecret(null)}
      />

      {/* Delete confirmation */}
      <AlertDialog open={!!deleting} onOpenChange={(o) => !o && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete access key “{deleting?.name}”?</AlertDialogTitle>
            <AlertDialogDescription>
              Applications using this key will immediately lose access. This action cannot be
              undone.
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

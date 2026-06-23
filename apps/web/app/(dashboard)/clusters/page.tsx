"use client";

import * as React from "react";
import Link from "next/link";
import { Check, HardDrive, Loader2, Minus, Plus, Settings, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  addCluster,
  listClusters,
  removeCluster,
  ApiError,
  type AddClusterBody,
  type Cluster,
  type ClusterProvider,
} from "@/lib/api";
import {
  ADDABLE_PROVIDERS,
  CAPABILITY_ROWS,
  defaultRegion,
  endpointRequired,
  providerLabel,
} from "@/lib/clusters";
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

function statusVariant(status: string): "default" | "secondary" | "destructive" {
  const s = status.toLowerCase();
  if (s === "healthy" || s === "ok") return "default";
  if (s === "degraded" || s === "unhealthy" || s === "error") return "destructive";
  return "secondary";
}

function CapabilityCell({ supported }: { supported: boolean }) {
  return supported ? (
    <span className="text-primary inline-flex items-center justify-center">
      <Check className="size-4" />
      <span className="sr-only">Supported</span>
    </span>
  ) : (
    <span className="text-muted-foreground inline-flex items-center justify-center">
      <Minus className="size-4" />
      <span className="sr-only">Not supported</span>
    </span>
  );
}

export default function ClustersPage() {
  const [clusters, setClusters] = React.useState<Cluster[] | null>(null);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [creating, setCreating] = React.useState(false);
  const [provider, setProvider] = React.useState<ClusterProvider>("aws_s3");
  const [name, setName] = React.useState("");
  const [endpoint, setEndpoint] = React.useState("");
  const [region, setRegion] = React.useState("");
  const [accessKeyId, setAccessKeyId] = React.useState("");
  const [secretAccessKey, setSecretAccessKey] = React.useState("");
  const [publicEndpoint, setPublicEndpoint] = React.useState("");

  const [removing, setRemoving] = React.useState<Cluster | null>(null);
  const [removePending, setRemovePending] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await listClusters();
      setClusters(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load storage backends");
      setClusters([]);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  function resetForm() {
    setProvider("aws_s3");
    setName("");
    setEndpoint("");
    setRegion("");
    setAccessKeyId("");
    setSecretAccessKey("");
    setPublicEndpoint("");
  }

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setCreating(true);
    const body: AddClusterBody = {
      name: name.trim(),
      provider,
      s3_endpoint: endpoint.trim(),
      s3_region: region.trim(),
      access_key_id: accessKeyId.trim(),
      secret_access_key: secretAccessKey,
      public_endpoint: publicEndpoint.trim() || undefined,
    };
    try {
      const created = await addCluster(body);
      toast.success(`Backend "${created.name}" connected`);
      setCreateOpen(false);
      resetForm();
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to add storage backend");
    } finally {
      setCreating(false);
    }
  }

  async function onRemove() {
    if (!removing) return;
    setRemovePending(true);
    try {
      await removeCluster(removing.id);
      toast.success(`Backend "${removing.name}" removed`);
      setRemoving(null);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to remove storage backend");
    } finally {
      setRemovePending(false);
    }
  }

  const endpointIsRequired = endpointRequired(provider);

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Storage backends" }]}
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="size-4" />
            Add backend
          </Button>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
        {clusters === null ? (
          <Skeleton className="h-64 w-full" />
        ) : clusters.length === 0 ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <HardDrive className="size-4" /> No storage backends
              </CardTitle>
              <CardDescription>
                Connect an external S3-compatible backend to store buckets outside the primary
                cluster.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="size-4" />
                Add backend
              </Button>
            </CardContent>
          </Card>
        ) : (
          <>
            <Card>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Provider</TableHead>
                      <TableHead>Mode</TableHead>
                      <TableHead>Status</TableHead>
                      <TableHead className="text-right">Buckets</TableHead>
                      <TableHead className="w-20" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {clusters.map((c) => (
                      <TableRow key={c.id}>
                        <TableCell className="font-medium">
                          <span className="flex items-center gap-2">
                            <Link href={`/clusters/${c.id}`} className="hover:underline">
                              {c.name}
                            </Link>
                            {c.is_primary && (
                              <Badge variant="secondary" className="font-normal">
                                Primary
                              </Badge>
                            )}
                          </span>
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="font-normal">
                            {providerLabel(c.provider)}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <Badge variant="secondary" className="font-normal capitalize">
                            {c.mode}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <Badge variant={statusVariant(c.status)} className="font-normal capitalize">
                            {c.status}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-right tabular-nums">{c.bucket_count}</TableCell>
                        <TableCell>
                          <div className="flex items-center justify-end gap-1">
                            <Button variant="ghost" size="icon" className="size-8" asChild>
                              <Link href={`/clusters/${c.id}`}>
                                <Settings className="size-4" />
                                <span className="sr-only">Manage backend</span>
                              </Link>
                            </Button>
                            <Button
                              variant="ghost"
                              size="icon"
                              className="size-8"
                              disabled={c.is_primary}
                              onClick={() => setRemoving(c)}
                            >
                              <Trash2 className="size-4" />
                              <span className="sr-only">Remove backend</span>
                            </Button>
                          </div>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle className="text-base">Capabilities</CardTitle>
                <CardDescription>
                  Which control-plane features each backend supports. Unsupported features are
                  hidden or disabled for buckets on that backend.
                </CardDescription>
              </CardHeader>
              <CardContent className="p-0">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Capability</TableHead>
                      {clusters.map((c) => (
                        <TableHead key={c.id} className="text-center">
                          {c.name}
                        </TableHead>
                      ))}
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {CAPABILITY_ROWS.map((row) => (
                      <TableRow key={row.key}>
                        <TableCell className="font-medium">{row.label}</TableCell>
                        {clusters.map((c) => (
                          <TableCell key={c.id} className="text-center">
                            <CapabilityCell supported={c.capabilities[row.key]} />
                          </TableCell>
                        ))}
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </CardContent>
            </Card>
          </>
        )}
      </div>

      {/* Add backend dialog */}
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
              <DialogTitle>Add storage backend</DialogTitle>
              <DialogDescription>
                Connect an external S3-compatible backend. The secret is used once to verify the
                connection and is never shown again.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="cluster-provider">Provider</Label>
                <Select
                  value={provider}
                  onValueChange={(v) => setProvider(v as ClusterProvider)}
                >
                  <SelectTrigger id="cluster-provider">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {ADDABLE_PROVIDERS.map((p) => (
                      <SelectItem key={p.value} value={p.value}>
                        {p.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="cluster-name">Name</Label>
                <Input
                  id="cluster-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="prod-aws"
                  required
                  autoFocus
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="cluster-endpoint">
                  S3 endpoint{endpointIsRequired ? "" : " (optional)"}
                </Label>
                <Input
                  id="cluster-endpoint"
                  value={endpoint}
                  onChange={(e) => setEndpoint(e.target.value)}
                  placeholder="https://s3.example.com"
                  required={endpointIsRequired}
                  className="font-mono text-xs"
                />
                <span className="text-muted-foreground text-xs">
                  {endpointIsRequired
                    ? "Required for this provider."
                    : "Optional for AWS S3 — leave blank to use the regional default."}
                </span>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="cluster-region">Region (optional)</Label>
                <Input
                  id="cluster-region"
                  value={region}
                  onChange={(e) => setRegion(e.target.value)}
                  placeholder={defaultRegion(provider)}
                />
                <span className="text-muted-foreground text-xs">
                  Defaults to <span className="font-mono">{defaultRegion(provider)}</span> when left
                  blank.
                </span>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="cluster-access-key">Access key ID</Label>
                <Input
                  id="cluster-access-key"
                  value={accessKeyId}
                  onChange={(e) => setAccessKeyId(e.target.value)}
                  placeholder="AKIA…"
                  required
                  className="font-mono text-xs"
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="cluster-secret-key">Secret access key</Label>
                <Input
                  id="cluster-secret-key"
                  type="password"
                  value={secretAccessKey}
                  onChange={(e) => setSecretAccessKey(e.target.value)}
                  placeholder="••••••••"
                  required
                  className="font-mono text-xs"
                />
                <span className="text-muted-foreground text-xs">
                  Sent once to verify and store the connection — it is never returned.
                </span>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="cluster-public-endpoint">Public endpoint (optional)</Label>
                <Input
                  id="cluster-public-endpoint"
                  value={publicEndpoint}
                  onChange={(e) => setPublicEndpoint(e.target.value)}
                  placeholder="https://cdn.example.com"
                  className="font-mono text-xs"
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={
                  creating ||
                  !name.trim() ||
                  !accessKeyId.trim() ||
                  !secretAccessKey ||
                  (endpointIsRequired && !endpoint.trim())
                }
              >
                {creating && <Loader2 className="size-4 animate-spin" />}
                Add backend
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Remove confirmation */}
      <AlertDialog open={!!removing} onOpenChange={(o) => !o && setRemoving(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove backend “{removing?.name}”?</AlertDialogTitle>
            <AlertDialogDescription>
              This disconnects the backend from the control plane. Buckets must be removed first.
              This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removePending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                onRemove();
              }}
              disabled={removePending}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {removePending && <Loader2 className="size-4 animate-spin" />}
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

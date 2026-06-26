"use client";

import * as React from "react";
import { useSearchParams } from "next/navigation";
import { HardDrive, Info, Loader2, Plus, Server, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  addNode,
  getCluster,
  getLayout,
  listNodes,
  previewLayout,
  removeNode,
  revertLayout,
  ApiError,
  type AddNodeBody,
  type Cluster,
  type ClusterLayout,
  type ClusterNode,
} from "@/lib/api";
import { providerLabel } from "@/lib/clusters";
import { formatBytes } from "@/lib/format";
import { PageHeader } from "@/app/(dashboard)/page-header";
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/components/ui/alert";
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
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const GIB = 1024 * 1024 * 1024;

/** "not_supported_on_backend" maps a 422 to the generic-S3 read-only notice. */
function isNotSupported(err: unknown): boolean {
  return err instanceof ApiError && err.status === 422;
}

function statusVariant(status: string): "default" | "secondary" | "destructive" {
  const s = status.toLowerCase();
  if (s === "healthy" || s === "ok") return "default";
  if (s === "degraded" || s === "unhealthy" || s === "error") return "destructive";
  return "secondary";
}

function roleLabel(role: ClusterNode["role"]): string {
  if (role === "storage") return "Storage";
  if (role === "gateway") return "Gateway";
  return "—";
}

export default function ClusterDetailPage() {
  // useSearchParams must be read inside a Suspense boundary for static export.
  return (
    <React.Suspense fallback={<div className="flex flex-1 flex-col gap-4 p-4 md:p-6" />}>
      <ClusterDetail />
    </React.Suspense>
  );
}

function ClusterDetail() {
  const id = useSearchParams().get("id") ?? "";

  const [cluster, setCluster] = React.useState<Cluster | null>(null);
  const [notFound, setNotFound] = React.useState(false);

  const [nodes, setNodes] = React.useState<ClusterNode[] | null>(null);
  const [layout, setLayout] = React.useState<ClusterLayout | null>(null);
  /** True when the backend reports no node topology (generic S3 / 422). */
  const [topologyUnsupported, setTopologyUnsupported] = React.useState(false);

  const [addOpen, setAddOpen] = React.useState(false);
  const [adding, setAdding] = React.useState(false);
  const [nodeId, setNodeId] = React.useState("");
  const [peer, setPeer] = React.useState("");
  const [zone, setZone] = React.useState("dc1");
  const [capacityGib, setCapacityGib] = React.useState("100");

  const [removingNode, setRemovingNode] = React.useState<ClusterNode | null>(null);
  const [removePending, setRemovePending] = React.useState(false);

  const [previewLines, setPreviewLines] = React.useState<string[] | null>(null);
  const [previewPending, setPreviewPending] = React.useState(false);
  const [revertOpen, setRevertOpen] = React.useState(false);
  const [revertPending, setRevertPending] = React.useState(false);

  const hasHealth = cluster?.capabilities.has_cluster_health ?? false;

  const loadCluster = React.useCallback(async () => {
    try {
      const c = await getCluster(id);
      setCluster(c);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setNotFound(true);
      } else {
        toast.error(err instanceof ApiError ? err.message : "Failed to load cluster");
      }
    }
  }, [id]);

  const loadTopology = React.useCallback(async () => {
    try {
      const [nodesRes, layoutRes] = await Promise.all([listNodes(id), getLayout(id)]);
      setNodes(nodesRes.data);
      setLayout(layoutRes);
      setTopologyUnsupported(false);
    } catch (err) {
      if (isNotSupported(err)) {
        setTopologyUnsupported(true);
        setNodes([]);
        setLayout(null);
        return;
      }
      toast.error(err instanceof ApiError ? err.message : "Failed to load node topology");
      setNodes([]);
    }
  }, [id]);

  React.useEffect(() => {
    loadCluster();
  }, [loadCluster]);

  React.useEffect(() => {
    if (hasHealth) loadTopology();
  }, [hasHealth, loadTopology]);

  function resetAddForm() {
    setNodeId("");
    setPeer("");
    setZone("dc1");
    setCapacityGib("100");
  }

  async function onAddNode(e: React.FormEvent) {
    e.preventDefault();
    const gib = Number(capacityGib);
    const bytes = Number.isFinite(gib) && gib > 0 ? Math.round(gib * GIB) : 0;
    const body: AddNodeBody = {
      node_id: nodeId.trim(),
      peer: peer.trim() || undefined,
      zone: zone.trim() || undefined,
      capacity_bytes: bytes,
    };
    setAdding(true);
    try {
      await addNode(id, body);
      toast.success("Node added — re-balancing runs in the background");
      setAddOpen(false);
      resetAddForm();
      await loadTopology();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to add node");
    } finally {
      setAdding(false);
    }
  }

  async function onRemoveNode() {
    if (!removingNode) return;
    setRemovePending(true);
    try {
      await removeNode(id, removingNode.id);
      toast.success("Node draining — it will be removed once data has moved off");
      setRemovingNode(null);
      await loadTopology();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to remove node");
    } finally {
      setRemovePending(false);
    }
  }

  async function onPreview() {
    setPreviewPending(true);
    try {
      const res = await previewLayout(id);
      setPreviewLines(res.message ?? []);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to preview layout");
    } finally {
      setPreviewPending(false);
    }
  }

  async function onRevert() {
    setRevertPending(true);
    try {
      await revertLayout(id);
      toast.success("Staged layout changes reverted");
      setRevertOpen(false);
      setPreviewLines(null);
      await loadTopology();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to revert layout");
    } finally {
      setRevertPending(false);
    }
  }

  const stagedChanges = layout?.staged_role_changes ?? [];
  const hasStaged = stagedChanges.length > 0;

  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Storage backends", href: "/clusters" },
          { label: cluster?.name ?? "…" },
        ]}
        actions={
          cluster && hasHealth && !topologyUnsupported ? (
            <Button size="sm" onClick={() => setAddOpen(true)}>
              <Plus className="size-4" />
              Add node
            </Button>
          ) : undefined
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
        {notFound ? (
          <div className="text-muted-foreground text-sm">Cluster not found.</div>
        ) : !cluster ? (
          <Skeleton className="h-96 w-full" />
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-3">
              <h2 className="text-xl font-semibold">{cluster.name}</h2>
              <Badge variant="outline" className="font-normal">
                {providerLabel(cluster.provider)}
              </Badge>
              <Badge variant="secondary" className="font-normal capitalize">
                {cluster.mode}
              </Badge>
              {cluster.is_primary && (
                <Badge variant="secondary" className="font-normal">
                  Primary
                </Badge>
              )}
              <Badge variant={statusVariant(cluster.status)} className="font-normal capitalize">
                {cluster.status}
              </Badge>
            </div>

            {!hasHealth ? (
              <Alert>
                <Info className="size-4" />
                <AlertTitle>No node topology</AlertTitle>
                <AlertDescription>
                  This backend is a single S3 endpoint with no node topology to manage. Nodes and
                  layout are only available for Garage clusters.
                </AlertDescription>
              </Alert>
            ) : topologyUnsupported ? (
              <Alert>
                <Info className="size-4" />
                <AlertTitle>No node topology</AlertTitle>
                <AlertDescription>
                  This backend does not expose node-level topology, so there is nothing to manage
                  here.
                </AlertDescription>
              </Alert>
            ) : (
              <>
                {/* Nodes */}
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">Nodes</CardTitle>
                    <CardDescription>
                      Storage and gateway nodes that make up this cluster.
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-0">
                    {nodes === null ? (
                      <div className="p-4">
                        <Skeleton className="h-40 w-full" />
                      </div>
                    ) : nodes.length === 0 ? (
                      <div className="text-muted-foreground flex flex-col items-center gap-2 p-8 text-sm">
                        <Server className="size-5" />
                        No nodes in this cluster yet.
                      </div>
                    ) : (
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>ID</TableHead>
                            <TableHead>Hostname</TableHead>
                            <TableHead>Zone</TableHead>
                            <TableHead>Role</TableHead>
                            <TableHead>State</TableHead>
                            <TableHead className="text-right">Capacity</TableHead>
                            <TableHead className="text-right">Disk used / total</TableHead>
                            <TableHead className="w-12" />
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {nodes.map((n) => {
                            const diskUsed = n.disk_total_bytes - n.disk_avail_bytes;
                            return (
                              <TableRow key={n.id}>
                                <TableCell className="font-mono text-xs">
                                  {n.id.slice(0, 12)}
                                </TableCell>
                                <TableCell className="font-medium">{n.hostname || "—"}</TableCell>
                                <TableCell>
                                  <Badge variant="outline" className="font-normal">
                                    {n.zone || "—"}
                                  </Badge>
                                </TableCell>
                                <TableCell>
                                  <Badge variant="secondary" className="font-normal">
                                    {roleLabel(n.role)}
                                  </Badge>
                                </TableCell>
                                <TableCell>
                                  <div className="flex flex-wrap gap-1">
                                    <Badge
                                      variant={n.is_up ? "default" : "destructive"}
                                      className="font-normal"
                                    >
                                      {n.is_up ? "Up" : "Down"}
                                    </Badge>
                                    {n.draining && (
                                      <Badge variant="secondary" className="font-normal">
                                        Draining
                                      </Badge>
                                    )}
                                  </div>
                                </TableCell>
                                <TableCell className="text-right tabular-nums">
                                  {n.capacity_bytes === null
                                    ? "gateway"
                                    : formatBytes(n.capacity_bytes)}
                                </TableCell>
                                <TableCell className="text-right tabular-nums">
                                  {formatBytes(diskUsed)} / {formatBytes(n.disk_total_bytes)}
                                </TableCell>
                                <TableCell>
                                  <Button
                                    variant="ghost"
                                    size="icon"
                                    className="size-8"
                                    disabled={n.draining}
                                    onClick={() => setRemovingNode(n)}
                                  >
                                    <Trash2 className="size-4" />
                                    <span className="sr-only">Drain and remove node</span>
                                  </Button>
                                </TableCell>
                              </TableRow>
                            );
                          })}
                        </TableBody>
                      </Table>
                    )}
                  </CardContent>
                </Card>

                {/* Layout */}
                <Card>
                  <CardHeader>
                    <CardTitle className="flex items-center gap-2 text-base">
                      <HardDrive className="size-4" /> Layout
                      {layout && (
                        <Badge variant="outline" className="font-normal">
                          v{layout.version}
                        </Badge>
                      )}
                    </CardTitle>
                    <CardDescription>
                      The applied role assignments that determine how data is distributed.
                    </CardDescription>
                  </CardHeader>
                  <CardContent className="p-0">
                    {layout === null ? (
                      <div className="p-4">
                        <Skeleton className="h-24 w-full" />
                      </div>
                    ) : layout.roles.length === 0 ? (
                      <div className="text-muted-foreground p-8 text-center text-sm">
                        No roles assigned yet.
                      </div>
                    ) : (
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>Node ID</TableHead>
                            <TableHead>Zone</TableHead>
                            <TableHead className="text-right">Capacity</TableHead>
                            <TableHead>Tags</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {layout.roles.map((r) => (
                            <TableRow key={r.id}>
                              <TableCell className="font-mono text-xs">{r.id.slice(0, 12)}</TableCell>
                              <TableCell>
                                <Badge variant="outline" className="font-normal">
                                  {r.zone || "—"}
                                </Badge>
                              </TableCell>
                              <TableCell className="text-right tabular-nums">
                                {r.capacity === null ? "gateway" : formatBytes(r.capacity)}
                              </TableCell>
                              <TableCell>
                                <div className="flex flex-wrap gap-1">
                                  {r.tags.length === 0 ? (
                                    <span className="text-muted-foreground text-xs">—</span>
                                  ) : (
                                    r.tags.map((t) => (
                                      <Badge key={t} variant="secondary" className="font-normal">
                                        {t}
                                      </Badge>
                                    ))
                                  )}
                                </div>
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    )}
                  </CardContent>

                  {hasStaged && (
                    <CardContent className="flex flex-col gap-3 border-t pt-6">
                      <div className="flex flex-wrap items-center justify-between gap-2">
                        <div className="flex items-center gap-2 text-sm font-medium">
                          Staged changes
                          <Badge variant="secondary" className="font-normal">
                            {stagedChanges.length}
                          </Badge>
                        </div>
                        <div className="flex items-center gap-2">
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={onPreview}
                            disabled={previewPending}
                          >
                            {previewPending && <Loader2 className="size-4 animate-spin" />}
                            Preview
                          </Button>
                          <Button
                            variant="outline"
                            size="sm"
                            onClick={() => setRevertOpen(true)}
                          >
                            Revert staged
                          </Button>
                        </div>
                      </div>
                      {previewLines !== null && (
                        <Alert>
                          <Info className="size-4" />
                          <AlertTitle>Layout preview</AlertTitle>
                          <AlertDescription>
                            {previewLines.length === 0 ? (
                              <span>No changes to preview.</span>
                            ) : (
                              <ul className="list-inside list-disc">
                                {previewLines.map((line, i) => (
                                  <li key={i} className="font-mono text-xs">
                                    {line}
                                  </li>
                                ))}
                              </ul>
                            )}
                          </AlertDescription>
                        </Alert>
                      )}
                    </CardContent>
                  )}
                </Card>
              </>
            )}
          </>
        )}
      </div>

      {/* Add node dialog */}
      <Dialog
        open={addOpen}
        onOpenChange={(o) => {
          setAddOpen(o);
          if (!o) resetAddForm();
        }}
      >
        <DialogContent className="max-h-[90vh] overflow-y-auto sm:max-w-lg">
          <form onSubmit={onAddNode}>
            <DialogHeader>
              <DialogTitle>Add node</DialogTitle>
              <DialogDescription>
                Assign a Garage node a role in this cluster. Re-balancing runs on the backend after
                the layout is applied.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="node-id">Node ID</Label>
                <Input
                  id="node-id"
                  value={nodeId}
                  onChange={(e) => setNodeId(e.target.value)}
                  placeholder="Garage node id"
                  required
                  autoFocus
                  className="font-mono text-xs"
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="node-peer">Peer (optional)</Label>
                <Input
                  id="node-peer"
                  value={peer}
                  onChange={(e) => setPeer(e.target.value)}
                  placeholder="<node_id>@host:port"
                  className="font-mono text-xs"
                />
                <span className="text-muted-foreground text-xs">
                  Provide <span className="font-mono">&lt;node_id&gt;@host:port</span> to connect a
                  new peer before assigning it.
                </span>
              </div>

              <div className="grid gap-2">
                <Label htmlFor="node-zone">Zone</Label>
                <Input
                  id="node-zone"
                  value={zone}
                  onChange={(e) => setZone(e.target.value)}
                  placeholder="dc1"
                />
              </div>

              <div className="grid gap-2">
                <Label htmlFor="node-capacity">Capacity (GiB)</Label>
                <Input
                  id="node-capacity"
                  type="number"
                  min="0"
                  value={capacityGib}
                  onChange={(e) => setCapacityGib(e.target.value)}
                  placeholder="100"
                />
                <span className="text-muted-foreground text-xs">
                  Use 0 for a gateway node (no storage capacity).
                </span>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setAddOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={adding || !nodeId.trim()}>
                {adding && <Loader2 className="size-4 animate-spin" />}
                Add node
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Drain/remove node confirmation */}
      <AlertDialog open={!!removingNode} onOpenChange={(o) => !o && setRemovingNode(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Drain and remove this node?</AlertDialogTitle>
            <AlertDialogDescription>
              Data is moved off{" "}
              <span className="font-mono">{removingNode?.hostname || removingNode?.id.slice(0, 12)}</span>{" "}
              before it leaves the cluster. This can take a while and cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removePending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                onRemoveNode();
              }}
              disabled={removePending}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {removePending && <Loader2 className="size-4 animate-spin" />}
              Drain &amp; remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Revert staged layout confirmation */}
      <AlertDialog open={revertOpen} onOpenChange={(o) => !revertPending && setRevertOpen(o)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Revert staged layout changes?</AlertDialogTitle>
            <AlertDialogDescription>
              This drops all pending staged role changes without applying them. The current applied
              layout is left untouched.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={revertPending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                onRevert();
              }}
              disabled={revertPending}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {revertPending && <Loader2 className="size-4 animate-spin" />}
              Revert
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

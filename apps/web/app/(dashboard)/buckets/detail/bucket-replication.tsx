"use client";

import * as React from "react";
import { ArrowRight, Loader2, Copy as CopyIcon } from "lucide-react";
import { toast } from "sonner";

import {
  apiGet,
  listReplications,
  startReplication,
  ApiError,
  type Bucket,
  type ListEnvelope,
  type ReplicationJob,
} from "@/lib/api";
import { formatBytes, formatDate, formatNumber } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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
  if (status === "completed") return "default";
  if (status === "failed" || status === "canceled") return "destructive";
  return "secondary";
}

export function BucketReplication({ bucketId }: { bucketId: string }) {
  const [buckets, setBuckets] = React.useState<Bucket[]>([]);
  const [dst, setDst] = React.useState("");
  const [jobs, setJobs] = React.useState<ReplicationJob[] | null>(null);
  const [starting, setStarting] = React.useState(false);

  const loadJobs = React.useCallback(async () => {
    try {
      const res = await listReplications(bucketId);
      setJobs(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load replication jobs");
      setJobs([]);
    }
  }, [bucketId]);

  React.useEffect(() => {
    apiGet<ListEnvelope<Bucket>>("/buckets")
      .then((res) => setBuckets(res.data.filter((b) => b.id !== bucketId)))
      .catch(() => {});
    loadJobs();
  }, [bucketId, loadJobs]);

  // Poll while a job is in flight so progress updates live.
  const anyRunning = (jobs ?? []).some((j) => j.status === "running" || j.status === "pending");
  React.useEffect(() => {
    if (!anyRunning) return;
    const t = setInterval(loadJobs, 3000);
    return () => clearInterval(t);
  }, [anyRunning, loadJobs]);

  async function start() {
    if (!dst) return;
    setStarting(true);
    try {
      await startReplication(bucketId, dst);
      toast.success("Replication started");
      setDst("");
      await loadJobs();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to start replication");
    } finally {
      setStarting(false);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Replicate to another bucket</CardTitle>
          <CardDescription>
            Copy this bucket&apos;s objects into another bucket — including one on a different
            backend (e.g. mirror to an off-site S3 provider). Re-running skips objects already
            present with the same size, so it acts as an incremental sync.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex flex-wrap items-end gap-3">
            <div className="grid min-w-64 flex-1 gap-2">
              <span className="text-sm font-medium">Destination bucket</span>
              <Select value={dst} onValueChange={setDst}>
                <SelectTrigger>
                  <SelectValue placeholder="Select a destination…" />
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
            <Button onClick={start} disabled={starting || !dst}>
              {starting ? <Loader2 className="size-4 animate-spin" /> : <CopyIcon className="size-4" />}
              Start replication
            </Button>
          </div>
          {buckets.length === 0 && (
            <p className="text-muted-foreground mt-3 text-xs">
              Create another bucket (optionally on a different backend) to replicate into.
            </p>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Replication jobs</CardTitle>
          <CardDescription>Recent runs from this bucket. Running jobs update live.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {jobs === null ? (
            <div className="p-6">
              <Skeleton className="h-24 w-full" />
            </div>
          ) : jobs.length === 0 ? (
            <p className="text-muted-foreground px-6 pb-6 text-sm">No replication jobs yet.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Destination</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead className="text-right">Copied</TableHead>
                  <TableHead className="text-right">Skipped</TableHead>
                  <TableHead className="text-right">Bytes</TableHead>
                  <TableHead>Updated</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {jobs.map((j) => {
                  const name = buckets.find((b) => b.id === j.dst_bucket_id)?.name;
                  return (
                    <TableRow key={j.id}>
                      <TableCell className="font-medium">
                        <span className="flex items-center gap-1.5">
                          <ArrowRight className="text-muted-foreground size-3.5" />
                          {name ?? <span className="font-mono text-xs">{j.dst_bucket_id}</span>}
                        </span>
                      </TableCell>
                      <TableCell>
                        <Badge variant={statusVariant(j.status)} className="font-normal capitalize">
                          {j.status === "running" && <Loader2 className="size-3 animate-spin" />}
                          {j.status}
                        </Badge>
                        {j.error && <p className="text-destructive mt-1 text-xs">{j.error}</p>}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatNumber(j.copied_objects)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatNumber(j.skipped_objects)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatBytes(j.copied_bytes)}
                      </TableCell>
                      <TableCell className="text-muted-foreground">{formatDate(j.updated_at)}</TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

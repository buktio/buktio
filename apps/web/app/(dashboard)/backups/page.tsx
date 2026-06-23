"use client";

import * as React from "react";
import {
  DatabaseBackup,
  Info,
  Loader2,
  RefreshCw,
  TriangleAlert,
} from "lucide-react";
import { toast } from "sonner";

import {
  createBackup,
  getBackup,
  listBackups,
  reconcileReport,
  ApiError,
  type BackupJob,
  type BackupStatus,
  type ReconcileReport,
} from "@/lib/api";
import { formatBytes, formatDate } from "@/lib/format";
import { useFeatures } from "@/app/(dashboard)/user-context";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { BackupSchedules } from "@/app/(dashboard)/backups/schedules";
import { CodeBlock } from "@/components/code-block";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const RESTORE_COMMAND = "buktio restore <file> --yes";

const TERMINAL: BackupStatus[] = ["succeeded", "failed"];
const POLL_INTERVAL_MS = 2000;
const POLL_ATTEMPTS = 10;

function statusVariant(status: BackupStatus): "default" | "secondary" | "destructive" {
  switch (status) {
    case "succeeded":
      return "default";
    case "failed":
      return "destructive";
    case "running":
    case "pending":
    default:
      return "secondary";
  }
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export default function BackupsPage() {
  const features = useFeatures();
  const [jobs, setJobs] = React.useState<BackupJob[] | null>(null);
  const [creating, setCreating] = React.useState(false);

  const [reconcile, setReconcile] = React.useState<ReconcileReport | null>(null);
  const [reconciling, setReconciling] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await listBackups();
      setJobs(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load backups");
      setJobs([]);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  async function onCreate() {
    setCreating(true);
    try {
      const started = await createBackup();
      toast.info("Backup started — this may take a moment.");
      await load();

      let job = started;
      for (let i = 0; i < POLL_ATTEMPTS && !TERMINAL.includes(job.status); i++) {
        await sleep(POLL_INTERVAL_MS);
        job = await getBackup(started.id);
      }
      await load();

      if (job.status === "succeeded") {
        toast.success(`Backup completed (${formatBytes(job.size_bytes)})`);
      } else if (job.status === "failed") {
        toast.error(job.error ? `Backup failed: ${job.error}` : "Backup failed");
      } else {
        toast.info("Backup is still running — refresh to check its status.");
      }
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create backup");
      await load();
    } finally {
      setCreating(false);
    }
  }

  async function onReconcile() {
    setReconciling(true);
    try {
      const report = await reconcileReport();
      setReconcile(report);
      if (report.in_sync) {
        toast.success("Database and storage backend are in sync");
      } else {
        toast.warning("Drift detected between database and storage backend");
      }
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to run reconcile");
    } finally {
      setReconciling(false);
    }
  }

  const dbOnly = reconcile?.buckets_only_in_db ?? [];
  const backendOnly = reconcile?.buckets_only_in_backend ?? [];

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Backups" }]}
        actions={
          <Button size="sm" onClick={onCreate} disabled={creating}>
            {creating ? (
              <Loader2 className="size-4 animate-spin" />
            ) : (
              <DatabaseBackup className="size-4" />
            )}
            Create backup
          </Button>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
        <Alert>
          <Info className="size-4" />
          <AlertTitle>What a backup includes</AlertTitle>
          <AlertDescription className="flex flex-col gap-2">
            <span>
              Backups capture control-plane <strong>metadata and config only</strong> (the
              PostgreSQL database). They do <strong>not</strong> include object data and do{" "}
              <strong>not</strong> include the master key — store those separately.
            </span>
            <span>
              Restoring is a destructive CLI operation that overwrites the current database. Run it
              on the server:
            </span>
            <CodeBlock code={RESTORE_COMMAND} className="mt-1" />
          </AlertDescription>
        </Alert>

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Recent backups</CardTitle>
            <CardDescription>Metadata + config snapshots, newest first.</CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            {jobs === null ? (
              <div className="p-6">
                <Skeleton className="h-48 w-full" />
              </div>
            ) : jobs.length === 0 ? (
              <div className="text-muted-foreground flex flex-col items-center gap-2 px-6 py-12 text-center text-sm">
                <DatabaseBackup className="size-6" />
                <p>No backups yet. Create one to capture the current metadata and config.</p>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Created</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead className="text-right">Size</TableHead>
                    <TableHead>Path</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {jobs.map((job) => (
                    <TableRow key={job.id}>
                      <TableCell className="text-muted-foreground whitespace-nowrap">
                        {formatDate(job.created_at)}
                      </TableCell>
                      <TableCell>
                        <div className="flex flex-col gap-1">
                          <Badge
                            variant={statusVariant(job.status)}
                            className="font-normal capitalize"
                          >
                            {job.status}
                          </Badge>
                          {job.status === "failed" && job.error && (
                            <span className="text-destructive text-xs">{job.error}</span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {job.status === "succeeded" ? formatBytes(job.size_bytes) : "—"}
                      </TableCell>
                      <TableCell className="text-muted-foreground font-mono text-xs break-all">
                        {job.path || "—"}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>

        {features.scheduled_backups && <BackupSchedules />}

        <Card>
          <CardHeader>
            <CardTitle className="text-base">Reconcile</CardTitle>
            <CardDescription>
              Compare the control-plane database against the storage backend to detect drift.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            <div>
              <Button variant="outline" size="sm" onClick={onReconcile} disabled={reconciling}>
                <RefreshCw className={reconciling ? "size-4 animate-spin" : "size-4"} />
                Run reconcile
              </Button>
            </div>
            {reconcile === null ? (
              <p className="text-muted-foreground text-sm">
                Run a reconcile to compare the database and the storage backend.
              </p>
            ) : reconcile.in_sync ? (
              <Alert>
                <Info className="size-4" />
                <AlertTitle>In sync</AlertTitle>
                <AlertDescription>
                  Every bucket in the database matches the storage backend.
                </AlertDescription>
              </Alert>
            ) : (
              <Alert variant="destructive">
                <TriangleAlert className="size-4" />
                <AlertTitle>Drift detected</AlertTitle>
                <AlertDescription className="flex flex-col gap-2">
                  {dbOnly.length > 0 && (
                    <span>
                      <span className="font-medium">Only in database:</span>{" "}
                      <span className="font-mono text-xs">{dbOnly.join(", ")}</span>
                    </span>
                  )}
                  {backendOnly.length > 0 && (
                    <span>
                      <span className="font-medium">Only in backend:</span>{" "}
                      <span className="font-mono text-xs">{backendOnly.join(", ")}</span>
                    </span>
                  )}
                </AlertDescription>
              </Alert>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  );
}

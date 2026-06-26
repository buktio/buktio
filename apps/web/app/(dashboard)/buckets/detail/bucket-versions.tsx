"use client";

import * as React from "react";
import { History, Loader2, RotateCcw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  getVersioning,
  setVersioning,
  listVersions,
  restoreVersion,
  deleteVersion,
  ApiError,
  type ObjectVersion,
} from "@/lib/api";
import { formatBytes, formatDate } from "@/lib/format";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Label } from "@/components/ui/label";
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

function fileName(key: string): string {
  const idx = key.lastIndexOf("/");
  return idx === -1 ? key : key.slice(idx + 1);
}

export function BucketVersions({ bucketId }: { bucketId: string }) {
  const [enabled, setEnabled] = React.useState<boolean | null>(null);
  const [toggling, setToggling] = React.useState(false);
  const [versions, setVersions] = React.useState<ObjectVersion[] | null>(null);
  const [busy, setBusy] = React.useState<string | null>(null);

  const loadVersions = React.useCallback(async () => {
    try {
      const res = await listVersions(bucketId);
      setVersions(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to list versions");
      setVersions([]);
    }
  }, [bucketId]);

  React.useEffect(() => {
    getVersioning(bucketId)
      .then((r) => setEnabled(r.enabled))
      .catch(() => setEnabled(false));
    loadVersions();
  }, [bucketId, loadVersions]);

  async function toggle(on: boolean) {
    setToggling(true);
    try {
      await setVersioning(bucketId, on);
      setEnabled(on);
      toast.success(on ? "Versioning enabled" : "Versioning suspended");
      await loadVersions();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to update versioning");
    } finally {
      setToggling(false);
    }
  }

  async function restore(v: ObjectVersion) {
    setBusy(v.version_id);
    try {
      await restoreVersion(bucketId, v.key, v.version_id);
      toast.success("Version restored as current");
      await loadVersions();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to restore version");
    } finally {
      setBusy(null);
    }
  }

  async function remove(v: ObjectVersion) {
    setBusy(v.version_id);
    try {
      await deleteVersion(bucketId, v.key, v.version_id);
      toast.success("Version deleted");
      await loadVersions();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete version");
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Object versioning</CardTitle>
          <CardDescription>
            When enabled, overwriting or deleting an object keeps the prior copy so you can restore
            it. Versions count toward storage until removed.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {enabled === null ? (
            <Skeleton className="h-10 w-full" />
          ) : (
            <div className="flex items-center justify-between rounded-lg border p-3">
              <Label htmlFor="versioning-switch" className="text-sm font-medium">
                Versioning {enabled ? "enabled" : "suspended"}
              </Label>
              <Switch
                id="versioning-switch"
                checked={enabled}
                disabled={toggling}
                onCheckedChange={toggle}
              />
            </div>
          )}
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Versions</CardTitle>
          <CardDescription>Every stored version and delete marker, newest first per object.</CardDescription>
        </CardHeader>
        <CardContent className="p-0">
          {versions === null ? (
            <div className="p-6">
              <Skeleton className="h-32 w-full" />
            </div>
          ) : versions.length === 0 ? (
            <div className="text-muted-foreground flex flex-col items-center gap-2 px-6 py-10 text-center text-sm">
              <History className="size-6" />
              No versions yet. Enable versioning, then overwrite or delete an object.
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Object</TableHead>
                  <TableHead>Version</TableHead>
                  <TableHead className="text-right">Size</TableHead>
                  <TableHead>Modified</TableHead>
                  <TableHead className="w-24" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {versions.map((v) => (
                  <TableRow key={`${v.key}@${v.version_id}`}>
                    <TableCell className="font-medium">
                      <span className="flex items-center gap-2">
                        {fileName(v.key)}
                        {v.is_latest && (
                          <Badge variant="secondary" className="font-normal">
                            Latest
                          </Badge>
                        )}
                        {v.is_delete_marker && (
                          <Badge variant="outline" className="font-normal">
                            Delete marker
                          </Badge>
                        )}
                      </span>
                    </TableCell>
                    <TableCell className="font-mono text-xs">{v.version_id}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {v.is_delete_marker ? "—" : formatBytes(v.size_bytes)}
                    </TableCell>
                    <TableCell className="text-muted-foreground">{formatDate(v.last_modified)}</TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        {!v.is_latest && !v.is_delete_marker && (
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-8"
                            disabled={busy === v.version_id}
                            onClick={() => restore(v)}
                          >
                            {busy === v.version_id ? (
                              <Loader2 className="size-4 animate-spin" />
                            ) : (
                              <RotateCcw className="size-4" />
                            )}
                            <span className="sr-only">Restore this version</span>
                          </Button>
                        )}
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          disabled={busy === v.version_id}
                          onClick={() => remove(v)}
                        >
                          <Trash2 className="size-4" />
                          <span className="sr-only">Delete this version</span>
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  );
}

"use client";

import * as React from "react";
import { Loader2, RefreshCw, RotateCcw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  listTrash,
  restoreTrash,
  purgeTrash,
  ApiError,
  type TrashItem,
} from "@/lib/api";
import { formatBytes, formatDate } from "@/lib/format";
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
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { cn } from "@/lib/utils";

function fileName(key: string): string {
  const idx = key.lastIndexOf("/");
  return idx === -1 ? key : key.slice(idx + 1);
}

export function BucketTrash({ bucketId }: { bucketId: string }) {
  const [items, setItems] = React.useState<TrashItem[] | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [restoringId, setRestoringId] = React.useState<string | null>(null);
  const [purgeTarget, setPurgeTarget] = React.useState<TrashItem | null>(null);
  const [purgePending, setPurgePending] = React.useState(false);

  const load = React.useCallback(async () => {
    setLoading(true);
    try {
      const res = await listTrash(bucketId);
      setItems(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load trash");
      setItems([]);
    } finally {
      setLoading(false);
    }
  }, [bucketId]);

  React.useEffect(() => {
    load();
  }, [load]);

  async function onRestore(item: TrashItem) {
    setRestoringId(item.id);
    try {
      await restoreTrash(bucketId, item.id);
      toast.success(`Restored ${fileName(item.key)}`);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to restore object");
    } finally {
      setRestoringId(null);
    }
  }

  async function onPurge() {
    if (!purgeTarget) return;
    setPurgePending(true);
    try {
      await purgeTrash(bucketId, purgeTarget.id);
      toast.success(`Permanently deleted ${fileName(purgeTarget.key)}`);
      setPurgeTarget(null);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete object");
    } finally {
      setPurgePending(false);
    }
  }

  const empty = !loading && items !== null && items.length === 0;

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-2">
        <p className="text-muted-foreground text-sm">
          Deleted objects are kept here and auto-purged after 7 days. Restore them or delete them
          permanently.
        </p>
        <Button variant="ghost" size="icon" className="size-8" onClick={load}>
          <RefreshCw className={cn("size-4", loading && "animate-spin")} />
          <span className="sr-only">Refresh</span>
        </Button>
      </div>

      <Card>
        <CardContent className="p-0">
          {loading ? (
            <div className="flex flex-col gap-2 p-4">
              {Array.from({ length: 3 }).map((_, i) => (
                <Skeleton key={i} className="h-8 w-full" />
              ))}
            </div>
          ) : empty ? (
            <div className="text-muted-foreground p-8 text-center text-sm">Trash is empty.</div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead className="text-right">Size</TableHead>
                  <TableHead>Deleted</TableHead>
                  <TableHead>Purges</TableHead>
                  <TableHead className="w-24" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {items?.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="font-medium">{fileName(item.key)}</TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatBytes(item.size_bytes)}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatDate(item.deleted_at)}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatDate(item.purge_after)}
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          disabled={restoringId === item.id}
                          onClick={() => onRestore(item)}
                        >
                          {restoringId === item.id ? (
                            <Loader2 className="size-4 animate-spin" />
                          ) : (
                            <RotateCcw className="size-4" />
                          )}
                          <span className="sr-only">Restore</span>
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          onClick={() => setPurgeTarget(item)}
                        >
                          <Trash2 className="size-4" />
                          <span className="sr-only">Delete forever</span>
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

      <AlertDialog open={!!purgeTarget} onOpenChange={(o) => !o && setPurgeTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Delete “{purgeTarget ? fileName(purgeTarget.key) : ""}” forever?
            </AlertDialogTitle>
            <AlertDialogDescription>
              This permanently removes the object from trash. It cannot be restored afterwards.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={purgePending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                onPurge();
              }}
              disabled={purgePending}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {purgePending && <Loader2 className="size-4 animate-spin" />}
              Delete forever
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

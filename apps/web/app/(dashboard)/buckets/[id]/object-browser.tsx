"use client";

import * as React from "react";
import {
  Copy,
  Download,
  File as FileIcon,
  Folder,
  Home,
  Loader2,
  Lock,
  PencilLine,
  RefreshCw,
  Search,
  Trash2,
  Upload,
  X,
} from "lucide-react";
import { toast } from "sonner";

import {
  apiGet,
  apiSend,
  apiUploadProgress,
  apiDownloadSSEC,
  copyObject,
  moveObject,
  presignUpload,
  uploadPresigned,
  random32B64,
  PRESIGN_THRESHOLD_BYTES,
  ApiError,
  objectContentUrl,
  objectUploadPath,
  type ObjectsResponse,
  type S3Object,
} from "@/lib/api";
import { formatBytes, formatDate } from "@/lib/format";
import { CopyButton } from "@/components/copy-button";
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
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "@/components/ui/breadcrumb";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
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
import { Progress } from "@/components/ui/progress";
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
import { TriangleAlert } from "lucide-react";
import { cn } from "@/lib/utils";

function lastSegment(prefix: string): string {
  const trimmed = prefix.endsWith("/") ? prefix.slice(0, -1) : prefix;
  const idx = trimmed.lastIndexOf("/");
  return idx === -1 ? trimmed : trimmed.slice(idx + 1);
}

function fileName(key: string): string {
  const idx = key.lastIndexOf("/");
  return idx === -1 ? key : key.slice(idx + 1);
}

interface UploadJob {
  name: string;
  pct: number;
  status: "uploading" | "done" | "error";
}

interface MoveDialogState {
  mode: "copy" | "move";
  srcKey: string;
  dst: string;
}

export function ObjectBrowser({
  bucketId,
  bucketName,
}: {
  bucketId: string;
  bucketName: string;
}) {
  const [prefix, setPrefix] = React.useState("");
  const [data, setData] = React.useState<ObjectsResponse | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [uploading, setUploading] = React.useState(false);
  const [uploadJobs, setUploadJobs] = React.useState<UploadJob[]>([]);
  const [dragOver, setDragOver] = React.useState(false);
  const [filter, setFilter] = React.useState("");
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [deleteKeys, setDeleteKeys] = React.useState<string[] | null>(null);
  const [deletePending, setDeletePending] = React.useState(false);
  const [moveState, setMoveState] = React.useState<MoveDialogState | null>(null);
  const [movePending, setMovePending] = React.useState(false);
  const fileInputRef = React.useRef<HTMLInputElement>(null);

  // SSE-C (client-side encryption) state.
  const [ssecEnabled, setSsecEnabled] = React.useState(false);
  const [ssecKey, setSsecKey] = React.useState("");

  const load = React.useCallback(
    async (p: string) => {
      setLoading(true);
      setSelected(new Set());
      try {
        const qs = new URLSearchParams({ delimiter: "/" });
        if (p) qs.set("prefix", p);
        const res = await apiGet<ObjectsResponse>(
          `/buckets/${bucketId}/objects?${qs.toString()}`,
        );
        setData(res);
      } catch (err) {
        toast.error(err instanceof ApiError ? err.message : "Failed to list objects");
        setData({ data: [], common_prefixes: [], is_truncated: false });
      } finally {
        setLoading(false);
      }
    },
    [bucketId],
  );

  React.useEffect(() => {
    load(prefix);
  }, [load, prefix]);

  const crumbs = React.useMemo(() => {
    if (!prefix) return [];
    const segments = prefix.replace(/\/$/, "").split("/").filter(Boolean);
    let acc = "";
    return segments.map((seg) => {
      acc += seg + "/";
      return { label: seg, prefix: acc };
    });
  }, [prefix]);

  function genSsecKey() {
    const k = random32B64();
    setSsecKey(k);
    return k;
  }

  function setJobProgress(name: string, pct: number, status: UploadJob["status"]) {
    setUploadJobs((jobs) =>
      jobs.map((j) => (j.name === name ? { ...j, pct, status } : j)),
    );
  }

  async function uploadFiles(files: FileList | File[]) {
    const list = Array.from(files);
    if (list.length === 0) return;

    // When SSE-C is on, require a key (generate one if absent).
    let key = ssecKey;
    if (ssecEnabled) {
      if (!key) key = genSsecKey();
    }

    setUploading(true);
    setUploadJobs(list.map((f) => ({ name: f.name, pct: 0, status: "uploading" })));
    let ok = 0;
    try {
      for (const file of list) {
        const objKey = prefix + file.name;
        try {
          if (ssecEnabled) {
            // SSE-C uploads must go through the API (header-based key).
            await apiUploadProgress(objectUploadPath(bucketId, objKey), file, {
              ssecKeyB64: key,
              onProgress: (pct) => setJobProgress(file.name, pct, "uploading"),
            });
          } else if (file.size > PRESIGN_THRESHOLD_BYTES) {
            // Large files: presign a PUT and upload directly to storage.
            const presigned = await presignUpload(bucketId, objKey, file.type);
            await uploadPresigned(presigned.url, file, (pct) =>
              setJobProgress(file.name, pct, "uploading"),
            );
          } else {
            await apiUploadProgress(objectUploadPath(bucketId, objKey), file, {
              onProgress: (pct) => setJobProgress(file.name, pct, "uploading"),
            });
          }
          setJobProgress(file.name, 100, "done");
          ok += 1;
        } catch (err) {
          setJobProgress(file.name, 0, "error");
          toast.error(
            err instanceof ApiError ? `${file.name}: ${err.message}` : `Failed to upload ${file.name}`,
          );
        }
      }
      if (ok > 0) toast.success(`Uploaded ${ok} file${ok === 1 ? "" : "s"}`);
      await load(prefix);
    } finally {
      setUploading(false);
      // Clear finished jobs shortly after.
      setTimeout(() => setUploadJobs([]), 2500);
    }
  }

  async function download(o: S3Object) {
    if (ssecEnabled) {
      let key = ssecKey;
      if (!key) {
        const entered = window.prompt(
          "This download requires the SSE-C key (base64). Paste it to decrypt:",
        );
        if (!entered) return;
        key = entered.trim();
      }
      const toastId = toast.loading(`Decrypting ${fileName(o.key)}…`);
      try {
        const blob = await apiDownloadSSEC(bucketId, o.key, key);
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = fileName(o.key);
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
        toast.success(`Downloaded ${fileName(o.key)}`, { id: toastId });
      } catch (err) {
        toast.error(
          err instanceof ApiError ? err.message : "Failed to download (wrong key?)",
          { id: toastId },
        );
      }
      return;
    }
    // Plain GET streams through the API (cookie sent automatically).
    window.open(objectContentUrl(bucketId, o.key), "_blank", "noopener,noreferrer");
  }

  async function performDelete() {
    if (!deleteKeys) return;
    setDeletePending(true);
    try {
      await apiSend<void>("DELETE", `/buckets/${bucketId}/objects`, { keys: deleteKeys });
      toast.success(
        `Moved ${deleteKeys.length} object${deleteKeys.length === 1 ? "" : "s"} to trash`,
      );
      setDeleteKeys(null);
      await load(prefix);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete objects");
    } finally {
      setDeletePending(false);
    }
  }

  async function performMove() {
    if (!moveState) return;
    const dst = moveState.dst.trim();
    if (!dst) {
      toast.error("Destination key is required");
      return;
    }
    if (dst === moveState.srcKey) {
      toast.error("Destination must differ from the source");
      return;
    }
    setMovePending(true);
    try {
      if (moveState.mode === "copy") {
        await copyObject(bucketId, moveState.srcKey, dst);
        toast.success("Object copied");
      } else {
        await moveObject(bucketId, moveState.srcKey, dst);
        toast.success("Object moved");
      }
      setMoveState(null);
      await load(prefix);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to update object");
    } finally {
      setMovePending(false);
    }
  }

  function toggleSelect(key: string, checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (checked) next.add(key);
      else next.delete(key);
      return next;
    });
  }

  const objects = React.useMemo(() => {
    const all = data?.data ?? [];
    const q = filter.trim().toLowerCase();
    if (!q) return all;
    return all.filter((o) => fileName(o.key).toLowerCase().includes(q));
  }, [data, filter]);

  const prefixes = React.useMemo(() => {
    const all = data?.common_prefixes ?? [];
    const q = filter.trim().toLowerCase();
    if (!q) return all;
    return all.filter((p) => lastSegment(p).toLowerCase().includes(q));
  }, [data, filter]);

  const allSelected = objects.length > 0 && objects.every((o) => selected.has(o.key));
  const empty = !loading && objects.length === 0 && prefixes.length === 0;

  return (
    <div className="flex flex-col gap-4">
      {/* Upload dropzone */}
      <Card
        className={cn(
          "border-dashed transition-colors",
          dragOver && "border-primary bg-accent/40",
        )}
        onDragOver={(e) => {
          e.preventDefault();
          setDragOver(true);
        }}
        onDragLeave={() => setDragOver(false)}
        onDrop={(e) => {
          e.preventDefault();
          setDragOver(false);
          if (e.dataTransfer.files.length) uploadFiles(e.dataTransfer.files);
        }}
      >
        <CardContent className="flex flex-col items-center justify-center gap-3 py-8 text-center">
          <Upload className="text-muted-foreground size-6" />
          <div className="text-sm">
            <span className="font-medium">Drag &amp; drop files here</span>
            <span className="text-muted-foreground">
              {" "}
              to upload into{" "}
              <span className="font-mono">{prefix || `${bucketName}/`}</span>
            </span>
          </div>
          <Button
            variant="secondary"
            size="sm"
            disabled={uploading}
            onClick={() => fileInputRef.current?.click()}
          >
            {uploading ? <Loader2 className="size-4 animate-spin" /> : <Upload className="size-4" />}
            Browse files
          </Button>
          <input
            ref={fileInputRef}
            type="file"
            multiple
            hidden
            onChange={(e) => {
              if (e.target.files?.length) uploadFiles(e.target.files);
              e.target.value = "";
            }}
          />

          {/* Advanced: client-side encryption (SSE-C) */}
          <div className="mt-2 w-full max-w-xl text-left">
            <div className="flex items-center justify-between rounded-lg border p-3">
              <div className="flex flex-col gap-0.5">
                <Label htmlFor="ssec-switch" className="flex items-center gap-1.5 text-sm font-medium">
                  <Lock className="size-3.5" />
                  Advanced: client-side encryption (SSE-C)
                </Label>
                <span className="text-muted-foreground text-xs">
                  Encrypt objects with a key only you hold. The same key is required to download.
                </span>
              </div>
              <Switch
                id="ssec-switch"
                checked={ssecEnabled}
                onCheckedChange={(c) => {
                  setSsecEnabled(c);
                  if (c && !ssecKey) genSsecKey();
                }}
              />
            </div>

            {ssecEnabled && (
              <div className="mt-3 flex flex-col gap-3">
                <div className="grid gap-2">
                  <Label htmlFor="ssec-key">Encryption key (base64, 32 bytes)</Label>
                  <div className="flex gap-2">
                    <Input
                      id="ssec-key"
                      value={ssecKey}
                      onChange={(e) => setSsecKey(e.target.value)}
                      placeholder="Paste an existing key, or generate one"
                      className="font-mono text-xs"
                    />
                    <CopyButton value={ssecKey} variant="outline" label="Copy key" />
                    <Button type="button" variant="outline" size="sm" onClick={genSsecKey}>
                      Generate
                    </Button>
                  </div>
                </div>
                <Alert variant="destructive">
                  <TriangleAlert className="size-4" />
                  <AlertTitle>Lose the key, lose the data</AlertTitle>
                  <AlertDescription>
                    buktio stores nothing about this key. Save it somewhere safe — without it,
                    encrypted objects can never be decrypted, by anyone.
                  </AlertDescription>
                </Alert>
              </div>
            )}
          </div>

          {/* Per-file upload progress */}
          {uploadJobs.length > 0 && (
            <div className="mt-2 flex w-full max-w-xl flex-col gap-2 text-left">
              {uploadJobs.map((j) => (
                <div key={j.name} className="flex flex-col gap-1">
                  <div className="flex items-center justify-between text-xs">
                    <span className="truncate font-medium">{j.name}</span>
                    <span
                      className={cn(
                        "tabular-nums",
                        j.status === "error" && "text-destructive",
                        j.status === "done" && "text-muted-foreground",
                      )}
                    >
                      {j.status === "error" ? "Failed" : j.status === "done" ? "Done" : `${j.pct}%`}
                    </span>
                  </div>
                  <Progress value={j.status === "error" ? 0 : j.pct} />
                </div>
              ))}
            </div>
          )}
        </CardContent>
      </Card>

      {/* Path + toolbar */}
      <div className="flex flex-wrap items-center justify-between gap-2">
        <Breadcrumb>
          <BreadcrumbList>
            <BreadcrumbItem>
              {prefix ? (
                <BreadcrumbLink asChild>
                  <button type="button" onClick={() => setPrefix("")} className="flex items-center gap-1">
                    <Home className="size-3.5" />
                    {bucketName}
                  </button>
                </BreadcrumbLink>
              ) : (
                <BreadcrumbPage className="flex items-center gap-1">
                  <Home className="size-3.5" />
                  {bucketName}
                </BreadcrumbPage>
              )}
            </BreadcrumbItem>
            {crumbs.map((c, i) => {
              const last = i === crumbs.length - 1;
              return (
                <React.Fragment key={c.prefix}>
                  <BreadcrumbSeparator />
                  <BreadcrumbItem>
                    {last ? (
                      <BreadcrumbPage>{c.label}</BreadcrumbPage>
                    ) : (
                      <BreadcrumbLink asChild>
                        <button type="button" onClick={() => setPrefix(c.prefix)}>
                          {c.label}
                        </button>
                      </BreadcrumbLink>
                    )}
                  </BreadcrumbItem>
                </React.Fragment>
              );
            })}
          </BreadcrumbList>
        </Breadcrumb>

        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="text-muted-foreground pointer-events-none absolute left-2.5 top-1/2 size-4 -translate-y-1/2" />
            <Input
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter by name…"
              className="h-8 w-48 pl-8"
            />
            {filter && (
              <Button
                variant="ghost"
                size="icon"
                className="absolute right-0.5 top-1/2 size-7 -translate-y-1/2"
                onClick={() => setFilter("")}
              >
                <X className="size-3.5" />
                <span className="sr-only">Clear filter</span>
              </Button>
            )}
          </div>
          {selected.size > 0 && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => setDeleteKeys(Array.from(selected))}
            >
              <Trash2 className="size-4" />
              Move to trash ({selected.size})
            </Button>
          )}
          <Button variant="ghost" size="icon" className="size-8" onClick={() => load(prefix)}>
            <RefreshCw className={cn("size-4", loading && "animate-spin")} />
            <span className="sr-only">Refresh</span>
          </Button>
        </div>
      </div>

      {/* Listing */}
      <Card>
        <CardContent className="p-0">
          {loading ? (
            <div className="flex flex-col gap-2 p-4">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-8 w-full" />
              ))}
            </div>
          ) : empty ? (
            <div className="text-muted-foreground p-8 text-center text-sm">
              {filter
                ? "No objects match your filter."
                : `This ${prefix ? "folder" : "bucket"} is empty.`}
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead className="w-10">
                    <Checkbox
                      checked={allSelected}
                      aria-label="Select all"
                      onCheckedChange={(c) => {
                        if (c) setSelected(new Set(objects.map((o) => o.key)));
                        else setSelected(new Set());
                      }}
                    />
                  </TableHead>
                  <TableHead>Name</TableHead>
                  <TableHead className="text-right">Size</TableHead>
                  <TableHead>Modified</TableHead>
                  <TableHead className="w-28" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {prefixes.map((p) => (
                  <TableRow
                    key={p}
                    className="cursor-pointer"
                    onClick={() => setPrefix(p)}
                  >
                    <TableCell />
                    <TableCell className="font-medium">
                      <span className="flex items-center gap-2">
                        <Folder className="text-muted-foreground size-4" />
                        {lastSegment(p)}/
                      </span>
                    </TableCell>
                    <TableCell className="text-muted-foreground text-right">—</TableCell>
                    <TableCell className="text-muted-foreground">—</TableCell>
                    <TableCell />
                  </TableRow>
                ))}
                {objects.map((o: S3Object) => (
                  <TableRow key={o.key}>
                    <TableCell onClick={(e) => e.stopPropagation()}>
                      <Checkbox
                        checked={selected.has(o.key)}
                        aria-label={`Select ${o.key}`}
                        onCheckedChange={(c) => toggleSelect(o.key, c === true)}
                      />
                    </TableCell>
                    <TableCell className="font-medium">
                      <span className="flex items-center gap-2">
                        <FileIcon className="text-muted-foreground size-4" />
                        {fileName(o.key)}
                      </span>
                    </TableCell>
                    <TableCell className="text-right tabular-nums">
                      {formatBytes(o.size_bytes)}
                    </TableCell>
                    <TableCell className="text-muted-foreground">
                      {formatDate(o.last_modified)}
                    </TableCell>
                    <TableCell>
                      <div className="flex justify-end gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          onClick={() => download(o)}
                        >
                          <Download className="size-4" />
                          <span className="sr-only">Download</span>
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          onClick={() => setMoveState({ mode: "copy", srcKey: o.key, dst: o.key })}
                        >
                          <Copy className="size-4" />
                          <span className="sr-only">Copy</span>
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          onClick={() => setMoveState({ mode: "move", srcKey: o.key, dst: o.key })}
                        >
                          <PencilLine className="size-4" />
                          <span className="sr-only">Rename or move</span>
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          onClick={() => setDeleteKeys([o.key])}
                        >
                          <Trash2 className="size-4" />
                          <span className="sr-only">Move to trash</span>
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

      {data?.is_truncated && (
        <p className="text-muted-foreground text-center text-xs">
          More objects exist than shown. Navigate into folders to narrow the listing.
        </p>
      )}

      {/* Copy / Move dialog */}
      <Dialog open={!!moveState} onOpenChange={(o) => !o && setMoveState(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>
              {moveState?.mode === "copy" ? "Copy object" : "Rename or move object"}
            </DialogTitle>
            <DialogDescription>
              {moveState?.mode === "copy"
                ? "Copy this object to a new key (the original is kept)."
                : "Move this object to a new key (acts as rename)."}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-2">
            <div className="grid gap-2">
              <Label>Source</Label>
              <Input readOnly value={moveState?.srcKey ?? ""} className="font-mono text-xs" />
            </div>
            <div className="grid gap-2">
              <Label htmlFor="dst-key">Destination key</Label>
              <Input
                id="dst-key"
                value={moveState?.dst ?? ""}
                onChange={(e) =>
                  setMoveState((s) => (s ? { ...s, dst: e.target.value } : s))
                }
                placeholder="folder/new-name.txt"
                className="font-mono text-xs"
                autoFocus
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setMoveState(null)} disabled={movePending}>
              Cancel
            </Button>
            <Button onClick={performMove} disabled={movePending}>
              {movePending && <Loader2 className="size-4 animate-spin" />}
              {moveState?.mode === "copy" ? "Copy" : "Move"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Move-to-trash confirmation */}
      <AlertDialog open={!!deleteKeys} onOpenChange={(o) => !o && setDeleteKeys(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              Move {deleteKeys?.length} object{deleteKeys?.length === 1 ? "" : "s"} to trash?
            </AlertDialogTitle>
            <AlertDialogDescription>
              The selected objects move to trash and auto-purge after 7 days. You can restore them
              from the Trash tab until then.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={deletePending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                performDelete();
              }}
              disabled={deletePending}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {deletePending && <Loader2 className="size-4 animate-spin" />}
              Move to trash
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  );
}

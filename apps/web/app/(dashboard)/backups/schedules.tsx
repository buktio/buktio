"use client";

import * as React from "react";
import { CalendarClock, Loader2, Pencil, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  createBackupSchedule,
  deleteBackupSchedule,
  listBackupSchedules,
  updateBackupSchedule,
  ApiError,
  type BackupSchedule,
} from "@/lib/api";
import { formatDate } from "@/lib/format";
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
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

/** Render an interval in minutes as a short human-readable cadence. */
function formatInterval(minutes: number): string {
  if (minutes % 1440 === 0) {
    const days = minutes / 1440;
    return days === 1 ? "Every day" : `Every ${days} days`;
  }
  if (minutes % 60 === 0) {
    const hours = minutes / 60;
    return hours === 1 ? "Every hour" : `Every ${hours} hours`;
  }
  return `Every ${minutes} min`;
}

interface DraftSchedule {
  interval_minutes: string;
  retention_count: string;
  offsite_enabled: boolean;
  enabled: boolean;
}

function toDraft(schedule?: BackupSchedule): DraftSchedule {
  return {
    interval_minutes: schedule ? String(schedule.interval_minutes) : "1440",
    retention_count: schedule ? String(schedule.retention_count) : "7",
    offsite_enabled: schedule ? schedule.offsite_enabled : false,
    enabled: schedule ? schedule.enabled : true,
  };
}

export function BackupSchedules() {
  const [schedules, setSchedules] = React.useState<BackupSchedule[] | null>(null);

  // null = closed; { schedule: null } = create; { schedule } = edit.
  const [editor, setEditor] = React.useState<{ schedule: BackupSchedule | null } | null>(null);
  const [draft, setDraft] = React.useState<DraftSchedule>(toDraft());
  const [saving, setSaving] = React.useState(false);

  const [deleting, setDeleting] = React.useState<BackupSchedule | null>(null);
  const [deletePending, setDeletePending] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await listBackupSchedules();
      setSchedules(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load schedules");
      setSchedules([]);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  function openCreate() {
    setDraft(toDraft());
    setEditor({ schedule: null });
  }

  function openEdit(schedule: BackupSchedule) {
    setDraft(toDraft(schedule));
    setEditor({ schedule });
  }

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    if (!editor) return;
    const interval = Number(draft.interval_minutes);
    const retention = Number(draft.retention_count);
    if (!Number.isFinite(interval) || interval <= 0) {
      toast.error("Interval must be a positive number of minutes");
      return;
    }
    if (!Number.isFinite(retention) || retention <= 0) {
      toast.error("Retention must be a positive count");
      return;
    }
    setSaving(true);
    try {
      if (editor.schedule) {
        await updateBackupSchedule(editor.schedule.id, {
          enabled: draft.enabled,
          interval_minutes: Math.round(interval),
          retention_count: Math.round(retention),
          offsite_enabled: draft.offsite_enabled,
        });
        toast.success("Schedule updated");
      } else {
        await createBackupSchedule({
          interval_minutes: Math.round(interval),
          retention_count: Math.round(retention),
          offsite_enabled: draft.offsite_enabled,
        });
        toast.success("Schedule created");
      }
      setEditor(null);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save schedule");
    } finally {
      setSaving(false);
    }
  }

  async function onToggleEnabled(schedule: BackupSchedule, enabled: boolean) {
    try {
      await updateBackupSchedule(schedule.id, {
        enabled,
        interval_minutes: schedule.interval_minutes,
        retention_count: schedule.retention_count,
        offsite_enabled: schedule.offsite_enabled,
      });
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to update schedule");
      await load();
    }
  }

  async function onDelete() {
    if (!deleting) return;
    setDeletePending(true);
    try {
      await deleteBackupSchedule(deleting.id);
      toast.success("Schedule deleted");
      setDeleting(null);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete schedule");
    } finally {
      setDeletePending(false);
    }
  }

  const isEdit = !!editor?.schedule;

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between gap-2 space-y-0">
        <div className="flex flex-col gap-1.5">
          <CardTitle className="text-base">Schedules</CardTitle>
          <CardDescription>Run metadata + config backups automatically on a cadence.</CardDescription>
        </div>
        <Button size="sm" variant="outline" onClick={openCreate}>
          <Plus className="size-4" />
          New schedule
        </Button>
      </CardHeader>
      <CardContent className="p-0">
        {schedules === null ? (
          <div className="p-6">
            <Skeleton className="h-32 w-full" />
          </div>
        ) : schedules.length === 0 ? (
          <div className="text-muted-foreground flex flex-col items-center gap-2 px-6 py-12 text-center text-sm">
            <CalendarClock className="size-6" />
            <p>No schedules yet. Create one to run backups automatically.</p>
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Interval</TableHead>
                <TableHead className="text-right">Retention</TableHead>
                <TableHead>Off-box</TableHead>
                <TableHead>Next run</TableHead>
                <TableHead>Last run</TableHead>
                <TableHead>Enabled</TableHead>
                <TableHead className="w-20" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {schedules.map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="font-medium">{formatInterval(s.interval_minutes)}</TableCell>
                  <TableCell className="text-right tabular-nums">{s.retention_count}</TableCell>
                  <TableCell>
                    {s.offsite_enabled ? (
                      <Badge variant="secondary" className="font-normal">
                        On
                      </Badge>
                    ) : (
                      <span className="text-muted-foreground text-xs">Off</span>
                    )}
                  </TableCell>
                  <TableCell className="text-muted-foreground whitespace-nowrap">
                    {s.enabled ? formatDate(s.next_run_at) : "—"}
                  </TableCell>
                  <TableCell className="text-muted-foreground whitespace-nowrap">
                    {formatDate(s.last_run_at)}
                  </TableCell>
                  <TableCell>
                    <Switch
                      checked={s.enabled}
                      onCheckedChange={(c) => onToggleEnabled(s, c)}
                      aria-label="Enabled"
                    />
                  </TableCell>
                  <TableCell>
                    <div className="flex items-center gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-8"
                        onClick={() => openEdit(s)}
                      >
                        <Pencil className="size-4" />
                        <span className="sr-only">Edit schedule</span>
                      </Button>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-8"
                        onClick={() => setDeleting(s)}
                      >
                        <Trash2 className="size-4" />
                        <span className="sr-only">Delete schedule</span>
                      </Button>
                    </div>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>

      {/* Create / edit dialog */}
      <Dialog open={!!editor} onOpenChange={(o) => !o && setEditor(null)}>
        <DialogContent className="sm:max-w-md">
          <form onSubmit={onSave}>
            <DialogHeader>
              <DialogTitle>{isEdit ? "Edit schedule" : "New schedule"}</DialogTitle>
              <DialogDescription>
                Backups capture control-plane metadata and config only — not object data.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="schedule-interval">Interval (minutes)</Label>
                <Input
                  id="schedule-interval"
                  type="number"
                  min="1"
                  value={draft.interval_minutes}
                  onChange={(e) => setDraft((d) => ({ ...d, interval_minutes: e.target.value }))}
                  required
                  autoFocus
                />
                <span className="text-muted-foreground text-xs">
                  How often a backup runs. 1440 = once a day, 60 = hourly.
                </span>
              </div>
              <div className="grid gap-2">
                <Label htmlFor="schedule-retention">Retention (count)</Label>
                <Input
                  id="schedule-retention"
                  type="number"
                  min="1"
                  value={draft.retention_count}
                  onChange={(e) => setDraft((d) => ({ ...d, retention_count: e.target.value }))}
                  required
                />
                <span className="text-muted-foreground text-xs">
                  How many of the most recent backups to keep.
                </span>
              </div>
              <div className="flex items-center justify-between rounded-lg border p-3">
                <div className="flex flex-col gap-0.5">
                  <Label htmlFor="schedule-offsite" className="text-sm font-medium">
                    Off-box copy
                  </Label>
                  <span className="text-muted-foreground text-xs">
                    Also upload each backup to off-box storage.
                  </span>
                </div>
                <Switch
                  id="schedule-offsite"
                  checked={draft.offsite_enabled}
                  onCheckedChange={(c) => setDraft((d) => ({ ...d, offsite_enabled: c }))}
                />
              </div>
              {isEdit && (
                <div className="flex items-center justify-between rounded-lg border p-3">
                  <div className="flex flex-col gap-0.5">
                    <Label htmlFor="schedule-enabled" className="text-sm font-medium">
                      Enabled
                    </Label>
                    <span className="text-muted-foreground text-xs">
                      Pause the schedule without deleting it.
                    </span>
                  </div>
                  <Switch
                    id="schedule-enabled"
                    checked={draft.enabled}
                    onCheckedChange={(c) => setDraft((d) => ({ ...d, enabled: c }))}
                  />
                </div>
              )}
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setEditor(null)}>
                Cancel
              </Button>
              <Button type="submit" disabled={saving}>
                {saving && <Loader2 className="size-4 animate-spin" />}
                {isEdit ? "Save changes" : "Create schedule"}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <AlertDialog open={!!deleting} onOpenChange={(o) => !o && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this schedule?</AlertDialogTitle>
            <AlertDialogDescription>
              Automatic backups on this cadence will stop. Existing backup files are not removed.
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
    </Card>
  );
}

"use client";

import * as React from "react";
import { Download, ScrollText, Search, X } from "lucide-react";
import { toast } from "sonner";

import {
  auditExportUrl,
  listAudit,
  ApiError,
  type AuditEvent,
  type AuditFilter,
} from "@/lib/api";
import { formatDate } from "@/lib/format";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
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

function actionVariant(action: string): "default" | "secondary" | "destructive" {
  const a = action.toLowerCase();
  if (a.includes("delete") || a.includes("remove")) return "destructive";
  if (a.includes("create") || a.includes("add")) return "default";
  return "secondary";
}

/** Convert a datetime-local value ("2026-06-18T14:30") to an RFC3339 string. */
function toRfc3339(local: string): string | undefined {
  if (!local) return undefined;
  const d = new Date(local);
  if (Number.isNaN(d.getTime())) return undefined;
  return d.toISOString();
}

const DEFAULT_LIMIT = 100;

export default function AuditPage() {
  const [events, setEvents] = React.useState<AuditEvent[] | null>(null);

  // Draft filter inputs (controlled).
  const [from, setFrom] = React.useState("");
  const [to, setTo] = React.useState("");
  const [action, setAction] = React.useState("");
  const [actor, setActor] = React.useState("");
  const [target, setTarget] = React.useState("");
  const [limit, setLimit] = React.useState("100");

  // The filter actually applied to the current results (used by the export links).
  const [applied, setApplied] = React.useState<AuditFilter>({ limit: DEFAULT_LIMIT });

  const buildFilter = React.useCallback((): AuditFilter => {
    const parsedLimit = Number(limit);
    return {
      from: toRfc3339(from),
      to: toRfc3339(to),
      action: action.trim() || undefined,
      actor: actor.trim() || undefined,
      target: target.trim() || undefined,
      limit: Number.isFinite(parsedLimit) && parsedLimit > 0 ? Math.round(parsedLimit) : DEFAULT_LIMIT,
    };
  }, [from, to, action, actor, target, limit]);

  const run = React.useCallback(async (filter: AuditFilter) => {
    setEvents(null);
    try {
      const res = await listAudit(filter);
      setEvents(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load audit log");
      setEvents([]);
    }
  }, []);

  React.useEffect(() => {
    const initial: AuditFilter = { limit: DEFAULT_LIMIT };
    setApplied(initial);
    run(initial);
  }, [run]);

  function onApply(e: React.FormEvent) {
    e.preventDefault();
    const filter = buildFilter();
    setApplied(filter);
    run(filter);
  }

  function onReset() {
    setFrom("");
    setTo("");
    setAction("");
    setActor("");
    setTarget("");
    setLimit("100");
    const initial: AuditFilter = { limit: DEFAULT_LIMIT };
    setApplied(initial);
    run(initial);
  }

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Audit" }]}
        actions={
          <div className="flex items-center gap-2">
            <Button size="sm" variant="outline" asChild>
              <a href={auditExportUrl(applied, "csv")}>
                <Download className="size-4" />
                CSV
              </a>
            </Button>
            <Button size="sm" variant="outline" asChild>
              <a href={auditExportUrl(applied, "json")}>
                <Download className="size-4" />
                JSON
              </a>
            </Button>
          </div>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
        <Card>
          <CardHeader>
            <CardTitle className="text-base">Filters</CardTitle>
            <CardDescription>Narrow the log by time range, action, actor, or target.</CardDescription>
          </CardHeader>
          <CardContent>
            <form onSubmit={onApply} className="flex flex-col gap-4">
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
                <div className="grid gap-2">
                  <Label htmlFor="audit-from">From</Label>
                  <Input
                    id="audit-from"
                    type="datetime-local"
                    value={from}
                    onChange={(e) => setFrom(e.target.value)}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="audit-to">To</Label>
                  <Input
                    id="audit-to"
                    type="datetime-local"
                    value={to}
                    onChange={(e) => setTo(e.target.value)}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="audit-limit">Limit</Label>
                  <Input
                    id="audit-limit"
                    type="number"
                    min="1"
                    value={limit}
                    onChange={(e) => setLimit(e.target.value)}
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="audit-action">Action</Label>
                  <Input
                    id="audit-action"
                    value={action}
                    onChange={(e) => setAction(e.target.value)}
                    placeholder="bucket.create"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="audit-actor">Actor</Label>
                  <Input
                    id="audit-actor"
                    value={actor}
                    onChange={(e) => setActor(e.target.value)}
                    placeholder="user id"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="audit-target">Target</Label>
                  <Input
                    id="audit-target"
                    value={target}
                    onChange={(e) => setTarget(e.target.value)}
                    placeholder="bucket / key / …"
                  />
                </div>
              </div>
              <div className="flex items-center gap-2">
                <Button type="submit" size="sm">
                  <Search className="size-4" />
                  Apply filters
                </Button>
                <Button type="button" size="sm" variant="ghost" onClick={onReset}>
                  <X className="size-4" />
                  Reset
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>

        {events === null ? (
          <Skeleton className="h-64 w-full" />
        ) : events.length === 0 ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <ScrollText className="size-4" /> No audit events
              </CardTitle>
              <CardDescription>
                No events match the current filters. Adjust them or reset to see all activity.
              </CardDescription>
            </CardHeader>
          </Card>
        ) : (
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Action</TableHead>
                    <TableHead>Actor</TableHead>
                    <TableHead>Target</TableHead>
                    <TableHead>When</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {events.map((e) => (
                    <TableRow key={e.id}>
                      <TableCell>
                        <Badge variant={actionVariant(e.action)} className="font-mono font-normal">
                          {e.action}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground capitalize">
                        {e.actor_type}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        <span className="capitalize">{e.target_type}</span>
                        {e.target_id && (
                          <span className="ml-1 font-mono text-xs">{e.target_id}</span>
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground whitespace-nowrap">
                        {formatDate(e.created_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}
      </div>
    </>
  );
}

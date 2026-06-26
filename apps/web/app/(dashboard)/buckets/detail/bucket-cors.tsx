"use client";

import * as React from "react";
import { Loader2, Plus, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  getBucketCors,
  setBucketCors,
  deleteBucketCors,
  ApiError,
  type CorsRule,
} from "@/lib/api";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Textarea } from "@/components/ui/textarea";

/** Draft uses textarea-friendly newline strings for the list fields. */
interface DraftRule {
  allowed_origins: string;
  allowed_methods: string;
  allowed_headers: string;
  expose_headers: string;
  max_age_seconds: string;
}

function toDraft(r: CorsRule): DraftRule {
  return {
    allowed_origins: r.allowed_origins.join("\n"),
    allowed_methods: r.allowed_methods.join("\n"),
    allowed_headers: r.allowed_headers.join("\n"),
    expose_headers: r.expose_headers.join("\n"),
    max_age_seconds: String(r.max_age_seconds ?? 0),
  };
}

function emptyDraft(): DraftRule {
  return {
    allowed_origins: "*",
    allowed_methods: "GET\nPUT\nPOST",
    allowed_headers: "*",
    expose_headers: "",
    max_age_seconds: "3600",
  };
}

function splitLines(value: string): string[] {
  return value
    .split(/[\n,]/)
    .map((s) => s.trim())
    .filter(Boolean);
}

function toRule(d: DraftRule): CorsRule {
  const n = Number(d.max_age_seconds);
  return {
    allowed_origins: splitLines(d.allowed_origins),
    allowed_methods: splitLines(d.allowed_methods),
    allowed_headers: splitLines(d.allowed_headers),
    expose_headers: splitLines(d.expose_headers),
    max_age_seconds: Number.isFinite(n) && n > 0 ? Math.round(n) : 0,
  };
}

export function BucketCors({ bucketId }: { bucketId: string }) {
  const [rules, setRules] = React.useState<DraftRule[] | null>(null);
  const [saving, setSaving] = React.useState(false);
  const [clearing, setClearing] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await getBucketCors(bucketId);
      setRules(res.data.map(toDraft));
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load CORS rules");
      setRules([]);
    }
  }, [bucketId]);

  React.useEffect(() => {
    load();
  }, [load]);

  function updateRule(idx: number, patch: Partial<DraftRule>) {
    setRules((rs) => (rs ? rs.map((r, i) => (i === idx ? { ...r, ...patch } : r)) : rs));
  }

  function addRule() {
    setRules((rs) => [...(rs ?? []), emptyDraft()]);
  }

  function removeRule(idx: number) {
    setRules((rs) => (rs ? rs.filter((_, i) => i !== idx) : rs));
  }

  async function onSave() {
    if (!rules) return;
    const payload = rules.map(toRule);
    if (payload.some((r) => r.allowed_origins.length === 0 || r.allowed_methods.length === 0)) {
      toast.error("Each rule needs at least one origin and one method");
      return;
    }
    setSaving(true);
    try {
      await setBucketCors(bucketId, payload);
      toast.success("CORS rules saved");
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save CORS rules");
    } finally {
      setSaving(false);
    }
  }

  async function onClear() {
    setClearing(true);
    try {
      await deleteBucketCors(bucketId);
      toast.success("CORS configuration cleared");
      setRules([]);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to clear CORS configuration");
    } finally {
      setClearing(false);
    }
  }

  if (!rules) {
    return <Skeleton className="h-64 w-full" />;
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>CORS configuration</CardTitle>
        <CardDescription>
          Control cross-origin browser access to this bucket. Enter one value per line (origins,
          methods, headers). Use <span className="font-mono">*</span> to allow any.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {rules.length === 0 ? (
          <div className="text-muted-foreground rounded-lg border border-dashed p-6 text-center text-sm">
            No CORS rules. Browsers on other origins cannot call this bucket directly.
          </div>
        ) : (
          rules.map((r, i) => (
            <div key={i} className="flex flex-col gap-4 rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <span className="text-sm font-medium">Rule {i + 1}</span>
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="size-8"
                  onClick={() => removeRule(i)}
                >
                  <Trash2 className="size-4" />
                  <span className="sr-only">Remove rule</span>
                </Button>
              </div>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="grid gap-2">
                  <Label htmlFor={`origins-${i}`}>Allowed origins</Label>
                  <Textarea
                    id={`origins-${i}`}
                    value={r.allowed_origins}
                    onChange={(e) => updateRule(i, { allowed_origins: e.target.value })}
                    placeholder="https://app.example.com"
                    className="font-mono text-xs"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor={`methods-${i}`}>Allowed methods</Label>
                  <Textarea
                    id={`methods-${i}`}
                    value={r.allowed_methods}
                    onChange={(e) => updateRule(i, { allowed_methods: e.target.value })}
                    placeholder="GET&#10;PUT"
                    className="font-mono text-xs"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor={`headers-${i}`}>Allowed headers</Label>
                  <Textarea
                    id={`headers-${i}`}
                    value={r.allowed_headers}
                    onChange={(e) => updateRule(i, { allowed_headers: e.target.value })}
                    placeholder="*"
                    className="font-mono text-xs"
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor={`expose-${i}`}>Expose headers</Label>
                  <Textarea
                    id={`expose-${i}`}
                    value={r.expose_headers}
                    onChange={(e) => updateRule(i, { expose_headers: e.target.value })}
                    placeholder="ETag"
                    className="font-mono text-xs"
                  />
                </div>
              </div>
              <div className="grid max-w-xs gap-2">
                <Label htmlFor={`maxage-${i}`}>Max age (seconds)</Label>
                <Input
                  id={`maxage-${i}`}
                  type="number"
                  min="0"
                  value={r.max_age_seconds}
                  onChange={(e) => updateRule(i, { max_age_seconds: e.target.value })}
                  placeholder="3600"
                />
              </div>
            </div>
          ))
        )}
        <div>
          <Button type="button" variant="secondary" size="sm" onClick={addRule}>
            <Plus className="size-4" />
            Add rule
          </Button>
        </div>
      </CardContent>
      <Separator />
      <CardFooter className="flex justify-between gap-2">
        <Button onClick={onSave} disabled={saving}>
          {saving && <Loader2 className="size-4 animate-spin" />}
          Save CORS rules
        </Button>
        <AlertDialog>
          <AlertDialogTrigger asChild>
            <Button variant="outline" disabled={clearing}>
              {clearing && <Loader2 className="size-4 animate-spin" />}
              Clear all
            </Button>
          </AlertDialogTrigger>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>Clear CORS configuration?</AlertDialogTitle>
              <AlertDialogDescription>
                This removes every CORS rule from the bucket. Cross-origin browser requests will be
                blocked again.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel disabled={clearing}>Cancel</AlertDialogCancel>
              <AlertDialogAction
                onClick={(e) => {
                  e.preventDefault();
                  onClear();
                }}
                disabled={clearing}
                className="bg-destructive text-white hover:bg-destructive/90"
              >
                Clear
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </CardFooter>
    </Card>
  );
}

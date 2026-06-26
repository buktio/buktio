"use client";

import * as React from "react";
import { Loader2, Plus, Trash2, Webhook } from "lucide-react";
import { toast } from "sonner";

import {
  listWebhooks,
  createWebhook,
  deleteWebhook,
  WEBHOOK_EVENTS,
  ApiError,
  type WebhookSub,
} from "@/lib/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
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

export function BucketWebhooks({ bucketId }: { bucketId: string }) {
  const [subs, setSubs] = React.useState<WebhookSub[] | null>(null);
  const [open, setOpen] = React.useState(false);
  const [url, setUrl] = React.useState("");
  const [secret, setSecret] = React.useState("");
  const [events, setEvents] = React.useState<Set<string>>(new Set(WEBHOOK_EVENTS));
  const [creating, setCreating] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await listWebhooks(bucketId);
      setSubs(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load webhooks");
      setSubs([]);
    }
  }, [bucketId]);

  React.useEffect(() => {
    load();
  }, [load]);

  function toggleEvent(e: string, on: boolean) {
    setEvents((prev) => {
      const next = new Set(prev);
      if (on) next.add(e);
      else next.delete(e);
      return next;
    });
  }

  async function create() {
    setCreating(true);
    try {
      await createWebhook(bucketId, url, Array.from(events), secret);
      toast.success("Webhook added");
      setOpen(false);
      setUrl("");
      setSecret("");
      setEvents(new Set(WEBHOOK_EVENTS));
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to add webhook");
    } finally {
      setCreating(false);
    }
  }

  async function remove(id: string) {
    try {
      await deleteWebhook(bucketId, id);
      toast.success("Webhook removed");
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to remove webhook");
    }
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-start justify-between gap-4 space-y-0">
        <div className="space-y-1.5">
          <CardTitle>Event webhooks</CardTitle>
          <CardDescription>
            POST a callback when objects are created or deleted through buktio (uploads, copies,
            moves, deletes). Direct-to-storage presigned uploads are not captured. Best-effort
            delivery with retries; set a secret to receive an{" "}
            <span className="font-mono">X-Buktio-Signature</span> HMAC header.
          </CardDescription>
        </div>
        <Dialog open={open} onOpenChange={setOpen}>
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="size-4" />
              Add webhook
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-lg">
            <DialogHeader>
              <DialogTitle>Add webhook</DialogTitle>
              <DialogDescription>Deliver object events to an HTTP endpoint.</DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-2">
              <div className="grid gap-2">
                <Label htmlFor="wh-url">Endpoint URL</Label>
                <Input
                  id="wh-url"
                  value={url}
                  onChange={(e) => setUrl(e.target.value)}
                  placeholder="https://example.com/hooks/buktio"
                  className="font-mono text-xs"
                  autoFocus
                />
              </div>
              <div className="grid gap-2">
                <Label>Events</Label>
                {WEBHOOK_EVENTS.map((e) => (
                  <label key={e} className="flex items-center gap-2 text-sm">
                    <Checkbox
                      checked={events.has(e)}
                      onCheckedChange={(c) => toggleEvent(e, c === true)}
                    />
                    <span className="font-mono">{e}</span>
                  </label>
                ))}
              </div>
              <div className="grid gap-2">
                <Label htmlFor="wh-secret">Signing secret (optional)</Label>
                <Input
                  id="wh-secret"
                  value={secret}
                  onChange={(e) => setSecret(e.target.value)}
                  placeholder="Used to HMAC-sign each payload"
                  className="font-mono text-xs"
                />
              </div>
            </div>
            <DialogFooter>
              <Button variant="outline" onClick={() => setOpen(false)} disabled={creating}>
                Cancel
              </Button>
              <Button onClick={create} disabled={creating || !url || events.size === 0}>
                {creating && <Loader2 className="size-4 animate-spin" />}
                Add webhook
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      </CardHeader>
      <CardContent className="p-0">
        {subs === null ? (
          <div className="p-6">
            <Skeleton className="h-24 w-full" />
          </div>
        ) : subs.length === 0 ? (
          <div className="text-muted-foreground flex flex-col items-center gap-2 px-6 py-10 text-center text-sm">
            <Webhook className="size-6" />
            No webhooks yet. Add one to react to object events.
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Endpoint</TableHead>
                <TableHead>Events</TableHead>
                <TableHead>Signed</TableHead>
                <TableHead className="w-10" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {subs.map((s) => (
                <TableRow key={s.id}>
                  <TableCell className="font-mono text-xs break-all">{s.url}</TableCell>
                  <TableCell>
                    <div className="flex flex-wrap gap-1">
                      {s.events.map((e) => (
                        <Badge key={e} variant="secondary" className="font-mono font-normal">
                          {e}
                        </Badge>
                      ))}
                    </div>
                  </TableCell>
                  <TableCell>
                    {s.has_secret ? (
                      <Badge variant="outline" className="font-normal">
                        HMAC
                      </Badge>
                    ) : (
                      <span className="text-muted-foreground">—</span>
                    )}
                  </TableCell>
                  <TableCell>
                    <Button
                      variant="ghost"
                      size="icon"
                      className="size-8"
                      onClick={() => remove(s.id)}
                    >
                      <Trash2 className="size-4" />
                      <span className="sr-only">Remove webhook</span>
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        )}
      </CardContent>
    </Card>
  );
}

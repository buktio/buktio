"use client";

import * as React from "react";
import { KeyRound, Loader2, Plus, Trash2, TriangleAlert } from "lucide-react";
import { toast } from "sonner";

import {
  listApiTokens,
  createApiToken,
  revokeApiToken,
  ApiError,
  type ApiToken,
  type CreatedApiToken,
} from "@/lib/api";
import { formatDate, formatDateShort } from "@/lib/format";
import { PageHeader } from "@/app/(dashboard)/page-header";
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

export default function TokensPage() {
  const [tokens, setTokens] = React.useState<ApiToken[] | null>(null);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [creating, setCreating] = React.useState(false);
  const [name, setName] = React.useState("");
  const [expiresDays, setExpiresDays] = React.useState("90");

  const [created, setCreated] = React.useState<CreatedApiToken | null>(null);
  const [acknowledged, setAcknowledged] = React.useState(false);

  const [revoking, setRevoking] = React.useState<ApiToken | null>(null);
  const [revokePending, setRevokePending] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await listApiTokens();
      setTokens(res.data);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load API tokens");
      setTokens([]);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  function resetForm() {
    setName("");
    setExpiresDays("90");
  }

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    const days = Number(expiresDays);
    const expiresInDays = Number.isFinite(days) && days > 0 ? Math.round(days) : 0;
    setCreating(true);
    try {
      const res = await createApiToken(name.trim(), expiresInDays);
      setCreateOpen(false);
      resetForm();
      setAcknowledged(false);
      setCreated(res);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create API token");
    } finally {
      setCreating(false);
    }
  }

  async function onRevoke() {
    if (!revoking) return;
    setRevokePending(true);
    try {
      await revokeApiToken(revoking.id);
      toast.success(`Token “${revoking.name}” revoked`);
      setRevoking(null);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to revoke token");
    } finally {
      setRevokePending(false);
    }
  }

  return (
    <>
      <PageHeader
        crumbs={[{ label: "API Tokens" }]}
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="size-4" />
            Create token
          </Button>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
        {tokens === null ? (
          <Skeleton className="h-64 w-full" />
        ) : tokens.length === 0 ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <KeyRound className="size-4" /> No API tokens yet
              </CardTitle>
              <CardDescription>
                Personal access tokens authenticate API requests to the buktio control plane (prefix{" "}
                <span className="font-mono">bk_pat_</span>).
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Button onClick={() => setCreateOpen(true)}>
                <Plus className="size-4" />
                Create token
              </Button>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Secret</TableHead>
                    <TableHead>Scopes</TableHead>
                    <TableHead>Expires</TableHead>
                    <TableHead>Last used</TableHead>
                    <TableHead className="w-12" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tokens.map((t) => (
                    <TableRow key={t.id}>
                      <TableCell className="font-medium">{t.name}</TableCell>
                      <TableCell className="font-mono text-xs">…{t.secret_last_four}</TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-1">
                          {t.scopes.length === 0 ? (
                            <span className="text-muted-foreground text-xs">—</span>
                          ) : (
                            t.scopes.map((s) => (
                              <Badge key={s} variant="secondary" className="font-normal">
                                {s}
                              </Badge>
                            ))
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {t.expires_at ? formatDateShort(t.expires_at) : "Never"}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {t.last_used_at ? formatDate(t.last_used_at) : "Never"}
                      </TableCell>
                      <TableCell>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          onClick={() => setRevoking(t)}
                        >
                          <Trash2 className="size-4" />
                          <span className="sr-only">Revoke</span>
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Create token dialog */}
      <Dialog
        open={createOpen}
        onOpenChange={(o) => {
          setCreateOpen(o);
          if (!o) resetForm();
        }}
      >
        <DialogContent className="sm:max-w-md">
          <form onSubmit={onCreate}>
            <DialogHeader>
              <DialogTitle>Create API token</DialogTitle>
              <DialogDescription>
                The token is shown only once after creation — store it securely.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="token-name">Name</Label>
                <Input
                  id="token-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="ci-pipeline"
                  required
                  autoFocus
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="token-expiry">Expires in (days)</Label>
                <Input
                  id="token-expiry"
                  type="number"
                  min="0"
                  value={expiresDays}
                  onChange={(e) => setExpiresDays(e.target.value)}
                  placeholder="0 = never"
                />
                <span className="text-muted-foreground text-xs">Use 0 for a token that never expires.</span>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={creating || !name.trim()}>
                {creating && <Loader2 className="size-4 animate-spin" />}
                Create token
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Shown-once token dialog */}
      <Dialog
        open={!!created}
        onOpenChange={(o) => {
          if (!o && acknowledged) setCreated(null);
        }}
      >
        <DialogContent
          className="sm:max-w-lg"
          showCloseButton={false}
          onEscapeKeyDown={(e) => {
            if (!acknowledged) e.preventDefault();
          }}
          onInteractOutside={(e) => {
            if (!acknowledged) e.preventDefault();
          }}
        >
          <DialogHeader>
            <DialogTitle>API token created</DialogTitle>
            <DialogDescription>
              Copy your token now. For security, it will never be shown again.
            </DialogDescription>
          </DialogHeader>
          {created && (
            <div className="flex flex-col gap-4 py-2">
              <div className="grid gap-2">
                <Label htmlFor="created-token">Token</Label>
                <div className="flex gap-2">
                  <Input
                    id="created-token"
                    readOnly
                    value={created.token}
                    className="font-mono text-xs"
                  />
                  <CopyButton value={created.token} variant="outline" label="Copy token" />
                </div>
              </div>
              <Alert variant="destructive">
                <TriangleAlert className="size-4" />
                <AlertTitle>Save this token now</AlertTitle>
                <AlertDescription>
                  This is the only time the token is displayed. If you lose it, revoke it and create
                  a new one.
                </AlertDescription>
              </Alert>
              <label className="flex items-center gap-2 text-sm">
                <Checkbox
                  checked={acknowledged}
                  onCheckedChange={(c) => setAcknowledged(c === true)}
                />
                I have securely saved my API token.
              </label>
            </div>
          )}
          <DialogFooter>
            <Button disabled={!acknowledged} onClick={() => setCreated(null)}>
              Done
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Revoke confirmation */}
      <AlertDialog open={!!revoking} onOpenChange={(o) => !o && setRevoking(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Revoke token “{revoking?.name}”?</AlertDialogTitle>
            <AlertDialogDescription>
              Any client using this token will immediately lose access. This cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={revokePending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                onRevoke();
              }}
              disabled={revokePending}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {revokePending && <Loader2 className="size-4 animate-spin" />}
              Revoke
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

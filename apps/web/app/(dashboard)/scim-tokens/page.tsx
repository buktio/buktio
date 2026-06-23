"use client";

import * as React from "react";
import { Copy, Fingerprint, Loader2, Plus, Trash2, TriangleAlert } from "lucide-react";
import { toast } from "sonner";

import {
  listScimTokens,
  createScimToken,
  revokeScimToken,
  ApiError,
  type ScimToken,
  type CreatedScimToken,
} from "@/lib/api";
import { formatDate, formatDateShort } from "@/lib/format";
import { PageHeader } from "@/app/(dashboard)/page-header";
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export default function ScimTokensPage() {
  const [tokens, setTokens] = React.useState<ScimToken[] | null>(null);
  const [loadError, setLoadError] = React.useState<string | null>(null);
  const [baseUrl, setBaseUrl] = React.useState("");

  const [createOpen, setCreateOpen] = React.useState(false);
  const [creating, setCreating] = React.useState(false);
  const [name, setName] = React.useState("");

  const [created, setCreated] = React.useState<CreatedScimToken | null>(null);

  const [revoking, setRevoking] = React.useState<ScimToken | null>(null);
  const [revokePending, setRevokePending] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await listScimTokens();
      setTokens(res.tokens);
      setLoadError(null);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to load SCIM tokens";
      toast.error(message);
      setLoadError(message);
      setTokens([]);
    }
  }, []);

  React.useEffect(() => {
    setBaseUrl(`${window.location.origin}/scim/v2`);
    load();
  }, [load]);

  function resetForm() {
    setName("");
  }

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    setCreating(true);
    try {
      const res = await createScimToken(name.trim());
      setCreateOpen(false);
      resetForm();
      setCreated(res);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create SCIM token");
    } finally {
      setCreating(false);
    }
  }

  async function onRevoke() {
    if (!revoking) return;
    setRevokePending(true);
    try {
      await revokeScimToken(revoking.id);
      toast.success(`Token “${revoking.name}” revoked`);
      setRevoking(null);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to revoke token");
    } finally {
      setRevokePending(false);
    }
  }

  async function onCopy(value: string) {
    try {
      await navigator.clipboard.writeText(value);
      toast.success("Copied");
    } catch {
      toast.error("Failed to copy to clipboard");
    }
  }

  return (
    <>
      <PageHeader
        crumbs={[{ label: "SCIM" }]}
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="size-4" />
            Create token
          </Button>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Fingerprint className="size-4" /> SCIM provisioning tokens
            </CardTitle>
            <CardDescription>
              Paste this bearer token into your IdP&apos;s SCIM configuration. The SCIM base URL is{" "}
              <span className="font-mono">{baseUrl || "…"}</span>. Configure it as a SCIM 2.0
              provisioning endpoint in Okta or Azure AD (Microsoft Entra ID).
            </CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            {tokens === null ? (
              <div className="p-6">
                <Skeleton className="h-40 w-full" />
              </div>
            ) : loadError ? (
              <div className="p-6">
                <Alert variant="destructive">
                  <TriangleAlert className="size-4" />
                  <AlertTitle>Unable to load SCIM tokens</AlertTitle>
                  <AlertDescription>{loadError}</AlertDescription>
                </Alert>
              </div>
            ) : tokens.length === 0 ? (
              <div className="flex flex-col items-start gap-3 p-6">
                <p className="text-muted-foreground text-sm">
                  No SCIM tokens yet. Create one to connect your identity provider.
                </p>
                <Button onClick={() => setCreateOpen(true)}>
                  <Plus className="size-4" />
                  Create token
                </Button>
              </div>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Last four</TableHead>
                    <TableHead>Created</TableHead>
                    <TableHead>Last used</TableHead>
                    <TableHead className="w-12" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tokens.map((t) => (
                    <TableRow key={t.id}>
                      <TableCell className="font-medium">{t.name}</TableCell>
                      <TableCell className="font-mono text-xs">
                        {t.last_four ? `••••${t.last_four}` : "—"}
                      </TableCell>
                      <TableCell className="text-muted-foreground whitespace-nowrap">
                        {formatDateShort(t.created_at)}
                      </TableCell>
                      <TableCell className="text-muted-foreground whitespace-nowrap">
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
            )}
          </CardContent>
        </Card>
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
              <DialogTitle>Create SCIM token</DialogTitle>
              <DialogDescription>
                The bearer token is shown only once after creation — store it securely.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="scim-token-name">Name</Label>
                <Input
                  id="scim-token-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="okta-production"
                  required
                  autoFocus
                />
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
      <AlertDialog
        open={!!created}
        onOpenChange={(o) => {
          if (!o) setCreated(null);
        }}
      >
        <AlertDialogContent className="sm:max-w-lg">
          <AlertDialogHeader>
            <AlertDialogTitle>SCIM token created</AlertDialogTitle>
            <AlertDialogDescription>
              Copy this bearer token now and paste it into your IdP&apos;s SCIM configuration.
            </AlertDialogDescription>
          </AlertDialogHeader>
          {created && (
            <div className="flex flex-col gap-4 py-2">
              <div className="grid gap-2">
                <Label htmlFor="created-scim-token">Token</Label>
                <div className="flex gap-2">
                  <Input
                    id="created-scim-token"
                    readOnly
                    value={created.token}
                    className="font-mono text-xs"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    onClick={() => onCopy(created.token)}
                  >
                    <Copy className="size-4" />
                    Copy
                  </Button>
                </div>
              </div>
              <Alert variant="destructive">
                <TriangleAlert className="size-4" />
                <AlertTitle>Save this token now</AlertTitle>
                <AlertDescription>
                  You won&apos;t be able to see this token again. If you lose it, revoke it and
                  create a new one.
                </AlertDescription>
              </Alert>
            </div>
          )}
          <AlertDialogFooter>
            <AlertDialogAction onClick={() => setCreated(null)}>Done</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Revoke confirmation */}
      <AlertDialog open={!!revoking} onOpenChange={(o) => !o && setRevoking(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Revoke token “{revoking?.name}”?</AlertDialogTitle>
            <AlertDialogDescription>
              Your identity provider will immediately lose its ability to provision users. This
              cannot be undone.
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

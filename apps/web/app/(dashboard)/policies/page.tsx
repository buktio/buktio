"use client";

import * as React from "react";
import { Loader2, Plus, ShieldCheck, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  listPolicies,
  createPolicy,
  setPolicyEnabled,
  deletePolicy,
  ApiError,
  type Policy,
} from "@/lib/api";
import { formatDateShort } from "@/lib/format";
import { PageHeader } from "@/app/(dashboard)/page-header";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
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

const TEMPLATES = ["ip_allowlist", "business_hours", "read_only"] as const;
type Template = (typeof TEMPLATES)[number];

const ASSIGNABLE_ROLES = ["admin", "member", "viewer"] as const;
type AssignableRole = (typeof ASSIGNABLE_ROLES)[number];

const TEMPLATE_LABELS: Record<string, string> = {
  ip_allowlist: "IP allowlist",
  business_hours: "Business hours",
  read_only: "Read only",
};

function templateLabel(template: string): string {
  return TEMPLATE_LABELS[template] ?? template;
}

export default function PoliciesPage() {
  const [policies, setPolicies] = React.useState<Policy[] | null>(null);
  const [loadError, setLoadError] = React.useState<string | null>(null);

  const [togglingId, setTogglingId] = React.useState<string | null>(null);

  const [createOpen, setCreateOpen] = React.useState(false);
  const [creating, setCreating] = React.useState(false);
  const [name, setName] = React.useState("");
  const [template, setTemplate] = React.useState<Template>("ip_allowlist");
  const [roles, setRoles] = React.useState<Record<AssignableRole, boolean>>({
    admin: false,
    member: false,
    viewer: false,
  });
  const [cidrs, setCidrs] = React.useState("");
  const [startHour, setStartHour] = React.useState("9");
  const [endHour, setEndHour] = React.useState("17");

  const [deleting, setDeleting] = React.useState<Policy | null>(null);
  const [deletePending, setDeletePending] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await listPolicies();
      setPolicies(res.policies);
      setLoadError(null);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to load policies";
      setLoadError(message);
      setPolicies([]);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  function resetForm() {
    setName("");
    setTemplate("ip_allowlist");
    setRoles({ admin: false, member: false, viewer: false });
    setCidrs("");
    setStartHour("9");
    setEndHour("17");
  }

  async function onToggle(policy: Policy, enabled: boolean) {
    setTogglingId(policy.id);
    try {
      await setPolicyEnabled(policy.id, enabled);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to update policy");
      // Reload to revert the optimistic switch state on failure.
      await load();
    } finally {
      setTogglingId(null);
    }
  }

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    const trimmedName = name.trim();
    if (!trimmedName) {
      toast.error("Name is required");
      return;
    }
    const selectedRoles = ASSIGNABLE_ROLES.filter((r) => roles[r]);
    if (selectedRoles.length === 0) {
      toast.error("Select at least one role");
      return;
    }

    let config: Record<string, string> = {};
    if (template === "ip_allowlist") {
      config = { cidrs: cidrs.trim() };
    } else if (template === "business_hours") {
      config = { start: startHour.trim(), end: endHour.trim() };
    }

    setCreating(true);
    try {
      await createPolicy({ name: trimmedName, template, config, roles: selectedRoles });
      toast.success(`Policy “${trimmedName}” created`);
      setCreateOpen(false);
      resetForm();
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create policy");
    } finally {
      setCreating(false);
    }
  }

  async function onDelete() {
    if (!deleting) return;
    setDeletePending(true);
    try {
      await deletePolicy(deleting.id);
      toast.success(`Policy “${deleting.name}” deleted`);
      setDeleting(null);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete policy");
    } finally {
      setDeletePending(false);
    }
  }

  const selectedRoleCount = ASSIGNABLE_ROLES.filter((r) => roles[r]).length;

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Policies" }]}
        actions={
          <Button size="sm" onClick={() => setCreateOpen(true)}>
            <Plus className="size-4" />
            New policy
          </Button>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
        {loadError ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <ShieldCheck className="size-4" /> Policies unavailable
              </CardTitle>
              <CardDescription>{loadError}</CardDescription>
            </CardHeader>
          </Card>
        ) : (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <ShieldCheck className="size-4" /> Policies
              </CardTitle>
              <CardDescription>
                Attribute-based policies further restrict roles on top of RBAC. Owners and platform
                admins are exempt.
              </CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              {policies === null ? (
                <div className="p-4">
                  <Skeleton className="h-48 w-full" />
                </div>
              ) : policies.length === 0 ? (
                <div className="text-muted-foreground py-12 text-center text-sm">
                  No policies yet
                </div>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Template</TableHead>
                      <TableHead>Roles</TableHead>
                      <TableHead>Enabled</TableHead>
                      <TableHead>Created</TableHead>
                      <TableHead className="w-12" />
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {policies.map((p) => (
                      <TableRow key={p.id}>
                        <TableCell className="font-medium">{p.name}</TableCell>
                        <TableCell>
                          <Badge variant="outline" className="font-normal">
                            {templateLabel(p.template)}
                          </Badge>
                        </TableCell>
                        <TableCell>
                          <div className="flex flex-wrap gap-1">
                            {p.roles.length === 0 ? (
                              <span className="text-muted-foreground text-xs">—</span>
                            ) : (
                              p.roles.map((r) => (
                                <Badge key={r} variant="secondary" className="font-normal">
                                  {r}
                                </Badge>
                              ))
                            )}
                          </div>
                        </TableCell>
                        <TableCell>
                          <Switch
                            checked={p.enabled}
                            disabled={togglingId === p.id}
                            onCheckedChange={(c) => onToggle(p, c)}
                            aria-label={`Toggle ${p.name}`}
                          />
                        </TableCell>
                        <TableCell className="text-muted-foreground whitespace-nowrap">
                          {formatDateShort(p.created_at)}
                        </TableCell>
                        <TableCell>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="size-8"
                            onClick={() => setDeleting(p)}
                          >
                            <Trash2 className="size-4" />
                            <span className="sr-only">Delete policy</span>
                          </Button>
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </CardContent>
          </Card>
        )}
      </div>

      {/* Create policy dialog */}
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
              <DialogTitle>New policy</DialogTitle>
              <DialogDescription>
                Attach an attribute-based restriction to one or more roles.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="policy-name">Name</Label>
                <Input
                  id="policy-name"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="office-hours-only"
                  required
                  autoFocus
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="policy-template">Template</Label>
                <Select value={template} onValueChange={(v) => setTemplate(v as Template)}>
                  <SelectTrigger id="policy-template">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {TEMPLATES.map((t) => (
                      <SelectItem key={t} value={t}>
                        {templateLabel(t)}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {/* Template-specific configuration */}
              {template === "ip_allowlist" && (
                <div className="grid gap-2">
                  <Label htmlFor="policy-cidrs">Allowed CIDRs</Label>
                  <Input
                    id="policy-cidrs"
                    value={cidrs}
                    onChange={(e) => setCidrs(e.target.value)}
                    placeholder="10.0.0.0/8, 192.168.0.0/16"
                  />
                </div>
              )}
              {template === "business_hours" && (
                <div className="grid grid-cols-2 gap-4">
                  <div className="grid gap-2">
                    <Label htmlFor="policy-start">Start hour (UTC)</Label>
                    <Input
                      id="policy-start"
                      type="number"
                      min={0}
                      max={23}
                      value={startHour}
                      onChange={(e) => setStartHour(e.target.value)}
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="policy-end">End hour (UTC)</Label>
                    <Input
                      id="policy-end"
                      type="number"
                      min={0}
                      max={23}
                      value={endHour}
                      onChange={(e) => setEndHour(e.target.value)}
                    />
                  </div>
                </div>
              )}

              <div className="grid gap-2">
                <Label>Roles</Label>
                <div className="flex flex-col gap-2">
                  {ASSIGNABLE_ROLES.map((r) => (
                    <label key={r} className="flex items-center gap-2 text-sm">
                      <Checkbox
                        checked={roles[r]}
                        onCheckedChange={(c) =>
                          setRoles((prev) => ({ ...prev, [r]: c === true }))
                        }
                      />
                      {r}
                    </label>
                  ))}
                </div>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setCreateOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={creating || !name.trim() || selectedRoleCount === 0}>
                {creating && <Loader2 className="size-4 animate-spin" />}
                Create policy
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Delete confirmation */}
      <AlertDialog open={!!deleting} onOpenChange={(o) => !o && setDeleting(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete policy “{deleting?.name}”?</AlertDialogTitle>
            <AlertDialogDescription>
              The roles it restricts will no longer be subject to this policy. This action cannot be
              undone.
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
    </>
  );
}

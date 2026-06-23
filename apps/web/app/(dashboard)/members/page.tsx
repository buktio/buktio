"use client";

import * as React from "react";
import { Loader2, Mail, Plus, Trash2, Users } from "lucide-react";
import { toast } from "sonner";

import {
  changeMemberRole,
  inviteMember,
  listMembers,
  removeMember,
  ApiError,
  type CreatedInvitation,
  type Invitation,
  type Member,
  type Role,
} from "@/lib/api";
import { formatDate, formatDateShort } from "@/lib/format";
import { useUser } from "@/app/(dashboard)/user-context";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { CopyButton } from "@/components/copy-button";
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

const ROLES: { value: Role; label: string }[] = [
  { value: "owner", label: "Owner" },
  { value: "admin", label: "Admin" },
  { value: "member", label: "Member" },
  { value: "viewer", label: "Viewer" },
];

function roleLabel(role: Role): string {
  return ROLES.find((r) => r.value === role)?.label ?? role;
}

export default function MembersPage() {
  const me = useUser();
  const [members, setMembers] = React.useState<Member[] | null>(null);
  const [invitations, setInvitations] = React.useState<Invitation[]>([]);

  const [savingRole, setSavingRole] = React.useState<string | null>(null);

  const [inviteOpen, setInviteOpen] = React.useState(false);
  const [inviting, setInviting] = React.useState(false);
  const [inviteEmail, setInviteEmail] = React.useState("");
  const [inviteRole, setInviteRole] = React.useState<Role>("member");

  const [created, setCreated] = React.useState<CreatedInvitation | null>(null);

  const [removing, setRemoving] = React.useState<Member | null>(null);
  const [removePending, setRemovePending] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const res = await listMembers();
      setMembers(res.members);
      setInvitations(res.invitations);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to load members");
      setMembers([]);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  async function onChangeRole(member: Member, role: Role) {
    if (role === member.role) return;
    setSavingRole(member.user_id);
    try {
      await changeMemberRole(member.user_id, role);
      toast.success(`${member.email} is now ${roleLabel(role).toLowerCase()}`);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to change role");
      // Reload to revert the optimistic Select value on failure.
      await load();
    } finally {
      setSavingRole(null);
    }
  }

  function resetInvite() {
    setInviteEmail("");
    setInviteRole("member");
  }

  async function onInvite(e: React.FormEvent) {
    e.preventDefault();
    setInviting(true);
    try {
      const res = await inviteMember(inviteEmail.trim(), inviteRole);
      setInviteOpen(false);
      resetInvite();
      setCreated(res);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to create invitation");
    } finally {
      setInviting(false);
    }
  }

  async function onRemove() {
    if (!removing) return;
    setRemovePending(true);
    try {
      await removeMember(removing.user_id);
      toast.success(`${removing.email} removed`);
      setRemoving(null);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to remove member");
    } finally {
      setRemovePending(false);
    }
  }

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Members" }]}
        actions={
          <Button size="sm" onClick={() => setInviteOpen(true)}>
            <Plus className="size-4" />
            Invite member
          </Button>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
        {/* Members */}
        {members === null ? (
          <Skeleton className="h-64 w-full" />
        ) : members.length === 0 ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Users className="size-4" /> No members yet
              </CardTitle>
              <CardDescription>
                Invite teammates to give them access to this control plane.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <Button onClick={() => setInviteOpen(true)}>
                <Plus className="size-4" />
                Invite member
              </Button>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardHeader>
              <CardTitle className="text-base">Members</CardTitle>
              <CardDescription>People with access to this control plane.</CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Email</TableHead>
                    <TableHead>Name</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Joined</TableHead>
                    <TableHead className="w-12" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {members.map((m) => (
                    <TableRow key={m.user_id}>
                      <TableCell className="font-medium">
                        {m.email}
                        {m.user_id === me.id && (
                          <Badge variant="outline" className="ml-2 font-normal">
                            you
                          </Badge>
                        )}
                      </TableCell>
                      <TableCell className="text-muted-foreground">
                        {m.full_name || "—"}
                      </TableCell>
                      <TableCell>
                        <Select
                          value={m.role}
                          onValueChange={(v) => onChangeRole(m, v as Role)}
                          disabled={savingRole === m.user_id}
                        >
                          <SelectTrigger size="sm" className="w-32">
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {ROLES.map((r) => (
                              <SelectItem key={r.value} value={r.value}>
                                {r.label}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </TableCell>
                      <TableCell className="text-muted-foreground whitespace-nowrap">
                        {formatDateShort(m.created_at)}
                      </TableCell>
                      <TableCell>
                        <Button
                          variant="ghost"
                          size="icon"
                          className="size-8"
                          onClick={() => setRemoving(m)}
                        >
                          <Trash2 className="size-4" />
                          <span className="sr-only">Remove member</span>
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}

        {/* Pending invitations */}
        {invitations.length > 0 && (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Mail className="size-4" /> Pending invitations
              </CardTitle>
              <CardDescription>
                Invitations that have not been accepted yet.
              </CardDescription>
            </CardHeader>
            <CardContent className="p-0">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Email</TableHead>
                    <TableHead>Role</TableHead>
                    <TableHead>Expires</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {invitations.map((inv) => (
                    <TableRow key={inv.id}>
                      <TableCell className="font-medium">{inv.email}</TableCell>
                      <TableCell>
                        <Badge variant="secondary" className="font-normal">
                          {roleLabel(inv.role)}
                        </Badge>
                      </TableCell>
                      <TableCell className="text-muted-foreground whitespace-nowrap">
                        {formatDate(inv.expires_at)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </CardContent>
          </Card>
        )}
      </div>

      {/* Invite dialog */}
      <Dialog
        open={inviteOpen}
        onOpenChange={(o) => {
          setInviteOpen(o);
          if (!o) resetInvite();
        }}
      >
        <DialogContent className="sm:max-w-md">
          <form onSubmit={onInvite}>
            <DialogHeader>
              <DialogTitle>Invite member</DialogTitle>
              <DialogDescription>
                Create an invitation link. No email is sent — copy the link and share it yourself.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="invite-email">Email</Label>
                <Input
                  id="invite-email"
                  type="email"
                  value={inviteEmail}
                  onChange={(e) => setInviteEmail(e.target.value)}
                  placeholder="teammate@example.com"
                  autoComplete="off"
                  required
                  autoFocus
                />
              </div>
              <div className="grid gap-2">
                <Label htmlFor="invite-role">Role</Label>
                <Select value={inviteRole} onValueChange={(v) => setInviteRole(v as Role)}>
                  <SelectTrigger id="invite-role">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {ROLES.map((r) => (
                      <SelectItem key={r.value} value={r.value}>
                        {r.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setInviteOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={inviting || !inviteEmail.trim()}>
                {inviting && <Loader2 className="size-4 animate-spin" />}
                Create invitation
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>

      {/* Invitation link (shown once) */}
      <Dialog open={!!created} onOpenChange={(o) => !o && setCreated(null)}>
        <DialogContent className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Invitation created</DialogTitle>
            <DialogDescription>
              Share this one-time link with {created?.email}. It expires{" "}
              {created ? formatDate(created.expires_at).toLowerCase() : ""}.
            </DialogDescription>
          </DialogHeader>
          {created && (
            <div className="flex flex-col gap-4 py-2">
              <div className="grid gap-2">
                <Label htmlFor="invite-link">Invitation link</Label>
                <div className="flex gap-2">
                  <Input
                    id="invite-link"
                    readOnly
                    value={created.link}
                    className="font-mono text-xs"
                  />
                  <CopyButton value={created.link} variant="outline" label="Copy link" />
                </div>
              </div>
            </div>
          )}
          <DialogFooter>
            <Button onClick={() => setCreated(null)}>Done</Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Remove confirmation */}
      <AlertDialog open={!!removing} onOpenChange={(o) => !o && setRemoving(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove {removing?.email}?</AlertDialogTitle>
            <AlertDialogDescription>
              They will immediately lose access to this control plane. This action cannot be undone.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={removePending}>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={(e) => {
                e.preventDefault();
                onRemove();
              }}
              disabled={removePending}
              className="bg-destructive text-white hover:bg-destructive/90"
            >
              {removePending && <Loader2 className="size-4 animate-spin" />}
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}

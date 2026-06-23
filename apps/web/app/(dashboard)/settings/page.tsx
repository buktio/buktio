"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { LogOut } from "lucide-react";
import { toast } from "sonner";

import { apiGet, apiSend, type Dashboard } from "@/lib/api";
import { useUser } from "@/app/(dashboard)/user-context";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { CopyButton } from "@/components/copy-button";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";

function Row({
  label,
  value,
  mono,
  copy,
  trailing,
}: {
  label: string;
  value: string;
  mono?: boolean;
  copy?: boolean;
  trailing?: React.ReactNode;
}) {
  return (
    <div className="flex items-center justify-between gap-2 py-2">
      <div className="flex min-w-0 flex-col">
        <span className="text-muted-foreground text-xs">{label}</span>
        <span className={mono ? "truncate font-mono text-sm" : "truncate text-sm"}>{value}</span>
      </div>
      {copy ? <CopyButton value={value} label={`Copy ${label.toLowerCase()}`} /> : trailing}
    </div>
  );
}

export default function SettingsPage() {
  const router = useRouter();
  const user = useUser();
  const [dashboard, setDashboard] = React.useState<Dashboard | null>(null);

  React.useEffect(() => {
    apiGet<Dashboard>("/dashboard")
      .then(setDashboard)
      .catch(() => {});
  }, []);

  async function logout() {
    try {
      await apiSend<void>("POST", "/auth/logout");
    } catch {
      // ignore — navigate regardless
    }
    toast.success("Signed out");
    router.replace("/login");
  }

  return (
    <>
      <PageHeader crumbs={[{ label: "Settings" }]} />
      <div className="flex flex-1 flex-col gap-6 p-4 md:p-6">
        <Card>
          <CardHeader>
            <CardTitle>Account</CardTitle>
            <CardDescription>The currently signed-in administrator.</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-0">
            <Row label="Name" value={user.full_name || "—"} />
            <Separator />
            <Row label="Email" value={user.email} />
            <Separator />
            <Row
              label="Role"
              value={user.role}
              trailing={
                <Badge variant="secondary" className="capitalize">
                  {user.role}
                </Badge>
              }
            />
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Instance</CardTitle>
            <CardDescription>Software versions and S3 endpoint for this instance.</CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-0">
            {!dashboard ? (
              <div className="flex flex-col gap-3 py-2">
                <Skeleton className="h-9 w-full" />
                <Skeleton className="h-9 w-full" />
                <Skeleton className="h-9 w-full" />
              </div>
            ) : (
              <>
                <Row
                  label="Edition"
                  value="Open Source"
                  trailing={<Badge variant="outline">OSS</Badge>}
                />
                <Separator />
                <Row label="buktio version" value={dashboard.versions.buktio} mono />
                <Separator />
                <Row label="Storage engine" value={dashboard.versions.storage_engine} mono />
                <Separator />
                <Row
                  label="Storage mode"
                  value={dashboard.cluster.mode === "external" ? "External" : "Managed"}
                  trailing={
                    <Badge variant={dashboard.cluster.mode === "external" ? "outline" : "secondary"}>
                      {dashboard.cluster.mode === "external" ? "External" : "Managed"}
                    </Badge>
                  }
                />
                <Separator />
                <Row label="S3 endpoint" value={dashboard.s3_endpoint} mono copy />
              </>
            )}
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Session</CardTitle>
            <CardDescription>Sign out of the panel on this device.</CardDescription>
          </CardHeader>
          <CardContent>
            <Button variant="outline" onClick={logout}>
              <LogOut className="size-4" />
              Sign out
            </Button>
          </CardContent>
        </Card>
      </div>
    </>
  );
}

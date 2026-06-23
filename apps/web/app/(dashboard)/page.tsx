"use client";

import * as React from "react";
import {
  AlertCircle,
  Database,
  HardDrive,
  Info,
  KeyRound,
  Boxes,
  TriangleAlert,
} from "lucide-react";

import { apiGet, type Dashboard } from "@/lib/api";
import { formatBytes, formatNumber } from "@/lib/format";
import { CopyButton } from "@/components/copy-button";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Badge } from "@/components/ui/badge";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";

function clusterBadgeVariant(status: string): "default" | "secondary" | "destructive" {
  const s = status.toLowerCase();
  if (s === "healthy" || s === "ok" || s === "online") return "default";
  if (s === "degraded" || s === "warning") return "secondary";
  return "destructive";
}

function alertVariant(level: string): "default" | "destructive" {
  return level.toLowerCase() === "error" || level.toLowerCase() === "critical"
    ? "destructive"
    : "default";
}

function StatCard({
  title,
  value,
  icon: Icon,
  sub,
}: {
  title: string;
  value: string;
  icon: React.ComponentType<{ className?: string }>;
  sub?: string;
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-muted-foreground text-sm font-medium">{title}</CardTitle>
        <Icon className="text-muted-foreground size-4" />
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-semibold tabular-nums">{value}</div>
        {sub && <p className="text-muted-foreground mt-1 text-xs">{sub}</p>}
      </CardContent>
    </Card>
  );
}

export default function DashboardPage() {
  const [data, setData] = React.useState<Dashboard | null>(null);
  const [loading, setLoading] = React.useState(true);

  React.useEffect(() => {
    let active = true;
    apiGet<Dashboard>("/dashboard")
      .then((d) => active && setData(d))
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, []);

  return (
    <>
      <PageHeader crumbs={[{ label: "Dashboard" }]} />
      <div className="flex flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
        {loading ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-28 w-full" />
            ))}
          </div>
        ) : data ? (
          <>
            {data.alerts.length > 0 && (
              <div className="flex flex-col gap-3">
                {data.alerts.map((a, i) => (
                  <Alert key={`${a.code}-${i}`} variant={alertVariant(a.level)}>
                    {alertVariant(a.level) === "destructive" ? (
                      <TriangleAlert className="size-4" />
                    ) : (
                      <AlertCircle className="size-4" />
                    )}
                    <AlertTitle className="capitalize">{a.level}: {a.code}</AlertTitle>
                    <AlertDescription>{a.message}</AlertDescription>
                  </Alert>
                ))}
              </div>
            )}

            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <StatCard
                title="Buckets"
                value={formatNumber(data.totals.buckets)}
                icon={Database}
              />
              <StatCard
                title="Access Keys"
                value={formatNumber(data.totals.access_keys)}
                icon={KeyRound}
              />
              <StatCard
                title="Objects"
                value={formatNumber(data.totals.objects)}
                icon={Boxes}
              />
              <StatCard
                title="Space Used"
                value={formatBytes(data.totals.bytes_used)}
                icon={HardDrive}
                sub={`${formatBytes(data.capacity.disk_avail_bytes)} free of ${formatBytes(
                  data.capacity.disk_total_bytes,
                )}`}
              />
            </div>

            <div className="grid gap-4 lg:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle>Capacity</CardTitle>
                  <CardDescription>Disk utilization across the cluster.</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-3">
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-muted-foreground">Used</span>
                    <span className="font-medium tabular-nums">
                      {data.capacity.used_pct.toFixed(1)}%
                    </span>
                  </div>
                  <Progress value={Math.min(100, Math.max(0, data.capacity.used_pct))} />
                  <div className="text-muted-foreground flex items-center justify-between text-xs">
                    <span>{formatBytes(data.capacity.disk_avail_bytes)} available</span>
                    <span>{formatBytes(data.capacity.disk_total_bytes)} total</span>
                  </div>
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Cluster Health</CardTitle>
                  <CardDescription>Storage engine node status.</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-3">
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground text-sm">Status</span>
                    <Badge variant={clusterBadgeVariant(data.cluster.status)} className="capitalize">
                      {data.cluster.status}
                    </Badge>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-muted-foreground text-sm">Nodes online</span>
                    <span className="text-sm font-medium tabular-nums">
                      {data.cluster.nodes_ok} / {data.cluster.nodes_total}
                    </span>
                  </div>
                </CardContent>
              </Card>
            </div>

            <Card>
              <CardHeader>
                <CardTitle>Connection &amp; Versions</CardTitle>
                <CardDescription>
                  S3 endpoint and the running software versions.
                </CardDescription>
              </CardHeader>
              <CardContent className="flex flex-col gap-1">
                <div className="flex items-center justify-between gap-2 py-2">
                  <div className="flex min-w-0 flex-col">
                    <span className="text-muted-foreground text-xs">S3 endpoint</span>
                    <span className="truncate font-mono text-sm">{data.s3_endpoint}</span>
                  </div>
                  <CopyButton value={data.s3_endpoint} label="Copy endpoint" />
                </div>
                <Separator />
                <div className="flex items-center justify-between gap-2 py-2">
                  <div className="flex min-w-0 flex-col">
                    <span className="text-muted-foreground text-xs">buktio</span>
                    <span className="font-mono text-sm">{data.versions.buktio}</span>
                  </div>
                  <Badge variant="outline">OSS</Badge>
                </div>
                <Separator />
                <div className="flex items-center justify-between gap-2 py-2">
                  <div className="flex min-w-0 flex-col">
                    <span className="text-muted-foreground text-xs">Storage engine</span>
                    <span className="font-mono text-sm">{data.versions.storage_engine}</span>
                  </div>
                </div>
              </CardContent>
            </Card>
          </>
        ) : (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Info className="size-4" /> Dashboard unavailable
              </CardTitle>
              <CardDescription>
                Could not load dashboard data. Please refresh and try again.
              </CardDescription>
            </CardHeader>
          </Card>
        )}
      </div>
    </>
  );
}

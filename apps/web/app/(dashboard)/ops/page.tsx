"use client";

import * as React from "react";
import { Cell, Pie, PieChart } from "recharts";
import {
  Activity,
  AlertCircle,
  HardDrive,
  Layers,
  RefreshCw,
  TriangleAlert,
} from "lucide-react";
import { toast } from "sonner";

import { fetchGarageMetrics, trafficUsage, ApiError, type TrafficRow } from "@/lib/api";
import { formatBytes, formatNumber } from "@/lib/format";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import {
  ChartContainer,
  ChartLegend,
  ChartLegendContent,
  type ChartConfig,
} from "@/components/ui/chart";
import { Progress } from "@/components/ui/progress";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

/**
 * Minimal Prometheus exposition parser: returns the summed value across every
 * sample whose metric name matches `family` (labels ignored). Robust to missing
 * metrics — returns null when no matching sample is found.
 */
function sumMetric(text: string, family: string): number | null {
  let sum = 0;
  let found = false;
  for (const raw of text.split("\n")) {
    const line = raw.trim();
    if (!line || line.startsWith("#")) continue;
    // Match "family" or "family{labels}" then a numeric value.
    const nameEnd = line.search(/[\s{]/);
    if (nameEnd === -1) continue;
    const name = line.slice(0, nameEnd);
    if (name !== family) continue;
    const parts = line.split(/\s+/);
    const value = Number(parts[parts.length - 1]);
    if (Number.isFinite(value)) {
      sum += value;
      found = true;
    }
  }
  return found ? sum : null;
}

interface Metrics {
  requests: number | null;
  errors: number | null;
  resyncQueue: number | null;
  diskAvail: number | null;
  diskTotal: number | null;
}

function parseMetrics(text: string): Metrics {
  return {
    requests: sumMetric(text, "api_s3_request_counter"),
    errors: sumMetric(text, "api_s3_error_counter"),
    resyncQueue: sumMetric(text, "block_resync_queue_length"),
    diskAvail: sumMetric(text, "garage_local_disk_avail"),
    diskTotal: sumMetric(text, "garage_local_disk_total"),
  };
}

function KpiCard({
  title,
  value,
  icon: Icon,
  sub,
  tone,
}: {
  title: string;
  value: string;
  icon: React.ComponentType<{ className?: string }>;
  sub?: string;
  tone?: "default" | "warning";
}) {
  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle className="text-muted-foreground text-sm font-medium">{title}</CardTitle>
        <Icon className={tone === "warning" ? "text-destructive size-4" : "text-muted-foreground size-4"} />
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-semibold tabular-nums">{value}</div>
        {sub && <p className="text-muted-foreground mt-1 text-xs">{sub}</p>}
      </CardContent>
    </Card>
  );
}

const diskChartConfig = {
  used: { label: "Used", color: "var(--chart-1)" },
  free: { label: "Free", color: "var(--chart-2)" },
} satisfies ChartConfig;

export default function OpsPage() {
  const [metrics, setMetrics] = React.useState<Metrics | null>(null);
  const [loading, setLoading] = React.useState(true);
  const [failed, setFailed] = React.useState(false);
  const [traffic, setTraffic] = React.useState<TrafficRow[] | null>(null);

  const load = React.useCallback(async () => {
    setLoading(true);
    setFailed(false);
    try {
      const text = await fetchGarageMetrics();
      setMetrics(parseMetrics(text));
    } catch (err) {
      setFailed(true);
      toast.error(err instanceof ApiError ? err.message : "Failed to load metrics");
    } finally {
      setLoading(false);
    }

    try {
      const res = await trafficUsage(24);
      setTraffic(res.data);
    } catch (err) {
      setTraffic([]);
      toast.error(err instanceof ApiError ? err.message : "Failed to load traffic");
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  const diskUsed =
    metrics?.diskTotal != null && metrics?.diskAvail != null
      ? Math.max(0, metrics.diskTotal - metrics.diskAvail)
      : null;
  const diskPct =
    metrics?.diskTotal && metrics.diskTotal > 0 && diskUsed != null
      ? Math.min(100, Math.max(0, (diskUsed / metrics.diskTotal) * 100))
      : null;

  const diskData =
    diskUsed != null && metrics?.diskAvail != null
      ? [
          { name: "used", value: diskUsed, fill: "var(--color-used)" },
          { name: "free", value: metrics.diskAvail, fill: "var(--color-free)" },
        ]
      : [];

  return (
    <>
      <PageHeader
        crumbs={[{ label: "Ops" }]}
        actions={
          <Button variant="outline" size="sm" onClick={load} disabled={loading}>
            <RefreshCw className={loading ? "size-4 animate-spin" : "size-4"} />
            Refresh
          </Button>
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
        {loading ? (
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <Skeleton key={i} className="h-28 w-full" />
            ))}
          </div>
        ) : failed ? (
          <Alert variant="destructive">
            <TriangleAlert className="size-4" />
            <AlertTitle>Metrics unavailable</AlertTitle>
            <AlertDescription>
              Could not load storage-engine metrics. The metrics endpoint may be disabled on this
              deployment.
            </AlertDescription>
          </Alert>
        ) : (
          <>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <KpiCard
                title="S3 requests"
                value={metrics?.requests != null ? formatNumber(metrics.requests) : "—"}
                icon={Activity}
                sub="Total since node start"
              />
              <KpiCard
                title="S3 errors"
                value={metrics?.errors != null ? formatNumber(metrics.errors) : "—"}
                icon={AlertCircle}
                tone={metrics?.errors ? "warning" : "default"}
                sub="Total error responses"
              />
              <KpiCard
                title="Resync backlog"
                value={metrics?.resyncQueue != null ? formatNumber(metrics.resyncQueue) : "—"}
                icon={Layers}
                tone={metrics?.resyncQueue ? "warning" : "default"}
                sub="Blocks awaiting resync"
              />
              <KpiCard
                title="Disk used"
                value={diskPct != null ? `${diskPct.toFixed(1)}%` : "—"}
                icon={HardDrive}
                sub={
                  diskUsed != null && metrics?.diskTotal != null
                    ? `${formatBytes(diskUsed)} of ${formatBytes(metrics.diskTotal)}`
                    : "Disk metrics unavailable"
                }
              />
            </div>

            <div className="grid gap-4 lg:grid-cols-2">
              <Card>
                <CardHeader>
                  <CardTitle>Disk utilization</CardTitle>
                  <CardDescription>Local storage used across the cluster.</CardDescription>
                </CardHeader>
                <CardContent className="flex flex-col gap-4">
                  {diskPct != null ? (
                    <>
                      <div className="flex items-center justify-between text-sm">
                        <span className="text-muted-foreground">Used</span>
                        <span className="font-medium tabular-nums">{diskPct.toFixed(1)}%</span>
                      </div>
                      <Progress value={diskPct} />
                      <div className="text-muted-foreground flex items-center justify-between text-xs">
                        <span>{formatBytes(metrics?.diskAvail)} available</span>
                        <span>{formatBytes(metrics?.diskTotal)} total</span>
                      </div>
                    </>
                  ) : (
                    <p className="text-muted-foreground text-sm">Disk metrics unavailable.</p>
                  )}
                </CardContent>
              </Card>

              <Card>
                <CardHeader>
                  <CardTitle>Disk breakdown</CardTitle>
                  <CardDescription>Used vs. free space.</CardDescription>
                </CardHeader>
                <CardContent>
                  {diskData.length > 0 ? (
                    <ChartContainer config={diskChartConfig} className="mx-auto aspect-square max-h-56">
                      <PieChart>
                        <Pie data={diskData} dataKey="value" nameKey="name" innerRadius={50}>
                          {diskData.map((entry) => (
                            <Cell key={entry.name} fill={entry.fill} />
                          ))}
                        </Pie>
                        <ChartLegend content={<ChartLegendContent nameKey="name" />} />
                      </PieChart>
                    </ChartContainer>
                  ) : (
                    <p className="text-muted-foreground text-sm">Disk metrics unavailable.</p>
                  )}
                </CardContent>
              </Card>
            </div>
          </>
        )}

        <Card>
          <CardHeader>
            <CardTitle>Traffic (per key)</CardTitle>
            <CardDescription>
              Requests and bytes per access key over the last 24 hours. This data comes from the
              metering proxy and stays empty until traffic flows through it.
            </CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            {traffic === null ? (
              <div className="p-6">
                <Skeleton className="h-40 w-full" />
              </div>
            ) : traffic.length === 0 ? (
              <p className="text-muted-foreground px-6 pb-6 text-sm">
                No traffic recorded in the last 24 hours.
              </p>
            ) : (
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Key</TableHead>
                    <TableHead className="text-right">Requests</TableHead>
                    <TableHead className="text-right">Bytes in</TableHead>
                    <TableHead className="text-right">Egress</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {traffic.map((row) => (
                    <TableRow key={row.access_key_id}>
                      <TableCell className="font-medium">
                        {row.key_name || (
                          <span className="font-mono text-xs">{row.access_key_id}</span>
                        )}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatNumber(row.requests)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatBytes(row.bytes_in)}
                      </TableCell>
                      <TableCell className="text-right tabular-nums">
                        {formatBytes(row.bytes_out)}
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            )}
          </CardContent>
        </Card>
      </div>
    </>
  );
}

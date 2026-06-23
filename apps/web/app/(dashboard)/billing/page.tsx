"use client";

import * as React from "react";
import {
  ArrowUpFromLine,
  CreditCard,
  HardDrive,
  Hash,
  Loader2,
} from "lucide-react";
import { toast } from "sonner";

import {
  getBilling,
  setupBilling,
  ApiError,
  type BillingStatus,
} from "@/lib/api";
import { formatDate } from "@/lib/format";
import { useUser } from "@/app/(dashboard)/user-context";
import { PageHeader } from "@/app/(dashboard)/page-header";
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
import { Skeleton } from "@/components/ui/skeleton";

const gb = (n: number) => (n / 1024 ** 3).toFixed(2) + " GB";

function Kpi({
  icon,
  label,
  children,
}: {
  icon: React.ReactNode;
  label: string;
  children: React.ReactNode;
}) {
  return (
    <Card>
      <CardHeader className="pb-2">
        <CardDescription className="flex items-center gap-2">
          {icon}
          {label}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="text-2xl font-semibold">{children}</div>
      </CardContent>
    </Card>
  );
}

export default function BillingPage() {
  const user = useUser();

  const [data, setData] = React.useState<BillingStatus | null>(null);
  const [loadError, setLoadError] = React.useState<string | null>(null);

  const [setupOpen, setSetupOpen] = React.useState(false);
  const [submitting, setSubmitting] = React.useState(false);
  const [email, setEmail] = React.useState(user.email);

  const load = React.useCallback(async () => {
    try {
      const res = await getBilling();
      setData(res);
      setLoadError(null);
    } catch (err) {
      setLoadError(
        err instanceof ApiError ? err.message : "Failed to load billing details",
      );
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  async function onSetup(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    try {
      await setupBilling(email.trim());
      toast.success("Billing set up");
      setSetupOpen(false);
      await load();
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to set up billing");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <>
      <PageHeader crumbs={[{ label: "Billing" }]} />
      <div className="flex flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
        {loadError ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <CreditCard className="size-4" /> Billing unavailable
              </CardTitle>
              <CardDescription>{loadError}</CardDescription>
            </CardHeader>
          </Card>
        ) : data === null ? (
          <Skeleton className="h-64 w-full" />
        ) : !data.enabled ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <CreditCard className="size-4" /> Billing not enabled
              </CardTitle>
              <CardDescription>
                Usage-based billing is not enabled on this instance. It is available on the
                Hosted plan, where usage is metered and reported to the payment processor.
              </CardDescription>
            </CardHeader>
          </Card>
        ) : (
          <>
            <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
              <Kpi icon={<CreditCard className="size-4" />} label="Status">
                <Badge variant={data.customer_set ? "default" : "secondary"}>
                  {data.customer_set ? data.status || "active" : "not set up"}
                </Badge>
              </Kpi>
              <Kpi icon={<HardDrive className="size-4" />} label="Storage">
                {gb(data.storage_bytes)}
              </Kpi>
              <Kpi icon={<ArrowUpFromLine className="size-4" />} label="Egress">
                {gb(data.egress_bytes)}
              </Kpi>
              <Kpi icon={<Hash className="size-4" />} label="Requests">
                {data.requests.toLocaleString()}
              </Kpi>
            </div>

            <p className="text-muted-foreground text-sm">
              Current period since {data.period_start ? formatDate(data.period_start) : "—"};
              the reporter aggregates this to the payment processor.
            </p>

            {!data.customer_set && (
              <Card>
                <CardHeader>
                  <CardTitle className="text-base">Set up billing</CardTitle>
                  <CardDescription>
                    Add a billing contact to start usage-based billing for this organization.
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <Button onClick={() => setSetupOpen(true)}>
                    <CreditCard className="size-4" />
                    Set up billing
                  </Button>
                </CardContent>
              </Card>
            )}
          </>
        )}
      </div>

      {/* Set up billing dialog */}
      <Dialog
        open={setupOpen}
        onOpenChange={(o) => {
          setSetupOpen(o);
          if (!o) setEmail(user.email);
        }}
      >
        <DialogContent className="sm:max-w-md">
          <form onSubmit={onSetup}>
            <DialogHeader>
              <DialogTitle>Set up billing</DialogTitle>
              <DialogDescription>
                Provide a billing contact email. A customer record is created with the payment
                processor for usage-based billing.
              </DialogDescription>
            </DialogHeader>
            <div className="flex flex-col gap-4 py-4">
              <div className="grid gap-2">
                <Label htmlFor="billing-email">Billing email</Label>
                <Input
                  id="billing-email"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="billing@example.com"
                  autoComplete="off"
                  required
                  autoFocus
                />
              </div>
            </div>
            <DialogFooter>
              <Button type="button" variant="outline" onClick={() => setSetupOpen(false)}>
                Cancel
              </Button>
              <Button type="submit" disabled={submitting || !email.trim()}>
                {submitting && <Loader2 className="size-4 animate-spin" />}
                Set up billing
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  );
}

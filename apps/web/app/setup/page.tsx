"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import {
  apiGet,
  apiSend,
  ApiError,
  type AuthResponse,
  type SetupStatus,
} from "@/lib/api";
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
import { Skeleton } from "@/components/ui/skeleton";

export default function SetupPage() {
  const router = useRouter();
  const [checking, setChecking] = React.useState(true);
  const [submitting, setSubmitting] = React.useState(false);
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [fullName, setFullName] = React.useState("");

  React.useEffect(() => {
    let active = true;
    apiGet<SetupStatus>("/setup/status")
      .then((status) => {
        if (!active) return;
        if (status.initialized) {
          router.replace("/login");
        } else {
          setChecking(false);
        }
      })
      .catch(() => active && setChecking(false));
    return () => {
      active = false;
    };
  }, [router]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    try {
      await apiSend<AuthResponse>("POST", "/setup/create-admin", {
        email,
        password,
        full_name: fullName,
      });
      toast.success("Admin account created");
      router.replace("/");
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to create admin";
      toast.error(message);
      setSubmitting(false);
    }
  }

  return (
    <main className="bg-muted/30 flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
      <div className="flex w-full max-w-sm flex-col gap-6">
        <div className="flex flex-col items-center gap-1 text-center">
          <h1 className="text-2xl font-semibold tracking-tight">buktio</h1>
          <p className="text-muted-foreground text-sm">Self-hosted S3 control plane</p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>Create your admin account</CardTitle>
            <CardDescription>
              This is the first run. Set up the initial administrator to continue.
            </CardDescription>
          </CardHeader>
          {checking ? (
            <CardContent className="flex flex-col gap-4">
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
              <Skeleton className="h-9 w-full" />
            </CardContent>
          ) : (
            <form onSubmit={onSubmit}>
              <CardContent className="flex flex-col gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="full_name">Full name</Label>
                  <Input
                    id="full_name"
                    value={fullName}
                    onChange={(e) => setFullName(e.target.value)}
                    placeholder="Ada Lovelace"
                    autoComplete="name"
                    required
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="email">Email</Label>
                  <Input
                    id="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    placeholder="admin@example.com"
                    autoComplete="email"
                    required
                  />
                </div>
                <div className="grid gap-2">
                  <Label htmlFor="password">Password</Label>
                  <Input
                    id="password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    autoComplete="new-password"
                    required
                  />
                </div>
              </CardContent>
              <CardFooter className="mt-2">
                <Button type="submit" className="w-full" disabled={submitting}>
                  {submitting && <Loader2 className="size-4 animate-spin" />}
                  Create admin account
                </Button>
              </CardFooter>
            </form>
          )}
        </Card>
      </div>
    </main>
  );
}

"use client";

import * as React from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Loader2, TriangleAlert } from "lucide-react";
import { toast } from "sonner";

import { acceptInvite, ApiError } from "@/lib/api";
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
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

function AcceptInviteForm() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token") ?? "";

  const [submitting, setSubmitting] = React.useState(false);
  const [fullName, setFullName] = React.useState("");
  const [password, setPassword] = React.useState("");

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    try {
      await acceptInvite({ token, password, full_name: fullName.trim() });
      toast.success("Welcome aboard");
      router.replace("/");
    } catch (err) {
      const message =
        err instanceof ApiError && err.status === 404
          ? "This invitation is invalid or has expired."
          : err instanceof ApiError
            ? err.message
            : "Could not accept the invitation";
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
            <CardTitle>Accept your invitation</CardTitle>
            <CardDescription>Set your name and a password to activate your account.</CardDescription>
          </CardHeader>
          {token === "" ? (
            <CardContent>
              <Alert variant="destructive">
                <TriangleAlert className="size-4" />
                <AlertTitle>Missing invitation token</AlertTitle>
                <AlertDescription>
                  This link is missing its token. Ask whoever invited you to resend the invitation
                  link.
                </AlertDescription>
              </Alert>
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
                  Accept invitation
                </Button>
              </CardFooter>
            </form>
          )}
        </Card>
      </div>
    </main>
  );
}

export default function AcceptInvitePage() {
  return (
    <React.Suspense
      fallback={
        <div className="flex min-h-svh items-center justify-center">
          <Loader2 className="text-muted-foreground size-6 animate-spin" />
        </div>
      }
    >
      <AcceptInviteForm />
    </React.Suspense>
  );
}

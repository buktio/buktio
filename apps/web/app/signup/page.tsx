"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import { signup, ApiError } from "@/lib/api";
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

export default function SignupPage() {
  const router = useRouter();
  const [submitting, setSubmitting] = React.useState(false);
  const [sent, setSent] = React.useState(false);
  const [orgName, setOrgName] = React.useState("");
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    try {
      const res = await signup(email, password, orgName);
      // Dev convenience: if the backend returns the token, jump straight to verify.
      if (res.verification_token) {
        router.replace(`/signup/verify?token=${encodeURIComponent(res.verification_token)}`);
        return;
      }
      setSent(true);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Sign up failed";
      toast.error(message);
      setSubmitting(false);
    }
  }

  return (
    <main className="bg-muted/30 flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
      <div className="flex w-full max-w-sm flex-col gap-6">
        <div className="flex flex-col items-center gap-1 text-center">
          <h1 className="text-2xl font-semibold tracking-tight">buktio</h1>
          <p className="text-muted-foreground text-sm">Create your organization</p>
        </div>

        <Card>
          <CardHeader>
            <CardTitle>{sent ? "Check your email" : "Sign up"}</CardTitle>
            <CardDescription>
              {sent
                ? "We sent a verification link to your inbox. Open it to finish setting up your account."
                : "Start a new buktio organization in seconds."}
            </CardDescription>
          </CardHeader>
          {sent ? (
            <CardContent>
              <Button variant="outline" className="w-full" onClick={() => router.replace("/login")}>
                Back to sign in
              </Button>
            </CardContent>
          ) : (
            <form onSubmit={onSubmit}>
              <CardContent className="flex flex-col gap-4">
                <div className="grid gap-2">
                  <Label htmlFor="org">Organization name</Label>
                  <Input
                    id="org"
                    value={orgName}
                    onChange={(e) => setOrgName(e.target.value)}
                    placeholder="Acme Inc"
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
                    placeholder="you@example.com"
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
              <CardFooter className="mt-2 flex flex-col gap-3">
                <Button type="submit" className="w-full" disabled={submitting}>
                  {submitting && <Loader2 className="size-4 animate-spin" />}
                  Create organization
                </Button>
                <Button asChild variant="link" className="w-full">
                  <a href="/login">Already have an account? Sign in</a>
                </Button>
              </CardFooter>
            </form>
          )}
        </Card>
      </div>
    </main>
  );
}

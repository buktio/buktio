"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";

import {
  apiGet,
  apiSend,
  authMethods,
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
import { Separator } from "@/components/ui/separator";

/** SSO browser redirect to the configured IdP — a plain navigation, not a fetch. */
const SSO_LOGIN_URL = "/api/v1/auth/sso/login";

export default function LoginPage() {
  const router = useRouter();
  const [submitting, setSubmitting] = React.useState(false);
  const [email, setEmail] = React.useState("");
  const [password, setPassword] = React.useState("");
  const [ssoEnabled, setSsoEnabled] = React.useState(false);

  React.useEffect(() => {
    let active = true;
    // If the instance hasn't been initialized yet, send the user to setup.
    apiGet<SetupStatus>("/setup/status")
      .then((status) => {
        if (active && !status.initialized) router.replace("/setup");
      })
      .catch(() => {});
    authMethods()
      .then((methods) => active && setSsoEnabled(methods.sso))
      .catch(() => {});
    return () => {
      active = false;
    };
  }, [router]);

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault();
    setSubmitting(true);
    try {
      await apiSend<AuthResponse>("POST", "/auth/login", { email, password });
      toast.success("Signed in");
      router.replace("/");
    } catch (err) {
      const message =
        err instanceof ApiError && err.status === 401
          ? "Invalid email or password"
          : err instanceof ApiError
            ? err.message
            : "Sign in failed";
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
            <CardTitle>Sign in</CardTitle>
            <CardDescription>Enter your credentials to access the panel.</CardDescription>
          </CardHeader>
          <form onSubmit={onSubmit}>
            <CardContent className="flex flex-col gap-4">
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
                  autoComplete="current-password"
                  required
                />
              </div>
            </CardContent>
            <CardFooter className="mt-2 flex flex-col gap-3">
              <Button type="submit" className="w-full" disabled={submitting}>
                {submitting && <Loader2 className="size-4 animate-spin" />}
                Sign in
              </Button>
              {ssoEnabled && (
                <>
                  <div className="flex w-full items-center gap-3">
                    <Separator className="flex-1" />
                    <span className="text-muted-foreground text-xs">or</span>
                    <Separator className="flex-1" />
                  </div>
                  <Button asChild variant="outline" className="w-full">
                    <a href={SSO_LOGIN_URL}>Sign in with SSO</a>
                  </Button>
                </>
              )}
            </CardFooter>
          </form>
        </Card>
      </div>
    </main>
  );
}

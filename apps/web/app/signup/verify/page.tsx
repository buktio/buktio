"use client";

import * as React from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Loader2 } from "lucide-react";

import { verifySignup, ApiError } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";

function VerifyInner() {
  const router = useRouter();
  const params = useSearchParams();
  const token = params.get("token") ?? "";
  const [state, setState] = React.useState<"verifying" | "ok" | "error">("verifying");
  const [message, setMessage] = React.useState("");

  React.useEffect(() => {
    let active = true;
    if (!token) {
      setState("error");
      setMessage("Missing verification token.");
      return;
    }
    verifySignup(token)
      .then(() => {
        if (!active) return;
        setState("ok");
        setTimeout(() => router.replace("/"), 800);
      })
      .catch((err) => {
        if (!active) return;
        setState("error");
        setMessage(err instanceof ApiError ? err.message : "Verification failed.");
      });
    return () => {
      active = false;
    };
  }, [token, router]);

  return (
    <main className="bg-muted/30 flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
      <Card className="w-full max-w-sm">
        <CardHeader>
          <CardTitle>
            {state === "verifying" ? "Verifying…" : state === "ok" ? "Email verified" : "Verification failed"}
          </CardTitle>
          <CardDescription>
            {state === "verifying"
              ? "Confirming your email address."
              : state === "ok"
                ? "Taking you to your dashboard."
                : message}
          </CardDescription>
        </CardHeader>
        <CardContent>
          {state === "verifying" && <Loader2 className="text-muted-foreground size-6 animate-spin" />}
          {state === "error" && (
            <Button variant="outline" className="w-full" onClick={() => router.replace("/login")}>
              Back to sign in
            </Button>
          )}
        </CardContent>
      </Card>
    </main>
  );
}

export default function VerifyPage() {
  return (
    <React.Suspense fallback={null}>
      <VerifyInner />
    </React.Suspense>
  );
}

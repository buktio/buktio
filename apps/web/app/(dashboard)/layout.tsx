"use client";

import * as React from "react";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";

import { apiGet, authMe, getBranding, ApiError, type Me, type SetupStatus } from "@/lib/api";
import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar";
import { AppSidebar } from "@/app/(dashboard)/app-sidebar";
import { UserProvider } from "@/app/(dashboard)/user-context";

export default function DashboardLayout({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [session, setSession] = React.useState<Me | null>(null);
  const [status, setStatus] = React.useState<"loading" | "ready" | "redirecting">(
    "loading",
  );

  React.useEffect(() => {
    let active = true;

    async function bootstrap() {
      try {
        const setup = await apiGet<SetupStatus>("/setup/status");
        if (!active) return;
        if (!setup.initialized) {
          setStatus("redirecting");
          router.replace("/setup");
          return;
        }
        const me = await authMe();
        if (!active) return;
        setSession(me);
        setStatus("ready");
        // White-label theming (Enterprise): override the shadcn --primary token and
        // document title from the org's branding. No-op when unset/unlicensed.
        try {
          const brand = await getBranding();
          if (!active) return;
          if (brand.primary_color) {
            document.documentElement.style.setProperty("--primary", brand.primary_color);
          }
          if (brand.display_name) {
            document.title = `${brand.display_name} — buktio`;
          }
        } catch {
          // Branding is best-effort; never block the dashboard on it.
        }
      } catch (err) {
        if (!active) return;
        setStatus("redirecting");
        if (err instanceof ApiError && err.status === 401) {
          router.replace("/login");
        } else {
          // Network or server error — send to login as the safest fallback.
          router.replace("/login");
        }
      }
    }

    bootstrap();
    return () => {
      active = false;
    };
  }, [router]);

  if (status !== "ready" || !session) {
    return (
      <div className="flex min-h-svh items-center justify-center">
        <Loader2 className="text-muted-foreground size-6 animate-spin" />
      </div>
    );
  }

  return (
    <UserProvider user={session.user} features={session.features}>
      <SidebarProvider>
        <AppSidebar />
        <SidebarInset>{children}</SidebarInset>
      </SidebarProvider>
    </UserProvider>
  );
}

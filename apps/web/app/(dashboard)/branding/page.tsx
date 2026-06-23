"use client";

import * as React from "react";
import { Loader2, Palette, Save } from "lucide-react";
import { toast } from "sonner";

import { getBranding, setBranding, ApiError } from "@/lib/api";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Skeleton } from "@/components/ui/skeleton";

export default function BrandingPage() {
  const [loading, setLoading] = React.useState(true);
  const [loadError, setLoadError] = React.useState<string | null>(null);
  const [saving, setSaving] = React.useState(false);

  const [displayName, setDisplayName] = React.useState("");
  const [logoUrl, setLogoUrl] = React.useState("");
  const [primaryColor, setPrimaryColor] = React.useState("");
  const [emailFrom, setEmailFrom] = React.useState("");
  const [customDomain, setCustomDomain] = React.useState("");

  const load = React.useCallback(async () => {
    try {
      const b = await getBranding();
      setDisplayName(b.display_name ?? "");
      setLogoUrl(b.logo_url ?? "");
      setPrimaryColor(b.primary_color ?? "");
      setEmailFrom(b.email_from ?? "");
      setCustomDomain(b.custom_domain ?? "");
      setLoadError(null);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Failed to load branding";
      setLoadError(message);
      toast.error(message);
    } finally {
      setLoading(false);
    }
  }, []);

  React.useEffect(() => {
    load();
  }, [load]);

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    try {
      await setBranding({
        display_name: displayName.trim(),
        logo_url: logoUrl.trim(),
        primary_color: primaryColor.trim(),
        email_from: emailFrom.trim(),
        custom_domain: customDomain.trim(),
      });
      toast.success("Branding saved");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to save branding");
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <PageHeader crumbs={[{ label: "Branding" }]} />
      <div className="flex flex-1 flex-col gap-4 p-4 md:gap-6 md:p-6">
        {loading ? (
          <Skeleton className="h-96 w-full" />
        ) : loadError ? (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Palette className="size-4" /> Branding unavailable
              </CardTitle>
              <CardDescription>{loadError}</CardDescription>
            </CardHeader>
          </Card>
        ) : (
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <Palette className="size-4" /> White-label
              </CardTitle>
              <CardDescription>
                Primary color overrides the panel&apos;s accent. A custom domain needs DNS
                pointed here and is issued a cert on demand.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <form onSubmit={onSave}>
                <div className="grid gap-4">
                  <div className="grid gap-2">
                    <Label htmlFor="display-name">Display name</Label>
                    <Input
                      id="display-name"
                      value={displayName}
                      onChange={(e) => setDisplayName(e.target.value)}
                      placeholder="Acme Storage"
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="logo-url">Logo URL</Label>
                    <Input
                      id="logo-url"
                      value={logoUrl}
                      onChange={(e) => setLogoUrl(e.target.value)}
                      placeholder="https://acme.com/logo.svg"
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="primary-color">Primary color</Label>
                    <div className="flex items-center gap-2">
                      <Input
                        id="primary-color"
                        value={primaryColor}
                        onChange={(e) => setPrimaryColor(e.target.value)}
                        placeholder="#4f46e5 or oklch(...)"
                      />
                      <div
                        className="size-6 shrink-0 rounded border"
                        style={{ background: primaryColor }}
                      />
                    </div>
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="email-from">From email</Label>
                    <Input
                      id="email-from"
                      type="email"
                      value={emailFrom}
                      onChange={(e) => setEmailFrom(e.target.value)}
                      placeholder="no-reply@acme.com"
                    />
                  </div>
                  <div className="grid gap-2">
                    <Label htmlFor="custom-domain">Custom domain</Label>
                    <Input
                      id="custom-domain"
                      value={customDomain}
                      onChange={(e) => setCustomDomain(e.target.value)}
                      placeholder="storage.acme.com"
                    />
                  </div>
                  <div>
                    <Button type="submit" disabled={saving}>
                      {saving ? (
                        <Loader2 className="size-4 animate-spin" />
                      ) : (
                        <Save className="size-4" />
                      )}
                      Save
                    </Button>
                  </div>
                </div>
              </form>
            </CardContent>
          </Card>
        )}
      </div>
    </>
  );
}

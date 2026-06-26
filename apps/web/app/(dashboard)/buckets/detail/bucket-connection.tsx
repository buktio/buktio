"use client";

import * as React from "react";
import { toast } from "sonner";

import { apiGet, ApiError, type DocsSnippets } from "@/lib/api";
import { CopyButton } from "@/components/copy-button";
import { CodeBlock } from "@/components/code-block";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

const TOOLS: { key: keyof DocsSnippets["snippets"]; label: string }[] = [
  { key: "aws_cli", label: "AWS CLI" },
  { key: "rclone", label: "rclone" },
  { key: "boto3", label: "boto3" },
  { key: "node_sdk", label: "Node SDK" },
  { key: "restic", label: "restic" },
];

function Field({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-center justify-between gap-2 py-2">
      <div className="flex min-w-0 flex-col">
        <span className="text-muted-foreground text-xs">{label}</span>
        <span className="truncate font-mono text-sm">{value}</span>
      </div>
      <CopyButton value={value} label={`Copy ${label.toLowerCase()}`} />
    </div>
  );
}

export function BucketConnection({ bucketId }: { bucketId: string }) {
  const [snippets, setSnippets] = React.useState<DocsSnippets | null>(null);

  React.useEffect(() => {
    let active = true;
    apiGet<DocsSnippets>(`/docs/snippets?bucket_id=${bucketId}`)
      .then((s) => active && setSnippets(s))
      .catch((err) => {
        if (active) {
          toast.error(err instanceof ApiError ? err.message : "Failed to load connection info");
        }
      });
    return () => {
      active = false;
    };
  }, [bucketId]);

  if (!snippets) {
    return <Skeleton className="h-96 w-full" />;
  }

  return (
    <div className="flex flex-col gap-6">
      <Card>
        <CardHeader>
          <CardTitle>Endpoint</CardTitle>
          <CardDescription>S3-compatible connection details for this bucket.</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-0">
          <Field label="Endpoint" value={snippets.endpoint} />
          <Separator />
          <Field label="Region" value={snippets.region} />
          <Separator />
          <Field label="Bucket" value={snippets.bucket} />
          <Separator />
          <Field label="Addressing" value={snippets.addressing} />
          <Separator />
          <Field label="Signature" value={snippets.signature} />
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Client snippets</CardTitle>
          <CardDescription>Copy-paste configuration for common S3 clients.</CardDescription>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="aws_cli">
            <TabsList>
              {TOOLS.map((t) => (
                <TabsTrigger key={t.key} value={t.key}>
                  {t.label}
                </TabsTrigger>
              ))}
            </TabsList>
            {TOOLS.map((t) => (
              <TabsContent key={t.key} value={t.key} className="mt-4">
                <CodeBlock code={snippets.snippets[t.key]} />
              </TabsContent>
            ))}
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}

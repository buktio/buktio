"use client";

import * as React from "react";
import { toast } from "sonner";

import {
  apiGet,
  ApiError,
  type Bucket,
  type DocsSnippets,
  type ListEnvelope,
} from "@/lib/api";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { CodeBlock } from "@/components/code-block";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { Label } from "@/components/ui/label";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

const TOOLS: { key: keyof DocsSnippets["snippets"]; label: string }[] = [
  { key: "aws_cli", label: "AWS CLI" },
  { key: "rclone", label: "rclone" },
  { key: "boto3", label: "boto3" },
  { key: "node_sdk", label: "Node SDK" },
  { key: "restic", label: "restic" },
];

export default function DocsPage() {
  const [buckets, setBuckets] = React.useState<Bucket[]>([]);
  const [bucketId, setBucketId] = React.useState<string>("");
  const [snippets, setSnippets] = React.useState<DocsSnippets | null>(null);
  const [loading, setLoading] = React.useState(true);

  React.useEffect(() => {
    apiGet<ListEnvelope<Bucket>>("/buckets")
      .then((res) => {
        setBuckets(res.data);
        if (res.data.length > 0) setBucketId(res.data[0].id);
      })
      .catch(() => {});
  }, []);

  React.useEffect(() => {
    let active = true;
    setLoading(true);
    const qs = bucketId ? `?bucket_id=${bucketId}` : "";
    apiGet<DocsSnippets>(`/docs/snippets${qs}`)
      .then((s) => active && setSnippets(s))
      .catch((err) => {
        if (active) {
          toast.error(err instanceof ApiError ? err.message : "Failed to load snippets");
        }
      })
      .finally(() => active && setLoading(false));
    return () => {
      active = false;
    };
  }, [bucketId]);

  return (
    <>
      <PageHeader crumbs={[{ label: "Docs" }]} />
      <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
        <Card>
          <CardHeader>
            <CardTitle>Connect a client</CardTitle>
            <CardDescription>
              Ready-to-use configuration snippets for popular S3 clients. Choose a bucket to fill
              in the values.
            </CardDescription>
          </CardHeader>
          <CardContent className="flex flex-col gap-6">
            <div className="grid max-w-sm gap-2">
              <Label htmlFor="docs-bucket">Bucket</Label>
              <Select
                value={bucketId}
                onValueChange={setBucketId}
                disabled={buckets.length === 0}
              >
                <SelectTrigger id="docs-bucket">
                  <SelectValue
                    placeholder={buckets.length === 0 ? "No buckets available" : "Select a bucket"}
                  />
                </SelectTrigger>
                <SelectContent>
                  {buckets.map((b) => (
                    <SelectItem key={b.id} value={b.id}>
                      {b.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            {loading || !snippets ? (
              <Skeleton className="h-72 w-full" />
            ) : (
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
            )}
          </CardContent>
        </Card>
      </div>
    </>
  );
}

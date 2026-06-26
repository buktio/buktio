"use client";

import * as React from "react";
import { useSearchParams, useRouter } from "next/navigation";
import { Loader2, Trash2 } from "lucide-react";
import { toast } from "sonner";

import {
  apiGet,
  apiSend,
  ApiError,
  listClusters,
  type Bucket,
  type Cluster,
  type ClusterCapabilities,
} from "@/lib/api";
import { clusterForBucket, providerLabel } from "@/lib/clusters";
import { PageHeader } from "@/app/(dashboard)/page-header";
import { VisibilityBadge } from "@/components/visibility-badge";
import { Badge } from "@/components/ui/badge";
import { ObjectBrowser } from "@/app/(dashboard)/buckets/detail/object-browser";
import { BucketSettings } from "@/app/(dashboard)/buckets/detail/bucket-settings";
import { BucketConnection } from "@/app/(dashboard)/buckets/detail/bucket-connection";
import { BucketCors } from "@/app/(dashboard)/buckets/detail/bucket-cors";
import { BucketTrash } from "@/app/(dashboard)/buckets/detail/bucket-trash";
import { BucketWebhooks } from "@/app/(dashboard)/buckets/detail/bucket-webhooks";
import { BucketVersions } from "@/app/(dashboard)/buckets/detail/bucket-versions";
import { BucketReplication } from "@/app/(dashboard)/buckets/detail/bucket-replication";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export default function BucketDetailPage() {
  // useSearchParams must be read inside a Suspense boundary for static export.
  return (
    <React.Suspense fallback={<div className="flex flex-1 flex-col gap-4 p-4 md:p-6" />}>
      <BucketDetail />
    </React.Suspense>
  );
}

function BucketDetail() {
  const id = useSearchParams().get("id") ?? "";
  const router = useRouter();
  const [bucket, setBucket] = React.useState<Bucket | null>(null);
  const [clusters, setClusters] = React.useState<Cluster[]>([]);
  const [notFound, setNotFound] = React.useState(false);
  const [deleting, setDeleting] = React.useState(false);

  const load = React.useCallback(async () => {
    try {
      const b = await apiGet<Bucket>(`/buckets/${id}`);
      setBucket(b);
    } catch (err) {
      if (err instanceof ApiError && err.status === 404) {
        setNotFound(true);
      } else {
        toast.error(err instanceof ApiError ? err.message : "Failed to load bucket");
      }
    }
  }, [id]);

  React.useEffect(() => {
    load();
    listClusters()
      .then((res) => setClusters(res.data))
      .catch(() => {});
  }, [load]);

  const cluster = bucket ? clusterForBucket(clusters, bucket.cluster_id) : undefined;
  const capabilities: ClusterCapabilities | undefined = cluster?.capabilities;

  async function onDelete() {
    setDeleting(true);
    try {
      await apiSend<void>("DELETE", `/buckets/${id}`);
      toast.success("Bucket deleted");
      router.replace("/buckets");
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : "Failed to delete bucket");
      setDeleting(false);
    }
  }

  return (
    <>
      <PageHeader
        crumbs={[
          { label: "Buckets", href: "/buckets" },
          { label: bucket?.name ?? "…" },
        ]}
        actions={
          bucket ? (
            <AlertDialog>
              <AlertDialogTrigger asChild>
                <Button variant="outline" size="sm">
                  <Trash2 className="size-4" />
                  Delete
                </Button>
              </AlertDialogTrigger>
              <AlertDialogContent>
                <AlertDialogHeader>
                  <AlertDialogTitle>Delete bucket “{bucket.name}”?</AlertDialogTitle>
                  <AlertDialogDescription>
                    This permanently empties and removes the bucket along with all of its
                    objects. This action cannot be undone.
                  </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                  <AlertDialogCancel disabled={deleting}>Cancel</AlertDialogCancel>
                  <AlertDialogAction
                    onClick={(e) => {
                      e.preventDefault();
                      onDelete();
                    }}
                    disabled={deleting}
                    className="bg-destructive text-white hover:bg-destructive/90"
                  >
                    {deleting && <Loader2 className="size-4 animate-spin" />}
                    Delete
                  </AlertDialogAction>
                </AlertDialogFooter>
              </AlertDialogContent>
            </AlertDialog>
          ) : undefined
        }
      />
      <div className="flex flex-1 flex-col gap-4 p-4 md:p-6">
        {notFound ? (
          <div className="text-muted-foreground text-sm">Bucket not found.</div>
        ) : !bucket ? (
          <Skeleton className="h-96 w-full" />
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-3">
              <h2 className="text-xl font-semibold">{bucket.name}</h2>
              <VisibilityBadge visibility={bucket.visibility} />
              {cluster && (
                <Badge variant="outline" className="font-normal">
                  {providerLabel(cluster.provider)}
                </Badge>
              )}
            </div>

            <Tabs defaultValue="objects">
              <TabsList>
                <TabsTrigger value="objects">Objects</TabsTrigger>
                <TabsTrigger value="trash">Trash</TabsTrigger>
                <TabsTrigger value="settings">Settings</TabsTrigger>
                {(!capabilities || capabilities.bucket_cors) && (
                  <TabsTrigger value="cors">CORS</TabsTrigger>
                )}
                {capabilities?.object_versioning && (
                  <TabsTrigger value="versions">Versions</TabsTrigger>
                )}
                <TabsTrigger value="webhooks">Webhooks</TabsTrigger>
                <TabsTrigger value="replication">Replication</TabsTrigger>
                <TabsTrigger value="connection">Connection</TabsTrigger>
              </TabsList>

              <TabsContent value="objects" className="mt-4">
                <ObjectBrowser bucketId={bucket.id} bucketName={bucket.name} />
              </TabsContent>

              <TabsContent value="trash" className="mt-4">
                <BucketTrash bucketId={bucket.id} />
              </TabsContent>

              <TabsContent value="settings" className="mt-4">
                <BucketSettings
                  bucket={bucket}
                  onUpdated={setBucket}
                  capabilities={capabilities}
                />
              </TabsContent>

              {(!capabilities || capabilities.bucket_cors) && (
                <TabsContent value="cors" className="mt-4">
                  <BucketCors bucketId={bucket.id} />
                </TabsContent>
              )}

              {capabilities?.object_versioning && (
                <TabsContent value="versions" className="mt-4">
                  <BucketVersions bucketId={bucket.id} />
                </TabsContent>
              )}

              <TabsContent value="webhooks" className="mt-4">
                <BucketWebhooks bucketId={bucket.id} />
              </TabsContent>

              <TabsContent value="replication" className="mt-4">
                <BucketReplication bucketId={bucket.id} />
              </TabsContent>

              <TabsContent value="connection" className="mt-4">
                <BucketConnection bucketId={bucket.id} />
              </TabsContent>
            </Tabs>
          </>
        )}
      </div>
    </>
  );
}

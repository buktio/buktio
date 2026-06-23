import { Badge } from "@/components/ui/badge";

/** Badge showing whether a bucket is private or a public website. */
export function VisibilityBadge({ visibility }: { visibility: string }) {
  const isPublic = visibility === "public_website";
  return (
    <Badge variant={isPublic ? "default" : "secondary"}>
      {isPublic ? "Public website" : "Private"}
    </Badge>
  );
}

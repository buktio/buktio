import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Static HTML/JS export — the whole UI is a client-side SPA that gets embedded
  // into the Go binary (internal/webui) and served same-origin alongside /api/*,
  // so no Node runtime and no /api rewrite are needed in production.
  output: "export",
  // Dynamic segments (buckets/[id], clusters/[id]) read their id client-side via
  // useParams; the embedding Go handler serves index.html as the SPA fallback for
  // any unmatched route, so deep-links resolve through client-side routing.
  trailingSlash: true,
  images: { unoptimized: true },
};

export default nextConfig;

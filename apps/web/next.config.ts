import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Emit a self-contained server bundle for the Docker runtime image.
  output: "standalone",
  // The browser talks to the Go API through this rewrite (same-origin in prod the
  // edge proxy fronts /api/*; in dev we proxy to the API directly).
  async rewrites() {
    const apiUrl = process.env.BUKTIO_API_URL ?? "http://localhost:8080";
    return [{ source: "/api/:path*", destination: `${apiUrl}/api/:path*` }];
  },
};

export default nextConfig;

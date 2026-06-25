# syntax=docker/dockerfile:1
# buktio web image (Next.js standalone). Build context is the repo root.
FROM node:22-alpine AS deps
WORKDIR /app
RUN corepack enable
COPY apps/web/package.json apps/web/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM node:22-alpine AS build
WORKDIR /app
RUN corepack enable
COPY --from=deps /app/node_modules ./node_modules
COPY apps/web/ ./
ENV NEXT_TELEMETRY_DISABLED=1
RUN pnpm build

FROM node:22-alpine AS runtime
WORKDIR /app
ENV NODE_ENV=production NEXT_TELEMETRY_DISABLED=1 PORT=3000 HOSTNAME=0.0.0.0
RUN addgroup -g 10002 nodejs && adduser -D -u 10002 -G nodejs nextjs
# Next.js standalone output (requires output: 'standalone' in next.config.ts).
COPY --from=build /app/public ./public
COPY --from=build --chown=nextjs:nodejs /app/.next/standalone ./
COPY --from=build --chown=nextjs:nodejs /app/.next/static ./.next/static
LABEL org.opencontainers.image.source="https://github.com/buktio/buktio"
USER nextjs
EXPOSE 3000
CMD ["node", "server.js"]

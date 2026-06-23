# buktio web — Next.js panel (shadcn/ui only)

**Hard UI constraint:** the panel uses **only** official shadcn/ui registry components,
takes its app-shell/layout from official shadcn **blocks**, and writes **no bespoke CSS** —
only shadcn components, shadcn-shipped Tailwind utilities, the theme CSS variables in
`app/globals.css`, and built-in dark mode (`next-themes`).

## Setup

```bash
pnpm install

# Pull the app-shell + auth blocks (run once; generates components under components/ui).
pnpm dlx shadcn@latest add dashboard-01 sidebar-07 login-03

# Pull the component set used across the panel (one shot).
pnpm dlx shadcn@latest add \
  sidebar breadcrumb button button-group input input-group label textarea \
  select native-select combobox checkbox switch radio-group \
  card item separator badge avatar tooltip hover-card \
  table data-table pagination \
  dialog alert-dialog sheet drawer popover dropdown-menu command \
  tabs accordion collapsible \
  alert sonner skeleton spinner progress empty field \
  chart calendar date-picker scroll-area kbd input-otp aspect-ratio

pnpm dev   # http://localhost:3000  (proxies /api/* to BUKTIO_API_URL)
```

> `shadcn init` has already been run for you — `components.json`, the theme tokens in
> `app/globals.css`, and `lib/utils.ts` (`cn`) are in place. Do **not** re-run `init`.

## Routes (built in M5+)

`/setup` (wizard) · `/login` · `/` (dashboard) · `/buckets` · `/buckets/[name]` ·
`/buckets/[name]/objects` (object browser) · `/keys` · `/usage` · `/audit` · `/settings` ·
`/docs`. Each route maps to specific shadcn blocks/components — see the development plan §9
for the full page → component mapping, including how the few gaps with no native component
(stepper, dropzone, KPI card, code block) are composed strictly from shadcn primitives.

## Data layer

Reads via React Server Components + route handlers that proxy the Go API (secrets stay
server-side); interactive tables use TanStack Query. Show-secret-once values are returned only
in the create response and never persisted client-side.

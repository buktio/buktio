# Contributing to buktio

Thanks for taking the time to look at buktio! buktio is a free, open-source (Apache-2.0)
self-hosted control plane for S3-compatible object storage. It's maintained by a single
person, so please be patient — reviews and replies may take a little while. Clear,
focused contributions get merged fastest, and friendly questions are always welcome.

This guide covers the Apache-2.0 OSS core. Please read it before opening an issue or pull
request.

## Ways to contribute

You don't have to write code to help:

- **Bug reports.** Open a GitHub issue with what you did, what you expected, and what
  actually happened. Include version, deployment mode (full Docker stack vs. dev infra +
  local app), backend (the bundled Garage engine or an external S3 like R2/B2/Ceph), and
  relevant logs. Minimal reproductions are gold.
- **Feature ideas.** For anything bigger than a small tweak, please start a **GitHub
  Discussion** first so we can agree on the shape before you build it. This saves you from
  writing code that doesn't fit the project's direction. Small, obvious improvements can go
  straight to an issue or PR.
- **Docs.** Typos, unclear steps, missing prerequisites, better examples — docs PRs are
  some of the most valuable contributions and the easiest to review.
- **Code.** Bug fixes, tests, and well-scoped features. See the dev setup and PR checklist
  below.

## Project layout

A quick map of the repo:

- `apps/api/` — the Go REST API (the control plane); the server entrypoint is
  `apps/api/cmd/server`.
- `apps/web/` — the web panel: a separate Next.js + TypeScript + shadcn/ui project (Node 22+, pnpm).
- `internal/` — Go internal packages shared by the API and CLI (the Apache-2.0 core).
- `cmd/` — entrypoints: `cmd/buktio` (CLI), `cmd/buktio-s3proxy` (the S3 traffic-metering
  proxy), and `cmd/buktio-api-ee` (the paid API build).
- `ee/` — the paid (Pro/Enterprise/Hosted) code. **This tree is source-available, NOT
  Apache-2.0, and is not open to external pull requests.** Please don't send PRs that touch
  `ee/` or `cmd/buktio-api-ee` — they'll be closed with thanks but can't be merged. The
  open-core boundary is deliberate, and we'd rather be upfront about it than waste your
  time. Everything else above is fair game and genuinely welcomed.

The free OSS core has **no artificial limits** (unlimited buckets, keys, objects, and
nodes). The paid `ee/` tree adds *new* capabilities for teams — it never removes anything
from the free core.

An important invariant: **`internal/*` must never import `ee/*`.** The dependency arrow is
one-way (`ee/ -> internal/`) so the OSS build stays fully independent. This is enforced by a
test (`internal/app/import_guard_test.go`), so `make test` will catch violations.

## Dev setup

Full instructions live in **[DEVELOPMENT.md](DEVELOPMENT.md)**, which documents two modes:
dev infra + local app, or the full Docker stack. The short version, using the `make`
targets (run `make help` to list them all):

```sh
make dev-up        # start infra only (Postgres + Garage)
make api-local     # run the API locally against that infra
make web-install   # install web deps (pnpm)
make web-dev       # run the Next.js panel in dev mode

make build         # build the Go binaries
make test          # go test ./... -race -short
make lint          # go vet + gofmt check

make up            # build & start the full Docker stack
make dev-down      # tear down the dev infra
```

You'll need Go 1.25+, Node 22+ with pnpm, and Docker. Garage is shipped unmodified and
runs as a separate process — you don't need to install or configure it by hand, and
contributions must never modify, fork, vendor, or statically link Garage (buktio talks to
it over HTTP only). See **[docs/LICENSING.md](docs/LICENSING.md)** for why this boundary
matters.

## Before you open a PR

Please run, from the repo root:

```sh
make lint && make test && (cd apps/web && pnpm typecheck && pnpm build)
```

And a few habits that make review smooth:

- **Keep changes focused.** One logical change per PR. Unrelated cleanups belong in their
  own PR.
- **Update the docs** when you change behavior, flags, or APIs.
- **Add or update tests** for code changes where it makes sense.

## Commit and PR conventions

- **Small PRs.** Smaller is faster to review and easier to get right.
- **Descriptive messages.** Explain the *why*, not just the *what*. Reference the issue it
  addresses (e.g. `Fixes #123`).
- **DCO sign-off (required).** Sign your commits with `git commit -s`, which adds a
  `Signed-off-by:` line certifying you wrote the change and can submit it. Inbound
  contributions are accepted under Apache-2.0 (inbound = outbound) — the same license as the
  core.
- **Respect the import invariant.** Don't add imports of `ee/*` from `internal/*` (the test
  will fail anyway).

## Good first issues

New here? Look for issues labeled **`good-first-issue`** on GitHub. If none are open, here
are realistic, genuinely small examples of the kind of thing that makes a great first
contribution:

- Fix a typo or clarify a confusing step in the docs (DEVELOPMENT.md, the README, or `docs/`).
- Add a unit test for a small mapping/helper function in `internal/` (e.g. an S3-error or
  capability mapping) that isn't covered yet.
- Improve a vague error message in the API so it tells the user what actually went wrong
  and how to fix it.
- A small UI polish in the object browser in `apps/web` — e.g. a clearer empty state, a
  tooltip, or a loading indicator.
- Add a missing field to an API response and surface it in the panel (small, end-to-end,
  touches both sides without being large).

If you're unsure whether something is too big, ask in the issue first — happy to help scope
it down.

## Reporting security issues

**Please do not open public issues for security problems.** Follow the disclosure process
in **[SECURITY.md](SECURITY.md)** so the issue can be handled responsibly. Vulnerabilities
in the bundled Garage engine itself should be reported upstream to the Garage project.

## Code of conduct

Be kind, be patient, and assume good intent. We're all here to make a useful tool. Harassment
or hostility isn't welcome. With one maintainer behind the project, a little courtesy goes a
long way — thanks for contributing!

# Design: aptabase-plus (management API fork) + terraform-provider-aptabase

Date: 2026-09-10
Status: Approved for spec, pending user review before planning

## 1. Motivation

Aptabase (aptabase.com / aptabase/aptabase on GitHub) has no API-key or
bearer-token authentication for its account-management endpoints
(`/api/_apps`, sharing, ownership transfer). Those routes are gated purely
by an ASP.NET Core cookie session set via GitHub/Google OAuth or a
magic-link email flow. There is no way to authenticate a Terraform
provider — or any non-interactive automation — against stock Aptabase
today. This is a known, unimplemented gap upstream
([aptabase/aptabase#145](https://github.com/aptabase/aptabase/issues/145)).

We therefore build two things:

- **`aptabase-plus`** — a maintained downstream distribution of Aptabase
  that tracks upstream and adds a stable, versioned, API-key-authenticated
  management API. Self-hosted only.
- **`terraform-provider-aptabase`** — a Terraform provider (Go,
  terraform-plugin-framework) that manages apps, app shares, and API keys
  against an `aptabase-plus` instance.

The provider is **not usable against Aptabase Cloud (the official SaaS at
app.aptabase.com) or a stock self-hosted Aptabase instance** — only
against a self-hosted `aptabase-plus` instance. This must be documented
prominently in both repos' READMEs and in the provider's registry docs,
the same way `terraform-provider-bugsink` prominently documents its
delete-endpoint limitation.

We will also open an upstream design discussion on aptabase#145 proposing
the API-key model below, so a merge back into aptabase/aptabase remains
possible later. `aptabase-plus` does not block on that outcome.

## 2. Part A — `aptabase-plus`

### 2.1 Positioning

- Repo: `nmehlei/aptabase-plus`, public, MIT (matching upstream's license).
- A rebasing downstream fork: periodically merges `aptabase/aptabase`
  main, plus a small, isolated set of "plus" commits (API keys + `/api/v0`
  management routes + settings UI). Keeping the plus-specific diff small
  and additive (new files > modified files) minimizes merge conflicts on
  each upstream sync.
- Own README explaining the relationship to upstream, why it exists, and
  that it's not officially affiliated with Aptabase.
- Own Docker image: `ghcr.io/nmehlei/aptabase-plus`.

### 2.2 API key model

- One key type: **user-scoped**. A key inherits exactly the permissions
  of the user who created it (same as if that user were signed in via
  cookie) — including the ability to create/revoke other keys for that
  same user. No per-app or read-only scoping in v1. This mirrors the
  blast radius of the existing cookie session and keeps v1 small; scoped
  keys are a documented future improvement (and closer to what #145
  originally proposed).
- Format: `aptb_<32 random url-safe base64 chars>`. Stored as SHA-256
  hash + an 8-char display prefix (for the list UI) + `name` + `user_id`
  FK + `created_at` + `last_used_at` (best-effort, updated
  fire-and-forget on auth, not blocking the request) + nullable
  `expires_at`.
- New table `api_keys` via FluentMigrator migration `0014_AddApiKeys.cs`,
  following the exact pattern of `0005_AppShares.cs`.
- New `IAuthenticationHandler` (`ApiKeyAuthenticationHandler`) registered
  as an additional scheme alongside the existing cookie scheme. Reads
  `Authorization: Bearer <key>`, hashes, looks up `api_keys` joined to
  `users`, and builds the identical `ClaimsIdentity` (`id`/`name`/`email`)
  the cookie path builds — so existing `GetCurrentUserIdentity()` calls
  need no changes.
- `[IsAuthenticated]` is extended (or a new `[IsAuthenticatedOrApiKey]`
  is introduced and applied to the new `/api/v0` controllers) to accept
  either scheme via ASP.NET's policy/forwarding scheme mechanism.
- Key management (`POST/GET/DELETE /api/v0/api-keys`) is reachable by
  **either** cookie or API key — a key can mint further keys for the
  same user. Bootstrapping the very first key still requires signing in
  once through the web UI.

### 2.3 New `/api/v0` management namespace

A new, additive, versioned public namespace — **not** a modification of
the existing internal `/api/_apps` routes, which stay SPA-only and can
keep changing shape freely. `/api/v0` wraps the same underlying query/
command logic (`AppQueries`, existing SQL) behind a small, deliberately
stable contract:

- `GET /api/v0/apps` — list apps owned by or shared with the
  authenticated user.
- `POST /api/v0/apps` — create (`name`).
- `GET /api/v0/apps/{appId}` — read.
- `PUT /api/v0/apps/{appId}` — update (`name`, `icon`).
- `DELETE /api/v0/apps/{appId}` — soft-delete (matches existing
  `deleted_at` semantics — this already works, unlike Bugsink's
  situation).
- `GET /api/v0/apps/{appId}/shares` — list shares.
- `PUT /api/v0/apps/{appId}/shares/{email}` — add a share.
- `DELETE /api/v0/apps/{appId}/shares/{email}` — remove a share.
- `GET /api/v0/api-keys` — list the caller's keys (prefix + metadata
  only, never the secret).
- `POST /api/v0/api-keys` — create (`name`, optional `expires_at`);
  response includes the plaintext key **once**.
- `DELETE /api/v0/api-keys/{keyId}` — revoke.

Ownership-transfer requests (`app_requests`) are intentionally **not**
exposed in `/api/v0` — they're an interactive accept/reject workflow
between two humans and a poor fit for Terraform-managed state.

### 2.4 Settings UI

A "API Keys" panel under the existing account/settings area of the React
app (`src/webapp`), listing keys with prefix/created/last-used/expiry,
a "Create key" dialog that shows the secret once with a copy button and
a clear "this won't be shown again" warning, and revoke buttons. Scoped
to be a small, focused addition (~150–250 LOC), styled to match the
existing settings pages rather than introducing new UI patterns.

### 2.5 Testing

- Unit tests for the auth handler (valid key, expired key, revoked key,
  malformed header) and the key hashing/generation helpers.
- Integration tests under `tests/IntegrationTests`, extending the existing
  `CustomWebApplicationFactory`/`AccountClient` pattern, covering the
  full `/api/v0` surface (create app via key, list, update, delete,
  share/unshare, key create/list/revoke, and auth failure cases).

### 2.6 Release process

- `GitVersion.yml` (Mainline mode, matching `terraform-provider-bugsink`
  and `terraform-provider-revenuecat`).
- GitHub Actions: `ci.yml` (build, unit + integration tests, mirroring
  upstream's existing `ci.yml`) and a `release.yml` that tags via
  GitVersion and publishes a signed Docker image to GHCR, following the
  same single-workflow tag+release pattern used in
  `terraform-provider-bugsink` (avoids the tag-push-doesn't-trigger-
  workflows pitfall).
- A `docs/upstream-sync.md` runbook describing how periodic merges from
  `aptabase/aptabase` main are performed and conflict-resolved.

### 2.7 Upstream discussion

Post a comment on aptabase#145 summarizing the user-scoped key + `/api/v0`
design above, linking to `aptabase-plus`, and offering to send a
scoped-down PR if maintainers are interested. Not a blocking dependency
for anything else in this plan.

## 3. Part B — `terraform-provider-aptabase`

### 3.1 Layout and stack

Matches `terraform-provider-bugsink` exactly (the more recent, more
refined of the two reference providers), per the user's global
preference for Go/`.NET` backends and this project's established
pattern:

```
src/
  main.go
  aptabase/          # standalone HTTP client, no Terraform imports
    client.go
    apps.go
    shares.go
    api_keys.go
    errors.go
  provider/
    provider.go
    app_resource.go
    app_share_resource.go
    api_key_resource.go
    app_data_source.go
    apps_data_source.go
tests/
  unit/
  acceptance/        # disposable aptabase-plus via docker-compose, like bugsink
docs/                # generated by tfplugindocs
examples/
```

- Go 1.26+, `hashicorp/terraform-plugin-framework` (SDKv2 not used, per
  the user's stated preference and both reference providers).
- `GitVersion.yml` (Mainline), `.goreleaser.yml` (GPG-signed, same shape
  as bugsink's, `dir: src`), `terraform-registry-manifest.json`.
- `AGENTS.md`/`CLAUDE.md` documenting the self-hosted-`aptabase-plus`-only
  constraint prominently, mirroring bugsink's delete-endpoint policy note.

### 3.2 Provider configuration

```hcl
provider "aptabase" {
  endpoint = "https://aptabase.example.com"   # required
  token    = var.aptabase_token               # optional, sensitive; falls back to APTABASE_TOKEN
}
```

### 3.3 Resources and data sources (v1)

- `aptabase_app` — `id` (computed), `name` (required), `icon` (optional,
  base64), `app_key` (computed, sensitive — the SDK ingestion key). Full
  CRUD + import (`terraform import aptabase_app.x <app_id>`).
- `aptabase_app_share` — composite-keyed by `app_id` + `email`. Create =
  `PUT .../shares/{email}`, Delete = `DELETE .../shares/{email}`. No
  update (email/app_id are the whole resource); changing either forces
  replacement.
- `aptabase_api_key` — `id`, `name` (required, forces replacement — keys
  can't be renamed via the API), `expires_at` (optional, forces
  replacement), `key` (computed, sensitive, **only populated on create**;
  Terraform state will show it as known-after-apply going forward — this
  needs a prominent doc callout that losing the key means tainting and
  recreating the resource, generating a new secret). Delete = revoke.
- Data sources: `aptabase_app` (by `id`), `aptabase_apps` (list, for
  discovering existing apps to import).

### 3.4 Testing

- Unit tests for the client and schema/plan-modification logic.
- Acceptance tests (`tests/acceptance`) spin up a disposable
  `aptabase-plus` via `docker-compose.yml` (mirroring bugsink's
  `up.sh`/`down.sh` pattern) and run real `terraform apply`/`destroy`
  cycles through `hashicorp/terraform-plugin-testing`, covering
  create/read/update/delete, import, and drift detection for each
  resource.

### 3.5 Publishing

Terraform Registry from day one (per your answer): GPG signing key
registered with the registry, `terraform-registry-manifest.json`,
`tfplugindocs`-generated documentation committed under `docs/`, and a
registry listing at `registry.terraform.io/nmehlei/aptabase` once the
first signed GitHub release exists. The registry README/index page will
lead with the self-hosted-`aptabase-plus`-only requirement so nobody
points it at Aptabase Cloud by mistake.

## 4. Sequencing

The provider is entirely dependent on `aptabase-plus`'s `/api/v0`
contract, so work is sequential, not parallel, at the start:

1. `aptabase-plus`: migration + auth handler + `/api/v0` controllers +
   tests (backend-complete, usable via curl/Postman).
2. `aptabase-plus`: settings UI for key management.
3. `aptabase-plus`: CI/release workflow, first tagged image.
4. Post upstream discussion comment on aptabase#145 (can happen any time
   after step 1).
5. `terraform-provider-aptabase`: scaffold + client + `aptabase_app` +
   acceptance harness against a tagged `aptabase-plus` image.
6. `terraform-provider-aptabase`: `aptabase_app_share`, `aptabase_api_key`,
   data sources.
7. `terraform-provider-aptabase`: docs, GoReleaser, registry publish.

Given the size, this will be written as **two separate implementation
plans** (one per repo) via the writing-plans skill, executed in the order
above, rather than one combined plan.

## 5. Open risks

- **Upstream drift**: every `aptabase-plus` sync with upstream `main` can
  touch files the plus-specific commits also touch (e.g. `Program.cs`
  auth wiring, `AppQueries.cs`). Mitigated by keeping plus changes
  additive where possible and documenting the sync runbook (2.6).
- **No cloud SaaS support**: this is a permanent, by-design limitation,
  not a v1 gap — must stay prominent in docs indefinitely.
- **Key bootstrapping**: the very first API key always requires an
  interactive cookie sign-in; Terraform can't fully bootstrap a fresh
  instance from zero. Document this in the provider's "Getting Started."
- **Maintenance burden of two repos**: acceptable given the user chose
  the full upstream-quality path explicitly, but worth flagging that
  `aptabase-plus` needs ongoing attention (upstream syncs) independent of
  the provider's own release cadence.

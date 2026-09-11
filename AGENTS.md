# Terraform Provider for Aptabase

## Mission

Manage apps, app shares, and API keys on a self-hosted `aptabase-plus`
instance via Go and HashiCorp's Terraform Plugin Framework. Do not use
SDKv2 for new code.

## Scope constraint (permanent, not a v1 gap)

This provider only works against `aptabase-plus`
(github.com/nmehlei/aptabase-plus), a downstream distribution that adds
API-key authentication to Aptabase's management API. It does not and
cannot work against Aptabase Cloud or stock self-hosted Aptabase — neither
exposes a management API a non-interactive client can authenticate
against. Keep this documented prominently everywhere a user would land
before running `terraform init`.

## First action

Read `docs/superpowers/specs/2026-09-10-aptabase-plus-and-terraform-provider-design.md`.

## Security and testing

- Accept credentials only through a sensitive provider attribute or
  `APTABASE_TOKEN`; never log, return, or write them to fixtures.
- API keys returned by `aptabase_api_key` are shown by the server exactly
  once, at creation. Terraform state necessarily retains the plaintext
  (as any secret-managing resource's state does) — document this loudly
  and mark the attribute `Sensitive`.
- Acceptance tests run against a disposable `aptabase-plus` +
  Postgres + ClickHouse + Mailcatcher stack (`tests/acceptance`). Never
  point tests at a shared or production instance.

## Repository conventions

- Go source under `src/`, tests under `tests/`.
- Use semantic versioning, generated Registry documentation, signed
  releases, and a public GitHub release workflow.

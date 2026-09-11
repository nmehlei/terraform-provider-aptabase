# Acceptance tests

Run a disposable `aptabase-plus` stack (Postgres, ClickHouse, Mailcatcher,
and the app itself) and drive real `terraform apply`/`destroy` cycles
against it via `hashicorp/terraform-plugin-testing`.

```bash
./up.sh
TF_ACC=1 go test ./tests/acceptance/... -v
./down.sh
```

`bootstrap.go`'s `Bootstrap(t)` signs up a fresh account through the real
email flow (reading Mailcatcher) and mints an API key.

**Accounts are shared, not per-test.** aptabase-plus enforces a sign-up
rate limit of 4 registrations/hour/IP, so registering once per test would
exhaust that limit almost immediately in CI. Instead, every `TestAcc*`
function in this package calls `SharedBootstrap(t)`, which registers
exactly ONE account per test-binary run (guarded by `sync.Once`) and
hands every test the same endpoint/API key. Only that first call goes
through the real sign-up flow; every subsequent call reuses the result.

Consequence for future test authors: tests all run against the same
account, so **assertions must not depend on account-global state being
empty or exclusive to one test** — e.g. don't assert an exact count from
`aptabase_apps`, only that the specific resources a given test created
are present (and, after teardown, absent), the way the existing tests
already do.

## Stack

- `postgres` (15) — primary database
- `clickhouse` (23.8.4.69) — analytics store
- `mailcatcher` (dockage/mailcatcher 0.8.2) — catches outgoing magic-link
  emails; its REST API (`/messages`, `/messages/:id.html`) is read by
  `bootstrap.go` to fetch the confirmation link
- `aptabase-plus` (`ghcr.io/nmehlei/aptabase-plus:v0.1.534`, public) — the
  app itself, listening on `:8080` internally (`/healthz` for the compose
  healthcheck); runs its own Postgres/ClickHouse migrations on startup

Host ports are remapped off the aptabase-plus defaults (`8080` -> `18080`,
mailcatcher's REST API `1080` -> `11080`) because a separate aptabase-plus
dev sandbox may already be bound to `5432`/`8123`/`1025`/`1080` on this
machine. Postgres and ClickHouse are not exposed to the host at all —
only the app and mailcatcher containers need to be reached from outside
the compose network.

Versions and env var names (`DATABASE_URL`, `CLICKHOUSE_URL`,
`AUTH_SECRET`, `BASE_URL`, `SMTP_HOST`/`SMTP_PORT`/`SMTP_FROM_ADDRESS`)
were verified against the real aptabase-plus source
(`Features/EnvSettings.cs`, `Features/Notification/NotificationExtensions.cs`,
`Program.cs`) rather than assumed.

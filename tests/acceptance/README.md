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
email flow (reading Mailcatcher) and mints an API key — every test in
this package calls it once per test to get an isolated account.

## Stack

- `postgres` (15) — primary database
- `clickhouse` (23.8.4.69) — analytics store
- `mailcatcher` (dockage/mailcatcher 0.8.2) — catches outgoing magic-link
  emails; its REST API (`/messages`, `/messages/:id.html`) is read by
  `bootstrap.go` to fetch the confirmation link
- `aptabase-plus` (`ghcr.io/nmehlei/aptabase-plus:latest`, public) — the
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

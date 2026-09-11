# Terraform Provider for Aptabase

A Terraform provider for managing apps, app shares, and API keys on a
self-hosted **[aptabase-plus](https://github.com/nmehlei/aptabase-plus)**
instance.

> [!IMPORTANT]
> This provider **only works against a self-hosted `aptabase-plus`
> instance**. It does **not** work against Aptabase Cloud
> (app.aptabase.com) or a stock self-hosted
> [aptabase/aptabase](https://github.com/aptabase/aptabase) install —
> neither exposes the API-key-authenticated management API this provider
> depends on. See `aptabase-plus`'s README for why, and
> `docs/upstream-proposal.md` there for the upstream discussion status.

## Usage

```hcl
terraform {
  required_providers {
    aptabase = {
      source  = "nmehlei/aptabase"
      version = "~> 0.1"
    }
  }
}

provider "aptabase" {
  endpoint = "https://aptabase.example.com"
  # token falls back to the APTABASE_TOKEN environment variable
}

resource "aptabase_app" "example" {
  name = "My App"
}
```

See `docs/` for full resource and data source documentation, and
`examples/` for runnable configurations.

## Development

See `AGENTS.md`.

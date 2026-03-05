# supervisible-cli

`supervisible-cli` is the official command-line interface for the Supervisible Public API.

This project is implemented in Go and follows practical CLI patterns inspired by:

- [basecamp-cli](https://github.com/basecamp/basecamp-cli)
- [gogcli](https://github.com/steipete/gogcli)

## Install

### Homebrew (tap)

```bash
brew tap supervisible/tap
brew install supervisible
```

### Build from source

```bash
git clone https://github.com/supervisible/supervisible-cli.git
cd supervisible-cli
make build
./bin/supervisible --help
```

## Authentication

The CLI uses Supervisible API keys (`sv_live_...`) with Bearer auth.

```bash
supervisible auth login --api-key sv_live_xxx
supervisible auth status --verify
supervisible me
```

You can also provide credentials via environment variables:

- `SUPERVISIBLE_API_KEY`
- `SUPERVISIBLE_BASE_URL` (host or full `/api/v1` URL)

## Global flags

- `--json, -j` JSON output
- `--api-key` override API key for current command
- `--base-url` override API base URL
- `--timeout` HTTP timeout (default `30s`)
- `--config` custom config file path
- `--params '<json-object>'` raw query params merged into every request
- `--fields 'id,name'` field mask / local projection
- `--dry-run` validate and print request plan without executing mutating requests

## New schema introspection

- `supervisible schema endpoints --json`
- `supervisible schema describe "GET /projects" --json`
- `supervisible schema describe projects.get --json`

Schema source defaults to embedded OpenAPI and supports overrides:

- `SUPERVISIBLE_SCHEMA_URL`
- `SUPERVISIBLE_SCHEMA_FILE`

## Commands

### Core

- `supervisible me`
- `supervisible auth login|status|logout|token`
- `supervisible config show|set-base-url`
- `supervisible schema endpoints|describe`

### Users

- `supervisible users list [--limit --offset]`
- `supervisible users update <user-id> [flags] [--payload|--file]`

### Clients

- `supervisible clients list`
- `supervisible clients create [flags] [--payload|--file]`
- `supervisible clients update <client-id> [flags] [--payload|--file]`

### Projects

- `supervisible projects list`
- `supervisible projects create [flags] [--payload|--file]`
- `supervisible projects update <project-id> [flags] [--payload|--file]`

### Assignments

- `supervisible assignments list [filters]`
- `supervisible assignments upsert --payload '{"items":[...]}'`
- `supervisible assignments upsert --file payload.json`
- `supervisible assignments upsert --body '{...}'` (deprecated alias)

### Actual Hours

- `supervisible actual-hours list [filters]`
- `supervisible actual-hours upsert --payload '{"items":[...]}'`
- `supervisible actual-hours upsert --file payload.json`
- `supervisible actual-hours upsert --body '{...}'` (deprecated alias)

### Time Off

- `supervisible time-off list [filters]`
- `supervisible time-off create [flags] [--payload|--file]`
- `supervisible time-off update <request-id> [flags] [--payload|--file]`
- `supervisible time-off delete <request-id>`
- `supervisible time-off approve <request-id>`
- `supervisible time-off reject <request-id> --reason "..." [--payload|--file]`

## Precedence rules

### Query precedence

Final query is built as:

1. Command-specific flags (e.g. `--limit`)
2. `--fields` (when schema says operation supports `fields` query)
3. `--params` overrides all overlaps (raw wins)

### Body precedence

For mutating commands:

1. Build typed payload from command flags
2. Merge `--payload` or `--file` on top (raw wins)
3. Send merged payload

## Agent-safe usage examples

```bash
# List users with predictable, compact output
supervisible users list \
  --params '{"limit":10}' \
  --fields 'id,name,email' \
  --json

# Validate a write operation without executing
supervisible projects create \
  --name "Q2 Plan" \
  --client-id 019cb675-c4d6-7e90-8806-25e5145c3a06 \
  --start-date 2026-04-01 \
  --end-date 2026-06-30 \
  --payload '{"status":"planned"}' \
  --dry-run \
  --json
```

## Development

```bash
make tidy
make fmt
make test
make build
```

## Release and Homebrew

This repo includes a GoReleaser config (`.goreleaser.yaml`) with a Homebrew tap publish target.

### Dry-run release

```bash
goreleaser release --snapshot --clean
```

### Publish

Set `HOMEBREW_TAP_GITHUB_TOKEN` with write access to your tap repository and run:

```bash
goreleaser release
```

## Notes

- Response parsing follows Supervisible's `{"data": ...}` envelope and API error envelope.
- For secure token persistence, the CLI tries OS keyring first and falls back to local config storage.
- Input validation rejects common agent failure patterns (query/hash in IDs, control chars, malformed UUID/date).

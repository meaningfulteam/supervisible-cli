# supervisible-cli

[![CI](https://github.com/supervisible/supervisible-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/supervisible/supervisible-cli/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.24-blue)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![Homebrew](https://img.shields.io/badge/homebrew-supervisible%2Ftap-orange)](https://github.com/supervisible/homebrew-tap)

The official Go CLI for the [Supervisible](https://supervisible.com) Public API.

---

## Features

- **`--dry-run` safety** — validate and preview any mutating request before it executes
- **`--json` for agents** — compact, parseable output on every command
- **Schema introspection** — explore available endpoints and field shapes without leaving the terminal
- **OS keyring auth** — API keys stored securely in the system keychain with plain-file fallback
- **Multi-platform** — macOS, Linux, and Windows; ARM and AMD64

---

## Quickstart

```bash
# Install
brew tap supervisible/tap
brew install supervisible

# Authenticate
supervisible auth login --api-key sv_live_xxx

# First command
supervisible me --json
```

---

## Commands

### Core

| Command | Description |
|---------|-------------|
| `supervisible me` | Show the authenticated user |
| `supervisible auth login` | Store an API key |
| `supervisible auth status [--verify]` | Check stored credentials |
| `supervisible auth logout` | Remove stored credentials |
| `supervisible auth token` | Print the raw token |
| `supervisible config show` | Print resolved config |
| `supervisible config set-base-url <url>` | Persist a custom base URL |
| `supervisible schema endpoints` | List all API endpoints |
| `supervisible schema describe <endpoint>` | Show request/response shape |

### Users

| Command | Description |
|---------|-------------|
| `supervisible users list [--limit --offset]` | List users |
| `supervisible users update <user-id> [flags]` | Update a user |

### Clients

| Command | Description |
|---------|-------------|
| `supervisible clients list` | List clients |
| `supervisible clients create [flags]` | Create a client |
| `supervisible clients update <client-id> [flags]` | Update a client |

### Projects

| Command | Description |
|---------|-------------|
| `supervisible projects list` | List projects |
| `supervisible projects create [flags]` | Create a project |
| `supervisible projects update <project-id> [flags]` | Update a project |

### Assignments

| Command | Description |
|---------|-------------|
| `supervisible assignments list [filters]` | List assignments |
| `supervisible assignments upsert --payload '\{"items":[...]\}'` | Bulk upsert assignments |
| `supervisible assignments upsert --file payload.json` | Bulk upsert from file |

### Actual Hours

| Command | Description |
|---------|-------------|
| `supervisible actual-hours list [filters]` | List logged hours |
| `supervisible actual-hours upsert --payload '\{"items":[...]\}'` | Bulk upsert hours |
| `supervisible actual-hours upsert --file payload.json` | Bulk upsert from file |

### Time Off

| Command | Description |
|---------|-------------|
| `supervisible time-off list [filters]` | List time-off requests |
| `supervisible time-off create [flags]` | Create a request |
| `supervisible time-off update <request-id> [flags]` | Update a request |
| `supervisible time-off delete <request-id>` | Delete a request |
| `supervisible time-off approve <request-id>` | Approve a request |
| `supervisible time-off reject <request-id> --reason "..."` | Reject a request |

### Compound Commands

These commands fetch from multiple API endpoints and compute derived insights in a single call — designed for agents and humans who need answers, not raw data.

| Command | Description |
|---------|-------------|
| `supervisible capacity [--week YYYY-Www]` | Team capacity: assigned, available, and free hours per user |
| `supervisible bench [--week YYYY-Www] [--min-hours 8]` | Who has free capacity? Filtered and sorted by availability |
| `supervisible whois <name-or-email>` | Look up a person: projects, assignments, time-off |
| `supervisible context` | Org summary (users, clients, projects) for agent bootstrap |

The `--week` flag accepts ISO week format (`2026-W21`) or a date (`2026-05-18`). Defaults to the current week.

```bash
# Who has room for more work this week?
supervisible bench --json

# What's Juan working on?
supervisible whois juan --json

# Full team capacity for a specific week
supervisible capacity --week 2026-W21

# Agent bootstrap: get full org context
supervisible context --json
```

---

## Global Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--json`, `-j` | false | JSON output |
| `--api-key` | | Override API key for this invocation |
| `--base-url` | | Override API base URL |
| `--timeout` | `30s` | HTTP request timeout |
| `--config` | | Custom config file path |
| `--params '<json>'` | | Raw query params merged into every request |
| `--fields 'id,name'` | | Field mask / local projection |
| `--dry-run` | false | Validate and print request plan; skip execution |

---

## Authentication

API keys (`sv_live_...`) can be set via:

1. `supervisible auth login --api-key sv_live_xxx` — stored in OS keyring
2. `SUPERVISIBLE_API_KEY` environment variable
3. `SUPERVISIBLE_BASE_URL` — override host or full `/api/v1` URL

---

## Schema Introspection

```bash
supervisible schema endpoints --json
supervisible schema describe "GET /projects" --json
supervisible schema describe projects.get --json
```

Schema source defaults to the embedded OpenAPI spec. Override with:

- `SUPERVISIBLE_SCHEMA_URL` — remote URL
- `SUPERVISIBLE_SCHEMA_FILE` — local file path

---

## Precedence Rules

**Query params**: command flags → `--fields` → `--params` (raw wins)

**Request body**: typed flags → `--payload` / `--file` merge on top (raw wins)

---

## Agent-Safe Usage

`--json` + `--dry-run` + `--fields` make the CLI safe and predictable for automation:

```bash
# Compact, filtered output
supervisible users list \
  --params '{"limit":10}' \
  --fields 'id,name,email' \
  --json

# Preview a write without executing
supervisible projects create \
  --name "Q2 Plan" \
  --client-id 019cb675-c4d6-7e90-8806-25e5145c3a06 \
  --start-date 2026-04-01 \
  --end-date 2026-06-30 \
  --payload '{"status":"planned"}' \
  --dry-run \
  --json
```

---

## Install from Source

```bash
git clone https://github.com/supervisible/supervisible-cli.git
cd supervisible-cli
make build
./bin/supervisible --help
```

---

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for setup, testing, and PR guidelines.

---

## License

MIT — see [LICENSE](LICENSE).

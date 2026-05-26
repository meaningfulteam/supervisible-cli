---
name: supervisible
description: "Supervisible agency management — team capacity, project assignments, time-off, clients, and workload tracking. Trigger phrases: `check capacity`, `who's available`, `assign X to project Y`, `what's X working on`, `book time off`, `list projects`, `who's on the bench`, `org overview`, `use supervisible`."
version: 1.0.0
allowed-tools: "Read Bash"
---

# Supervisible CLI

## Prerequisites

Verify the CLI is installed:

```bash
supervisible --version
```

If missing, install:

```bash
brew tap supervisible/tap && brew install supervisible
```

Or from source:

```bash
go install github.com/supervisible/supervisible-cli/cmd/supervisible@latest
```

Authenticate:

```bash
supervisible auth login --api-key sv_live_xxx
supervisible auth status --verify
```

Do not proceed until `auth status --verify` succeeds.

## When to Use This CLI

Use for any Supervisible operation: checking team capacity, managing assignments, tracking time-off, listing projects/clients/users, or bootstrapping agent context.

Do **not** use for operations outside Supervisible's domain (invoicing, contracts, communication).

## Unique Capabilities

These compound commands answer real questions in a single call — no chaining required.

- **`capacity`** — Team capacity for any week: assigned hours, available hours, free hours, project breakdown, and time-off per user.

  _Use when asked "who has room?", "is anyone overloaded?", or before sprint planning._

  ```bash
  supervisible capacity --json
  supervisible capacity --week 2026-W21 --json
  ```

- **`bench`** — Who has free capacity above a threshold, sorted by most available first.

  _Use when asked "who can take on 20 hours?" or "who's on the bench?"_

  ```bash
  supervisible bench --min-hours 16 --json
  ```

- **`whois`** — Resolve a person by name or email. Returns their projects, assignments, time-off, and weekly capacity summary in one call.

  _Use when asked "what's Juan working on?" or before acting on a vague person reference._

  ```bash
  supervisible whois juan --json
  supervisible whois sarah@company.com --json
  ```

- **`context`** — Machine-readable org summary: all users, clients, and projects. Run once at session start to orient yourself.

  _Use at session start or when asked "give me the org overview."_

  ```bash
  supervisible context --json
  ```

## Command Reference

**me** — Show authenticated identity

- `supervisible me` — API key info, org ID, scopes

**users** — Manage team members

- `supervisible users list` — List all users
- `supervisible users update <user-id>` — Update name, availability, manager

**clients** — Manage clients

- `supervisible clients list` — List all clients
- `supervisible clients create` — Create a client
- `supervisible clients update <client-id>` — Update a client

**projects** — Manage projects

- `supervisible projects list` — List all projects
- `supervisible projects create` — Create a project (requires `--name`, `--client-id`, `--start-date`, `--end-date`)
- `supervisible projects update <project-id>` — Update a project

**assignments** — Manage project assignments

- `supervisible assignments list` — List assignments (filter: `--user-id`, `--project-id`, `--start-date`, `--end-date`)
- `supervisible assignments upsert` — Create or update assignments (single via flags, bulk via `--payload '{"items":[...]}'`)
- `supervisible assignments add` — Read-modify-write one row by `(user, project, capability, date)`. Auto-deletes the row when result is 0.
- `supervisible assignments move <id> --to-user <id>` — Atomic server-side move to another user (same project, same date). Use `--capability-id` to set the target capability; `--hours` to move a subset.
- `supervisible assignments remove-from-project --user-id <id> --project-id <id>` — Remove a user's future assignments from a project. Defaults to "next week onward" so the current week stays. Override with `--since YYYY-MM-DD` to wipe from a backdated cutoff.
- `supervisible assignments delete <id> [<id>...]` — Delete one or more rows in a single CLI call (one classifier check, N HTTP DELETEs).

**capabilities** — Inspect the org's capability catalog

- `supervisible capabilities list` — All capabilities for the caller's org
- `supervisible capabilities list --for-project <id>` — Only those attached to a specific project. Recovery path when `--auto-capability` fails on a user with no prior history.

**actual-hours** — Track logged hours

- `supervisible actual-hours list` — List logged hours (same filters as assignments)
- `supervisible actual-hours upsert` — Log hours (single or bulk)
- `supervisible actual-hours delete <id> [<id>...]` — Delete one or more rows in a single call

**time-off** — Manage time-off requests

- `supervisible time-off list` — List requests (filter: `--user-id`, `--status`)
- `supervisible time-off create --type-name "Vacation"` — Resolve type name → ID via API and create the request
- `supervisible time-off create --time-off-type-id <uuid>` — Create with explicit type ID
- `supervisible time-off update <id>` — Update a request
- `supervisible time-off delete <id>` — Delete a request
- `supervisible time-off approve <id>` — Approve a request
- `supervisible time-off reject <id> --reason "..."` — Reject a request

**schema** — API introspection

- `supervisible schema endpoints` — List all API endpoints
- `supervisible schema describe "GET /assignments"` — Show endpoint details

**config** — Configuration

- `supervisible config show` — Print resolved config
- `supervisible config set-base-url <url>` — Set custom API URL

## Recipes

### Before assigning someone — always check capacity first

```bash
# 1. Can Sarah take 10 more hours?
supervisible whois sarah --json
# → check weekSummary.freeHours

# 2. She's full — who else has room?
supervisible bench --min-hours 10 --json

# 3. Assign the available person
supervisible assignments upsert \
  --user-id <ID> --project-id <ID> \
  --date 2026-05-19 --hours 8 \
  --dry-run --json
# Review, then remove --dry-run to execute
```

### Weekly capacity review

```bash
supervisible capacity --week 2026-W21 --json
supervisible capacity --week 2026-W22 --json
```

### Session bootstrap

```bash
# Orient: all users, clients, projects
supervisible context --json
# Then answer any follow-up with the right IDs
```

### Resolve names to IDs

```bash
# Person → ID
supervisible whois juan --json

# Project by name
supervisible projects list --json --fields 'id,name,status'

# Client by name
supervisible clients list --json --fields 'id,companyName'
```

### Bulk assignment (full week)

```bash
supervisible assignments upsert --dry-run --json --payload '{
  "items": [
    {"userId": "<ID>", "projectId": "<ID>", "date": "2026-05-19", "hours": 8},
    {"userId": "<ID>", "projectId": "<ID>", "date": "2026-05-20", "hours": 8},
    {"userId": "<ID>", "projectId": "<ID>", "date": "2026-05-21", "hours": 8},
    {"userId": "<ID>", "projectId": "<ID>", "date": "2026-05-22", "hours": 8},
    {"userId": "<ID>", "projectId": "<ID>", "date": "2026-05-23", "hours": 8}
  ]
}'
```

### Remove someone from a project — "X is no longer on Y"

```bash
# 1. Find Mariana's user ID and EdVisorly's project ID
supervisible whois Mariana --json | jq '.user.id, .assignments[].client.name + " / " + .assignments[].project'

# 2. Preview the cleanup. Default cutoff = next-week Sunday; current week stays.
supervisible assignments remove-from-project \
  --user-id <user-id> --project-id <project-id> \
  --dry-run --json

# 3. Execute when the plan looks right
supervisible assignments remove-from-project \
  --user-id <user-id> --project-id <project-id>

# Backdated: "they actually stopped 2 months ago"
supervisible assignments remove-from-project \
  --user-id <user-id> --project-id <project-id> \
  --since 2026-04-01
```

### Move hours from one teammate to another (same project, same date)

```bash
# 1. Find the source assignment row
supervisible whois Juan --json | jq '.assignments[] | select(.project == "Web Redesign")'

# 2. Atomic move — server-side, no TOCTOU
supervisible assignments move <source-row-id> \
  --to-user <target-user-id> \
  --capability-id <target-cap> \
  --hours 4    # optional; defaults to all source hours

# Use `capabilities list --for-project <project-id>` if you don't know
# the target's capability ID on that project yet.
```

### Recovery: --auto-capability failed

When `assignments add --auto-capability` fails for a user with no prior history on a project, the canonical recovery path is one CLI call:

```bash
supervisible capabilities list --for-project <project-id> --json
# Pick an ID from the result, then:
supervisible assignments add \
  --user-id <id> --project-id <id> \
  --capability-id <picked-id> \
  --date 2026-05-31 --hours 10
```

### Create a time-off request without looking up the type ID

```bash
# --type-name resolves to ID via the API (one extra GET, then POST)
supervisible time-off create \
  --user-id <user-id> --type-name Vacation \
  --start-date 2026-07-15 --end-date 2026-07-19 \
  --reason "Family trip"
```

## Auth Setup

Store your API key:

```bash
supervisible auth login --api-key sv_live_xxx
```

Or set `SUPERVISIBLE_API_KEY` as an environment variable. Verify with:

```bash
supervisible auth status --verify
```

## Agent Mode

Use `--json` on every command for structured output. Combine with:

- **`--fields`** — Project to specific fields: `--fields 'id,name,email'`
- **`--expand`** — Include related objects: `--expand user,project`
- **`--dry-run`** — Preview mutations without executing
- **`--params`** — Raw query params: `--params '{"limit":100}'`

All output goes to stdout (JSON), errors to stderr.

### Safety rule

**Always `--dry-run --json` before any mutation.** Review the plan, then re-run without `--dry-run`.

### Key details

- All IDs are UUIDs
- All dates are `YYYY-MM-DD`
- Assignments are per-day (one user + project + date = one entry)
- Upsert semantics: same user + project + date updates hours (no duplicate)
- Default pagination limit is 50; use `--limit 200` for compound commands
- `--week` flag on capacity/bench accepts ISO week (`2026-W21`) or date (`2026-05-18`); defaults to current week
- `whois` matches by case-insensitive substring on name, or exact email if input contains `@`

---
name: supervisible-write
description: "Create, update, and delete Supervisible resources — users, clients, projects, assignments, actual hours, and time-off. Use when asked to create, modify, update, delete, or change any resource. Always dry-run first. Do not use for read-only operations (use supervisible-read) or assignment workflows (use supervisible-assignments which includes capacity checks)."
version: 2.0.0
---

Use this skill for mutating Supervisible operations — creating, updating, and deleting resources.

## Safety: dry-run first

Always preview mutations before executing:

```bash
supervisible <resource> <action> [args] --dry-run --json
```

Review the request plan (method, endpoint, body), then remove `--dry-run` to execute. This is especially important for bulk operations where a wrong UUID could affect the wrong person.

## Mutations by resource

### Assignments

**Create or update (upsert):**
```bash
supervisible assignments upsert \
  --user-id <UUID> --project-id <UUID> \
  --date 2026-04-21 --hours 8 \
  --dry-run --json
```

Bulk upsert via payload:
```bash
supervisible assignments upsert --dry-run --json --payload '{
  "items": [
    {"userId": "<UUID>", "projectId": "<UUID>", "date": "2026-04-21", "hours": 8}
  ]
}'
```

**Delete:**
```bash
supervisible assignments delete <UUID> --dry-run --json
```

### Actual hours

Same pattern as assignments:
```bash
supervisible actual-hours upsert \
  --user-id <UUID> --project-id <UUID> \
  --date 2026-04-21 --hours 6 \
  --dry-run --json
```

### Users
```bash
supervisible users update <UUID> --name "New Name" --dry-run --json
```

### Clients
```bash
supervisible clients create --company-name "Acme Corp" --dry-run --json
supervisible clients update <UUID> --company-name "New Name" --dry-run --json
```

### Projects
```bash
supervisible projects create --name "Project X" --client-id <UUID> \
  --start-date 2026-05-01 --end-date 2026-08-31 --dry-run --json
supervisible projects update <UUID> --name "New Name" --dry-run --json
```

### Time off
```bash
supervisible time-off create --user-id <UUID> --time-off-type-id <UUID> \
  --start-date 2026-05-01 --end-date 2026-05-05 \
  --availability 0 --reason "Vacation" --dry-run --json
```

## Payload and file input

For complex mutations, use `--payload` (inline JSON) or `--file` (path to JSON file):

```bash
supervisible assignments upsert --payload '{"items": [...]}' --dry-run --json
supervisible assignments upsert --file payload.json --dry-run --json
```

These are mutually exclusive — pick one.

## Before any mutation

- Verify UUIDs exist by listing the resource first (see `supervisible-read` skill)
- Use `YYYY-MM-DD` format for all dates
- Confirm date ranges are within resource bounds (e.g., project start/end dates)
- Confirm UUID/date formats before execution

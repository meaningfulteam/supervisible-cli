---
name: supervisible-assignments
description: "Add, remove, or change project assignments in Supervisible. Use when asked to 'assign X to project Y', 'add me to project X', 'remove Sarah from Nike', 'change my hours on project Z', 'I'm missing project Y', or 'add [client name] to my workload'. Always check capacity before assigning. Do not use for reading assignments only — use supervisible-read for that."
version: 1.0.0
---

Use this skill when asked to add, remove, or change someone's project assignments in Supervisible.

## Before you start

**Always check capacity first.** Before assigning someone, verify they have room:

```bash
supervisible whois <name> --json
# Check weekSummary.freeHours — if 0 or negative, they're full
```

If the person is overallocated, use `supervisible bench` to suggest alternatives:

```bash
supervisible bench --min-hours <needed-hours> --json
```

Workload requests are often vague — "add me to DD Marketplace" doesn't say when or how many hours. Before making any changes, make sure you know:

1. **Who** — which user? (may be "me", a first name, or an email)
2. **Which project** — by project name, client name, or both
3. **Dates** — what date range? (this week, next month, the project's full duration?)
4. **Hours per day** — how many hours per day? (4h half-day and 8h full-day are common)

If any of these are missing from the request, ask. Don't guess dates or hours — a wrong assumption means rework.

## Step 1: Resolve the user

```bash
supervisible users list --json --fields 'id,name,email'
```

Match the requested name against results. Names in requests are often informal — "Nicole" for "Nicole Ruiz", or just an email handle. If multiple users match, ask which one.

## Step 2: Resolve the project

```bash
supervisible projects list --json --fields 'id,name,status,startDate,endDate' --expand client
```

People refer to projects by:
- **Project name**: "DD Marketplace", "Nike Rebrand"
- **Client name**: "Colektia", "Acme" — meaning the project(s) for that client
- **Shortened names**: "DD" for "DD Marketplace"

When the request uses a client name, find the client first, then filter projects by their `clientId`:

```bash
supervisible clients list --json --fields 'id,companyName'
```

If a client has multiple active projects, ask which one — or confirm they mean all of them.

## Step 3: Check existing assignments

Before creating anything, see what already exists:

```bash
supervisible assignments list --json \
  --user-id <USER_ID> \
  --project-id <PROJECT_ID> \
  --expand user,project
```

This tells you whether the user is already assigned, for which dates, and how many hours. Avoid creating duplicates — upsert updates existing entries for the same user + project + date combination.

## Step 4: Create assignments

Each assignment is one entry: (user, project, date, hours). To assign someone for a full week, create 5 entries (Monday through Friday).

**Single day:**

```bash
supervisible assignments upsert \
  --user-id <USER_ID> \
  --project-id <PROJECT_ID> \
  --date 2026-04-21 \
  --hours 8 \
  --dry-run --json
```

**Multiple days (bulk):**

```bash
supervisible assignments upsert --dry-run --json --payload '{
  "items": [
    {"userId": "<ID>", "projectId": "<ID>", "date": "2026-04-21", "hours": 8},
    {"userId": "<ID>", "projectId": "<ID>", "date": "2026-04-22", "hours": 8},
    {"userId": "<ID>", "projectId": "<ID>", "date": "2026-04-23", "hours": 8},
    {"userId": "<ID>", "projectId": "<ID>", "date": "2026-04-24", "hours": 8},
    {"userId": "<ID>", "projectId": "<ID>", "date": "2026-04-25", "hours": 8}
  ]
}'
```

Always run with `--dry-run --json` first. Review the plan, confirm it looks right, then remove `--dry-run` to execute.

## Step 5: Remove assignments

To remove someone from a project, list their assignments to get the entry IDs:

```bash
supervisible assignments list --json \
  --user-id <USER_ID> \
  --project-id <PROJECT_ID>
```

Then delete each one:

```bash
supervisible assignments delete <ASSIGNMENT_ID> --dry-run --json
```

Review the dry-run, then remove `--dry-run` to execute.

## Key details

- **Upsert semantics**: same user + project + date updates hours instead of creating a duplicate.
- **Capability ID**: optional. Include `--capability-id <UUID>` if the request specifies a role or capability for the assignment.
- **Date format**: always `YYYY-MM-DD`.
- **Weekdays only**: assignments are typically Monday through Friday.
- **Stay within project bounds**: check the project's `startDate` and `endDate` — don't create assignments outside that range.
- **Expand for readability**: use `--expand user,project` on list commands to see names instead of raw UUIDs.

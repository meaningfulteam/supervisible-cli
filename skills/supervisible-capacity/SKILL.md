---
name: supervisible-capacity
description: "Check team capacity and availability for any week — who's free, who's overloaded, who's on bench. Use when asked about availability, workload, capacity, utilization, 'who has room', 'is anyone free', 'can X take more work', 'who's on the bench', or before assigning work to someone. Do not use when asked to create or modify assignments — use supervisible-assignments for that."
version: 1.0.0
---

Use this skill to answer capacity and availability questions. It replaces 3+ API calls with a single compound command.

## When to use

Activate this skill when the request involves:
- Checking if someone has room for more work
- Finding who's available this week (or any week)
- Reviewing team utilization before sprint planning
- Deciding who to assign to a new project
- Answering "who's on the bench?"

**Always check capacity before creating assignments.** If someone asks "assign Sarah to Nike for 10h", check capacity first — if Sarah is at 40/40h, suggest an alternative instead of blindly assigning.

## Commands

### `capacity` — Full team view

Shows assigned hours, available hours, and free capacity for every active team member in a given week.

```bash
supervisible capacity --json
supervisible capacity --week 2026-W21 --json
supervisible capacity --week 2026-05-18 --json
```

The `--week` flag accepts ISO week format (`2026-W21`) or a date (`2026-05-18`). Omit it to get the current week.

Output per user includes:
- `assignedHours` — total hours assigned across all projects
- `availableHours` — default availability minus approved time-off
- `freeHours` — available minus assigned (negative = overallocated)
- `projects` — list of projects with hours per project
- `timeOff` — approved time-off entries overlapping the week

### `bench` — Who has free capacity?

Same data as `capacity`, but filtered to users with free hours above a threshold and sorted by most available first.

```bash
supervisible bench --json
supervisible bench --week 2026-W21 --min-hours 16 --json
```

Default `--min-hours` is 8. Set to 0 to see everyone with any free time.

_Use `bench` when the question is "who can take on X hours of work?" Use `capacity` when you need the full picture._

## Recipes

### Before assigning work — check capacity first

```bash
# 1. Check if Sarah has room
supervisible capacity --json | jq '.users[] | select(.name | test("sarah"; "i"))'

# 2. If she's full, find someone with room
supervisible bench --min-hours 10 --json

# 3. Then assign using the supervisible-assignments skill
```

### Weekly capacity review

```bash
# This week
supervisible capacity --json

# Next week
supervisible capacity --week 2026-W22 --json

# Compare: who's free this week but booked next week?
```

### Find someone for a specific project load

```bash
# Need someone with 20+ free hours
supervisible bench --min-hours 20 --json
```

## Interpreting results

- `freeHours > 0` — has room for more work
- `freeHours == 0` — fully allocated (not necessarily overworked)
- `freeHours < 0` — **overallocated** — flag this to the user
- `timeOffHours > 0` — has approved time-off reducing availability
- `availableHours < defaultAvailability` — time-off is reducing their week

When suggesting assignments, prefer users with the most `freeHours` who aren't already on too many projects. A user with 20h free across 1 project has more focus time than one with 20h free across 4 projects.

## Output handling

Always use `--json` for structured output. Use `--fields` for projection:

```bash
supervisible capacity --json --fields 'users.name,users.freeHours,users.projects'
```

Without `--json`, output is a human-readable table — useful for showing the user directly but not for further processing.

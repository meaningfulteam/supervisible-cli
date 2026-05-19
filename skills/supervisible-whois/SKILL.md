---
name: supervisible-whois
description: "Look up a team member by name or email — shows their projects, assignments, time-off, and weekly capacity in one call. Use when asked 'what is X working on', 'tell me about X', 'who is X', 'what projects is X on', 'is X available', or when you need to resolve a vague person reference before taking action. Do not use for bulk team queries — use supervisible-capacity for team-wide views."
version: 1.0.0
---

Use this skill to resolve a person and get their full context in a single call. Replaces the pattern of listing users, then assignments, then time-off separately.

## When to use

Activate when:
- Someone asks about a specific person ("what's Juan working on?")
- You need to understand someone's current workload before acting
- Resolving a name or email to see their projects and availability
- Checking if someone has upcoming time-off

Do **not** use for team-wide questions like "who's available?" — use `supervisible-capacity` for that.

## Command

```bash
supervisible whois <name-or-email> --json
```

### Matching rules

- If the input contains `@`, it matches by **exact email** (case-insensitive)
- Otherwise, it matches by **substring on name** (case-insensitive)
- Zero matches → error with "no user found"
- Multiple matches → error listing all matches — be more specific

```bash
# By name fragment
supervisible whois juan --json
supervisible whois "ana martinez" --json

# By email
supervisible whois sarah@supervisible.com --json
```

### Output

```json
{
  "user": { "id": "...", "name": "...", "email": "..." },
  "assignments": [
    { "project": "Aplazo", "date": "2026-05-19", "hours": 8 }
  ],
  "timeOff": [
    { "type": "Vacation", "startDate": "2026-06-01", "endDate": "2026-06-05", "status": "approved" }
  ],
  "weekSummary": {
    "assignedHours": 32,
    "availableHours": 40,
    "freeHours": 8
  }
}
```

- `assignments` — this week's assignments with project names
- `timeOff` — upcoming time-off from today onwards (all statuses)
- `weekSummary` — capacity snapshot for the current week

## Recipes

### Resolve a person before assigning work

```bash
# 1. Look up the person
supervisible whois sarah --json

# 2. Check their freeHours in weekSummary
# 3. If they have room, proceed with supervisible-assignments
# 4. If not, check supervisible bench for alternatives
```

### Answer "what's X working on?"

```bash
supervisible whois juan --json
# Response includes assignments (projects + hours) and timeOff
# Format: "Juan is working on Aplazo (20h) and Zetta (12h) this week, 8h free"
```

### Check before booking time-off

```bash
# See what they're assigned to this week
supervisible whois emily --json
# If heavily assigned, warn that time-off may require reassignment
```

## Handling ambiguity

When a user says "check on Sarah" and there are multiple Sarahs:

1. `whois` will return an error listing all matches
2. Ask the user to clarify: "Did you mean Sarah Chen (sarah@supervisible.com) or Sarah Park (spark@supervisible.com)?"
3. Re-run with the email for an exact match

When the person is referenced by role ("the PM on Nike"), you'll need to resolve the project first:

```bash
supervisible projects list --json --fields 'id,name,projectManagerId' --expand projectManager
```

Then use the PM's name or email with `whois`.

---
name: supervisible-context
description: "Bootstrap agent understanding of a Supervisible organization — users, clients, and projects in one call. Use at the start of a session, when asked 'what org is this', 'list all users/clients/projects', 'give me the org overview', or when you need to orient yourself before taking any action. Do not use for capacity or workload questions — use supervisible-capacity for that."
version: 1.0.0
---

Use this skill to orient yourself in a Supervisible organization. One call gives you every user, client, and project — the foundation for all other skills.

## When to use

Activate when:
- Starting a new session and needing to understand the org
- Asked for an overview of the organization
- You need to map names to IDs across multiple resource types
- Building a mental model before complex multi-step operations

Run this **once** at session start, then use the results as a lookup table. Don't call it repeatedly — the data doesn't change within a session.

## Command

```bash
supervisible context --json
```

No flags needed. Always outputs JSON (table mode just shows summary counts).

### Output

```json
{
  "organization": "org-uuid",
  "users": [
    { "id": "...", "name": "Alberto", "email": "alberto@supervisible.com", "isActive": true }
  ],
  "clients": [
    { "id": "...", "companyName": "Acme Corp", "isActive": true }
  ],
  "projects": [
    { "id": "...", "name": "Website Redesign", "status": "active", "clientId": "..." }
  ]
}
```

Fields are intentionally slim — IDs, names, status. For detailed info on a specific resource, use `supervisible-read` or `supervisible-whois`.

## Recipes

### Session bootstrap

```bash
# 1. Orient: who and what exists
supervisible context --json

# 2. Now you can answer questions like:
#    - "assign Juan to the Acme project" → you know Juan's ID and Acme's project ID
#    - "who's active?" → filter users by isActive
#    - "what projects does client X have?" → filter projects by clientId
```

### Cross-reference with capacity

```bash
# 1. Get org context
supervisible context --json

# 2. Check this week's capacity
supervisible capacity --json

# Now you can map project IDs from context to assignment data in capacity
```

## When NOT to use

- For workload/availability → use `supervisible-capacity`
- For a specific person's details → use `supervisible-whois`
- For creating/updating resources → use `supervisible-write`
- If you already have the IDs you need → skip context, go direct

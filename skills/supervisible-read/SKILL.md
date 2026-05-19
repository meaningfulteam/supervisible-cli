---
name: supervisible-read
description: "List, search, and inspect Supervisible resources — users, clients, projects, assignments, actual hours, and time-off. Use when asked to look up data, list resources, search for something, resolve a name to an ID, or inspect API endpoints. Do not use for capacity questions (use supervisible-capacity), person lookups (use supervisible-whois), or mutations (use supervisible-write)."
version: 2.0.0
---

Use this skill for non-mutating Supervisible reads — listing, searching, and inspecting resources.

## Core pattern

Always pass `--json` for machine-readable output. Use `--fields` to request only what you need, and `--expand` to include related objects inline.

```bash
supervisible <resource> list --json --fields '<field1,field2>' --expand '<relation>'
```

## Resources

### Users
```bash
supervisible users list --json --fields 'id,name,email,userType,isActive'
```
Useful for resolving a person's name or email to their UUID.

### Clients
```bash
supervisible clients list --json --fields 'id,companyName,isActive'
```
Use `--expand accountManager` to see who manages each client.

### Projects
```bash
supervisible projects list --json --fields 'id,name,status,startDate,endDate,clientId' --expand client,projectManager
```
Projects are often referred to by shortened names or client names in conversation — match flexibly.

### Assignments
```bash
supervisible assignments list --json --expand user,project
```
Filter with `--user-id`, `--project-id`, `--start-date`, `--end-date`.

### Actual hours
```bash
supervisible actual-hours list --json --expand user,project
```
Same filters as assignments: `--user-id`, `--project-id`, `--start-date`, `--end-date`.

### Time off
```bash
supervisible time-off list --json --expand user,timeOffType
```
Filter with `--user-id`, `--status` (pending, approved, rejected).

## Resolving names to IDs

Most commands require UUIDs. To find a UUID from a human name:

1. List the resource with `--json --fields 'id,name'` (or `companyName` for clients)
2. Match the name from the request — allow partial and informal matches
3. If multiple results match, ask the user to clarify

## Pagination

Use `--limit` and `--offset` for large result sets:

```bash
supervisible projects list --json --limit 100 --offset 0
```

Default limit is 50.

## Inspecting the API

Use `schema describe` to learn what fields and query params an endpoint supports:

```bash
supervisible schema describe "GET /assignments" --json
```

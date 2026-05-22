---
type: plan
status: active
tags: [cli, agent, ergonomics, lookups, capabilities, assignments, move, polish]
created: 2026-05-21
follows: .thoughts/plans/2026-05-19-feat-cli-agent-experience.md
related_findings: .thoughts/findings/2026-05-19-cli-real-world-testing.md
upstream_api_plans:
  - ../../../supervisible/.thoughts/plans/2026-05-19-feat-public-api-agent-writes.md  # Unit 4 obsoleted by API capabilities endpoint
  - ../../../supervisible/.thoughts/plans/2026-05-21-feat-public-api-agent-reads.md   # Units 1, 2, 5 obsoleted by nested expand / name filter / move endpoint
---

# feat(cli): Phase 3 — Agent ergonomics (lookups, capability discovery, move compound)

## Overview

Phase 3 closes the remaining friction surfaced by real-world agent dogfooding after Phases 1 (foundation) and 2 (agent-safe writes) shipped. Six units in two phases: **Phase A** makes name-based and entity-based lookups one-shot for agents (client linkage in `whois`, `--name` filters with did-you-mean), and **Phase B** unblocks the `--auto-capability` recovery path with a derived `capabilities list` plus a safe `assignments move` compound.

Each unit is independently shippable and follows the conventions established by Phases 1+2 (Aux/Data/Table primitives, `App.Execute` for one-call commands, `argsWithUsage`, kebab-case files, `Example:` blocks). No new server endpoints required; Unit 4 explicitly documents its derived-view nature because the canonical `GET /capabilities` doesn't exist yet.

## Current State Analysis

### What Phases 1+2 left in place

- `WhoisAssignment` carries `id`, `projectId`, `capabilityId`, `project` (name), `date`, `hours`. No client name or capability name. (`internal/cmd/whois.go:16-23`)
- `projects list`, `clients list`, `users list` are pure passthroughs to `GET /projects|/clients|/users` with `limit`, `offset`. No name filtering anywhere. (`internal/cmd/projects.go:31-85`, `internal/cmd/clients.go:30-83`, `internal/cmd/users.go:28-82`)
- `whois <name-or-email>` does its own client-side substring match on `users.name`. (`internal/cmd/whois.go:150-172`)
- `--auto-capability` resolves via assignment history; on failure it tells callers to "pass --capability-id explicitly" with no path to discover one. (`internal/cmd/capability_resolver.go:25-67`)
- `assignments add` does single-row read-modify-write with time-off pre-flight. There is no equivalent "move from user A to user B" compound. (`internal/cmd/assignments.go:255-380`)
- `me` non-JSON output prints `Identity: map[keyId:... organizationId:...]` — Go's default `%v` for a map. Functional but ugly. (`internal/cmd/me.go:30-33`)

### Schema constraints (researched 2026-05-21)

- **No `/capabilities` endpoint.** The public API exposes capabilities only as expansion targets (`expand=capability` on `/assignments`) and as foreign keys on assignment rows. There is no way to list "what capabilities exist on project X."
- **`/projects`, `/clients`, `/users` accept only `limit`, `offset`, `expand`.** No `name=...` or `q=...` filter. Any name-based lookup must happen client-side after a full fetch.
- **`/projects?expand=client` works** — returns `client: {id, companyName}` on each `Project` row. The `Project` Go type already has `Client *ExpandedClient`. This is the cheapest path to client linkage in `whois`.
- **`/assignments` does NOT support nested expand** like `project.client`. Adding the client to whois output requires a separate fetch from `/projects?expand=client` and an in-memory join.

### Key discoveries from dogfooding (`.thoughts/findings/2026-05-19-cli-real-world-testing.md` + the Mariana Slack request)

- Agents reliably translate **client names** (Avask, EdVisorly) from human messages, not project names. Without client linkage in `whois`, every realistic admin task does 2 extra GETs to build a client→project map.
- When a user is new to a project (Mariana → Marketplace, Herbert → Odyssey), `--auto-capability` correctly fails loud, but the agent has no recovery path except scavenging from someone else's assignments.
- "Move hours from A to B on the same project" came up naturally in two of three dogfood sessions. Today it's `delete` + `add` and requires the agent to know the target's capability.
- `projects list` with no match (Mariana's "Iniciativa F30") returns `[]` with no signal — the user gets nothing back from the CLI.

## Requirements Trace

- **R1.** `whois` output MUST include the client's `id` and `name` on every assignment so agents can map "Avask" → projectId in one call.
- **R2.** `users list`, `projects list`, and `clients list` MUST accept a `--name <substring>` flag that case-insensitively filters on the human-readable name (or `companyName` for clients).
- **R3.** When `--name` returns zero matches, the command MUST emit a `Did you mean:` hint to stderr listing up to 5 close substring/edit-distance candidates, mirroring `schema describe`.
- **R4.** A new `capabilities list --for-project <id>` MUST list every distinct capability ID + name (when expandable) currently in use on the project, with usage counts. Output MUST clearly mark the result as derived from assignment history, not a canonical capability catalog.
- **R5.** A new `assignments move <assignment-id> --to-user <user-id>` MUST move hours from one user to another on the same project: read source row, upsert target row (with `--auto-capability` semantics), delete source. Source MUST remain untouched if the target upsert fails.
- **R6.** `me` non-JSON output MUST render the identity as readable `key: value` lines, not Go's `map[...]` syntax.

## Desired End State

After Phase 3:

```bash
# Mariana's flow becomes one call instead of three:
supervisible whois "Miquelajauregui" --weeks 12 --json | \
  jq '.assignments | group_by(.client.name) | map({client: .[0].client.name, total: ([.[].hours] | add)})'
# → [{"client": "Avask", "total": 1}, {"client": "EdVisorly", "total": 60}]

# Finding "Iniciativa F30" gives an actionable response:
supervisible projects list --name "F30" --json
# stderr: warning: no projects match "F30". Did you mean: Iniciativa Q4, F25 Migration, Iniciativa F40?

# Capability recovery when --auto-capability fails:
supervisible capabilities list --for-project 019c885e-... --json
# → [{"capabilityId": "0195f31b-...", "name": "Project Management", "usageCount": 12, "source": "derived-from-assignments"}]
# (stderr: warning: capability list is derived from assignment history, not the canonical catalog)

# One-shot move:
supervisible assignments move 019d27a4-... --to-user 01973141-... --auto-capability --dry-run
# stderr:
#   capability resolved for 01973141-.../<projectId>: cap-xyz
#   move: 10h from Mariana to Herbert on Web Redesign 2026-05-31
# stdout: dry-run plan showing both upsert and delete
```

`whois`'s `weekSummary` continues to scope to the current week (Phase 2 regression fix from 2026-05-20). The `me` command prints clean key-value lines for humans; agents still get JSON.

## What We're NOT Doing

- **No new server endpoints.** `GET /capabilities`, `GET /projects?name=...`, and `expand=project.client` on `/assignments` would each be the canonical fix for one of these items. Phase 3 routes around them client-side and documents the trade-off. Server-side asks are tracked separately in the main Supervisible repo.
- **No paginated full-fetch for `--name`.** Phase 3's CLI-side filter assumes the first page (default `--limit 50`, max 200) covers the whole list for typical agencies. Big customers hitting pagination get a soft note in stderr explaining the truncation; we don't auto-paginate.
- **No transactional `assignments move`.** Add-then-delete with loud error on partial failure. Phase 2's `assignments add` already accepts the same TOCTOU posture; documenting the trade-off is enough until a server-side `PATCH /assignments` with diff semantics lands.
- **No restructuring of the `whois` flow.** Still does its own user match; we add a second client-list fetch alongside, not in front of, the existing user fetch.
- **No `capabilities` write path.** Read-only listing only. Creating/editing capabilities stays out of the public API.
- **No retroactive backfill of `WhoisAssignment` for compound commands** (`bench`, `context`, `capacity`). They use their own report types. Unit 1 changes `WhoisAssignment` only; other compounds get the same treatment if/when their consumers need it.

## Stakeholder Impact

- **Agents (Claude, Slack bot, OpenClaw).** Biggest win. The "client in `whois`" change alone closes the most common dogfooding friction. `--name` makes name-based reasoning a one-shot CLI call. `capabilities list` provides the recovery path the agent was missing when `--auto-capability` fails. `assignments move` collapses a 2-step write into one safe primitive.
- **Human developers.** `--name` works the same way they already grep through `jq` output, just faster. `me` becomes readable. No breaking changes to existing JSON shapes (Unit 1 adds fields; doesn't rename or remove).
- **Scripts/CI.** No JSON shape regressions. `WhoisAssignment` gains an optional `client` object; consumers using `.[0].project` are unchanged.
- **Supervisible server team.** Phase 3 documents three concrete server-side asks (capabilities endpoint, name filters, nested expand on `/assignments`) with real CLI use cases attached. Each unblocks a cleaner Phase 3 implementation than the workarounds we're shipping.
- **No API server changes required to ship Phase 3.**

## Key Technical Decisions

- **Client linkage via a separate `/projects?expand=client` fetch + in-memory join.** The cheapest path that doesn't require server changes. One extra GET per `whois` call (bounded; agencies typically have <500 projects). Built as a `projectClientResolver` helper that mirrors `capabilityResolver`'s shape (per-invocation cache, fail-soft on empty).
- **`--name` filter is client-side, case-insensitive substring match.** Pre-flight to substring-or-edit-distance for "did you mean" hints. Same algorithm as `suggestOperations` in `internal/cmd/schema.go` from Phase 2 Unit 9.
- **`capabilities list` is derived, not canonical.** Output documents this. A `source: "derived-from-assignments"` field on every JSON row; a `warning: capability list is derived from assignment history` line on stderr. When `GET /capabilities` lands server-side, switch this command to use it and drop the warning.
- **`assignments move` does add-then-delete, not delete-then-add.** Add-first means source is intact if target fails (the worst case is no-op). Delete-first would mean source is gone if target fails (the worst case is data loss). Documented in `Long:`.
- **`assignments move` accepts `--keep-source` and `--keep-target-on-failure` flags** — but only `--keep-target-on-failure` ships in this plan as the documented default. `--keep-source` is a future option if anyone asks; the natural call is "move", not "copy".
- **`me` formatter is a typed render, not a generic map walker.** Inspect known fields (organizationId, keyId, scopes) and print them in a stable order. Unknown fields land in a trailing `Other: ...` line so the command degrades gracefully when the server adds new identity fields.

## Implementation Approach

Six units across two phases. Phase A ships as one PR series; Phase B ships after Phase A merges. Each unit is independent within its phase except where called out.

```
PHASE A — Lookup ergonomics
  Unit 1 (client linkage in whois)          ─► no dependencies
  Unit 2 (--name filter on list commands)   ─► no dependencies
  Unit 3 (did-you-mean on empty --name)     ─► depends on Unit 2

PHASE B — Capability discovery + write ergonomics
  Unit 4 (capabilities list --for-project)  ─► no dependencies
  Unit 5 (assignments move compound)        ─► depends on Unit 4 (recovery path)
  Unit 6 (me non-JSON polish)               ─► no dependencies
```

Phase A sequencing: Units 1–2 ship first; together they close the friction from the Mariana flow. Unit 3 layers a small UX improvement on top of Unit 2.

Phase B sequencing: Unit 4 is the building block; Unit 5 is the headline compound; Unit 6 is one-line polish that fits anywhere.

### Alternatives Considered

1. **Block Phase 3 on the server-side `GET /capabilities` endpoint.**
   - Pros: cleaner long-term API; no "derived from history" caveat needed; canonical source of truth.
   - Cons: ships nothing until server work lands; agents continue scavenging capabilities from assignment history with no CLI affordance.
   - Why not chosen: server endpoint is open-ended scope (capability CRUD, permissions, project association rules). Phase 3 should not block on it.

2. **Skip name filters and add a generic `--filter '<jq-expr>'` flag.**
   - Pros: maximum flexibility; one flag covers every filter need.
   - Cons: failure modes (bad jq) become CLI failures; the agent already has `| jq`; doesn't address the discovery problem (zero-match → no signal).
   - Why not chosen: solves the wrong problem. The agent needs the *result* to be actionable, not just easier to filter.

3. **Build `assignments move` as a server-side endpoint and have the CLI call it.**
   - Pros: real atomicity; no TOCTOU.
   - Cons: server work; doesn't help any other agent task; same justification as the capability endpoint.
   - Why not chosen: client-side compound is good enough for the single-actor CLI use case; matches the posture of `assignments add`.

### Chosen Approach

Client-side enrichment for Unit 1, client-side filtering for Units 2-3, derived discovery for Unit 4, client-side compound for Unit 5. Every workaround is documented with the server-side fix that would obsolete it, so the supersession path is clear.

## Implementation Units

### Phase A — Lookup ergonomics

- [ ] **Unit 1: Client linkage in whois**

  **Goal:** Every `WhoisAssignment` carries `client: {id, name}` so agents can resolve human-supplied client names in one call.
  **Requirements:** R1
  **Dependencies:** None.
  **Files:**
  - Create: `internal/cmd/project_client_resolver.go`
  - Create: `internal/cmd/project_client_resolver_test.go`
  - Modify: `internal/cmd/whois.go` (add `Client *WhoisClient` to `WhoisAssignment`; build resolver before `buildWhoisReport`)
  - Modify: `internal/cmd/whois_test.go` (extend)

  **Approach:**
  1. Add a sibling struct in `whois.go`:
     ```go
     type WhoisClient struct {
         ID   string `json:"id"`
         Name string `json:"name"`
     }
     type WhoisAssignment struct {
         ID           string       `json:"id"`
         ProjectID    string       `json:"projectId"`
         CapabilityID string       `json:"capabilityId"`
         Project      string       `json:"project"`
         Client       *WhoisClient `json:"client,omitempty"`
         Date         string       `json:"date"`
         Hours        int          `json:"hours"`
     }
     ```
     `*WhoisClient` (pointer + omitempty) keeps the JSON shape backward-compatible when the resolver can't find the client (network failure, project not in the list).

  2. Create `projectClientResolver` in `internal/cmd/project_client_resolver.go`:
     ```go
     type ProjectClient struct {
         ID   string
         Name string
     }
     type projectClientResolver struct {
         client *api.Client
         cache  map[string]*ProjectClient // projectID → client (nil = lookup failed)
         loaded bool
     }
     func newProjectClientResolver(client *api.Client) *projectClientResolver
     func (r *projectClientResolver) Resolve(ctx context.Context, projectID string) *ProjectClient
     ```
     - First `Resolve` call triggers a single `GET /projects?expand=client&limit=200` and populates the cache from the response.
     - Subsequent calls hit the cache.
     - Returns `nil` (not error) on any failure — Unit 1 is fail-soft; the assignment still renders, just without the `client` field.
     - The single bulk fetch is the right shape because real usage always calls `Resolve` for every assignment row anyway.

  3. In `whois.go` `RunE`, after the assignments fetch and before `buildWhoisReport`, construct the resolver and pass it as a parameter (or set on a struct receiver). `buildWhoisReport` consults the resolver for each row.

  4. Add a soft warning when the resolver's bulk fetch fails: `app.Printer().Aux("warning: could not load project/client map; client field will be omitted (%v)", err)`. Don't fail the command.

  **Execution note:** Pointer-with-omitempty keeps existing consumers (those that don't look at `.client`) unchanged. Adding a non-nullable `client: {}` would also be backward-compatible JSON-wise but introduces empty objects in failure paths; pointer is cleaner.

  **Test scenarios:**
  - `Resolve` populates client when project is in the fetched list → resolver returns `{ID, Name}`.
  - `Resolve` on an unknown projectID after bulk fetch → returns `nil`, no error.
  - `buildWhoisReport` populates `Assignment.Client` when resolver returns a value.
  - `buildWhoisReport` leaves `Assignment.Client` nil when resolver returns nil → JSON omits the field.
  - Bulk fetch network failure → soft warning fires, command still succeeds with `Client` omitted on every row.
  - Cache: calling `Resolve` twice triggers exactly one HTTP request.

  **Verification:** Run against dev: `whois "Miquelajauregui" --weeks 4 --json | jq '.assignments[].client'` returns `{id, name}` on every row. Unit tests green.

- [ ] **Unit 2: `--name` filter on `users list`, `projects list`, `clients list`**

  **Goal:** All three list commands accept `--name <substring>` for case-insensitive substring filtering. Agents stop reaching for `jq` to translate human-supplied names.
  **Requirements:** R2
  **Dependencies:** None.
  **Files:**
  - Create: `internal/cmd/name_filter.go` (shared helper)
  - Create: `internal/cmd/name_filter_test.go`
  - Modify: `internal/cmd/users.go`, `internal/cmd/projects.go`, `internal/cmd/clients.go` (add flag + apply filter after fetch)

  **Approach:**
  1. `name_filter.go` exposes a small helper:
     ```go
     // filterByName returns the subset of items whose getName(item) contains needle (case-insensitive).
     // Returns the original slice unchanged when needle is empty.
     func filterByName[T any](items []T, needle string, getName func(T) string) []T
     ```
     Generic; reusable across the three list commands without per-command duplication.

  2. Each list command:
     - Adds `cmd.Flags().StringVar(&nameFilter, "name", "", "Case-insensitive substring filter on the entity name")`.
     - After `app.Execute` populates the slice, applies `filterByName(items, nameFilter, func(u api.User) string { return output.CoalesceString(u.Name) })` (or `CompanyName` for clients, `Name` for projects).
     - The Table/JSON rendering paths operate on the filtered slice.

  3. When `--name` is set and the API returns the full page (`len(items) == limit`), emit a one-line stderr note: `note: list was paginated at <limit> rows before filtering by --name; pass --limit if you expect more.` Non-blocking; pure-information.

  4. Update each command's `Example:` block to show `--name`:
     ```go
     # Find a user by name
     supervisible users list --name "miquela" --json
     ```

  **Execution note:** Filtering is post-pagination by design — we explicitly tell the user when the pagination might have hidden matches. Auto-paginating to "find everything" is footgun shape; explicit `--limit` is the escape hatch.

  **Test scenarios:**
  - `filterByName` returns matching items case-insensitively.
  - `filterByName` returns empty slice when no items match.
  - `filterByName` returns input unchanged when needle is empty.
  - `users list --name "miquela"` returns only Mariana's row.
  - `users list --name "no-such-user"` returns an empty list/table, exit code 0.
  - When the fetch hits the limit, the paginated-note appears on stderr.

  **Verification:** Against dev: `users list --name "miquela" --json | jq length` returns 1 (Mariana). `projects list --name "marketplace" --json` matches "Marketplace | SEO - Web - CRO".

- [ ] **Unit 3: Did-you-mean on empty `--name` result**

  **Goal:** When `--name` finds zero matches, the command suggests close candidates instead of returning silent empty output.
  **Requirements:** R3
  **Dependencies:** Unit 2 (uses its filter helper).
  **Files:**
  - Modify: `internal/cmd/name_filter.go` (add suggestion helper)
  - Modify: `internal/cmd/users.go`, `internal/cmd/projects.go`, `internal/cmd/clients.go` (emit suggestion when result is empty AND `--name` was set)
  - Modify: `internal/cmd/name_filter_test.go` (extend)

  **Approach:**
  1. Add `suggestNames(items []T, needle string, getName func(T) string, max int) []string` in `name_filter.go`:
     - Lowercase needle.
     - For each item: score by (a) substring containment, (b) shared-prefix length, (c) Levenshtein-lite (count of differing characters at same positions, bounded). The same heuristic as `schema describe`'s `suggestOperations`, but generalised for arbitrary item types via the `getName` accessor.
     - Sort by score, take top `max` (default 5).
     - Return the names (not the items) for the warning string.

  2. In each list command, after applying the filter:
     ```go
     if nameFilter != "" && len(filtered) == 0 {
         hints := suggestNames(items, nameFilter, getName, 5)
         if len(hints) > 0 {
             app.Printer().Aux("warning: no matches for --name %q. Did you mean: %s?", nameFilter, strings.Join(hints, ", "))
         } else {
             app.Printer().Aux("warning: no matches for --name %q.", nameFilter)
         }
     }
     ```
     Note: the warning goes to stderr; stdout still gets `[]` / empty table, so existing piped consumers don't see the warning.

  3. The empty case still exits 0 — the CLI's "no results" is information, not failure. Mirrors how `git grep` behaves.

  **Test scenarios:**
  - `suggestNames("F30", [F25, Q4, F40, Marketplace])` returns `[F25, F40, ...]` (substring or near-substring matches first).
  - `users list --name "marian"` finds Mariana and emits no warning (non-empty result).
  - `users list --name "totally-not-a-name"` emits `warning: no matches ...` (no suggestion if nothing close).
  - `projects list --name "Iniciativ"` (mid-typo) suggests "Iniciativa Q4" etc.
  - Exit code 0 on no-match (informational, not error).

  **Verification:** Against dev: `projects list --name "F30"` produces `warning: no matches for --name "F30". Did you mean: <closest projects>?` on stderr.

### Phase B — Capability discovery + write ergonomics

- [ ] **Unit 4: `capabilities list --for-project` (derived from assignments)**

  **Goal:** Surface the capabilities currently in use on a project, with usage counts, so agents have a recovery path when `--auto-capability` fails. Output explicitly marks the result as derived from history.
  **Requirements:** R4
  **Dependencies:** None.
  **Files:**
  - Create: `internal/cmd/capabilities.go`
  - Create: `internal/cmd/capabilities_test.go`
  - Modify: `internal/cmd/root.go` (register `newCapabilitiesCommand()`)

  **Approach:**
  1. New parent command `capabilities` with single subcommand `list`. `--for-project <id>` is required (Phase 3 doesn't ship a `--for-user` variant; add it if dogfooding asks).

  2. Implementation:
     ```go
     // capabilities list --for-project <id>
     // 1. GET /assignments?project_id=X&expand=capability&limit=200
     // 2. Aggregate by capabilityId: {capabilityId, name?, usageCount}
     // 3. Sort by usageCount desc
     // 4. JSON: array of { capabilityId, name, usageCount, source }
     //    Table: ID | NAME | USAGE | SOURCE
     // 5. ALWAYS print to stderr first:
     //    "warning: capability list is derived from assignment history (no GET /capabilities endpoint). May be incomplete."
     ```

  3. The `source` field on each row is the literal string `"derived-from-assignments"`. When `GET /capabilities` lands server-side, swap the implementation, change `source` to `"canonical"`, and drop the warning.

  4. Empty result (project has no assignments yet) → exit 0, emit `note: no capabilities found via assignment history. Project may be new or unstaffed.` on stderr. Don't suggest the canonical endpoint exists.

  5. `--limit` flag (default 200) controls how many assignments to scan. Bounded; large projects with thousands of assignments may need a higher limit, but the typical case fits.

  **Execution note:** No `--auto-capability` integration here; this is purely a discovery command. The integration story is: agent runs `--auto-capability`, gets the "no prior history" error, then runs `capabilities list --for-project X` to see what's actually used on the project, then picks one and re-runs with `--capability-id`.

  **Test scenarios:**
  - Project with 3 capabilities in assignment history → 3 rows, sorted by usage.
  - Project with no assignments → empty list + informational note.
  - JSON output includes `source: "derived-from-assignments"` on every row.
  - Stderr always carries the derived-view warning when results are non-empty.
  - `--limit 10` caps the assignment scan.

  **Verification:** Against dev: `capabilities list --for-project <Marketplace-id> --json` returns project's Project Management capability (the one Mariana would want to inherit).

- [ ] **Unit 5: `assignments move <assignment-id> --to-user <user-id>` compound**

  **Goal:** Move hours from one user to another on the same project in a single command. Safe by default: source untouched if target fails.
  **Requirements:** R5
  **Dependencies:** Unit 4 (recovery path when target capability resolution fails).
  **Files:**
  - Modify: `internal/cmd/assignments.go` (add `newAssignmentsMoveCommand` to the parent's `AddCommand` list and define it alongside `Add`)
  - Modify: `internal/cmd/assignments_add_test.go` or create `internal/cmd/assignments_move_test.go`

  **Approach:**
  1. New cobra command:
     ```
     Use:   "move <assignment-id>"
     Args:  argsWithUsage(cobra.ExactArgs(1))
     Flags:
       --to-user <user-id>          (required; UUID)
       --hours <int>                (default: all of source's hours; positive)
       --capability-id <uuid>       (optional; target's capability on the project)
       --auto-capability            (default true; uses Phase 2 resolver against the target user)
     ```

  2. Flow:
     1. Read source row: `GET /assignments/<id>` — actually use `/assignments?limit=1` filtered by ID via params, or do a `whois`-style filtered fetch. (Plan-time check: confirm the API has `GET /assignments/{id}` or accepts `id=X` filter; if neither, fall back to scanning `/assignments?user_id=<sourceUser>` and finding the ID. This is an **implementation-time unknown** to verify.)
     2. Validate: source must exist; source.hours must be > 0; `--hours` (if provided) must be ≤ source.hours.
     3. Resolve target capability:
        - If `--capability-id` set: use it directly.
        - Else if `--auto-capability` (default true): call the Phase 2 resolver against `(--to-user, source.projectId)`. On failure, emit the resolver's error AND a hint: `Try: supervisible capabilities list --for-project <projectId> --json` then non-zero exit.
     4. Compute amounts:
        - hours-to-move = `--hours` or source.hours
        - target-new-hours = (existing target row hours on same date) + hours-to-move
        - source-new-hours = source.hours - hours-to-move
     5. Pre-flight time-off (Phase 2 Unit 13) for the target on the source's date.
     6. Print summary on stderr: `move: <hours>h on <date> from <source-user> to <target-user> (project: <name>, capability: <id>)`.
     7. **Execute order**: ADD first (upsert target), then DELETE source (or partial-update source if source-new-hours > 0).
        - If add fails: source untouched. Loud error.
        - If add succeeds but source mutation fails: print loud "PARTIAL FAILURE" warning with both row IDs so a human/agent can reconcile.
     8. Dry-run prints both planned writes (target upsert + source delete/update) on stdout as a JSON array.

  3. **Edge cases:**
     - Source user == target user: error before any write (`move requires different users; use 'assignments upsert' to adjust hours in place`).
     - Move would zombify source (source-new-hours == 0 → delete source row entirely; explicit, not a refusal).
     - Move would go negative: caller's `--hours` exceeds source. Reject before any write.

  4. Document the partial-failure mode in `Long:`:
     ```
     Atomicity: this command does add-then-delete. If the add succeeds but the
     delete fails, source and target both carry the moved hours and the command
     exits non-zero with both assignment IDs. Reconcile by deleting one manually.
     A server-side PATCH /assignments with diff semantics would make this atomic;
     none exists today.
     ```

  **Execution note:** Plan-time question — verify `GET /assignments/{id}` or equivalent at implementation. If neither, the alternative fallback (filter scan by user) costs one extra GET but works for every existing assignment.

  **Test scenarios:**
  - Move all hours from source to a target with no prior row on this date → source deleted, target gets a new row.
  - Move partial hours → source decremented, target incremented (or created).
  - Source user == target user → error before any write.
  - Target capability auto-resolve fails → error before any write, references `capabilities list`.
  - Target row already exists → upsert sums correctly.
  - Move count > source.hours → error before any write.
  - Source upsert succeeds, delete fails (httptest fixture forces DELETE 500) → partial-failure warning, both IDs printed, non-zero exit.
  - Dry-run prints planned target upsert AND planned source delete on stdout; no writes happen.

  **Verification:** Against dev: `assignments move <Mariana-EdVisorly-row-id> --to-user <some-other-pm> --auto-capability --dry-run` shows the planned target upsert + source delete and warns if the target's date overlaps approved time-off.

- [ ] **Unit 6: `me` non-JSON output polish**

  **Goal:** `me` non-JSON output renders identity as readable lines, not Go's `map[...]` syntax.
  **Requirements:** R6
  **Dependencies:** None.
  **Files:**
  - Modify: `internal/cmd/me.go`
  - Modify: `internal/cmd/me_test.go` (create if absent)

  **Approach:**
  1. Replace `fmt.Fprintf(app.Printer().Stdout(), "Identity: %v\n", identity)` with a typed renderer:
     ```go
     w := app.Printer().Stdout()
     if v, ok := identity["keyName"].(string); ok && v != "" {
         fmt.Fprintf(w, "Key: %s\n", v)
     }
     if v, ok := identity["organizationId"].(string); ok {
         fmt.Fprintf(w, "Organization: %s\n", v)
     }
     if v, ok := identity["actorUserId"].(string); ok {
         fmt.Fprintf(w, "Actor user: %s\n", v)
     }
     if scopes, ok := identity["scopes"].([]any); ok && len(scopes) > 0 {
         parts := make([]string, 0, len(scopes))
         for _, s := range scopes {
             if str, ok := s.(string); ok {
                 parts = append(parts, str)
             }
         }
         fmt.Fprintf(w, "Scopes: %s\n", strings.Join(parts, ", "))
     }
     ```
     Stable order matters: humans scan top-to-bottom. JSON-mode output is unchanged.

  2. Don't add a fallback "other keys" line. If the server adds a field we don't render, the JSON path is the agent's escape hatch — we don't want the human path to leak unknown shapes.

  **Test scenarios:**
  - All four fields present → all four lines printed in order.
  - Missing `keyName` → that line omitted, other three still print.
  - `me --json` output unchanged from current behavior.

  **Verification:** Against dev: `me` prints `Key: test`, `Organization: <uuid>`, `Actor user: <uuid>`, `Scopes: *` — no `map[...]`.

## Open Questions

### Resolved During Planning

- **Client linkage: `client: {id, name}` object vs `clientId` + `clientName` flat fields?** Object. Mirrors `WhoisUser` shape; nullable on lookup failure. (Decision in Key Technical Decisions.)
- **Server-side `?name=` filter?** No — confirmed from OpenAPI schema. Client-side only.
- **Nested `expand=project.client` on `/assignments`?** No — schema says expand only takes flat `user, project, capability`. Unit 1 does a separate `/projects?expand=client` fetch.
- **`/capabilities` endpoint?** Doesn't exist. Unit 4 derives from assignment history and marks the source.
- **`assignments move` atomicity?** Add-then-delete; source untouched on add failure; partial-failure warning on delete failure. Documented in `Long:`. Matches `assignments add`'s TOCTOU posture.
- **Should `--name` auto-paginate to "find everything"?** No. Informational stderr note when the fetch hits its limit; explicit `--limit` is the user/agent's escape hatch.
- **Phase 3 should ship in two PRs (one per phase)?** Yes; Phase A first (lookup ergonomics, no semantic risk), Phase B second after Phase A merges.

### Deferred to Implementation

- **Does `GET /assignments/{id}` exist?** Used by Unit 5 step 1 (read source). Schema search needed at implementation; fallback is filter-scan via `/assignments?user_id=...`.
- **Edge case in Unit 5: source.hours == 0 (zombie row).** Should `move` refuse, or treat as "nothing to move"? Decide once we know if the API can return zombie rows on a `GET /assignments?...id=X` filter. (Phase 2 Unit 7 filters them at the whois layer; they may still exist server-side.)
- **Unit 4 capability `name` resolution.** The `expand=capability` description says it's supported on `/assignments`. Verify at implementation that `ExpandedCapability.Name` populates; if not, fall back to just IDs + counts.
- **`me` test approach.** No `me_test.go` exists today. Add one stub when implementing Unit 6, or extend `auth_test.go` if it's a better home. Decide at implementation.

## System-Wide Impact

- **Interaction graph.** Unit 1's `projectClientResolver` is a third per-invocation cache (joining `capabilityResolver` and the time-off pre-flight). Each compound command (`whois`, `assignments add`, future `assignments move`) constructs the ones it needs. No global state.
- **Error propagation.** All Phase 3 units route failures through Phase 1's `FormatCLIError`. Soft failures (Unit 1 client lookup, Unit 5 partial failure) emit `warning:` lines via `Aux` per the convention. No changes to the error contract.
- **State lifecycle risks.**
  - Unit 1's bulk `/projects?expand=client` fetch can race with project creation but is read-only and best-effort, so the race is harmless (newest project temporarily appears without client name; resolver returns nil and the field is omitted).
  - Unit 5's partial-failure mode (add succeeds, delete fails) is the load-bearing risk and is documented in `Long:` + exits non-zero so callers can detect and reconcile.
- **API surface parity.** None of the existing CLI commands change behavior other than additive fields (Unit 1) and additive flags (Unit 2-3). `me`'s JSON mode is unchanged.
- **JSON shape stability.** `WhoisAssignment` gains `client?: { id, name }` (pointer, omitempty). Existing consumers using `.id`, `.projectId`, `.capabilityId`, `.project`, `.date`, `.hours` are unchanged. The Phase 2 `weeksCovered` field on the report stays.

## Required Tests

Go tests run via `make test` (= `go test ./...`).

### Unit Tests (new / changed)

| Phase | Function / Behavior | Test File | What to Verify |
|---|---|---|---|
| A | `projectClientResolver.Resolve` happy path | `internal/cmd/project_client_resolver_test.go` | Returns `{ID, Name}` from cached bulk fetch |
| A | `projectClientResolver.Resolve` unknown ID | `internal/cmd/project_client_resolver_test.go` | Returns nil, no error |
| A | `projectClientResolver` cache | `internal/cmd/project_client_resolver_test.go` | Two Resolve calls trigger exactly one HTTP request |
| A | `projectClientResolver` fetch failure | `internal/cmd/project_client_resolver_test.go` | Returns nil; caller can render assignments without client field |
| A | `buildWhoisReport` populates `Assignment.Client` | `internal/cmd/whois_test.go` | When resolver returns a value, every WhoisAssignment carries it |
| A | `WhoisAssignment` omits `client` when nil | `internal/cmd/whois_test.go` | JSON marshal of report skips the field |
| A | `filterByName` happy path | `internal/cmd/name_filter_test.go` | Case-insensitive substring match across the 3 entity shapes |
| A | `filterByName` empty needle | `internal/cmd/name_filter_test.go` | Returns input unchanged |
| A | `filterByName` no match | `internal/cmd/name_filter_test.go` | Returns empty slice |
| A | `users list --name` end-to-end | `internal/cmd/users_test.go` (extend or create) | Filter applied post-fetch |
| A | Pagination-limit stderr note | `internal/cmd/name_filter_test.go` | Fires when `len(items) == limit` AND `--name` set |
| A | `suggestNames` substring + near-match | `internal/cmd/name_filter_test.go` | Returns up to 5 candidates sorted by relevance |
| A | `users list --name` zero-match → stderr suggestion | `internal/cmd/users_test.go` (extend) | Warning on stderr; stdout still empty list |
| B | `capabilities list --for-project` happy path | `internal/cmd/capabilities_test.go` | Aggregates by capabilityId, sorted by usage |
| B | `capabilities list` derived-source warning | `internal/cmd/capabilities_test.go` | Stderr always carries the warning |
| B | `capabilities list` empty project | `internal/cmd/capabilities_test.go` | Empty list + informational note on stderr; exit 0 |
| B | `capabilities list --json` includes `source` field | `internal/cmd/capabilities_test.go` | Every row has `source: "derived-from-assignments"` |
| B | `assignments move` happy path (all hours, no target row) | `internal/cmd/assignments_move_test.go` | Source deleted, target created |
| B | `assignments move` partial hours | `internal/cmd/assignments_move_test.go` | Source decremented, target incremented or created |
| B | `assignments move` same user → error before write | `internal/cmd/assignments_move_test.go` | No HTTP write fires |
| B | `assignments move` target capability resolution fails → error w/ capabilities-list hint | `internal/cmd/assignments_move_test.go` | Error message references `capabilities list` |
| B | `assignments move` add succeeds, delete fails → partial-failure warning | `internal/cmd/assignments_move_test.go` | Stderr carries both IDs, exit non-zero |
| B | `assignments move --dry-run` prints both planned writes | `internal/cmd/assignments_move_test.go` | No HTTP writes; stdout shows target upsert + source delete plan |
| B | `me` non-JSON renders typed lines | `internal/cmd/me_test.go` (new) | "Key:", "Organization:", "Actor user:", "Scopes:" all present; no `map[...]` |
| B | `me` non-JSON tolerates missing fields | `internal/cmd/me_test.go` (new) | Missing keyName → that line omitted, others present |
| B | `me --json` unchanged | `internal/cmd/me_test.go` (new) | JSON output identical to current behavior |

### Existing Tests to Verify (no regressions)

- [ ] `internal/cmd/whois_test.go` — current week behavior unchanged when `--weeks` is absent
- [ ] `internal/cmd/capability_resolver_test.go` — Phase 2 resolver untouched
- [ ] `internal/cmd/timeoff_preflight_test.go` — Phase 2 pre-flight untouched
- [ ] `internal/cmd/assignments_add_test.go` — Phase 2 `assignments add` flow untouched
- [ ] `internal/cmd/schema_test.go` — Phase 2 noun form + did-you-mean still works
- [ ] `internal/cmd/params_warning_test.go` — Phase 2 unknown-param warning still fires
- [ ] `internal/cmd/run_test.go` — `App.Execute` propagates errors / dry-run / decode unchanged
- [ ] `internal/cmd/output_test.go` (in `internal/output/`) — Printer contract untouched

## Success Criteria

### Automated Verification

- [ ] `make fmt` clean (no diff)
- [ ] `make test` green (all existing + new tests pass)
- [ ] `go vet ./...` clean
- [ ] `make build` produces `bin/supervisible` without warnings

### Manual Verification — Phase A

- [ ] `./bin/supervisible whois "Miquelajauregui" --weeks 4 --json | jq '.assignments[0].client'` returns `{id, name}`
- [ ] `./bin/supervisible users list --name "miquela" --json` returns only Mariana
- [ ] `./bin/supervisible projects list --name "F30"` emits a `Did you mean:` warning on stderr (no project matches)
- [ ] `./bin/supervisible clients list --name "avask" --json | jq length` returns 1
- [ ] When pagination kicks in (e.g. `users list --name "a"` on a >50-user org), stderr shows the pagination note
- [ ] All JSON paths still pipe cleanly through `jq` (Phase 1 contract holds)

### Manual Verification — Phase B

- [ ] `./bin/supervisible capabilities list --for-project <marketplace-id> --json` returns at least one capability with `source: "derived-from-assignments"` and a stderr warning
- [ ] `./bin/supervisible capabilities list --for-project <empty-project-id>` exits 0 with an informational note
- [ ] `./bin/supervisible assignments move <mariana-edvisorly-row> --to-user <other-pm> --auto-capability --dry-run` shows planned target upsert + source delete with time-off pre-flight if applicable
- [ ] `./bin/supervisible assignments move <id> --to-user <same-user>` errors before any write
- [ ] `./bin/supervisible me` prints `Key:`, `Organization:`, `Actor user:`, `Scopes:` (no `map[...]`)
- [ ] `./bin/supervisible me --json` output unchanged from Phase 1

## Dependencies & Risks

### Dependencies

- `golang.org/x/term` (already in `go.mod`, Phase 1).
- `github.com/spf13/cobra` (already in `go.mod`).
- No new dependencies introduced.

### Risks

**Phase A:**

- **Unit 1's bulk `/projects?expand=client` is wasteful for small whois lookups.** A single-assignment `whois` triggers a 200-project fetch.
  - **Mitigation:** Only build the resolver when there's ≥1 assignment to enrich. For users with no assignments, skip the fetch entirely. Cost is then proportional to the value delivered.
- **`--name` filter shadows true API filters in users' minds.** Someone may assume server-side filtering and be surprised when pagination matters.
  - **Mitigation:** The pagination note on stderr makes the truncation explicit. Document in `--help` that `--name` is post-fetch.
- **`suggestNames` is O(n) per call.** Fine for current data volumes; not a concern.

**Phase B:**

- **Unit 4's derived list is misleading.** Agents may treat it as canonical.
  - **Mitigation:** Stderr warning + explicit `source` field in JSON. When `GET /capabilities` lands, switch implementation and drop the warning.
- **Unit 5's partial-failure mode is the load-bearing trade-off.** A failed delete after a successful add doubles the moved hours.
  - **Mitigation:** Loud `PARTIAL FAILURE` warning, both row IDs printed, non-zero exit. The dry-run path is risk-free. The full atomicity path is server-side scope.
- **Unit 5 capability resolution against the target user.** A user new to the project still hits the Phase 2 "no prior history" failure.
  - **Mitigation:** Error message references `capabilities list --for-project <id>` so the agent has a recovery path; Unit 4 lands first.

## Performance Considerations

- Unit 1 adds one bounded GET (`/projects?expand=client?limit=200`) per `whois` invocation. Cached per call. Negligible for interactive use; for scripted high-frequency `whois` calls, consider an env-var-gated skip flag in a follow-up.
- Unit 4 issues one GET per `capabilities list` call. Same cost shape as Phase 2's `capabilityResolver`.
- Unit 5 issues: 1 GET (source read) + 1 GET (target existing-hours check) + 1 POST (target upsert) + 1 DELETE (source). Four calls for one logical move. Acceptable for an interactive write; not a hot path.
- Unit 2/3 add zero HTTP overhead — pure post-fetch processing.

## Security Considerations

- **No new attack surface.** Every Phase 3 unit reads from already-authorized endpoints (`/projects`, `/clients`, `/users`, `/assignments`, `/time-off`) and writes only via existing primitives (`POST /assignments`, `DELETE /assignments/{id}`).
- **No new secrets handled.** Auth token plumbing is unchanged.
- **Unit 5's partial-failure exit code is non-zero**, so CI / orchestration systems detect the divergent state automatically.
- **`me` polish (Unit 6) does not change which fields are surfaced** — same data as today, just rendered in stable typed lines.

## References

- Source plan / Phases 1-2: `.thoughts/plans/2026-05-19-feat-cli-agent-experience.md` (status: implemented)
- Implementation notes (Phases 1-2): `.thoughts/implementation-notes/2026-05-19-feat-cli-agent-experience.md`
- Real-world findings: `.thoughts/findings/2026-05-19-cli-real-world-testing.md`
- Phase 1 PR: https://github.com/meaningfulteam/supervisible-cli/pull/2 (merged)
- Phase 2 PR: https://github.com/meaningfulteam/supervisible-cli/pull/3 (open)
- OpenAPI schema: `internal/schema/openapi/public-api-v1.json`
- Existing patterns to mirror:
  - `internal/cmd/capability_resolver.go` (per-invocation cache shape for Unit 1)
  - `internal/cmd/timeoff_preflight.go` (soft-warning posture for Unit 5)
  - `internal/cmd/schema.go` `suggestOperations` (did-you-mean for Unit 3)
  - `internal/cmd/assignments.go` `newAssignmentsAddCommand` (compound-command shape for Unit 5)

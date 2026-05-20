---
type: plan
status: superseded
superseded_by: .thoughts/plans/2026-05-19-feat-cli-agent-experience.md
superseded_on: 2026-05-19
tags: [cli, agent, writes, whois, assignments, safety]
created: 2026-05-19
related_findings: .thoughts/findings/2026-05-19-cli-real-world-testing.md
---

> **Superseded 2026-05-19.** Merged into `.thoughts/plans/2026-05-19-feat-cli-agent-experience.md` as **Phase 2 (Agent-safe writes), Units 7–13**. The merged plan keeps every unit, every test, and every requirement from this document; only the unit numbering changed (Units 1–7 here became 7–13 there, with the `SetEscapeHTML(false)` fix folded into Phase 1 Unit 1 since it lives in the same `output.go` rewrite). Reasoning for the merge: both plans touch the same files (`output.go`, `whois.go`, `assignments.go`, `root.go`) over weeks of work, and Phase 2 explicitly inherits Phase 1's `Aux` contract and `App.Execute` shape — keeping them as one plan prevents drift and the cross-references resolve in-document instead of across files. This file is kept for history; **do not implement from it**.

# feat(cli): Agent-safe writes — compound resolvers, pre-flight checks, output hygiene

## Overview

Real-world test of "translate a Slack message into `assignments upsert`" surfaced a class of agent-correctness gaps the prior plan (CLI ergonomics) doesn't address. This plan adds the compound commands and pre-flight checks needed for an agent to write safely, plus four small output fixes that came out of the same test. Source: `.thoughts/findings/2026-05-19-cli-real-world-testing.md` (F1–F15).

## Current State Analysis

### Key Discoveries (from real test, 2026-05-19)

- **Capability resolution has no agent-safe path.** Test run picked the wrong `capabilityId` because the only heuristic available ("any cap ever used on this project") doesn't match the user's actual role. The correct heuristic is "most recent cap **this user** used on **this project**". `internal/cmd/whois.go:117-127` already fetches assignments by user — extending this is straightforward.
- **`assignments upsert` is a replace, not an add.** Natural-language requests like "2h más" require read-modify-write that every agent has to reimplement. Upsert flow at `internal/cmd/assignments.go:101-198`.
- **`hours: 0` upsert is currently used as pseudo-delete** (no `DELETE /assignments/{id}` exists) and creates zombie rows that `whois` then displays. `internal/cmd/whois.go:18-22` defines `WhoisAssignment` — a one-line filter on `Hours > 0` cleans this up at the CLI boundary while server fix is pending.
- **`whois.assignments[]` returns only `project` name, `date`, `hours`** (`whois.go:16-22`) — no IDs, so you can't modify what you just read.
- **`Printer.PrintJSON` uses `json.NewEncoder` defaults** (`output.go:30, 39`) which HTML-escape `&` to `&`. Visible in every `--json` output that includes ampersands.
- **`schema describe` accepts only full op IDs** like `assignments.get`, not the noun-form `assignments` shown by `schema endpoints`. Agent first guess fails.
- **`--params` accepts unknown keys silently** — no warning when an agent types `startDate` instead of `start_date`. Smaller deal than F1 originally suggested (filters actually work), but still a UX trap.
- **Assignments can be written that overlap approved time-off** with no warning. Juan's sabbatical (May 10 – Jul 3) was bypassed silently when we wrote 6 rows in that window.

### Out-of-scope (server-side, file separately in main app)

| Server ask | Source finding | Why blocking |
|---|---|---|
| `GET /capabilities` endpoint | F3 | Without it, the CLI can't name-resolve capabilities |
| `DELETE /assignments/{id}` (or `hours:0` = delete) | F12 | Currently zombie rows accumulate |
| Soft-warn at write time on time-off overlap | F4 | CLI pre-flight is best-effort; server is source of truth |
| `expand=project` default on POST response | F9 | Readable success output |
| Reject unknown query params | F10 (server side) | Currently silent accept |

## Requirements Trace

- **R1.** Output hygiene: JSON output MUST NOT HTML-escape ampersands; `whois.assignments[]` MUST omit zero-hour rows; `whois.assignments[]` MUST include `id`, `projectId`, `capabilityId` so the output is actionable.
- **R2.** Discoverability: `schema describe <noun>` MUST accept both the noun form (`assignments`) and the full op ID (`assignments.get`); unknown noun MUST list valid alternatives.
- **R3.** Param validation: when a `--params` key isn't recognized for the target endpoint, the CLI MUST emit a warning to stderr (not block; the server is still the source of truth).
- **R4.** Window flexibility: `whois` MUST accept `--weeks N` (default 1) so a single call covers multi-week planning.
- **R5.** Capability auto-resolution: `assignments upsert` MUST accept `--auto-capability`, which fills missing `capabilityId` per (user, project) using "most recent capability **this user** used on this project". MUST fail loudly when no history exists.
- **R6.** Delta writes: a new compound `assignments add` MUST do read-modify-write (current hours + delta, by `(user, project, capability, date)`), using `--auto-capability` semantics by default.
- **R7.** Time-off pre-flight: when `--dry-run` is used on `assignments upsert` or `assignments add`, the CLI MUST surface any items whose `(user, date)` overlaps approved time-off, as warnings on stderr.

## Desired End State

```bash
# Natural-language Slack request: "2h más para Juan en Odyssey esta semana"
# Becomes a one-line, capability-safe, conflict-aware write:
supervisible assignments add \
  --user-id 019404f3-... --project-id 019e1cde-... \
  --date 2026-05-24 --hours 2 --auto-capability --dry-run

# Stderr (warnings):
#   ⚠ Time-off overlap: Juan Méndez has approved Sabbatical 2026-05-10 → 2026-07-03
#   capability resolved: Web Dev (0194b2e1-b918-7447-a88e-a85ccdea5634) — most recent on this project
#
# Stdout (dry-run plan):
#   { method: POST, endpoint: /assignments, body: { items: [{...hours: 4 (current 2 + delta 2)}] } }
```

`supervisible whois "Juan Méndez" --weeks 4 --json` returns 4 weeks of assignments + time-off, each assignment carrying `id`, `projectId`, `capabilityId`, and no zero-hour rows. `&` renders as `&` in JSON. `schema describe assignments` shows both `assignments.get` and `assignments.post`.

## What We're NOT Doing

- **Server-side endpoints** (`GET /capabilities`, `DELETE /assignments/{id}`, time-off enforcement). Tracked separately in the main Supervisible repo.
- **Transactional batched writes.** `assignments add` does read-then-write client-side; a server-side `PATCH /assignments` with diff semantics would be cleaner but is out of scope.
- **Interactive confirmation prompts on writes.** Covered by TTY guard in prior plan; not adding `Are you sure?` prompts here.
- **Refactoring `whois` into a generic "person profile" command.** Keep its scope as-is, just extend the time window.

## Stakeholder Impact

- **Agents (highest impact)**: writes go from footgun → safe. `assignments add` + `--auto-capability` together make "2h más" a one-liner. Pre-flight conflict warnings prevent silent overbooking.
- **Human developers**: cleaner `--json` output, fewer surprises when piping to `jq`. `whois --weeks 4` saves several manual `assignments list` calls.
- **Supervisible product team**: the server-side ask list now has concrete justification from real testing rather than speculation.

## Key Technical Decisions

- **Auto-capability heuristic is "most recent hours > 0 row for (user, project)".** Skip zombie rows. If no history exists, fail with a clear message — never guess from project-level capabilities (that's the bug this fixes).
- **`assignments add` is implemented client-side.** Read existing assignment(s) for `(user, project, capability, date)`, sum with delta, upsert the new total. Race condition with concurrent writers is acceptable — we're a CLI, not a multi-master DB. Document the trade-off.
- **Pre-flight time-off check fires on `--dry-run` only.** Without server enforcement, running it on every write would double API calls for non-dry-run paths. Dry-run is the agent's "did I get this right?" moment — best place for the warning.
- **`whois --weeks N` extends the window but keeps the single-week summary.** Returns `assignments[]` for all N weeks but `weekSummary` stays scoped to the current week to preserve existing semantics.
- **`SetEscapeHTML(false)` is safe** because the CLI prints to terminals/pipes, not HTML contexts. Standard practice for CLI tools.
- **Plan depends on prior plan's Unit 1 (stderr discipline)** for warnings to land on stderr. If that ships first, this plan inherits the routing. If not, this plan's `PrintWarning` helper writes directly to `p.err`.

## Implementation Approach

```
Unit 1 (output hygiene) ──┐
Unit 2 (schema describe) ─┤  (independent, small)
Unit 3 (param warnings)  ─┘

Unit 4 (whois --weeks N)            ── independent

Unit 5 (auto-capability resolver)   ── building block
Unit 6 (assignments add)            ── depends on Unit 5
Unit 7 (time-off pre-flight)        ── depends on Unit 5/6 (uses dry-run path)
```

Ship Units 1–4 first (low-risk, immediate UX wins). Units 5–7 are the agent-safety work and can land together.

## Implementation Units

- [ ] **Unit 1: Output hygiene — JSON escape, zombie filter, whois IDs**

  **Goal:** Three small fixes to make `--json` output clean and `whois` output actionable.
  **Requirements:** R1
  **Dependencies:** None
  **Files:**
  - Modify: `internal/output/output.go` (lines 30, 39 — `json.NewEncoder` calls)
  - Modify: `internal/cmd/whois.go` (lines 16-22 — `WhoisAssignment` struct; lines 145+ — `buildWhoisReport`)
  - Modify: `internal/cmd/whois.go` printer (lines 244+ — `printWhoisProfile` if it renders assignments)
  - Test: `internal/output/output_test.go`, `internal/cmd/whois_test.go` (extend or create)

  **Approach:**
  1. In `output.go`, after each `json.NewEncoder(...)`, call `enc.SetEscapeHTML(false)`. Two call sites (lines 30, 39).
  2. In `whois.go`, extend `WhoisAssignment`:
     ```go
     type WhoisAssignment struct {
         ID           string `json:"id"`
         ProjectID    string `json:"projectId"`
         CapabilityID string `json:"capabilityId"`
         Project      string `json:"project"`
         Date         string `json:"date"`
         Hours        int    `json:"hours"`
     }
     ```
  3. In `buildWhoisReport`, when iterating `assignments []api.Assignment`, skip entries where `a.Hours <= 0` and populate the new fields.
  4. Update `printWhoisProfile` if it renders per-row hours — it should also exclude zero-hour rows (currently aggregates by project name so this is probably a no-op).

  **Test scenarios:**
  - `PrintJSON({"name":"A & B"})` produces `"name": "A & B"`, not `"&"`.
  - `buildWhoisReport` with one 0h assignment + one 2h assignment returns one assignment in the output.
  - JSON output of `whois` contains `id`, `projectId`, `capabilityId` on each assignment.

  **Verification:** `./bin/supervisible whois "Juan Méndez" --json | jq '.assignments[].hours' | grep -v 0` shows no zombie rows; ampersands render literally.

- [ ] **Unit 2: `schema describe` accepts short noun form**

  **Goal:** `schema describe assignments` returns descriptions of `assignments.get` AND `assignments.post`. Unknown noun gives a "did you mean" hint.
  **Requirements:** R2
  **Dependencies:** None
  **Files:**
  - Modify: `internal/cmd/schema.go`
  - Test: `internal/cmd/schema_test.go` (create if missing)

  **Approach:**
  1. In the `describe` handler, before failing with `operation not found`, check if the arg matches a noun prefix (everything before the `.`). If yes, list all matching ops with their summaries.
  2. If still no match, build a "did you mean" list of operation IDs sharing a substring, sorted by edit distance (or just substring match if simpler).
  3. Keep existing exact-match behavior intact.

  **Test scenarios:**
  - `schema describe assignments` → describes both `.get` and `.post`.
  - `schema describe assignment` (typo) → suggests `assignments.get`, `assignments.post`.
  - `schema describe assignments.get` (exact) → unchanged behavior.
  - `schema describe nonsense` → "no match. Available operations: ..." with full list or hint.

  **Verification:** `./bin/supervisible schema describe assignments` prints both endpoints.

- [ ] **Unit 3: Warn on unknown `--params` keys**

  **Goal:** When an agent passes `--params '{"startDate":"..."}'` (camelCase typo) the CLI emits a stderr warning rather than silently accepting.
  **Requirements:** R3
  **Dependencies:** None (but synergizes with prior plan's Unit 1 stderr discipline)
  **Files:**
  - Modify: `internal/cmd/root.go` (`ResolvedQuery` or a new check in `PersistentPreRunE`)
  - Modify: `internal/schema/provider.go` (add `KnownQueryParams(method, endpoint) []string` if not present)
  - Test: `internal/schema/provider_test.go`

  **Approach:**
  1. Add `Provider.KnownQueryParams(method, endpoint) []string` returning the set of accepted query params for that operation (driven by the static schema).
  2. In `App.ResolvedQuery`, after merging user params, diff against known and warn for each unknown key:
     ```
     warning: unknown query param "startDate" for GET /assignments (allowed: user_id, project_id, start_date, end_date, limit, offset)
     ```
  3. Warning is non-blocking — request still goes through. Server remains source of truth.
  4. Suppress warnings when no schema is loaded (defensive).

  **Test scenarios:**
  - Unknown key triggers a single stderr warning with allowed list.
  - Known keys produce no warning.
  - `--params` with no schema match (e.g. unrecognized endpoint) is silent.

  **Verification:** `./bin/supervisible assignments list --params '{"startDate":"..."}' 2>&1 >/dev/null` shows the warning.

- [ ] **Unit 4: `whois --weeks N` flag**

  **Goal:** Extend `whois` to cover N weeks in one call. Default N=1 (current behavior).
  **Requirements:** R4
  **Dependencies:** None
  **Files:**
  - Modify: `internal/cmd/whois.go` (add `--weeks` flag, extend the assignments date window, update `WhoisReport` shape if needed)
  - Test: `internal/cmd/whois_test.go` extend

  **Approach:**
  1. Add `--weeks N` flag (int, default 1, validate 1 ≤ N ≤ 12).
  2. In `RunE`, compute `weekEnd = weekStart + N*7 days - 1`. Pass to assignments fetch.
  3. `WhoisReport.WeekSummary` stays scoped to the current week (first of N) for backward compat. Add `WhoisReport.WeeksCovered int` to the JSON so consumers know the window.
  4. Update `printWhoisProfile` (table mode) to group by week when N > 1, else keep current single-week rendering.
  5. Update the `Example:` block on the command to show `--weeks 4`.

  **Test scenarios:**
  - `--weeks 1` (default) returns exactly today's week assignments — no change in shape from current.
  - `--weeks 4` returns 4 weeks of assignments; `WeeksCovered: 4`.
  - `--weeks 0` rejects with a clear validation error.
  - `--weeks 13` rejects with a clear validation error (sanity cap).

  **Verification:** `./bin/supervisible whois "Juan Méndez" --weeks 4 --json | jq '.assignments | length'` returns more rows than `--weeks 1`.

- [ ] **Unit 5: Auto-capability resolver helper**

  **Goal:** Centralize the "most recent capability this user used on this project" lookup as a reusable helper. Used by Units 6 and 7.
  **Requirements:** R5 (foundation)
  **Dependencies:** None
  **Files:**
  - Create: `internal/cmd/capability_resolver.go`
  - Test: `internal/cmd/capability_resolver_test.go`

  **Approach:**
  1. Function signature:
     ```go
     // resolveCapability returns the capabilityId most recently used by userID on projectID
     // in any assignment with hours > 0. Returns "" + error if no history exists.
     func resolveCapability(ctx context.Context, client *api.Client, userID, projectID string) (string, error)
     ```
  2. Implementation: fetch `/assignments?user_id=X&project_id=Y&limit=50`, filter `hours > 0`, sort by `date desc`, return first row's `capabilityId`.
  3. Error message must be specific: `"no prior capability found for user %s on project %s — pass --capability-id explicitly"`.
  4. Cache results per `(userID, projectID)` in-process for the duration of a single command invocation (helps Unit 6 batch resolution).

  **Test scenarios:**
  - History exists → returns most recent capability.
  - Only zombie (0h) history → returns error.
  - No history → returns error with explicit message.
  - Two rows with same date → returns either deterministically (sort secondary by ID).

  **Verification:** Unit-tested only; verified end-to-end in Unit 6.

- [ ] **Unit 6: `assignments upsert --auto-capability` + `assignments add` compound**

  **Goal:** Two related additions:
  - `--auto-capability` flag on existing `upsert` fills in `capabilityId` per item using Unit 5.
  - New `assignments add` compound does read-modify-write: existing hours + delta, idempotent if you specify the capability.
  **Requirements:** R5, R6
  **Dependencies:** Unit 5
  **Files:**
  - Modify: `internal/cmd/assignments.go` (extend `newAssignmentsUpsertCommand`, add `newAssignmentsAddCommand`)
  - Modify: `internal/cmd/assignments.go` `newAssignmentsCommand` registration
  - Test: `internal/cmd/assignments_test.go` (extend)

  **Approach (upsert `--auto-capability`):**
  1. Add `--auto-capability` bool flag. When set, iterate items in the parsed payload; for any item missing `capabilityId`, call `resolveCapability` (Unit 5).
  2. Surface the resolution on stderr (one line per item): `capability resolved for <user>/<project>: <name? or id>`.
  3. If resolution fails for any item, exit non-zero with the combined list of failures (don't partial-write).

  **Approach (`assignments add` compound):**
  1. New cobra command `assignments add` with flags: `--user-id`, `--project-id`, `--date`, `--hours` (delta, can be negative), `--capability-id` (optional), `--auto-capability` (default true).
  2. Resolve `capabilityId` if not provided.
  3. Fetch existing assignment for `(user, project, capability, date)`. If found: `new = existing.hours + delta`. If not found: `new = delta`.
  4. Reject if `new < 0` (don't allow phantom negative hours).
  5. Reject if `new == 0` and existing exists (until DELETE endpoint lands, document this as an intentional limit).
  6. Build the upsert payload and route through the same path as `upsert` (so `--dry-run` works identically).
  7. Print one-line summary on stderr: `assignments add: <user> <project> <date> <existing>h + <delta>h = <new>h`.

  **Test scenarios:**
  - `add` with no existing row: new = delta.
  - `add` with existing row: new = existing + delta.
  - `add --auto-capability` with no history: fails with helpful message.
  - `add --hours -10` with existing 8h: fails (would go negative).
  - `add` in `--dry-run`: shows the computed `new` value, doesn't write.
  - `upsert --auto-capability` with a payload of 3 items: all three get resolved (or all three fail with combined error).

  **Verification:** Replay the original Juan test with `assignments add --auto-capability --dry-run` and confirm the right capability is picked.

- [ ] **Unit 7: Time-off conflict pre-flight in dry-run**

  **Goal:** When `--dry-run` runs on `assignments upsert` or `assignments add`, emit a stderr warning for each item whose `(user, date)` overlaps approved time-off.
  **Requirements:** R7
  **Dependencies:** Unit 5/6 (shares the dry-run code path)
  **Files:**
  - Modify: `internal/cmd/assignments.go` (extend dry-run branch in upsert)
  - Modify: `internal/cmd/root.go` `MaybeDryRun` if pre-flight needs to be generic
  - Test: `internal/cmd/assignments_test.go`

  **Approach:**
  1. Before printing the dry-run plan, collect distinct `(userID, minDate, maxDate)` tuples from the items.
  2. For each unique userID, fetch `/time-off?user_id=X&status=approved&start_date=<minDate>&end_date=<maxDate>`.
  3. For each item, compare against returned time-off windows; emit a warning per overlap:
     ```
     ⚠ time-off overlap: <user name> has approved <type> 2026-05-10 → 2026-07-03 (item: project=<id> date=2026-05-24)
     ```
  4. Aggregate by user when many items overlap the same time-off entry (don't spam).
  5. Pre-flight failures (e.g. API down) emit a single "could not verify time-off (API unreachable); proceeding without check" warning — don't block the dry-run.

  **Test scenarios:**
  - Item with no overlap → no warning.
  - Item inside approved time-off → one warning.
  - Multiple items same user, same time-off → one aggregated warning.
  - Time-off fetch fails → soft warning, dry-run continues.
  - Not in dry-run mode → no pre-flight (don't double the request rate on real writes).

  **Verification:** Replay the original Juan test — adding to Sabbatical window now produces a visible warning on stderr.

## Open Questions

### Resolved During Planning

- **Should `assignments add` accept multiple items via `--file`?** Yes — it should mirror `upsert`'s shape. Defer the multi-item read-modify-write to a follow-up if it gets gnarly. For v1, support single-item add via flags; multi-item via `--file` does read-modify-write per item with a clear summary.
- **Auto-capability heuristic edge case: user is new to the project.** Fail loudly — `"no prior capability found"`. Don't silently fall back to a project-level guess (that's the bug we're fixing).
- **What if the upsert response doesn't include capability names?** We don't have them anyway (no `GET /capabilities`). Show IDs in success output until server adds expansion. Document this limitation.

### Deferred to Implementation

- **Format of the stderr warning prefix** — `⚠`, `warning:`, `[WARN]`? Pick during Unit 7; align with whatever prior plan's Unit 1 establishes for stderr output style.
- **Whether `--auto-capability` should be opt-in or default.** Lean opt-in (explicit) for `upsert`; default-on for `add` since that's a higher-level convenience command. Confirm during code review.

## System-Wide Impact

- **Interaction graph:** Units 5–7 add new read calls (assignments query, time-off query) inside write commands. Document and bound them — Unit 5 caches per-invocation, Unit 7 fires only in dry-run.
- **Error propagation:** Unit 6 must distinguish "this item failed to resolve" (specific) from "API call failed" (general). Propagate via standard `*APIError` for the latter so prior plan's error formatter renders them well.
- **State lifecycle risks:** Unit 6's `assignments add` has a TOCTOU race: read existing → some other client writes → we upsert with stale base. Acceptable for a CLI; document the trade-off in the command's `Long:` field.
- **API surface parity:** `assignments add` is CLI-only. The server's `POST /assignments` semantics are unchanged. If the server later grows a delta-aware endpoint, `add` becomes a thin wrapper.

## Required Tests

Go tests run via `make test` (= `go test ./...`).

### Unit Tests (new / changed)

| Function / Behavior | Test File | What to Verify |
|---|---|---|
| `PrintJSON` does not HTML-escape | `internal/output/output_test.go` | Output contains literal `&`, not `&` |
| `buildWhoisReport` filters zero-hour rows | `internal/cmd/whois_test.go` | Input with one 0h + one 2h returns one assignment |
| `WhoisAssignment` carries `id`, `projectId`, `capabilityId` | `internal/cmd/whois_test.go` | JSON marshal includes the new fields |
| `schema describe assignments` lists both .get and .post | `internal/cmd/schema_test.go` | Output mentions both operations |
| `schema describe <typo>` suggests close matches | `internal/cmd/schema_test.go` | Non-empty "did you mean" list |
| `KnownQueryParams` returns endpoint params | `internal/schema/provider_test.go` | Returns expected slice for `GET /assignments` |
| Unknown `--params` key warns | `internal/cmd/root_test.go` (new) | Stderr contains "unknown query param" |
| `--weeks N` validation | `internal/cmd/whois_test.go` | 0 / 13 rejected; 1 / 4 accepted |
| `resolveCapability` happy path | `internal/cmd/capability_resolver_test.go` | Returns most recent hours>0 capability |
| `resolveCapability` no history | `internal/cmd/capability_resolver_test.go` | Returns specific error |
| `resolveCapability` only zombie rows | `internal/cmd/capability_resolver_test.go` | Returns error (skips 0h) |
| `assignments add` read-modify-write | `internal/cmd/assignments_test.go` | Existing 2h + delta 2h → upsert with 4h |
| `assignments add` no existing | `internal/cmd/assignments_test.go` | Upserts delta as the full value |
| `assignments add` negative result rejected | `internal/cmd/assignments_test.go` | Returns error before any write |
| `--auto-capability` resolves per item | `internal/cmd/assignments_test.go` | Multi-item payload all get capabilityId filled |
| Time-off pre-flight warns on overlap | `internal/cmd/assignments_test.go` | Stderr contains warning for overlapping item |
| Time-off pre-flight skipped without `--dry-run` | `internal/cmd/assignments_test.go` | No extra API call when not dry-run |

### Existing Tests to Verify (no regressions)

- [ ] `internal/cmd/whois_test.go` — current week behavior unchanged when `--weeks` is absent or `--weeks 1`
- [ ] `internal/cmd/compound_test.go` — JSON output of `whois` includes the new fields but doesn't break consumers expecting `project`/`date`/`hours`
- [ ] `internal/cmd/agent_flags_test.go` — `--json`, `--fields`, `--dry-run` semantics unchanged
- [ ] `internal/cmd/delete_commands_test.go` — error messages unchanged
- [ ] `internal/output/output_test.go` — table rendering unaffected

## Success Criteria

### Automated Verification

- [ ] `make fmt` clean
- [ ] `make test` green (existing + new)
- [ ] `go vet ./...` clean
- [ ] `make build` produces `bin/supervisible`

### Manual Verification

- [ ] `./bin/supervisible whois "Juan Méndez" --weeks 4 --json | jq '.assignments[0] | keys'` includes `id`, `projectId`, `capabilityId`, no `&` anywhere
- [ ] `./bin/supervisible whois "Juan Méndez" --json | jq '.assignments[] | select(.hours == 0)'` returns empty
- [ ] `./bin/supervisible schema describe assignments` lists both operations
- [ ] `./bin/supervisible assignments list --params '{"startDate":"2026-05-18"}' 2>&1 1>/dev/null` shows unknown-param warning
- [ ] Replay the Juan test: `./bin/supervisible assignments add --user-id 019404f3-... --project-id 019e1cde-... --date 2026-05-24 --hours 2 --auto-capability --dry-run` picks `0194b2e1-b918-7447-a88e-a85ccdea5634` (Web Dev) and warns about the Sabbatical overlap
- [ ] `./bin/supervisible assignments add --hours -100` against a row with 2h existing fails with a non-write error

## Dependencies & Risks

### Dependencies

- No new external Go modules (everything builds on existing `cobra`, `api`, `output`, `schema` packages).
- **Soft dependency on prior plan's Unit 1** (stderr discipline) for warning routing. If prior plan ships first, this plan's warnings automatically route correctly. If not, this plan uses `p.err` directly via a `PrintWarning` helper.

### Risks

- **TOCTOU race in `assignments add`.**
  - Mitigation: Document in command help. The CLI is single-actor in practice; acceptable.
- **Pre-flight time-off check makes dry-run slower** (extra API call per user).
  - Mitigation: Batch by user (one query covers all items for the same user). Worst case: dry-run takes 1-2s longer.
- **Auto-capability resolves to wrong capability if a user historically did different work on the same project.**
  - Mitigation: Always print the resolved capability to stderr so the human/agent can spot a mismatch. Failure mode is visible, not silent.
- **Schema-based unknown-param warning lists wrong params if schema is out of date.**
  - Mitigation: Schema is regenerated alongside the API contract. Warn-only (not block) keeps this from being a hard failure.

## Performance Considerations

- Unit 5's resolver issues one `GET /assignments` per `(user, project)`. Worst case for a multi-item upsert: N additional calls. Cache per-invocation so duplicates collapse.
- Unit 7's time-off pre-flight: one `GET /time-off` per distinct user across all dry-run items. Bounded by item count.
- All read calls already use `limit=50` or similar bounds. No new pagination work needed.

## Security Considerations

- No new auth surfaces. All new commands route through `App.RequireClient()` which already validates the API key.
- Pre-flight time-off check reveals approved time-off for users you query — same scope as existing `time-off list`. No new info exposure.
- Auto-capability resolution reads assignments — same scope as existing `assignments list`. No escalation.

## References

- Findings doc: `.thoughts/findings/2026-05-19-cli-real-world-testing.md`
- Prior plan (CLI ergonomics, depends on Unit 1): `.thoughts/plans/2026-05-19-feat-cli-agent-experience.md`
- Parent vision: `CLAUDE-CODE-TASK.md`, `PLAN-AI-NATIVE.md` (in main Supervisible repo)
- Whois implementation: `internal/cmd/whois.go:16-148`
- Upsert implementation: `internal/cmd/assignments.go:101-198`
- JSON encoder calls: `internal/output/output.go:30, 39`
- Schema describe: `internal/cmd/schema.go`

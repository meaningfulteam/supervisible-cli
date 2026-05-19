---
type: plan
status: active
tags: [cli, agent, ergonomics, auth, errors, structure, writes, whois, assignments, safety]
created: 2026-05-19
supersedes: .thoughts/plans/2026-05-19-feat-agent-safe-writes.md
related_findings: .thoughts/findings/2026-05-19-cli-real-world-testing.md
---

# feat(cli): Agent-grade CLI — ergonomics, structure, and safe writes

## Overview

Two bodies of work, one PR series, one narrative. **Phase 1 (Foundation)** applies four Notion-CLI principles (progressive disclosure, actionable errors, stdout/stderr separation, interactive vs non-interactive) plus a DHH-pragmatic structural cleanup. **Phase 2 (Agent-safe writes)** adds the compound resolvers, pre-flight checks, and output hygiene fixes that real-world testing surfaced (findings doc: `.thoughts/findings/2026-05-19-cli-real-world-testing.md`).

Thirteen tightly-scoped units, ordered so each phase compounds the next: Phase 1 establishes the output/error/structural contract; Phase 2 inherits it (Phase 2's warnings route through Phase 1's `Aux`; Phase 2's new `assignments add` slots into Phase 1's `App.Execute`; Phase 2's failures render through Phase 1's `FormatCLIError`). Ship Phase 1 first as one PR series, then Phase 2 — but they're one plan because they touch the same files and reasoning about them together prevents drift.

## Current State Analysis

### Foundation gaps (Phase 1)

- **stdout/stderr plumbing exists but is misused.** `internal/output/output.go:14` defines `Printer` with separate `out`/`err` writers, but `PrintMessage` (`output.go:48`) writes to `p.out`. Every status/info line in the codebase goes to stdout — e.g. `auth login` prints `"Authentication successful"` to stdout (`auth.go:100-103`), which corrupts any future shell piping of human-readable output.
- **One critical PrintMessage caller is actually data, not status.** `auth.go:214` prints the raw token via `PrintMessage("%s", token)` — that's the data channel by design. The refactor must distinguish "auxiliary" from "data" callers rather than blindly rerouting.
- **`Printer.PrintJSON` uses `json.NewEncoder` defaults** (`output.go:30, 39`) which HTML-escape `&` to `&amp;`. Visible in every `--json` output that includes ampersands. (Originally tracked as Phase 2; folded into Phase 1 because it lives in the same file rewrite.)
- **Errors are opaque.** `APIError.Error()` (`internal/api/client.go:72-77`) returns `"api error (404 not_found): <body> [request_id=...]"` and `main.go` (`cmd/supervisible/main.go:11`) prints that verbatim to stderr. There's no per-status-code hint, no next-step suggestion, no separation of message vs request_id.
- **Cobra commands have no `Example:` field.** `grep -c "Example:" internal/cmd/*.go` returns 0 across all 23 files. `Short:`/`Long:` are present but sparse.
- **`golang.org/x/term v0.35.0` is already a dependency** (`go.mod:18`, used in `auth.go` for `ReadPassword`). TTY detection is one helper away.
- **Auth login already adapts to input mode** (`auth.go:50-66`) — flag, then stdin, then interactive password prompt. But if stdin is a closed pipe and no flag is passed, `term.ReadPassword` will hang or fail with a cryptic error instead of guiding the agent.
- **`SilenceUsage: true, SilenceErrors: true`** is set on root (`root.go:174-175`), so cobra's default usage-on-error is off. Missing-arg errors fall through to `main.go` as a one-liner like `Error: requires at least 1 arg(s), only received 0`.
- **`api.Client` has 18 typed methods (`ListUsers`, `CreateProject`, `UpdateClient`, …) but commands call `client.Do(...)` directly.** Verified: only `Me`, `DeleteActualHour`, `DeleteAssignment` are still called from cmd code (`grep -rn 'client\.[A-Z]' internal/cmd/`). The rest is dead weight that drifts as endpoints change.
- **Every command repeats ~12 lines of boilerplate.** `appFromCommand → RequireClient → ResolvedQuery → MaybeDryRun → client.Do → PrintData/Table`. 13 leaf commands × 12 lines = ~150 lines of pure copy-paste, and the shape silently encourages "just one more variant" drift.
- **Four near-identical pointer helpers** in `internal/cmd/helpers.go:14-32` (`stringPtr`, `intPtr`, `float64Ptr`, `boolPtr`) that Go 1.18+ generics collapse to one.
- **`PrintMessage` is the wrong name.** It reads like "Print" (stdout); the plan needs it to mean "auxiliary text to stderr". Names that lie are the actual bug we're chasing.
- **No `.thoughts/` or `.claude/` directory** in this repo — created by this plan.

### Agent-write gaps (Phase 2) — from real-world test, 2026-05-19

- **Capability resolution has no agent-safe path.** Test run picked the wrong `capabilityId` because the only heuristic available ("any cap ever used on this project") doesn't match the user's actual role. The correct heuristic is "most recent cap **this user** used on **this project**". `internal/cmd/whois.go:117-127` already fetches assignments by user — extending this is straightforward.
- **`assignments upsert` is a replace, not an add.** Natural-language requests like "2h más" require read-modify-write that every agent has to reimplement. Upsert flow at `internal/cmd/assignments.go:101-198`.
- **`hours: 0` upsert is currently used as pseudo-delete** (no `DELETE /assignments/{id}` exists) and creates zombie rows that `whois` then displays. `internal/cmd/whois.go:18-22` defines `WhoisAssignment` — a one-line filter on `Hours > 0` cleans this up at the CLI boundary while server fix is pending.
- **`whois.assignments[]` returns only `project` name, `date`, `hours`** (`whois.go:16-22`) — no IDs, so you can't modify what you just read.
- **`schema describe` accepts only full op IDs** like `assignments.get`, not the noun-form `assignments` shown by `schema endpoints`. Agent first guess fails.
- **`--params` accepts unknown keys silently** — no warning when an agent types `startDate` instead of `start_date`. Filters work, but the typo is silently lost.
- **Assignments can be written that overlap approved time-off** with no warning. Juan's sabbatical (May 10 – Jul 3) was bypassed silently when we wrote 6 rows in that window.

## Requirements Trace

### Phase 1 — Foundation

- **R1.** Auxiliary output (status, progress, hints, request metadata) MUST go to stderr; data output (JSON, tables, raw tokens) MUST go to stdout. Piping `--json` through `jq` must work for every command. JSON output MUST NOT HTML-escape ampersands.
- **R2.** API errors MUST surface an actionable next step based on HTTP status code (401/403/404/422/429/5xx) and MUST print `request_id` on stderr when available.
- **R3.** A `--verbose` flag (and `SUPERVISIBLE_DEBUG=1` env var) MUST dump the full HTTP request (method, URL, query, sanitized headers, body) and response (status, headers, body) to stderr for self-diagnosis by humans and agents.
- **R4.** When stdin is not a TTY and an interactive prompt would be required, the command MUST fail fast with a non-interactive-friendly hint rather than blocking on a hung prompt.
- **R5.** Every leaf command MUST have a cobra `Example:` block; missing-arg errors MUST print usage + example.

### Phase 2 — Agent-safe writes

- **R6.** `whois.assignments[]` MUST omit zero-hour rows and MUST include `id`, `projectId`, `capabilityId` so the output is actionable.
- **R7.** `schema describe <noun>` MUST accept both the noun form (`assignments`) and the full op ID (`assignments.get`); unknown noun MUST list valid alternatives.
- **R8.** When a `--params` key isn't recognized for the target endpoint, the CLI MUST emit a warning to stderr (non-blocking; server is source of truth).
- **R9.** `whois` MUST accept `--weeks N` (default 1) so a single call covers multi-week planning.
- **R10.** `assignments upsert` MUST accept `--auto-capability`, which fills missing `capabilityId` per (user, project) using "most recent capability **this user** used on this project". MUST fail loudly when no history exists.
- **R11.** A new compound `assignments add` MUST do read-modify-write (current hours + delta, by `(user, project, capability, date)`), using `--auto-capability` semantics by default.
- **R12.** When `--dry-run` is used on `assignments upsert` or `assignments add`, the CLI MUST surface any items whose `(user, date)` overlaps approved time-off, as warnings on stderr.

## Desired End State

### After Phase 1

- `supervisible auth login --api-key sv_live_xxx --json | jq .` produces a clean JSON document (no "Authentication successful" preamble in the JSON stream, no `&amp;` for ampersands).
- A 404 from `supervisible projects list` (e.g. when an API key has no project scope) prints to stderr:
  ```
  Error: not found — verify the resource ID, or it may not be shared with this API key.
  Request ID: 01H...   (for support)
  Run with --verbose to see the full request.
  ```
- `SUPERVISIBLE_DEBUG=1 supervisible capacity --week 2026-W21` prints the GET URL, query params, response status + `x-request-id` header, and the unparsed body to stderr; the table or JSON still lands on stdout.
- `echo "" | supervisible auth login` (no `--api-key`, no `--stdin`, piped stdin) exits with `Error: API key required. Use --api-key sv_live_..., --from-stdin, or run interactively.`
- `supervisible whois` (no arg) prints the cobra Usage block plus an example: `Example: supervisible whois alberto@m8l.com --json`.
- A new leaf command takes ~25 lines (was ~50): parse flags → `app.Execute(...)` → render. Dead `api.Client` typed methods are gone; one generic `ptr[T]` replaces the four pointer helpers.

### After Phase 2

```bash
# Natural-language Slack request: "2h más para Juan en Odyssey esta semana"
# Becomes a one-line, capability-safe, conflict-aware write:
supervisible assignments add \
  --user-id 019404f3-... --project-id 019e1cde-... \
  --date 2026-05-24 --hours 2 --auto-capability --dry-run

# Stderr (warnings, all routed through Aux from Phase 1):
#   ⚠ Time-off overlap: Juan Méndez has approved Sabbatical 2026-05-10 → 2026-07-03
#   capability resolved: Web Dev (0194b2e1-b918-7447-a88e-a85ccdea5634) — most recent on this project
#
# Stdout (dry-run plan, JSON-clean, ampersands literal):
#   { method: POST, endpoint: /assignments, body: { items: [{...hours: 4 (current 2 + delta 2)}] } }
```

`supervisible whois "Juan Méndez" --weeks 4 --json` returns 4 weeks of assignments + time-off, each assignment carrying `id`, `projectId`, `capabilityId`, and no zero-hour rows. `schema describe assignments` shows both `assignments.get` and `assignments.post`. `--params '{"startDate":"..."}'` (camelCase typo) warns to stderr.

## What We're NOT Doing

### Phase 1

- **Device-code OAuth login.** Notion's `--device` flow requires server-side support (device authorization endpoint + polling endpoint) — out of scope for a CLI-only plan. Track separately if/when the public API adds it.
- **Spinner / color / progress bars.** Not currently present; not adding them under this plan.
- **Restructuring the cobra command tree.** The noun-verb hierarchy is already correct.
- **Rewriting `schema` or `--dry-run`.** Already work; left alone (Phase 2 extends `schema describe`, doesn't rewrite).
- **Adding `request_id` retrieval to non-`*APIError` failures** (e.g. network errors). Only `APIError` carries it; other failures stay as-is.

### Phase 2

- **Server-side endpoints** (`GET /capabilities`, `DELETE /assignments/{id}`, time-off enforcement). Tracked separately in the main Supervisible repo:

  | Server ask | Source finding | Why blocking |
  |---|---|---|
  | `GET /capabilities` endpoint | F3 | Without it, the CLI can't name-resolve capabilities |
  | `DELETE /assignments/{id}` (or `hours:0` = delete) | F12 | Currently zombie rows accumulate |
  | Soft-warn at write time on time-off overlap | F4 | CLI pre-flight is best-effort; server is source of truth |
  | `expand=project` default on POST response | F9 | Readable success output |
  | Reject unknown query params | F10 (server side) | Currently silent accept |

- **Transactional batched writes.** `assignments add` does read-then-write client-side; a server-side `PATCH /assignments` with diff semantics would be cleaner but is out of scope.
- **Interactive confirmation prompts on writes.** Phase 1's TTY guard establishes the pattern; not adding `Are you sure?` prompts here.
- **Refactoring `whois` into a generic "person profile" command.** Keep its scope as-is, just extend the time window.

## Stakeholder Impact

- **Agents (Claude, OpenClaw, Slack bot)** — biggest win across both phases. Phase 1: error responses become self-explanatory, `--verbose` enables curl-style introspection, piping `--json | jq` becomes reliable. Phase 2: writes go from footgun → safe; `assignments add` + `--auto-capability` together make "2h más" a one-liner; pre-flight conflict warnings prevent silent overbooking.
- **Human developers** — identical UX on the human-readable paths (auxiliary text still shows in terminal because stderr renders inline), plus actionable error guidance. Cleaner `--json` output for piping. `whois --weeks 4` saves several manual `assignments list` calls.
- **Future contributors** — After Phase 1 Unit 6, the shape of a new leaf command is obvious: `parse flags → app.Execute → render`. Onboarding burden drops; new endpoints take 20 lines instead of 50.
- **Supervisible product team** — Phase 2's server-side ask list has concrete justification from real testing rather than speculation.
- **Scripts/CI** — `--json | jq` becomes safe; exit codes unchanged.
- **No API server changes required to ship either phase.** Phase 2 prepares the ground for server-side changes but doesn't depend on them.

## Key Technical Decisions

### Phase 1

- **Rename `PrintMessage` → `Aux`; rename `Print` → `Data`; drop `PrintRaw`.** The plan's whole thesis is "stdout = data, stderr = aux". Naming the methods after the *intent* (`Aux`, `Data`) makes wrong calls visually obvious in PR review and removes the "is this stdout or stderr?" cognitive tax. ~50 call sites is a single mechanical rename — IDE/`gofmt -r` handles it. Sticking with `PrintMessage` for ergonomics reasons would be the opposite of the discipline we're trying to establish.
- **Error wrapper lives at the entry point, not the API client.** `*APIError` stays as-is (machine-readable for callers that want the code/status). The friendly formatting happens in `cmd/supervisible/main.go` via a `FormatError(err) string` helper, so the API package stays free of presentation concerns.
- **`--verbose` is a global persistent flag** (alongside `--json`, `--dry-run`), implemented via a debug hook on `*http.Client.Transport` (`http.RoundTripper`). Single point of injection in `api.NewClient`; works for every endpoint automatically.
- **TTY check is a free function**, not a Printer method, so it can be called from `auth.go` and any future interactive command without coupling to Printer state.

### Phase 2

- **Auto-capability heuristic is "most recent hours > 0 row for (user, project)".** Skip zombie rows. If no history exists, fail with a clear message — never guess from project-level capabilities (that's the bug this fixes).
- **`assignments add` is implemented client-side.** Read existing assignment(s) for `(user, project, capability, date)`, sum with delta, upsert the new total. Race condition with concurrent writers is acceptable — we're a CLI, not a multi-master DB. Document the trade-off.
- **Pre-flight time-off check fires on `--dry-run` only.** Without server enforcement, running it on every write would double API calls for non-dry-run paths. Dry-run is the agent's "did I get this right?" moment — best place for the warning.
- **`whois --weeks N` extends the window but keeps the single-week summary.** Returns `assignments[]` for all N weeks but `weekSummary` stays scoped to the current week to preserve existing semantics.
- **`SetEscapeHTML(false)` is safe** because the CLI prints to terminals/pipes, not HTML contexts. Standard practice for CLI tools. Folded into Phase 1 Unit 1 (same file).
- **Warning prefix is `warning:`** (lowercase, no glyph), matching Go convention. Established in Phase 1 Unit 1's `Aux` documentation and inherited by Phase 2 Units 10 and 13.

### Cross-cutting

- **Plan file lives in this repo, not the Next.js app.** This is a separate Go repo with its own lifecycle. The `.thoughts/` directory is created under `/Users/aesadde/repos/supervisible-cli/`.
- **Phase 2 inherits Phase 1's contracts.** No fallback `PrintWarning` helper; Phase 2 writes through `Aux`. No bypass for `App.Execute`; Phase 2's new compound goes through it.

### Codebase Conventions (reinforced + new)

DHH-pragmatic: be opinionated, kill copy-paste, delete dead code, prefer one obvious way. The shape commands should follow after Phase 1 Unit 6 lands:

- **One package per layer, flat.** `internal/cmd/` holds *all* cobra commands (don't nest by noun). `internal/api/` is the HTTP layer. `internal/output/` owns presentation. `internal/config/`, `internal/inputs/`, `internal/validate/`, `internal/schema/`, `internal/version/` stay as-is. Resist the temptation to introduce a "service" layer — for a thin API CLI it adds folders and removes nothing.
- **One file per noun.** `internal/cmd/users.go` contains `list` + `update` for users, full stop. Mirrors API noun-verb. The compound commands (`capacity.go`, `bench.go`, `whois.go`, `context.go`) keep their own files because they orchestrate multiple endpoints — that's the only reason to split.
- **Commands are skinny: parse flags → call `App.Execute(...)` → render.** Business logic that spans multiple calls (e.g. `computeCapacity` in `capacity.go:53-148`, or Phase 2's `resolveCapability`) is fine *colocated* with its only caller. Don't extract a `internal/capacity/` package until a second caller appears.
- **Output API has three primitives.** `p.Data(v)` (stdout, JSON or formatted), `p.Aux(format, args)` (stderr, status/hints/dry-run/warnings), `p.Table(headers, rows)` (stdout). Anything else is a mistake — there is no `Print`, no `PrintMessage`, no `Println`, no `PrintWarning`. Contract documented at the top of `internal/output/output.go`.
- **`api.Client` is the HTTP transport, not a typed SDK.** `Do(ctx, method, endpoint, query, body, out)` is the public surface. Typed wrappers exist only where they earn their keep: `Me()` (called twice, with auth-specific context) and the two `Delete*` helpers. Unused typed methods get deleted in Phase 1 Unit 6.
- **No premature abstractions.** No interface for `Client` (only one impl, no mocking — tests use `httptest.Server`). No middleware framework (the one cross-cutting concern, `--verbose`, is a single `http.RoundTripper`). No flag-binding DSL (cobra's `Flags().StringVar` is fine).
- **Pointer helpers: one generic.** `ptr[T any](v T) *T` in `helpers.go` replaces the four `*Ptr` functions. The pattern `input.Foo = ptr(foo)` reads identically.

## Implementation Approach

Thirteen units across two phases. Phase 1 ships as one PR series; Phase 2 ships after Phase 1 merges. Each unit is independent within a phase unless noted; cross-phase dependencies are explicit.

```
PHASE 1 — Foundation
  Unit 1 (stderr discipline + rename + SetEscapeHTML) ─► no dependencies
  Unit 2 (error formatter)                            ─► no dependencies
  Unit 3 (verbose mode)                               ─► depends on Unit 1
  Unit 4 (TTY guard)                                  ─► no dependencies
  Unit 5 (examples + usage)                           ─► depends on Unit 2
  Unit 6 (pragmatic structural cleanup)               ─► depends on Unit 1

PHASE 2 — Agent-safe writes
  Unit 7  (whois output hygiene)                      ─► depends on Phase 1 (Aux contract)
  Unit 8  (whois --weeks N)                           ─► no deps (independent)
  Unit 9  (schema describe noun form)                 ─► no deps
  Unit 10 (warn on unknown --params keys)             ─► depends on Phase 1 Unit 1 (Aux on stderr)
  Unit 11 (auto-capability resolver)                  ─► no deps (building block)
  Unit 12 (assignments upsert --auto-cap + add)       ─► depends on Units 6, 11
  Unit 13 (time-off pre-flight in dry-run)            ─► depends on Units 11, 12
```

Phase 1 sequencing: Units 1–2 are the highest-leverage agent-facing changes; ship them first and the CLI is dramatically better even without 3–6. Unit 6 is deliberately last in Phase 1 because it benefits from the rename + new conventions landing first — and is the kind of "now that we're touching it, tidy" pass that DHH would do *in the same PR series* rather than parking for "later".

Phase 2 sequencing: 7–10 are quick wins shippable in any order. 11 is a building block for 12/13. 12 is the headline compound command. 13 is the safety net that closes the agent-write loop.

## Implementation Units

### Phase 1 — Foundation

- [x] **Unit 1: stderr/stdout discipline + Printer rename + SetEscapeHTML**

  **Goal:** Auxiliary output (status, hints, dry-run preview, warnings) goes to stderr; data output (JSON, tables, raw values) goes to stdout. Method names communicate intent. JSON output stops HTML-escaping ampersands. Piping `--json` through `jq` works for every command.
  **Requirements:** R1
  **Dependencies:** None
  **Files:**
  - Modify: `internal/output/output.go` (rename + writer flip + `SetEscapeHTML(false)` + doc comment)
  - Modify: every file under `internal/cmd/` and `cmd/supervisible/` that calls `Printer` methods (~16 files, mechanical)
  - Modify: `internal/cmd/auth.go` (raw token output uses new `Data` method)
  - Modify: `internal/output/output_test.go` (test under new names + ampersand assertion)

  **Approach:**
  1. In `output.go`, define the final shape:
     ```go
     // Output contract:
     //   stdout = data (JSON, tables, raw values that callers parse or display as result)
     //   stderr = auxiliary (status, hints, dry-run previews, warnings, error messages)
     // Tests rely on this. Don't add a method that violates it.
     // Convention: warning lines start with "warning: " (lowercase, no glyph).

     // Data writes a value to stdout. JSON-encoded (with SetEscapeHTML(false))
     // when --json is set, otherwise %v + newline. Use this for command results.
     func (p *Printer) Data(value any) error { ... }

     // Aux writes auxiliary text to stderr (Printf semantics + newline).
     // Use this for status, hints, dry-run previews, warnings, "Updated: <id>" lines.
     func (p *Printer) Aux(format string, args ...any) { ... }

     // Table writes a tab-aligned table to stdout. Part of the data view.
     func (p *Printer) Table(headers []string, rows [][]string) error { ... }
     ```
     `PrintJSON` merges into `Data` (it's just "JSON to stdout"). `PrintError` is removed in favor of `Aux` (errors are aux text, just routed through `FormatCLIError` in Unit 2). Both JSON encoder calls (current `output.go:30, 39`) get `enc.SetEscapeHTML(false)` immediately after `json.NewEncoder(...)`.
  2. Run a mechanical rename across the repo:
     - `PrintMessage(` → `Aux(`
     - `.Print(` → `.Data(` (for `Printer.Print`, not arbitrary `Print*`)
     - `PrintError(` → `Aux(`
     - `PrintJSON(` → `Data(` (semantically equivalent after merge)
     Use `gopls` rename or `sed -i '' 's/\.PrintMessage(/\.Aux(/g' internal/**/*.go cmd/**/*.go`. Verify with `go build ./...`.
  3. Fix `auth.go:214` (token print) to use `Data(token)` — that's the data channel by design.
  4. Audit `printCapacityTable` (`capacity.go:264-265`) and `printWhoisProfile` — table headers that are part of the human-readable data view must stay on stdout. Use `fmt.Fprintln(p.Stdout(), ...)` directly, or introduce a thin `Headerln(s string)` on Printer if it appears in 3+ places. (Expose `Stdout() io.Writer` on Printer if needed.)
  5. Confirm the cobra `MaybeDryRun` text branch (`root.go:154-164`) ends up on stderr after rename — that's correct: dry-run preview is auxiliary.

  **Execution note:** This is the single largest mechanical diff in the plan, but it's all rename. Run `go build ./... && go test ./...` after step 2; failures will be table-rendering tests that asserted against stderr. Fix in step 4. Keep the rename in one commit so `git blame` stays readable.

  **Test scenarios:**
  - `Printer.Aux` writes to the err writer (assert via separate `bytes.Buffer`s for out/err); `Printer.Data` writes to out.
  - `Printer.Data("plain")` (non-JSON mode) emits `plain\n` to stdout, no JSON quotes.
  - `Printer.Data(struct{Name string}{Name: "A & B"})` in JSON mode emits literal `"A & B"`, not `&amp;`.
  - `printCapacityTable` produces both header + table on stdout — capture out only and assert both present.
  - `auth login --json` produces only valid JSON on stdout (no preamble).

  **Verification:** `go test ./...` green, plus manual: `./bin/supervisible auth status --json 2>/dev/null | jq .` returns a JSON object (and only a JSON object). `grep -rn 'PrintMessage\|PrintError\|PrintJSON\b' internal/ cmd/` returns nothing.

- [x] **Unit 2: Actionable error formatter**

  **Goal:** API errors print a status-aware next-step hint and surface `request_id` separately. Errors stay machine-readable inside Go but become human-actionable at the boundary.
  **Requirements:** R2
  **Dependencies:** None
  **Files:**
  - Create: `internal/output/errors.go`
  - Create: `internal/output/errors_test.go`
  - Modify: `cmd/supervisible/main.go`

  **Approach:**
  1. New `output.FormatCLIError(err error) string` in `errors.go`. Behavior:
     - If `errors.As(err, &apiErr)` matches, build a multi-line message:
       - Line 1: `Error: <hint based on apiErr.StatusCode>` (fall back to `apiErr.Message` if no hint mapped).
       - Line 2 (if RequestID): `Request ID: <id>` — for support copy/paste.
       - Line 3 (if not 401): `Run with --verbose to see the full request.`
     - Otherwise: `Error: <err.Error()>`.
  2. Status-code hint table:
     - 401 → `"Authentication failed. Run: supervisible auth login --api-key sv_live_..."`
     - 403 → `"Forbidden. Your API key doesn't have access to this resource. Check key scopes with 'supervisible auth status --verify'."`
     - 404 → `"Not found — verify the resource ID, or it may not be shared with this API key."`
     - 409 → `"Conflict. The resource state changed; refetch and retry."`
     - 422 → `"Validation failed: <apiErr.Message>. Try --dry-run to inspect the payload."`
     - 429 → `"Rate limited. Retry after a few seconds."`
     - 5xx → `"Server error (<status>). Try again; contact support with the request ID if persistent."`
  3. In `main.go`, replace `fmt.Fprintln(os.Stderr, err)` with `fmt.Fprintln(os.Stderr, output.FormatCLIError(err))`.

  **Test scenarios:**
  - Each status code (401/403/404/409/422/429/500) produces the expected hint string.
  - `RequestID` is rendered when present, omitted when empty.
  - A wrapped `*APIError` (`fmt.Errorf("foo: %w", apiErr)`) is still detected via `errors.As`.
  - A non-`*APIError` (`errors.New("network down")`) falls through to plain `Error: network down`.

  **Verification:** With the dev server stopped, `./bin/supervisible me` produces a friendly hint, not a Go-style error chain. `go test ./internal/output/...` green.

- [x] **Unit 3: Verbose / debug mode**

  **Goal:** `--verbose` (or `SUPERVISIBLE_DEBUG=1`) dumps full HTTP request + response to stderr so humans and agents can self-diagnose without us shipping a separate debug build.
  **Requirements:** R3
  **Dependencies:** Unit 1 (writes go to stderr — must be wired correctly first).
  **Files:**
  - Modify: `internal/api/client.go` (add `Verbose bool` field, debug RoundTripper)
  - Modify: `internal/cmd/root.go` (add `--verbose` persistent flag + env var read, pass to `App` + `api.NewClient`)
  - Modify: `internal/cmd/root.go` `RequireClient` to thread `verbose`
  - Test: `internal/api/client_test.go` (add if missing) — verify dump fires only when enabled, headers sanitized

  **Approach:**
  1. Add `Verbose bool` and `DebugOut io.Writer` to `api.NewClient` options (extend signature or add `NewClientWithOptions`). Default `DebugOut = io.Discard`.
  2. Wrap `httpClient.Transport` with a `debugRoundTripper` that, when verbose:
     - Before `RoundTrip`: dumps method, URL, sanitized headers (mask `Authorization`), and body to `DebugOut`.
     - After: dumps response status, headers (including `x-request-id`), and body. Restores body for downstream readers via `bytes.NewReader`.
  3. In `root.go`, add `--verbose` persistent flag, read `SUPERVISIBLE_DEBUG` env var (truthy = `1|true|yes`), OR the two. Store on `App.verbose`.
  4. `App.RequireClient()` passes `verbose=true, DebugOut=p.err` to `api.NewClient`.

  **Execution note:** Mask the `Authorization` header in the dump — never log the bearer token. Use `output.MaskToken` for consistency.

  **Test scenarios:**
  - Verbose off → no dump output.
  - Verbose on → request line, `Authorization: Bearer sv_li...xxxx` (masked), response status, `x-request-id` all appear on the debug writer.
  - Response body is not consumed by the dumper (downstream parser still works).
  - `SUPERVISIBLE_DEBUG=1` alone (no flag) activates verbose.

  **Verification:** `SUPERVISIBLE_DEBUG=1 ./bin/supervisible me 2>verbose.log >data.json` produces a clean `data.json` and a request/response transcript in `verbose.log`.

- [x] **Unit 4: TTY detection + non-interactive guard**

  **Goal:** When stdin is not a TTY and a command would require an interactive prompt, fail fast with a scripting-friendly hint. Lays groundwork for future commands that need confirmation.
  **Requirements:** R4
  **Dependencies:** None
  **Files:**
  - Create: `internal/cmd/tty.go`
  - Modify: `internal/cmd/auth.go` (`newAuthLoginCommand`)
  - Test: `internal/cmd/tty_test.go`

  **Approach:**
  1. In `tty.go`, add:
     ```go
     func isStdinInteractive() bool {
         return term.IsTerminal(int(os.Stdin.Fd()))
     }
     func isStdoutInteractive() bool {
         return term.IsTerminal(int(os.Stdout.Fd()))
     }
     ```
  2. In `auth login`'s `RunE`, before the `term.ReadPassword` fallback (`auth.go:60-66`), check `isStdinInteractive()`. If false, return:
     ```
     api key required. Use --api-key sv_live_..., pipe via --from-stdin, or run interactively in a terminal.
     ```
  3. Keep the existing interactive password prompt for the TTY case.

  **Test scenarios:**
  - `auth login` with no flags + non-TTY stdin → returns the friendly error.
  - `auth login --api-key X` → unaffected (skips the TTY check).
  - `auth login --from-stdin` with piped stdin → unaffected.
  - Helpers can be called concurrently without panic.

  **Verification:** `echo "" | ./bin/supervisible auth login` exits non-zero with the hint string, instead of hanging or printing `inappropriate ioctl for device`.

- [x] **Unit 5: `Example:` on every command + better missing-arg usage**

  **Goal:** Every leaf command has a copy-pasteable example in `--help`. When a required arg is missing, the error includes usage + example (the Notion `ntn worker sync trigger` pattern).
  **Requirements:** R5
  **Dependencies:** Unit 2 (error formatter — missing-arg errors will be friendlier when funneled through it).
  **Files:**
  - Modify: `internal/cmd/auth.go` (login, status, token, logout)
  - Modify: `internal/cmd/me.go`
  - Modify: `internal/cmd/users.go` (list, update)
  - Modify: `internal/cmd/clients.go` (list, create, update)
  - Modify: `internal/cmd/projects.go` (list, create, update)
  - Modify: `internal/cmd/assignments.go` (list, upsert)
  - Modify: `internal/cmd/actual_hours.go` (list, upsert, delete)
  - Modify: `internal/cmd/time_off.go` (list, create, update, delete, approve, reject)
  - Modify: `internal/cmd/capacity.go`, `bench.go`, `whois.go`, `context.go`
  - Modify: `internal/cmd/schema.go` (endpoints, describe)
  - Modify: `internal/cmd/config.go` (show, set-base-url)

  **Approach:**
  1. For every leaf cobra command, set `Example:` with at least one realistic invocation. Example shape:
     ```go
     Example: `  # Show capacity for the current week
       supervisible capacity

       # Specific week in JSON for agents
       supervisible capacity --week 2026-W21 --json`,
     ```
  2. Leave parent commands (`auth`, `users`, `time-off`) with `Short:` only — `--help` will list subcommands.
  3. For commands with `cobra.ExactArgs(N)` or `MinimumNArgs(N)`, override missing-arg behavior: set `SilenceUsage: false` on those specific subcommands so cobra prints usage. The error message stays one line; usage is printed below.
  4. Verify by running `./bin/supervisible <cmd> --help` for each: example block visible, formatting clean.

  **Execution note:** Pure additive change — no behavior modification beyond cobra's built-in usage rendering on missing args. Low risk.

  **Test scenarios:**
  - `supervisible whois --help` output contains "Example:".
  - `supervisible whois` (no arg) produces usage + example in stderr.
  - JSON output of any command is unchanged (examples only appear in help text).

  **Verification:** `./bin/supervisible help <cmd>` for each command shows an example; `./bin/supervisible whois` (no args) shows usage.

- [x] **Unit 6: Pragmatic structural cleanup (DHH pass)**

  **Goal:** Pay down structural debt that Units 1–5 expose. Three small, mechanical refactors that compound: kill command boilerplate, delete dead code, collapse pointer helpers. No new abstractions; no new packages.
  **Requirements:** Codebase Conventions section (this plan)
  **Dependencies:** Unit 1 (works with renamed Printer API).
  **Files:**
  - Modify: `internal/cmd/root.go` (add `App.Execute`)
  - Modify: every leaf command file in `internal/cmd/` that follows the boilerplate pattern (users, projects, clients, assignments, actual_hours, time_off, me, schema) — call sites collapse to `app.Execute(...)`
  - Modify: `internal/api/client.go` (delete unused typed methods)
  - Modify: `internal/cmd/helpers.go` (replace 4 pointer helpers with one generic `ptr`)
  - Modify: every file using `stringPtr/intPtr/float64Ptr/boolPtr` (mechanical rename to `ptr`)
  - Test: `internal/cmd/run_test.go` (new — covers `App.Execute` dry-run + real-call paths)

  **Approach:**

  **6a. Consolidate the command runner.** Add a single helper on `App` that absorbs the four steps every command repeats. Signature:

  ```go
  // ExecuteOpts is the per-call configuration for App.Execute.
  type ExecuteOpts struct {
      CommandPath string         // e.g. "projects update"
      Method      string         // http.MethodGet, etc.
      Endpoint    string         // schema endpoint pattern, e.g. "/users/{user_id}"
      Path        string         // actual request path, e.g. "/users/abc-123" (defaults to Endpoint if empty)
      Query       url.Values     // base query before ResolvedQuery merges --fields/--expand/--params
      Body        any            // request body (nil for GETs)
      Out         any            // pointer for JSON decode; nil if caller renders nothing on success
  }

  // Execute runs the standard "resolve query → dry-run → require client → call API" flow.
  // Returns (executed bool, err error): executed=false means dry-run printed the plan; caller skips rendering.
  func (a *App) Execute(ctx context.Context, opts ExecuteOpts) (bool, error) { ... }
  ```

  Implementation is ~30 lines: clone defaults, resolve query, build `RequestPlan`, call `MaybeDryRun`, `RequireClient`, `client.Do`. Every leaf command then shrinks from ~50 lines to ~25:

  ```go
  // Before
  query := app.ResolvedQuery("PATCH", "/users/{user_id}", nil)
  plan := RequestPlan{CommandPath: "users update", Method: "PATCH", Endpoint: "/users/"+args[0], Query: query, Body: body, RequiredScope: app.RequiredScope(...)}
  if app.MaybeDryRun(plan) { return nil }
  client, err := app.RequireClient(); if err != nil { return err }
  var user api.User
  if err := client.Do(cmd.Context(), "PATCH", "/users/"+args[0], query, body, &user); err != nil { return err }

  // After
  var user api.User
  executed, err := app.Execute(cmd.Context(), ExecuteOpts{
      CommandPath: "users update", Method: "PATCH",
      Endpoint: "/users/{user_id}", Path: "/users/"+args[0],
      Body: body, Out: &user,
  })
  if err != nil { return err }
  if !executed { return nil }
  ```

  Migrate file-by-file. Don't migrate `capacity.go`/`bench.go`/`whois.go`/`context.go` — they orchestrate multiple endpoints and don't fit the one-call shape.

  **6b. Prune dead `api.Client` typed methods.** Verified by grep that only `Me`, `DeleteActualHour`, `DeleteAssignment` are called outside tests. Delete the rest (`ListUsers`, `UpdateUser`, `ListClients`, `CreateClient`, `UpdateClient`, `ListProjects`, `CreateProject`, `UpdateProject`, `ListAssignments`, `UpsertAssignments`, `ListActualHours`, `UpsertActualHours`, `ListTimeOff`, `CreateTimeOff`, `UpdateTimeOff`, `DeleteTimeOff`, `ApproveTimeOff`, `RejectTimeOff`) and the associated filter structs that exist only to serve them (`AssignmentFilters`, `ActualHourFilters`, `TimeOffFilters`, `paginationQuery` if unused). Verify with `go build ./... && go vet ./...`. Keep `Me` (auth-specific, called twice) and the two `Delete*` helpers. Document at the top of `client.go`: `// Public surface: NewClient, Client.Do. Typed helpers exist only where they earn their keep (Me, deletes).`

  **6c. Collapse pointer helpers.** Replace the 4-function block in `helpers.go:14-32`:

  ```go
  // ptr returns a pointer to v. Use for building optional input structs:
  //   input.Name = ptr(name)
  func ptr[T any](v T) *T { return &v }
  ```

  Mechanical rename across `internal/cmd/`: `stringPtr(` / `intPtr(` / `float64Ptr(` / `boolPtr(` → `ptr(`. `gopls` rename or sed. Delete the originals.

  **Execution note:** Each sub-step is independent — land them in order (6a, 6b, 6c) as separate commits inside the Unit 6 PR so review stays small and bisects work cleanly.

  **Test scenarios:**
  - `App.Execute` with `dryRun=true` returns `(false, nil)` and prints the plan via the printer (assert on `--dry-run` flag path).
  - `App.Execute` with `dryRun=false` calls `client.Do` exactly once with the resolved query (use an `httptest.Server` fixture).
  - `App.Execute` propagates `*api.APIError` unchanged (so Unit 2's formatter still gets the typed error).
  - `App.Execute` returns the `RequireClient` error when no API key is configured.
  - Existing tests (`compound_test.go`, `agent_flags_test.go`, `capacity_test.go`, etc.) still pass — the rename is type-checked, dead-method removal is verified by `go build`.

  **Verification:** `go build ./... && go vet ./... && go test ./...` green. `grep -rn 'client\.\(List\|Create\|Update\|Upsert\|Approve\|Reject\)' internal/cmd/` returns nothing. `grep -rn 'stringPtr\|intPtr\|float64Ptr\|boolPtr' internal/ cmd/` returns nothing. Diff stat shows net negative LOC in `internal/cmd/`.

### Phase 2 — Agent-safe writes

- [ ] **Unit 7: Whois output hygiene — zombie filter + actionable IDs**

  **Goal:** `whois.assignments[]` omits zero-hour rows and carries the IDs callers need to modify what they just read. (The companion `SetEscapeHTML` fix lives in Phase 1 Unit 1.)
  **Requirements:** R6
  **Dependencies:** Phase 1 Unit 1 (already shipped `SetEscapeHTML(false)` and the `Aux` contract).
  **Files:**
  - Modify: `internal/cmd/whois.go` (lines 16-22 — `WhoisAssignment` struct; lines 145+ — `buildWhoisReport`)
  - Modify: `internal/cmd/whois.go` printer (`printWhoisProfile`)
  - Test: `internal/cmd/whois_test.go` (extend)

  **Approach:**
  1. Extend `WhoisAssignment`:
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
  2. In `buildWhoisReport`, when iterating `assignments []api.Assignment`, skip entries where `a.Hours <= 0` and populate the new fields.
  3. Update `printWhoisProfile` if it renders per-row hours — it should also exclude zero-hour rows (currently aggregates by project name so this is probably a no-op).

  **Test scenarios:**
  - `buildWhoisReport` with one 0h assignment + one 2h assignment returns one assignment in the output.
  - JSON output of `whois` contains `id`, `projectId`, `capabilityId` on each assignment.

  **Verification:** `./bin/supervisible whois "Juan Méndez" --json | jq '.assignments[].hours' | grep -v 0` shows no zombie rows.

- [ ] **Unit 8: `whois --weeks N` flag**

  **Goal:** Extend `whois` to cover N weeks in one call. Default N=1 (current behavior).
  **Requirements:** R9
  **Dependencies:** None
  **Files:**
  - Modify: `internal/cmd/whois.go` (add `--weeks` flag, extend the assignments date window, update `WhoisReport` shape)
  - Test: `internal/cmd/whois_test.go` (extend)

  **Approach:**
  1. Add `--weeks N` flag (int, default 1, validate 1 ≤ N ≤ 12).
  2. In `RunE`, compute `weekEnd = weekStart + N*7 days - 1`. Pass to assignments fetch.
  3. `WhoisReport.WeekSummary` stays scoped to the current week (first of N) for backward compat. Add `WhoisReport.WeeksCovered int` to the JSON so consumers know the window.
  4. Update `printWhoisProfile` (table mode) to group by week when N > 1, else keep current single-week rendering.
  5. Update the `Example:` block on the command (set in Phase 1 Unit 5) to show `--weeks 4`.

  **Test scenarios:**
  - `--weeks 1` (default) returns exactly today's week assignments — no change in shape from current.
  - `--weeks 4` returns 4 weeks of assignments; `WeeksCovered: 4`.
  - `--weeks 0` rejects with a clear validation error.
  - `--weeks 13` rejects with a clear validation error (sanity cap).

  **Verification:** `./bin/supervisible whois "Juan Méndez" --weeks 4 --json | jq '.assignments | length'` returns more rows than `--weeks 1`.

- [ ] **Unit 9: `schema describe` accepts short noun form**

  **Goal:** `schema describe assignments` returns descriptions of `assignments.get` AND `assignments.post`. Unknown noun gives a "did you mean" hint.
  **Requirements:** R7
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

- [ ] **Unit 10: Warn on unknown `--params` keys**

  **Goal:** When an agent passes `--params '{"startDate":"..."}'` (camelCase typo) the CLI emits a stderr warning via `Aux` rather than silently accepting.
  **Requirements:** R8
  **Dependencies:** Phase 1 Unit 1 (uses `Aux` for warning output, `warning:` prefix convention).
  **Files:**
  - Modify: `internal/cmd/root.go` (`ResolvedQuery` or a new check in `PersistentPreRunE`)
  - Modify: `internal/schema/provider.go` (add `KnownQueryParams(method, endpoint) []string` if not present)
  - Test: `internal/schema/provider_test.go`

  **Approach:**
  1. Add `Provider.KnownQueryParams(method, endpoint) []string` returning the set of accepted query params for that operation (driven by the static schema).
  2. In `App.ResolvedQuery`, after merging user params, diff against known and warn for each unknown key via `app.Printer().Aux(...)`:
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

- [ ] **Unit 11: Auto-capability resolver helper**

  **Goal:** Centralize the "most recent capability this user used on this project" lookup as a reusable helper. Used by Units 12 and 13.
  **Requirements:** R10 (foundation)
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
  4. Cache results per `(userID, projectID)` in-process for the duration of a single command invocation (helps Unit 12 batch resolution).

  **Test scenarios:**
  - History exists → returns most recent capability.
  - Only zombie (0h) history → returns error.
  - No history → returns error with explicit message.
  - Two rows with same date → returns either deterministically (sort secondary by ID).

  **Verification:** Unit-tested only; verified end-to-end in Unit 12.

- [ ] **Unit 12: `assignments upsert --auto-capability` + `assignments add` compound**

  **Goal:** Two related additions, both built on Phase 1 Unit 6's `App.Execute`:
  - `--auto-capability` flag on existing `upsert` fills in `capabilityId` per item using Unit 11.
  - New `assignments add` compound does read-modify-write: existing hours + delta, idempotent if you specify the capability.
  **Requirements:** R10, R11
  **Dependencies:** Phase 1 Unit 6 (`App.Execute`), Unit 11 (resolver)
  **Files:**
  - Modify: `internal/cmd/assignments.go` (extend `newAssignmentsUpsertCommand`, add `newAssignmentsAddCommand`)
  - Modify: `internal/cmd/assignments.go` `newAssignmentsCommand` registration
  - Test: `internal/cmd/assignments_test.go` (extend)

  **Approach (upsert `--auto-capability`):**
  1. Add `--auto-capability` bool flag. When set, iterate items in the parsed payload; for any item missing `capabilityId`, call `resolveCapability` (Unit 11).
  2. Surface the resolution on stderr (one line per item) via `app.Printer().Aux(...)`: `capability resolved for <user>/<project>: <name? or id>`.
  3. If resolution fails for any item, exit non-zero with the combined list of failures (don't partial-write).

  **Approach (`assignments add` compound):**
  1. New cobra command `assignments add` with flags: `--user-id`, `--project-id`, `--date`, `--hours` (delta, can be negative), `--capability-id` (optional), `--auto-capability` (default true).
  2. Resolve `capabilityId` if not provided.
  3. Fetch existing assignment for `(user, project, capability, date)` directly via `client.Do` (read step) — this is a multi-call command so it doesn't fit `App.Execute`'s one-call shape, but it lives alongside the compound commands (capacity/bench/whois/context).
  4. Compute `new = (existing.hours or 0) + delta`.
  5. Reject if `new < 0` (don't allow phantom negative hours).
  6. Reject if `new == 0` and existing exists (until DELETE endpoint lands, document this as an intentional limit).
  7. Build the upsert payload and route the *write* through `App.Execute` (so `--dry-run` works identically and inherits everything Phase 1 gives us).
  8. Print one-line summary on stderr via `Aux`: `assignments add: <user> <project> <date> <existing>h + <delta>h = <new>h`.

  **Test scenarios:**
  - `add` with no existing row: new = delta.
  - `add` with existing row: new = existing + delta.
  - `add --auto-capability` with no history: fails with helpful message.
  - `add --hours -10` with existing 8h: fails (would go negative).
  - `add` in `--dry-run`: shows the computed `new` value, doesn't write.
  - `upsert --auto-capability` with a payload of 3 items: all three get resolved (or all three fail with combined error).

  **Verification:** Replay the original Juan test with `assignments add --auto-capability --dry-run` and confirm the right capability is picked.

- [ ] **Unit 13: Time-off conflict pre-flight in dry-run**

  **Goal:** When `--dry-run` runs on `assignments upsert` or `assignments add`, emit a stderr warning for each item whose `(user, date)` overlaps approved time-off.
  **Requirements:** R12
  **Dependencies:** Units 11, 12 (shares the dry-run code path through `App.Execute`)
  **Files:**
  - Modify: `internal/cmd/assignments.go` (extend dry-run branch in upsert/add)
  - Modify: `internal/cmd/root.go` `App.Execute` if pre-flight hook needs to be generic
  - Test: `internal/cmd/assignments_test.go`

  **Approach:**
  1. Before the dry-run plan prints, collect distinct `(userID, minDate, maxDate)` tuples from the items.
  2. For each unique userID, fetch `/time-off?user_id=X&status=approved&start_date=<minDate>&end_date=<maxDate>`.
  3. For each item, compare against returned time-off windows; emit a warning per overlap via `Aux`:
     ```
     warning: time-off overlap — <user name> has approved <type> 2026-05-10 → 2026-07-03 (item: project=<id> date=2026-05-24)
     ```
  4. Aggregate by user when many items overlap the same time-off entry (don't spam).
  5. Pre-flight failures (e.g. API down) emit a single warning: `warning: could not verify time-off (API unreachable); proceeding without check`. Don't block the dry-run.

  **Test scenarios:**
  - Item with no overlap → no warning.
  - Item inside approved time-off → one warning.
  - Multiple items same user, same time-off → one aggregated warning.
  - Time-off fetch fails → soft warning, dry-run continues.
  - Not in dry-run mode → no pre-flight (don't double the request rate on real writes).

  **Verification:** Replay the original Juan test — adding to Sabbatical window now produces a visible warning on stderr.

## Open Questions

### Resolved During Planning

**Phase 1:**

- **Should we move ALL `PrintMessage` callers to stderr?** No — `auth.go:214` is the token print, which IS the data. After the rename it becomes `Data(token)`; table headers in `printCapacityTable` / `printWhoisProfile` write to stdout via `fmt.Fprintln(p.Stdout(), ...)`.
- **Is renaming `PrintMessage` worth the diff churn?** Yes. A mechanical rename across ~50 call sites is one commit and pays for itself the first time a future contributor asks "wait, does this go to stdout or stderr?". DHH-pragmatic: names that lie are the bug.
- **Device-code login: include here?** No. Requires server-side endpoints. Track separately.
- **Plan file location.** `.thoughts/plans/` in the `supervisible-cli` repo (not the Next.js app). Each repo owns its own plans.

**Phase 2:**

- **Should `assignments add` accept multiple items via `--file`?** Yes — it should mirror `upsert`'s shape. For v1, support single-item add via flags; multi-item via `--file` does read-modify-write per item with a clear summary.
- **Auto-capability heuristic edge case: user is new to the project.** Fail loudly — `"no prior capability found"`. Don't silently fall back to a project-level guess.
- **What if the upsert response doesn't include capability names?** We don't have them anyway (no `GET /capabilities`). Show IDs in success output until server adds expansion.
- **Warning prefix format.** `warning: ` (lowercase, no glyph), established in Phase 1 Unit 1 and inherited by all Phase 2 warnings.
- **`--auto-capability` opt-in or default?** Opt-in for `upsert` (explicit, mutates many rows); default-on for `add` (convenience command, single row, easy to undo with another `add`).

### Deferred to Implementation

- **Exact `Example:` text per command.** Will be written during Phase 1 Unit 5 with reference to the README's command table for accuracy.
- **Whether `--verbose` masks query params** (e.g. `?api_key=`). Spot-check during Phase 1 Unit 3 — currently we send the token via `Authorization` header only, so query masking is likely unnecessary.

## System-Wide Impact

- **Interaction graph:** Every cobra command flows through `main.go`'s error printer (Phase 1 Unit 2 affects all error paths), every API call flows through `api.NewClient` (Phase 1 Unit 3 affects all requests), and after Phase 1 Unit 6 every leaf command also flows through `App.Execute` — a third single point of injection that Phase 2 (and any future cross-cutting concern: retries, telemetry, rate limiting) hooks into without re-touching every file. High-leverage by design.
- **Error propagation:** `*APIError` continues to bubble unchanged through cobra's `RunE` chain — only the final string rendering changes at `main.go`. `App.Execute` returns the typed error as-is. Phase 2 errors (resolver, pre-flight) propagate through the same path. Callers that inspect `errors.As(err, &apiErr)` still work.
- **State lifecycle risks:**
  - Phase 1 Unit 3 verbose mode reads the response body for dumping. The debug round-tripper must restore the body (`io.NopCloser(bytes.NewReader(...))`) so downstream JSON parsing still works. Unit 3 test must explicitly cover this.
  - Phase 2 Unit 12's `assignments add` has a TOCTOU race: read existing → some other client writes → we upsert with stale base. Acceptable for a CLI; document the trade-off in the command's `Long:` field.
- **API surface parity:** None — no changes to API endpoints, request shapes, or response handling. Phase 1 Unit 6 deletes unused *Go* methods on `api.Client`, not HTTP endpoints. Phase 2 Unit 12's `assignments add` is CLI-only; the server's `POST /assignments` semantics are unchanged.

## Required Tests

Go tests run via `make test` (= `go test ./...`).

### Unit Tests (new / changed)

| Phase | Function / Behavior | Test File | What to Verify |
|---|---|---|---|
| 1 | `Printer.Aux` writes to stderr | `internal/output/output_test.go` | Captures err writer, asserts content present; out writer empty |
| 1 | `Printer.Data` writes to stdout (raw + JSON modes) | `internal/output/output_test.go` | Captures out writer; trailing newline; JSON mode emits valid JSON |
| 1 | `Printer.Data` does not HTML-escape `&` | `internal/output/output_test.go` | Output contains literal `&`, not `&amp;` |
| 1 | `FormatCLIError` per status code | `internal/output/errors_test.go` | Table-driven: 401/403/404/409/422/429/500 → expected hint substring |
| 1 | `FormatCLIError` with `RequestID` | `internal/output/errors_test.go` | Output contains `Request ID:` line; absent when empty |
| 1 | `FormatCLIError` with wrapped error | `internal/output/errors_test.go` | `fmt.Errorf("ctx: %w", apiErr)` still matched via `errors.As` |
| 1 | `debugRoundTripper` dumps when enabled | `internal/api/client_test.go` | Captures debug writer; verifies request + response present, Authorization masked |
| 1 | `debugRoundTripper` preserves body | `internal/api/client_test.go` | Downstream JSON decode succeeds after dumper runs |
| 1 | `isStdinInteractive` works | `internal/cmd/tty_test.go` | Smoke test (TTY detection is platform-dependent; minimum: function returns bool without panic) |
| 1 | `auth login` non-interactive guard | `internal/cmd/auth_test.go` (new) | With non-TTY stdin and no flags, RunE returns the friendly error |
| 1 | `App.Execute` dry-run path | `internal/cmd/run_test.go` (new) | `dryRun=true` returns `(false, nil)`, no HTTP call made |
| 1 | `App.Execute` real call path | `internal/cmd/run_test.go` (new) | Calls `client.Do` once against an `httptest.Server`, decodes into `Out` |
| 1 | `App.Execute` propagates `*APIError` | `internal/cmd/run_test.go` (new) | Server returns 404 → returned error matches `errors.As(&apiErr)` |
| 1 | `ptr[T]` returns pointer to value | `internal/cmd/helpers_test.go` | `*ptr("x") == "x"`; works for `string`, `int`, `bool`, `float64` |
| 2 | `buildWhoisReport` filters zero-hour rows | `internal/cmd/whois_test.go` | Input with one 0h + one 2h returns one assignment |
| 2 | `WhoisAssignment` carries `id`, `projectId`, `capabilityId` | `internal/cmd/whois_test.go` | JSON marshal includes the new fields |
| 2 | `--weeks N` validation | `internal/cmd/whois_test.go` | 0 / 13 rejected; 1 / 4 accepted |
| 2 | `schema describe assignments` lists both .get and .post | `internal/cmd/schema_test.go` | Output mentions both operations |
| 2 | `schema describe <typo>` suggests close matches | `internal/cmd/schema_test.go` | Non-empty "did you mean" list |
| 2 | `KnownQueryParams` returns endpoint params | `internal/schema/provider_test.go` | Returns expected slice for `GET /assignments` |
| 2 | Unknown `--params` key warns | `internal/cmd/root_test.go` (new) | Stderr contains "warning: unknown query param" |
| 2 | `resolveCapability` happy path | `internal/cmd/capability_resolver_test.go` | Returns most recent hours>0 capability |
| 2 | `resolveCapability` no history | `internal/cmd/capability_resolver_test.go` | Returns specific error |
| 2 | `resolveCapability` only zombie rows | `internal/cmd/capability_resolver_test.go` | Returns error (skips 0h) |
| 2 | `assignments add` read-modify-write | `internal/cmd/assignments_test.go` | Existing 2h + delta 2h → upsert with 4h |
| 2 | `assignments add` no existing | `internal/cmd/assignments_test.go` | Upserts delta as the full value |
| 2 | `assignments add` negative result rejected | `internal/cmd/assignments_test.go` | Returns error before any write |
| 2 | `--auto-capability` resolves per item | `internal/cmd/assignments_test.go` | Multi-item payload all get capabilityId filled |
| 2 | Time-off pre-flight warns on overlap | `internal/cmd/assignments_test.go` | Stderr contains warning for overlapping item |
| 2 | Time-off pre-flight skipped without `--dry-run` | `internal/cmd/assignments_test.go` | No extra API call when not dry-run |

### Existing Tests to Verify (no regressions)

- [ ] `internal/cmd/capacity_test.go` — table rendering still produces correct human view (header + body on stdout)
- [ ] `internal/cmd/compound_test.go` — JSON output unchanged for capacity/bench/whois/context (Phase 1); Phase 2 adds fields without breaking consumers expecting `project`/`date`/`hours`
- [ ] `internal/cmd/agent_flags_test.go` — `--json`, `--fields`, `--dry-run` semantics unchanged
- [ ] `internal/cmd/week_test.go` — unrelated, should pass untouched
- [ ] `internal/cmd/delete_commands_test.go` — error messages unchanged
- [ ] `internal/cmd/whois_test.go` — current week behavior unchanged when `--weeks` is absent or `--weeks 1`
- [ ] `internal/output/output_test.go` — table rendering unaffected

## Success Criteria

### Automated Verification

- [ ] `make fmt` clean (no diff)
- [ ] `make test` green (all existing + new tests pass)
- [ ] `go vet ./...` clean
- [ ] `make build` produces `bin/supervisible` without warnings

### Manual Verification — Phase 1

- [ ] `./bin/supervisible auth status --json 2>/dev/null | jq .` returns valid JSON (no `&amp;`)
- [ ] `./bin/supervisible auth login` without args, on non-TTY stdin → friendly error
- [ ] Hitting a 404 (e.g. `./bin/supervisible projects update bogus-id --name foo`) → status-aware hint + `Request ID:` line
- [ ] `SUPERVISIBLE_DEBUG=1 ./bin/supervisible me 2>verbose.log >/dev/null` produces a request/response transcript with `Authorization` masked
- [ ] `./bin/supervisible whois --help` shows an Example block
- [ ] `./bin/supervisible whois` (missing arg) prints usage + example to stderr
- [ ] `grep -rn 'PrintMessage\|PrintError\|PrintJSON\b\|stringPtr\|intPtr\|float64Ptr\|boolPtr' internal/ cmd/` returns nothing
- [ ] `grep -rn 'client\.\(List\|Create\|Update\|Upsert\|Approve\|Reject\)' internal/cmd/` returns nothing
- [ ] `wc -l internal/cmd/{users,projects,clients,assignments,actual_hours,time_off}.go` shows net reduction vs `git show main:`

### Manual Verification — Phase 2

- [ ] `./bin/supervisible whois "Juan Méndez" --weeks 4 --json | jq '.assignments[0] | keys'` includes `id`, `projectId`, `capabilityId`
- [ ] `./bin/supervisible whois "Juan Méndez" --json | jq '.assignments[] | select(.hours == 0)'` returns empty
- [ ] `./bin/supervisible schema describe assignments` lists both operations
- [ ] `./bin/supervisible assignments list --params '{"startDate":"2026-05-18"}' 2>&1 1>/dev/null` shows unknown-param warning
- [ ] Replay the Juan test: `./bin/supervisible assignments add --user-id 019404f3-... --project-id 019e1cde-... --date 2026-05-24 --hours 2 --auto-capability --dry-run` picks `0194b2e1-b918-7447-a88e-a85ccdea5634` (Web Dev) and warns about the Sabbatical overlap
- [ ] `./bin/supervisible assignments add --hours -100` against a row with 2h existing fails with a non-write error

## Dependencies & Risks

### Dependencies

- `golang.org/x/term` (already in `go.mod`)
- `github.com/spf13/cobra` (already in `go.mod`)
- No new dependencies introduced in either phase.

### Risks

**Phase 1:**

- **Table rendering tests break in Unit 1.** Several capacity/whois tests likely assert against stdout for header+body combined; moving headers via the renamed `Aux` would split them across streams.
  - **Mitigation:** Audit `printCapacityTable` / `printWhoisProfile` during Unit 1 and use direct `fmt.Fprintln(p.Stdout(), ...)` for headers that are part of the human-readable data view.
- **Big mechanical rename in Unit 1 causes merge conflicts** with anything in flight.
  - **Mitigation:** Land Unit 1 first as its own PR, fast-merge. Subsequent units rebase onto the new names.
- **`--verbose` accidentally logs the bearer token.**
  - **Mitigation:** Mask `Authorization` header in the dump; unit test asserts the unmasked token never appears in dump output.
- **Existing scripts that consume the human-readable output of `auth login` may grep for "Authentication successful".**
  - **Mitigation:** The literal text stays; only the stream changes. Most pipelines (e.g. `command 2>&1 | grep ...`) keep working. Document in CHANGELOG.
- **Unit 6 cleanup tempts scope creep.**
  - **Mitigation:** Unit 6 is *only* the three items listed (consolidate `App.Execute`, prune dead `api.Client` methods, generic `ptr`). Anything else gets a follow-up ticket, not an inline patch.

**Phase 2:**

- **TOCTOU race in `assignments add`.**
  - **Mitigation:** Document in command help. The CLI is single-actor in practice; acceptable.
- **Pre-flight time-off check makes dry-run slower** (extra API call per user).
  - **Mitigation:** Batch by user (one query covers all items for the same user). Worst case: dry-run takes 1-2s longer.
- **Auto-capability resolves to wrong capability if a user historically did different work on the same project.**
  - **Mitigation:** Always print the resolved capability to stderr (via `Aux`) so the human/agent can spot a mismatch. Failure mode is visible, not silent.
- **Schema-based unknown-param warning lists wrong params if schema is out of date.**
  - **Mitigation:** Schema is regenerated alongside the API contract. Warn-only (not block) keeps this from being a hard failure.

## Performance Considerations

- Phase 1 Unit 3 `debugRoundTripper` allocates a buffered copy of response bodies. Negligible for the CLI's usage pattern (interactive / agent calls), and skipped entirely when verbose is off (default path unchanged).
- Phase 2 Unit 11's resolver issues one `GET /assignments` per `(user, project)`. Worst case for a multi-item upsert: N additional calls. Cache per-invocation so duplicates collapse.
- Phase 2 Unit 13's time-off pre-flight: one `GET /time-off` per distinct user across all dry-run items. Bounded by item count.
- All read calls already use `limit=50` or similar bounds. No new pagination work needed.

## Security Considerations

- **Token masking in verbose mode is non-negotiable.** Direct unit test on `debugRoundTripper` must assert masking before merge.
- **No new attack surface** — all changes are output-side, TTY-detection-side, or additive read-modify-write logic that uses the existing API key.
- **Error messages do not leak server internals** — for 5xx hints, the generic "Server error, contact support" line replaces the raw `apiErr.Message` body. For 422, we preserve `apiErr.Message` since validation errors are intentionally user-facing.
- **Pre-flight time-off check reveals approved time-off for users you query** — same scope as existing `time-off list`. No new info exposure.
- **Auto-capability resolution reads assignments** — same scope as existing `assignments list`. No escalation.

## References

- Notion CLI design principles (podcast transcript, in conversation context, 2026-05-19)
- Findings doc: `.thoughts/findings/2026-05-19-cli-real-world-testing.md` (F1–F15)
- Superseded plan: `.thoughts/plans/2026-05-19-feat-agent-safe-writes.md` (folded into Phase 2 of this plan)
- Current `Printer` implementation: `internal/output/output.go:14-65`
- Current `APIError`: `internal/api/client.go:65-77`
- Current `main.go` error path: `cmd/supervisible/main.go:11`
- Current auth login flow: `internal/cmd/auth.go:34-110`
- Whois implementation: `internal/cmd/whois.go:16-148`
- Upsert implementation: `internal/cmd/assignments.go:101-198`
- JSON encoder calls: `internal/output/output.go:30, 39`
- Schema describe: `internal/cmd/schema.go`
- `CLAUDE-CODE-TASK.md` + `PLAN-AI-NATIVE.md` (parent agent-first vision)
- Recent compound command work: commit `bba1cbd feat(cli): add compound commands` (May 18, 2026)

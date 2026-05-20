---
type: findings
status: open
tags: [cli, agent, testing, real-world]
created: 2026-05-19
related_plan: .thoughts/plans/2026-05-19-feat-cli-agent-experience.md
---

# Findings — Real-world CLI test: "add hours to Juan based on his Slack message"

## Test scenario

Parse a Spanish-language Slack message from Juan Méndez listing 5 client/project hour estimates, then translate into `assignments upsert` calls. Run against local server at `http://localhost:3000/api/v1` with a real `sv_live_` key.

**Result:** 6 assignments created successfully via `assignments upsert --file`. End-state verified via `whois`. The CLI completed the task, but several rough edges emerged that affect agent reliability and would compound at scale.

## Findings (prioritized)

### F1. `assignments list` date filters work, but cross-user behavior is confusing — [P2, revised]

**Originally suspected:** date filters silently ignored.
**Revised after deeper testing:** they actually work. Juan Colmenares had `[]` for May 18 → June 15 simply because he has no future assignments (last activity was Jan 2026). Juan Méndez correctly returned only the 8 rows in the window.

**Remaining issue:** The CLI returns an empty array with no indication of "the user exists but has no rows in this window" vs "the filter didn't apply". For an agent, both look identical. This is partially addressed by the fact that you can verify `users list | grep` separately, but it's friction.

**Action:**
- Document filter param names in `schema describe assignments.get` output (currently `operation not found` — see F2).
- Lower priority than originally assessed.

### F2. `schema describe <op>` doesn't accept the operation IDs `schema endpoints` emits — [P1 UX bug]

**Observed:**
```bash
supervisible schema endpoints   # prints "assignments.get", "users.patch", etc.
supervisible schema describe assignments
# → operation not found: assignments
supervisible schema describe assignments.get
# (untested — but agents will try the short form first)
```

The agent sees `assignments` in the noun-verb tree and types it, but `describe` wants the full op ID. Either accept both forms or improve the error: `"operation not found. Try: assignments.get, assignments.post"`.

### F3. No `GET /capabilities` endpoint — [P0 blocker for autonomous writes]

**Observed:** To create assignments I had to guess `capabilityId` by sampling existing assignments per project. There's no way to:
- List capabilities by name (e.g. "Engineering", "Design")
- Translate a natural-language role ("Juan as engineer on Odyssey") → capabilityId
- Verify that the guessed capability is appropriate

**Impact:** Confirmed gap from `PLAN-AI-NATIVE.md`. Without this, the agent will pick wrong capabilities on new projects, which corrupts utilization/billing reports.

**Action:** Add `GET /capabilities` to public API. Add `supervisible capabilities list` to CLI.

### F4. `assignments upsert` accepts writes that conflict with approved time-off — [P1, agent guardrail]

**Observed:** Juan Méndez has an approved Sabbatical 2026-05-10 → 2026-07-03. I added 6 assignments inside that window. The server accepted all 6 with zero warning. His `weekSummary` now reads `assignedHours: 27, availableHours: 0, freeHours: -27`.

**Why this matters:** For an autonomous agent doing writes, this is dangerous — it can silently overbook someone on PTO. A human eyeballing the workload table catches it; an agent operating headlessly won't.

**Action options (server-side, but CLI could pre-flight):**
- Soft warning at upsert time when assignment overlaps approved time-off.
- CLI pre-flight: before upsert, fetch overlapping time-off and surface conflicts in the dry-run preview.
- Strict mode flag: `--reject-conflicts` rejects the batch; default mode warns.

### F5. `whois` is current-week-only — no `--week` or `--weeks N` flag — [P1 ergonomics]

**Observed:** `whois` returns only assignments for the current week. For the "what's Juan booked for the next 3 weeks?" question, I had to call `assignments list` separately and aggregate manually.

**Action:** Add `--weeks N` (default 1) to `whois`, returning the next N weeks of assignments + time-off in one call. Matches the "compound commands beat 10 round trips" principle and makes the Slack agent's job much easier.

### F6. `whois` JSON output escapes `&` to `&` — [P3 cosmetic]

**Observed:** `"project": "SEO & Web Migration"`. Standard Go `json.Encoder` HTML-escaping. Noise when piping to `jq` or eyeballing.

**Fix:** `enc.SetEscapeHTML(false)` in `internal/output/output.go:31`. One-line change.

### F7. `whois.assignments[i]` lacks `id` and `capabilityId` — [P2]

**Observed:**
```json
"assignments": [
  {"project": "Web, Ads", "date": "2026-05-24", "hours": 2}
]
```

**Why this matters:** If you want to **modify** what `whois` returned (delete an assignment, change hours), you can't — you have to round-trip via `assignments list`. An agent doing "remove all of Juan's Bitso hours this week" can't act directly on `whois` output.

**Action:** Include `id`, `projectId`, `capabilityId` in `whois.assignments[]`. Keep `project` name as the human-friendly field.

### F8. `--dry-run` and "Required scope" lines print to stdout — [P1, exactly Unit 1 of plan]

**Observed:**
```
$ supervisible assignments upsert --file payload.json --dry-run
Dry-run: POST /assignments        ← stdout
Body: { ... }                      ← stdout
Required scope: write:assignments  ← stdout
```

All three are auxiliary — they should be on stderr so `--dry-run --json` piped to `jq` works. Already covered in Unit 1 of the plan; this confirms the priority.

### F9. `assignments upsert` table output has no project NAME, only IDs — [P2]

**Observed:**
```
ID                                    USER_ID  PROJECT_ID  DATE       HOURS
019e41c0-...  019404f3-...  019e1cde-...  2026-05-24  2
```

For human review the table is useless without names. Either:
- Expand `project.name` automatically in the response (server-side).
- Have the CLI render `--expand project` by default for upsert results.
- Add a `project` column resolved via a second lookup (chatty but readable).

### F10. `--params` JSON parsing is permissive but undocumented — [P2 ergonomics]

**Observed:** I passed `{"limit":300}` — worked. Passed `{"start_date":"..."}` — silently ignored (per F1). There's no schema for what `--params` accepts per command, so agents will guess wrong key names indefinitely.

**Action:** `schema describe <op>` should list valid query params. Unknown params should at minimum produce a warning to stderr: `unknown query param: start_date (allowed: limit, offset, user_id, project_id)`.

### F11. `whois` exit code on no-match — needs verification

**Observed:** `whois juan` (ambiguous) exits 1 with a helpful error. Good. But what about `whois "Nobody"`? Did not test — worth confirming the agent sees a distinguishable exit code for "no match" vs "API error".

### F12. No DELETE endpoint on assignments — `hours: 0` creates zombie rows — [P0]

**Observed:**
```bash
# Attempt to "remove" a wrong assignment by upserting hours=0
supervisible assignments upsert --file /tmp/fix.json
# Server returns 200 with the row preserved, hours=0
```

State after the "fix":
```
2026-05-24 | 0h | cap=<wrong>  | id=019e41c0-... ← ZOMBIE row, still in DB
2026-05-24 | 2h | cap=<right>  | id=019e41c3-... ← correct row
2026-05-10 | 2h | cap=<right>  | id=019e1cfb-... ← pre-existing
```

`whois` still returns the 0h row in `assignments[]` (`weekSummary` correctly excludes it). The workload UI will likely render the zombie row too.

**Why this matters:** Any agent that needs to **correct a mistake** has no way to actually delete the wrong row. Over time, the assignments table accumulates 0h ghosts that pollute every list-by-user query.

**Action options:**
- Add `DELETE /assignments/{id}` to the public API.
- OR: server-side, treat `hours: 0` in an upsert as a delete.
- CLI side: filter out `hours: 0` from `whois` output regardless of server behavior (F13).
- CLI side: add `supervisible assignments delete <id>` once the endpoint exists.

### F13. `whois.assignments[]` includes zombie 0h rows — [P1]

**Observed:** After F12 cleanup attempt, `whois` returns:
```json
"assignments": [
  {"project": "Web + Dev Retainer 2026 II ", "date": "2026-05-24", "hours": 2},
  {"project": "Web + Dev Retainer 2026 II ", "date": "2026-05-24", "hours": 0},  ← noise
  ...
]
```

Agent/human both see "two Odyssey rows" when really there's one. `weekSummary.assignedHours` is correct (excludes 0h), but the per-row breakdown isn't.

**Fix:** In `internal/cmd/whois.go`, filter `hours > 0` when building the response. One-line guard.

### F14. Capability-resolution for upserts needs a per-user heuristic — [P1, the real cause of the test bug]

**The bug I introduced:** I picked the `capabilityId` for Odyssey by sampling "any capability ever used on this project" (`019b03dd...` = Webflow dev). The correct heuristic: "the capability **this user** has been using on this project most recently" (`0194b2e1-b918-7447` = Web Dev).

**Why my date window missed it:** Juan's pre-existing Odyssey row was dated **2026-05-10** (week 19). My `start_date: 2026-05-18` window excluded it. So even checking "Juan's future Odyssey rows" wasn't enough — I needed "Juan's most recent Odyssey row regardless of date".

**Why this matters for autonomous writes:** Without server-side `GET /capabilities` (F3), the only signal an agent has for "which capability?" is historical usage. The CLI should expose this heuristic as a compound helper rather than expecting every agent to reimplement it.

**Proposed compound command:**
```bash
supervisible suggest-capability --user-id <id> --project-id <id>
# → most recent capability this user used on this project, or "none" if never
```

OR, fold it into `assignments upsert` as an `--auto-capability` flag that resolves before posting:
```bash
supervisible assignments upsert --file <items-without-capability-ids> --auto-capability
# CLI fills in capabilityId per (user, project) from history; fails noisily if no history exists
```

**Bigger picture:** This is exactly the kind of thing the agent-facing CLI should hide behind a compound command. Writing `assignments upsert` raw is a footgun.

### F15. Add hours = total hours, not delta — agent must do "read-modify-write" — [P1]

**Observed:** Juan's message said "estimaría **un par de horas más** esta semana" ("a couple more hours this week"). The word "más" implies **incremental**. But `assignments upsert` is an UPSERT keyed on `(userId, projectId, capabilityId, date)` — it **replaces** the hours value.

If Juan had already been assigned 2h to Odyssey for 2026-05-24 (on the right capability), and I upserted "2h more", I'd have ended up with 2h, not 4h. The agent must do read-modify-write itself: fetch existing → compute new total → upsert.

**Proposed compound command:**
```bash
supervisible assignments add --user-id <id> --project-id <id> --date <date> --hours <delta>
# Reads existing, adds delta, upserts new total. Idempotent if you specify --capability-id.
```

Pair with F14's `--auto-capability` to make natural-language requests one-liners.

## Things that worked well (don't regress these)

- ✅ `whois "Juan"` (ambiguous) → clear actionable error: `multiple users match "juan": Juana Cajiao, Juan Colmenares, Juan Méndez. Be more specific` — this IS Notion-style "actionable error" done right.
- ✅ `whois "Juan Méndez"` (accent) just works.
- ✅ `--dry-run` body output is verbatim what gets posted — copy-paste-modifiable.
- ✅ `assignments upsert --file` is the right primitive for batch writes; agents can compose the payload deterministically.
- ✅ `projects list --fields "id,name,status"` + jq filter is fast and readable.
- ✅ The compound `whois` command saved 3 round trips (user lookup + assignments + time-off in one call).

## Proposed plan changes (to discuss in `/iterate-plan`)

Add these as **new units** to the existing plan, or split into a follow-up plan:

| New Unit | What | Rationale |
|---|---|---|
| **Unit 6** | Warn on unknown query params in `--params` (F10) | Silent failures are agent-hostile |
| **Unit 7** | Disable HTML-escape in JSON output (F6) | One-line fix, cleaner pipe-through-jq |
| **Unit 8** | Add `--weeks N` to `whois` (F5) | Compound-command philosophy; saves agent round trips |
| **Unit 9** | Include `id`, `projectId`, `capabilityId` in `whois.assignments[]` (F7) | Makes whois output actionable, not just informational |
| **Unit 10** | Time-off conflict warning at upsert time (F4) | CLI pre-flight: fetch overlapping time-off in `--dry-run` preview |
| **Unit 11** | `schema describe` accepts both short (`assignments`) and full (`assignments.get`) op IDs (F2) | Lower the friction on the introspection path |
| **Unit 12** | Filter `hours: 0` from `whois.assignments[]` (F13) | Hide zombie rows; one-line guard |
| **Unit 13** | New compound: `supervisible assignments add` — read-modify-write delta (F15) | Natural-language "2h más" requests become safe one-liners |
| **Unit 14** | `--auto-capability` flag on `assignments upsert` (F14) | Resolve `capabilityId` from user's history per (user, project) — fail loudly if no history |
| **Unit 15** | New compound: `supervisible assignments delete <id>` once server endpoint exists (F12) | Currently impossible to remove rows |

Server-side asks (out of CLI scope, file in main Supervisible app):

| Server change | Rationale |
|---|---|
| `GET /capabilities` | F3 — autonomous writes require capability ID lookup |
| `DELETE /assignments/{id}` (or treat `hours: 0` as delete) | F12 — currently no way to remove a wrong assignment; zombies accumulate |
| Soft-warn on assignment overlapping approved time-off | F4 — guardrail for headless writes |
| Expand `project.name` in `assignments` POST response | F9 — readable success output |
| Reject unknown query params on list endpoints | F10 — currently silently accepted |

## Open question for the user

Boca de Agua reduction: Juan said "creo que este número debería ser un poco menor". I skipped this because no specific number was given. Resolution paths:
1. Pick a default reduction (e.g. 12h → 10h) and apply.
2. Ask Juan directly via Slack.
3. Treat as a TODO surfaced in a "needs human" report.

Probably (3) — the agent's job is to flag "needs human", not invent numbers.

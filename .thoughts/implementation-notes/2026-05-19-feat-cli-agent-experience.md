---
plan: .thoughts/plans/2026-05-19-feat-cli-agent-experience.md
ticket: none
status: ready-for-review
---

# Implementation Notes: feat(cli) Agent-grade CLI

## Session 2026-05-19

### Design decisions

- **Branch strategy:** Started Phase 1 on a new branch `feat/cli-agent-experience` cut from `feat/sv-278-compound-commands` (rather than from `main`), because the parent branch contained two completed-but-uncommitted features (`DeleteActualHour` / `DeleteAssignment` with tests). Committed those as `feat(cli): add delete commands for actual-hours and assignments` (commit `e4e5a6d`) so the plan's mechanical rename in Unit 1 doesn't entangle with unrelated WIP. The plan itself in Unit 6b explicitly accounts for these as the two `Delete*` helpers to keep.
- **Plan-recommended PR-per-unit split was collapsed into one PR for Phase 1.** Plan suggests landing Unit 1 first as its own PR (to ease rename conflicts), but no other work is in flight on this repo, so a single Phase 1 PR is lower friction. Phase 2 will be its own PR series, as the plan recommends.

### Design decisions (Phase 1)

- **Read commands' non-JSON rendering goes to stdout, not stderr.** The plan only explicitly called out `printCapacityTable` and `printWhoisProfile` as data-view-on-stdout, but the same logic applies to `me`, `config show`, `version`, `schema describe`, `auth status`, `bench`, and `context`. Each of these has a parallel `IsJSON()` branch that writes to stdout; the human-readable branch is the *same result* in a different format and stays on stdout. Used `fmt.Fprintf(p.Stdout(), ...)` rather than adding a `Printer.DataLine` method, since the plan's contract explicitly bans extra Printer methods.
- **Write commands' confirmation lines stay on stderr.** "Updated: <id>", "Created project: …", "Deleted assignment: …" — these confirm a write happened, they aren't the result. The result in JSON mode is the returned record; in non-JSON mode the confirmation is auxiliary. Matches the plan's explicit example of `"Updated: <id>" lines` being Aux.
- **`argsWithUsage(validator)` wrapper instead of toggling `SilenceUsage` per command.** Verified empirically: cobra's `SilenceUsage` walks the parent chain, so setting `SilenceUsage: false` on a leaf doesn't override the root's `true`. The wrapper opts in per-command at the `Args` boundary, fires only on arg-validation errors (not API errors), and prints usage via `cmd.Usage()` before returning the error to main.
- **Delete commands kept their existing typed-helper boilerplate.** Plan Unit 6 says "migrate file-by-file" but also says to *keep* `DeleteActualHour`/`DeleteAssignment` typed helpers as documented public API surface. Migrating delete commands through `App.Execute` would route them through `client.Do` and make those helpers dead. Chose the latter constraint as load-bearing; the delete commands retain their hand-rolled flow. ~20 lines each, no boilerplate proliferation since there are only two.
- **`auth login` non-TTY guard fires before `term.ReadPassword`, not as a generic prompt wrapper.** Kept narrow to the one current interactive prompt to avoid building a "confirmation framework" before there's a second caller.

### Deviations

- **Phase 1 shipped as a single PR series, not split per unit.** Plan recommended landing Unit 1 first to minimize rename conflict surface. With no concurrent work in flight, that hedge didn't apply.

### Open questions

- **Should `App.Execute` accept a `cobra.Command` rather than a `context.Context`?** Currently every caller passes `cmd.Context()` and `cmd.Context()` could be derived once inside. Leaving as-is because callers might want a different context (e.g. for tests). Worth revisiting if the call sites all look identical for a while.

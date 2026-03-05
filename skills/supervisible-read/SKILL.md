---
name: supervisible-read
version: 1.0.0
---

Use this skill for non-mutating Supervisible reads.

## Recommended pattern

- Always pass `--json`
- Always pass `--fields`
- Use `--params` for paging/filtering

## Examples

```bash
supervisible users list --json --fields 'id,name,email' --params '{"limit":20}'
supervisible projects list --json --fields 'id,name,status,clientId' --params '{"limit":20}'
supervisible schema describe "GET /time-off" --json
```

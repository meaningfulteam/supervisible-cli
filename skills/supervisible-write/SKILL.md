---
name: supervisible-write
version: 1.0.0
---

Use this skill for mutating Supervisible operations.

## Safety requirements

- Always run with `--dry-run --json` first.
- Prefer `--payload`/`--file` for nested updates.
- Confirm UUID/date formats before execution.

## Examples

```bash
supervisible clients create \
  --company-name "Acme" \
  --payload '{"clientPriority":"high"}' \
  --dry-run --json

supervisible time-off reject 019cb675-c4d6-7e90-8806-25e5145c3a06 \
  --reason "Coverage needed" \
  --dry-run --json
```

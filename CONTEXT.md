# Supervisible CLI Agent Context

## Rules

1. Always prefer `--json` output.
2. Always use `--fields` on read/list commands to reduce payload size.
3. Use `--params` for explicit query control (`limit`, `offset`, filters).
4. Always run mutating operations with `--dry-run` first.
5. Prefer `--payload` or `--file` for complex nested payloads.
6. IDs must be UUIDs and must not include query strings (`?`), fragments (`#`), or `%` encoded segments.
7. Dates must use `YYYY-MM-DD`.

## API Defaults

- Base URL: `https://app.supervisible.com/api/v1`
- Auth header: `Authorization: Bearer sv_live_...`
- Envelope: `{ "data": ... }`

## Safe execution flow for writes

1. Compose payload with `--payload` or `--file`
2. Run with `--dry-run --json`
3. Validate method/path/body/scope in output
4. Re-run without `--dry-run`

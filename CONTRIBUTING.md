# Contributing to supervisible-cli

Thank you for taking the time to contribute!

---

## Prerequisites

- **Go 1.24+** — [install](https://go.dev/dl/)
- **make** — standard on macOS/Linux; use Git Bash or WSL on Windows

---

## Setup

```bash
git clone https://github.com/supervisible/supervisible-cli.git
cd supervisible-cli
make tidy
make build
```

The binary lands at `./bin/supervisible`.

---

## Running Tests

```bash
make test
```

This runs `go test ./...`. All tests must pass before opening a PR.

---

## Code Style

```bash
make fmt
```

This runs `gofmt -w` over `./cmd` and `./internal`. CI will fail if any files are unformatted. No additional linter config is required.

---

## Submitting Changes

1. **Fork** the repository and create a feature branch off `main`:
   ```bash
   git checkout -b feat/my-change
   ```
2. Make your changes; add or update tests as appropriate.
3. Run `make fmt && make test` locally and confirm both pass.
4. Open a **Pull Request** against `main`. The PR template will pre-fill a checklist — complete it before requesting review.

### PR checklist (summary)

- [ ] Change is described in the PR body
- [ ] `make test` passes
- [ ] `make fmt` produces no diff
- [ ] Any new mutating command was tested with `--dry-run`
- [ ] README updated if commands or flags changed

---

## API Keys for Local Development

Set the `SUPERVISIBLE_API_KEY` environment variable (or use `supervisible auth login`):

```bash
export SUPERVISIBLE_API_KEY=sv_live_xxx
./bin/supervisible me
```

You can also override the base URL for a local or staging API:

```bash
export SUPERVISIBLE_BASE_URL=http://localhost:3000/api/v1
```

---

## Release Process

Releases are **maintainer-only** and use [GoReleaser](https://goreleaser.com).

1. Tag the commit: `git tag vX.Y.Z && git push origin vX.Y.Z`
2. Run: `goreleaser release`
3. GoReleaser builds cross-platform binaries and publishes the Homebrew formula to the tap.

For a dry run without publishing:

```bash
goreleaser release --snapshot --clean
```

Requires `HOMEBREW_TAP_GITHUB_TOKEN` with write access to the tap repository.

# Contributing

Thanks for contributing to `opencode-helper`.

## Development Setup

Requirements:

- Go `1.23+`
- Optional: `task` if you want to use the provided `Taskfile.yml`
- Optional: `goreleaser` if you want to validate release packaging locally

Build locally:

```bash
mkdir -p dist
go build -o dist/opencode-helper .
```

Or:

```bash
task build
```

## Local Checks

Run these before opening a pull request:

```bash
go fmt ./...
go vet ./...
go test ./...
```

If you are changing release packaging, also run:

```bash
goreleaser check
goreleaser release --snapshot --clean
```

## Project Layout

- `main.go` bootstraps the CLI
- `internal/cli` contains Cobra commands and flag handling
- `internal/store` reads OpenCode storage and SQLite data
- `internal/restore` reconstructs file contents
- `internal/gitstore` loads git-backed fallback snapshots
- `internal/ui` contains interactive terminal selection and table output
- `internal/usage` builds usage and pricing reports
- `internal/model` contains shared data models

## Pull Requests

- Keep changes focused and avoid unrelated refactors.
- Add or update tests when behavior changes.
- Update `README.md` when user-facing behavior or flags change.
- Use conventional commit prefixes when possible, such as `fix:`, `feat:`, or `docs:`. This keeps release-please changelogs and version bumps accurate.

## Releases

- Releases are proposed by `release-please` and tagged as `vX.Y.Z`.
- GoReleaser publishes archives to GitHub Releases and updates the Homebrew formula in `aaronflorey/homebrew-tap`.
- If you change release packaging, update `.goreleaser.yaml` and `.github/workflows/release.yaml` together.

## Reporting Bugs

Open an issue with:

- what you ran
- what you expected
- what happened instead
- relevant paths, flags, and sample data shape when available

For security-sensitive issues, follow `SECURITY.md` instead of opening a public issue.

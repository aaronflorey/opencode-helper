# opencode-helper

[![CI](https://github.com/aaronflorey/opencode-helper/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/aaronflorey/opencode-helper/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/aaronflorey/opencode-helper?display_name=tag&sort=semver&style=flat-square)](https://github.com/aaronflorey/opencode-helper/releases)
[![License](https://img.shields.io/github/license/aaronflorey/opencode-helper?style=flat-square)](LICENSE)

`opencode-helper` is a Go CLI for inspecting and recovering data from a local OpenCode installation.

It currently focuses on three jobs:

- restoring files from OpenCode session history
- reporting token usage and estimated cost
- summarizing `bash` tool usage from recorded sessions

## Installation

### Homebrew

```bash
brew tap aaronflorey/homebrew-tap
brew install opencode-helper
```

### Go install

```bash
go install github.com/aaronflorey/opencode-helper@latest
```

### GitHub Releases

Download the latest archive from the [GitHub Releases](https://github.com/aaronflorey/opencode-helper/releases) page, extract it, and place `opencode-helper` on your `PATH`.

### Build from source

```bash
git clone https://github.com/aaronflorey/opencode-helper.git
cd opencode-helper
mkdir -p dist
go build -o dist/opencode-helper .
```

If you use `task`:

```bash
task build
```

## Setup

You need:

- a local OpenCode data directory; by default the CLI reads `~/.local/share/opencode/storage`
- an `opencode.db` file next to that storage directory if you want database-backed project and message discovery
- Go `1.23+` only if you want to build from source

## Usage

Show available commands:

```bash
opencode-helper --help
```

Restore a file interactively:

```bash
opencode-helper restore
```

Restore a file for a specific project and print the result to stdout:

```bash
opencode-helper restore \
  --project <project-id-or-worktree-fragment> \
  --file "path/or/substring"
```

Write the restored file back to its inferred path inside the selected project's worktree:

```bash
opencode-helper restore \
  --project <project-id-or-worktree-fragment> \
  --file "path/or/substring" \
  --output
```

Write the restored file to an explicit path:

```bash
opencode-helper restore \
  --project <project-id-or-worktree-fragment> \
  --file "path/or/substring" \
  --output reconstructed.txt
```

Generate usage reports:

```bash
opencode-helper usage --type daily
opencode-helper usage --type weekly --json
opencode-helper usage --type session
```

Inspect `bash` tool usage:

```bash
opencode-helper tool-usage
opencode-helper tool-usage --current-project
opencode-helper tool-usage --full-command --limit 50
```

## Commands

### `restore`

`restore` reconstructs file contents from OpenCode history.

It uses the best available source for each file, including session diffs, message summaries, tool snapshots, and git history when available.

Useful behavior:

- If `--project` is omitted, the CLI tries to match the current working directory to an OpenCode project.
- If `--file` is omitted, the CLI opens an interactive file picker.
- `--output` with no value writes to the inferred original path under the selected project.
- Inferred writes are restricted to paths inside the selected project's worktree.

Flags:

- `--storage` OpenCode storage directory. Default: `~/.local/share/opencode/storage`
- `--db` path to `opencode.db`. Default: sibling of `--storage`
- `--project` project ID or worktree substring
- `--file` file path filter. Use a substring match, or `/prefix` for a root-anchored match
- `--no-git` disable git history lookup
- `-o, --output <path>` write to a specific file
- `-o, --output` write to the inferred original file path

### `usage`

`usage` aggregates assistant usage from OpenCode storage and database records.

The report groups data by time period or session and includes token counts plus estimated costs. Stored message cost is used when available; otherwise the CLI falls back to LiteLLM pricing data with OpenRouter coverage where needed.

Flags:

- `--storage` OpenCode storage directory. Default: `~/.local/share/opencode/storage`
- `--db` path to `opencode.db`. Default: sibling of `--storage`
- `--type` grouping type: `daily|weekly|monthly|session`
- `--json` output the report as JSON

### `tool-usage`

`tool-usage` scans recorded OpenCode `part` data for `bash` tool calls and groups them by normalized command or full raw command.

Flags:

- `--storage` OpenCode storage directory. Default: `~/.local/share/opencode/storage`
- `--db` path to `opencode.db`. Default: sibling of `--storage`
- `--current-project` only include usage from the project matching the current working directory
- `--full-command` group by the exact command string instead of normalized command tokens
- `--limit` max rows to print. Use `0` for all rows

## Data Sources

`opencode-helper` reads from local OpenCode data, including:

- `storage/project`
- `storage/session/<project-id>`
- `storage/session_diff/<session-id>.json`
- `opencode.db`

When both filesystem data and SQLite metadata are present, the CLI uses both and prefers the strongest available source for each operation.

## Development

Run the standard local checks:

```bash
go fmt ./...
go vet ./...
go test ./...
```

See `CONTRIBUTING.md` for the development workflow.

## Releases

Releases are managed with `release-please` and tagged as `vX.Y.Z`.

When a release PR is merged, GitHub Actions publishes cross-platform binaries to GitHub Releases and updates the `aaronflorey/homebrew-tap` formula.

# opencode-helper

`opencode-helper` is a modular CLI for working with OpenCode local data.

The tool is command-oriented and designed to grow over time.

## Current Commands

- `restore` - reconstruct file contents from OpenCode history sources.
- `tool-usage` - summarize bash tool command usage and estimated output tokens.
- `usage` - report token usage and estimated cost by group/model.

## Usage Command

`usage` aggregates assistant token usage from both filesystem and SQLite sources:

- `storage/message/<sessionID>/*.json`
- `storage/session/<projectID>/<sessionID>.json`
- `opencode.db` (`message` and `session` tables)

Output is grouped with `--type` and each row is split by normalized model name:

- `daily` (default)
- `weekly`
- `monthly`
- `session`

Pricing is fetched from LiteLLM pricing data, with OpenRouter as fallback/coverage expansion.
Use `--json` for structured output.

## Restore Command

`restore` reads from OpenCode storage and database metadata:

- `~/.local/share/opencode/storage/project`
- `~/.local/share/opencode/storage/session/<projectID>`
- `~/.local/share/opencode/storage/session_diff/<sessionID>.json`

When available, it also reads `opencode.db` (SQLite) and treats DB records as the source of truth for project/session discovery.

Restore flow:

1. Lets you select a project.
2. Lets you select one or more files touched in session history.
3. Reconstructs each file from the best available source.

If `--project` is omitted, the CLI first tries to infer the project from your current working directory.
When this auto-match succeeds, the CLI prints the matched project/worktree.

If `--file` is omitted, the file picker uses multi-select so you can restore multiple files in one run.

Content source priority:

1. `storage/session_diff/<sessionID>.json` (preferred)
2. `message.data.summary.diffs` from `opencode.db` (fallback if session_diff is missing)
3. `part.data` tool snapshots from `opencode.db` (currently `read`/`write` tool payloads)
4. Git history snapshots from the project repository (when project is a git repo)

For each file, versions are collected from all available sources and the latest UTC-normalized timestamp wins.

## Build

```bash
go build -o opencode-helper .
```

With Taskfile (current OS/arch):

```bash
task build
```

## Usage

Interactive menus:

```bash
./opencode-helper restore
```

Non-interactive with filters:

```bash
./opencode-helper restore \
  --project <project-id-or-query> \
  --file "<file-substring>" \
  --output reconstructed.css
```

Print reconstructed content to stdout:

```bash
./opencode-helper restore --project <project-query> --file "<file-substring>"
```

Root-anchored filter example:

```bash
./opencode-helper restore --file "/.planning"
```

Write back to inferred original path:

```bash
./opencode-helper restore --file "STATE.md" --output
```

Token usage report:

```bash
./opencode-helper usage --type daily
./opencode-helper usage --type weekly --json
./opencode-helper usage --type session
```

## Flags

- `--storage` OpenCode storage directory (default `~/.local/share/opencode/storage`)
- `--db` path to `opencode.db` (default: sibling of `--storage`)
- `--project` project id or worktree substring
- `--file` file path filter (substring, or `/prefix` for root-anchored match)
- `--no-git` disable git history lookup source
- `-o, --output=<path>` write reconstructed content to a file instead of stdout
- `-o, --output` (no value) write to inferred original file path under the selected project's worktree

## Tool Usage Command

`tool-usage` scans OpenCode `part` records for `bash` tool calls and prints a grouped usage table:

- `COMMAND` grouped command key
- `RUNS` number of runs for that command key
- `OUTPUT_TOKENS` estimated tokens consumed by tool output (char-count approximation)

By default, the command scans all projects visible in the local `opencode.db`.

Examples:

```bash
./opencode-helper tool-usage
./opencode-helper tool-usage --limit 50
./opencode-helper tool-usage --current-project
./opencode-helper tool-usage --full-command
```

Flags:

- `--storage` OpenCode storage directory (default `~/.local/share/opencode/storage`)
- `--db` path to `opencode.db` (default: sibling of `--storage`)
- `--current-project` only include data from the project matching current working directory
- `--full-command` group by the exact raw command string instead of normalized command tokens
- `--limit` max rows to print (default `25`, use `0` to show all)
`usage` flags:

- `--storage` OpenCode storage directory (default `~/.local/share/opencode/storage`)
- `--db` path to `opencode.db` (default: sibling of `--storage`)
- `--type` grouping type: `daily|weekly|monthly|session`
- `--json` output usage report as JSON

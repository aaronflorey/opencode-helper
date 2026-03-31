# opencode-helper

`opencode-helper` is a modular CLI for working with OpenCode local data.

The tool is command-oriented and designed to grow over time.

## Current Commands

- `restore` - reconstruct file contents from OpenCode history sources.

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

## Flags

- `--storage` OpenCode storage directory (default `~/.local/share/opencode/storage`)
- `--db` path to `opencode.db` (default: sibling of `--storage`)
- `--project` project id or worktree substring
- `--file` file path filter (substring, or `/prefix` for root-anchored match)
- `--no-git` disable git history lookup source
- `-o, --output=<path>` write reconstructed content to a file instead of stdout
- `-o, --output` (no value) write to inferred original file path under the selected project's worktree

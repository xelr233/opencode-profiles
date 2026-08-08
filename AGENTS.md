# AGENTS.md

## Project

CLI tool to manage multiple [OpenCode](https://opencode.ai) configuration profiles via symlink switching. Create, backup, switch, and list profiles; import providers from existing configs when creating new ones. Manages per-profile skills via `skills.yml` and syncs symlinks in `~/.config/opencode/skills/`. Built with Go as a single static binary named `opencode-profiles` (entry point → `cmd/opencode-profiles`).

## Architecture

The core invariant: `~/.config/opencode/opencode.json` **must always be a symlink** pointing to `profiles/<name>/opencode.json` after initialization. Everything else follows from this.

- `internal/paths/paths.go` — `Paths` struct. All path resolution lives here. `New(baseDir, skillSourcesDir)` with empty values falling back to `~/.config/opencode` and `~/.cc-switch/skills/`. `RelativeTarget()` returns the symlink target for a profile.
- `internal/ops/ops.go` — Every function calls `EnsureInitialized()` first, which enforces the symlink invariant by migrating a bare `opencode.json` into a `default` profile. Don't skip this call when adding new operations. Also writes `skills.yml` for new/existing profiles. `SwitchDB(p, name, dbPath)` takes an injectable db path for tests; `Switch` uses the default `~/.cc-switch/cc-switch.db`.
- `internal/skills/skills.go` — Skills management: read/write `skills.yml` (yaml.v3), scan current symlinks, compute diff, sync symlinks, add/remove skills. `SyncSkills()` validates all target sources exist before modifying anything, then updates `cc-switch.db` (modernc.org/sqlite, pure Go). `UpdateDB()` silently ignores missing db / sqlite errors.
- `cmd/opencode-profiles/main.go` — stdlib `flag` CLI. `run(args, stdout, stderr, paths, dbPath)` returns an exit code and is fully injectable for tests; `main()` calls it with defaults.

## Commands

```bash
# Build
go build -trimpath -ldflags="-s -w" -o opencode-profiles ./cmd/opencode-profiles

# Run all tests
go test ./...

# Run a single package's tests
go test ./internal/ops/ -v

# Lint & format check
go vet ./...
gofmt -l .

# Cross-compile
GOOS=linux GOARCH=arm64 go build ./cmd/opencode-profiles
GOOS=darwin GOARCH=arm64 go build ./cmd/opencode-profiles
GOOS=windows GOARCH=amd64 go build ./cmd/opencode-profiles
```

## Testing

- Tests construct an isolated `Paths` via `t.TempDir()` and pass it directly into ops functions — no global state or monkeypatching.
- CLI tests invoke `run(args, &buf, &errBuf, paths, dbPath)` and assert on exit code and buffer contents.
- DB writes are isolated by passing a nonexistent `dbPath` (never touches the real `~/.cc-switch/cc-switch.db`).
- The Go implementation's behavioral equivalence to the original Python version was verified with a temporary comparison harness during the migration (`feat: port CLI to Go` through `chore: remove Python implementation` commits).

## Conventions

- Commit messages follow Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`).
- Go 1.25+ (see `go.mod`).
- Dependencies managed with `go.mod`/`go.sum`; only two external deps (`gopkg.in/yaml.v3`, `modernc.org/sqlite`).

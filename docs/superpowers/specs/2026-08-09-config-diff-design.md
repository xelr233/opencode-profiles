# Config Diff Design

## Goal

Add two capabilities to the CLI:

1. **Show config diff** — a `-d`/`--diff` command that compares configuration between two profiles across four dimensions: `provider`, `mcp`, `plugin`, and `skill`.
2. **Show changes on switch** — `-s <name>` prints the diff between the current active profile and the target profile *before* performing the switch.

The `db` (cc-switch.db) is managed by cc-switch itself and is **out of scope** — existing `SyncSkills`/`UpdateDB` behavior is unchanged.

## Architecture

### `internal/diff/diff.go` — New package

Pure parsing and set-comparison logic, no symlink manipulation.

```go
type Change struct {
    Added   []string
    Removed []string
}

type Result struct {
    A, B      string
    Providers Change
    MCP       Change
    Plugins   Change
    Skills    Change
}

// Diff reads the opencode.json + skills.yml of both profiles and computes
// set differences. Both A and B must be concrete profile names.
func Diff(p *paths.Paths, a, b string) (*Result, error)

// Render prints the diff to w in sectioned form.
func Render(w io.Writer, r *Result)
```

Parsing rules:

- `provider` / `mcp` — read config top-level key as `map[string]any`, take its key set, compute set difference.
- `plugin` / `skills` — read string list (config `plugin`, `skills.yml`), compute set difference.
- All sets sorted for deterministic output.
- Reads directly from `p.ProfileConfig(name)` and `p.ProfileSkillsYML(name)` — no dependency on the active symlink.
- Missing profile config → error.
- Missing `skills.yml` → treated as empty list.
- Missing `provider`/`mcp`/`plugin` key → treated as empty set.

### `internal/ops/ops.go` — `SwitchDB` signature change

```go
func SwitchDB(p *paths.Paths, name, dbPath string, out io.Writer) error
```

Flow (in order):

1. `EnsureInitialized`
2. `from := GetActive(p)` — resolve current profile before switching
3. Validate target profile exists (existing behavior) — this runs *before* diff so a missing target keeps the friendly `Profile 'x' not found. Available: ...` error
4. `result, err := diff.Diff(p, from, name)` — diff is informational: on error, print a `Warning: could not diff profiles: ...` line to `out` and continue switching (never block a switch because a config is malformed — otherwise a corrupted active profile could not be escaped via `-s`)
5. `diff.Render(out, result)` — print changes to the injected writer
6. Perform symlink switch + tui.json + skills sync (existing logic unchanged)

`Switch(p, name)` is kept for backwards compatibility:

```go
func Switch(p *paths.Paths, name string) error {
    return SwitchDB(p, name, "", io.Discard)
}
```

Callers of `SwitchDB` in `main.go` and tests must be updated to pass a writer (`os.Stdout` / buffer).

## CLI (`cmd/opencode-profiles/main.go`)

New flags:

```
-d, --diff <profile>          # compare active profile vs <profile>
-d, --diff <profileA> <profileB>  # compare two profiles
```

Argument rules:

- `-d work` → resolve current via `GetActive`, diff current vs `work`.
- `-d work personal` → diff `work` vs `personal`.
- `-d` with 0 args or more than 2 positional args → error, non-zero exit.
- `-d` is mutually exclusive with other operational flags (`-s`, `-c`, `-e`, `-b`, skill commands).

`-s <name>` now shows the diff first (via `SwitchDB` writer), then switches.

## Output Format

```
Diff: default -> work

[provider]
  - meituan
  + deepseek

[mcp]
  + codegraph

[plugin]
  - opencode-visual-cache@latest
  + opencode-subagent-magazine@latest

[skill]
  + rtk
  - brainstorming
```

- Each section renders only if it has changes; unchanged dimensions are omitted.
- `-` = removed (present in A, absent in B); `+` = added (present in B, absent in A).
- All within a dimension sorted alphabetically.
- If nothing differs: `No differences between '<A>' and '<B>'`.

## Edge Cases

| Scenario | Behavior |
|----------|----------|
| Profile A or B doesn't exist | Error, non-zero exit |
| Active profile unavailable for `-d <profile>` | Error, non-zero exit |
| Missing `provider`/`mcp`/`plugin` key | Treated as empty set |
| Missing `skills.yml` | Treated as empty list |
| No differences | Prints `No differences between ...` |
| Switch to profile with no diff | Prints no-diff line, still switches |
| `-s` target config is malformed JSON/YAML | Prints `Warning: could not diff profiles: ...`, still switches |

## Testing

- `internal/diff/diff_test.go` — two profiles with differing provider/mcp/plugin/skills; identical profiles → empty Result; missing profile → error; missing keys treated as empty.
- `internal/ops/ops_test.go` — `SwitchDB` with injected writer asserts diff output printed; db logic unchanged (nonexistent dbPath still ignored).
- `cmd/opencode-profiles/main_test.go` — `-d work` (vs active), `-d work personal`, `-d` with no args → error, `-d` with 3 args → error, `-s` prints diff before "Switched to".

## Out of Scope (YAGNI)

- No field-level recursive diff (provider `models`/`options` internals not expanded).
- No changes to db / cc-switch integration.
- No colors / terminal decorations.

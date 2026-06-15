# AGENTS.md

Instructions for AI agents working in this repository.

## What this project is

`p` is a Go CLI that wraps `systemd` with a pm2-like UX. Every command shells
out to `systemctl` / `journalctl` — there is no D-Bus client, no daemon, no
state outside the systemd unit files on disk. Keep it that way unless there's
a concrete reason to add a dependency.

## Layout

- `main.go` — entry point, nothing else belongs here
- `cmd/` — one file per subcommand, all wired up in `cmd/root.go`
- `cmd/unitflags.go` — shared `unitFlags` struct + `bindEditUnitFlags` helper used by `edit`
- `internal/systemd/`
  - `systemctl.go` — `systemctl` wrapper, mode detection (user vs system), unit list
  - `unit.go` — `UnitConfig` struct + `Render()` writes the on-disk unit
  - `parse.go` — reverse of `Render()`; only parses keys `Render()` knows about
  - `stats.go` — parses `systemctl show` properties into a `Stats` struct
- `internal/ui/` — colors, terminal-aware table, key/value card, formatters

## Conventions you must preserve

- **Unit naming.** Every managed unit is `p-<name>.service`. `systemd.List()`
  filters on this prefix. Do not introduce other prefixes; do not let user
  input bypass `nameRE` in `cmd/start.go`.
- **Mode.** `systemd.CurrentMode()` decides `--user` vs `--system` based on
  `geteuid()`. Anything new that shells out to `systemctl` / `journalctl` must
  go through that.
- **Render ↔ Parse round-trip.** If you add a field to `UnitConfig`, update
  `Render()` *and* `ParseUnit()`, otherwise `p edit` silently drops the value.
- **Command wrapping.** `ExecStart` is always `/bin/sh -c '<quoted>'`. Keep
  `shellQuote`/`shellUnquote` symmetric. If you change the wrapping format,
  every existing on-disk unit becomes unparseable.
- **No `--force` escape hatches.** The user explicitly rejected them. Validate
  early or not at all; do not add "but you can override with --force" flags.

## Defaults that are intentional

- `Restart=always`, `RestartSec=5`, `IPAccounting=yes`, `--auto-start=true`
- `WorkingDirectory` defaults to whatever `os.Getwd()` returns at `p start`
  time, not the user's `$HOME`.
- `PATH` from the caller's shell is baked into the unit at `p start` time
  (`--inherit-path`, default on). Without this, services can't find tools in
  `~/.local/bin` etc. — that's how the user hit "uv not found". Additional
  vars via `--inherit-env KEY`. User-supplied `-e KEY=…` always overrides
  inherited values for the same key. Inheritance is **not** automatic in
  `p edit`; users updating an existing unit must pass `-e PATH=$PATH`.
- `p logs` defaults to 50 lines and follows. `--no-follow` to turn off.
- `p status` samples CPU% with a 250ms window in parallel goroutines.

## Build & smoke-test

```sh
go build ./...
go build -o /tmp/p . && /tmp/p --help
```

There are no automated tests yet. If you add a feature, prefer a small
focused test over a sprawling suite — `internal/systemd/parse.go` and the
table renderer are the most test-worthy.

## UI rules

- Output must survive a 40-column terminal. The `ui.Table` drops columns by
  priority; new columns need a sensible `Priority` (0 = required, higher =
  drop first).
- ANSI codes are emitted via `fatih/color`. Use `ui.visibleLen` /
  `ui.StripANSI` when measuring strings, never `len()`.
- Status output uses one-line summaries (`● <name> <verb>`). Don't add prose.

## Things to avoid

- Don't add a config file or a state directory. Systemd is the source of
  truth.
- Don't shell out to `diff`, `awk`, `sed`, etc. — Go stdlib only.
- Don't catch and swallow systemctl errors; surface them via the
  `fmt.Errorf("…: %w", err)` chain so the root `Execute()` can print them red.
- Don't break the `p-` prefix invariant in a refactor.

## Committing

Commits go straight to `main`. Keep messages short and imperative
(`add reload command`, `fix table truncation on narrow terms`). The user
pushes to `https://github.com/ccev/p`.

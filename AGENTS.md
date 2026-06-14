# AGENTS.md

## Build & Test

```bash
# Build
go build -o mvave-chocolate-tui .

# Run
go run .

# Test (all)
go test -count=1 -timeout 30s ./...

# Test (headless E2E, needs device in U mode)
go test -v -run TestHeadless ./...

# Lint
golangci-lint run ./...
```

## Architecture

```
main.go          — Bubbletea TUI (model, update, view, keybindings)
midi.go          — Raw ALSA MIDI I/O + amidi subprocess writes
sysex.go         — SysEx message construction with verified checksums
detect.go        — Auto-detect SINCO device from /proc/asound/cards
midi_e2e_test.go — Headless E2E tests against real device
main_test.go     — TUI tests via teatest
sysex_test.go    — Checksum unit tests (18 known-good examples)
```

## Important code patterns

- **All MIDI writes are async** — `sendAllConfig`, `sendModeChange`, `requestConfig` each snapshot `midiDev` and start a goroutine. Status updates flow through `m.midiMsgs` channel.
- **Channel drain at top of Update** — `m.midiMsgs` is drained on every `Update()` call (not just `default:` case) to prevent goroutine deadlocks.
- **amidi for writes** — raw file writes to ALSA midi can block. Using the `amidi` subprocess (which uses libasound) avoids this.
- **O_RDONLY for reads** — the read goroutine opens the device read-only, separate from amidi's write access.
- **Checksum formula** — for config messages (header `00 32 09 49`): `checksum = 0x28A - sum_all - subcommand - value`. See `sysex_test.go` for all known-good test vectors from the cbix repo.

## SysEx protocol

Vendor `00 32` (Jieli). See [cbix/mvave-chocolate-sysex](https://github.com/cbix/mvave-chocolate-sysex) for the full reverse-engineered protocol.

Key messages:
- Mode change: `F0 00 32 09 49 ... <mode> <cs> F7`
- CC config: `F0 00 32 09 49 ... <sub> 00 00 00 10 00 00 00 <cc> <cs> F7`
- Read settings: `F0 00 32 0D 41 00 00 00 02 00 00 00 00 10 7E 00 00 07 00 F7`
- OK response: `F0 00 32 01 08 00 00 00 00 7F 01 F7`

## Dependencies

- `charmbracelet/bubbletea` — TUI framework
- `charmbracelet/bubbles` — table, viewport, help widgets
- `charmbracelet/lipgloss` — styling
- `charmbracelet/x/exp/teatest` — E2E testing
- `amidi` (system) — ALSA MIDI sends

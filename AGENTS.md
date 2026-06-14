# AGENTS.md

## Build & Test

```bash
# Build
go build -o mvave-chocolate-tui .

# Run
go run .

# Test (all packages)
go test -count=1 -timeout 30s ./...

# Test (headless E2E, needs device in U mode)
go test -v -run TestHeadless ./internal/midi/

# Lint
golangci-lint run ./...
```

## Architecture

```
tui.go                          — Bubbletea TUI (model, update, view, keybindings), main()
tui_test.go                     — TUI tests via teatest
internal/
  midi/
    device.go                   — Raw ALSA MIDI I/O + amidi subprocess writes
    device_test.go              — Headless E2E tests against real device
  sysex/
    protocol.go                 — SysEx message construction with verified checksums
    protocol_test.go            — Unit tests (18 known-good examples + quick)
  detect/
    detect.go                   — Auto-detect SINCO device from /proc/asound/cards
docs/
  sysex-protocol.md             — Full SysEx protocol reference
.golangci.yml                   — Linter config
```

## Important code patterns

- **All MIDI writes are async** — `sendAllConfig`, `sendModeChange`, `requestConfig` each snapshot the `*midi.Device` and start a goroutine. Status updates flow through `m.midiMsgs` channel.
- **Channel drain at top of Update** — `m.midiMsgs` is drained on every `Update()` call to prevent goroutine deadlocks.
- **amidi for writes** — the `midi` package uses `amidi` subprocess (libasound) for writes to avoid blocking on raw ALSA MIDI.
- **O_RDONLY for reads** — the `midi` package opens the device read-only, separate from amidi's write access.
- **Checksum formula** — for config messages (header `00 32 09 49`): `checksum = 0x28A - sum_all - subcommand - value`. See `internal/sysex/protocol_test.go` for all 18 known-good test vectors from the cbix repo.
- **Auto-detect via /proc** — `internal/detect/detect.go` scans `/proc/asound/cards` for `SINCO`/`FootCtrl`/`USB-Midi`.

## SysEx protocol

Vendor `00 32` (Jieli). See [cbix/mvave-chocolate-sysex](https://github.com/cbix/mvave-chocolate-sysex) for the full reverse-engineered protocol. Our `internal/sysex` package provides:

- `BuildModeChange(mode)` — switch operating mode
- `BuildCCConfig(idx, cc, latch, chan)` — set CC for a switch position
- `BuildReadSettings()` — request current device config
- `BuildDiscovery()` — device discovery
- `BuildInitSequence()` — 8-message handshake preamble
- `Checksum(data)` — 14-bit SysEx checksum
- `KnownExamples` — 18 pre-verified test vectors

## Dependencies

- `charmbracelet/bubbletea` — TUI framework
- `charmbracelet/bubbles` — table, viewport, help widgets
- `charmbracelet/lipgloss` — styling
- `charmbracelet/x/exp/teatest` — E2E testing
- `amidi` (system) — ALSA MIDI sends

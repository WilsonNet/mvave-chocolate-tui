# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build -o mvave-chocolate-tui .        # build binary
go run .                                  # run (auto-detects device)
go run . --write                          # allow SysEx writes
go run . /dev/snd/midiC5D0                # explicit ALSA path
./mvave-chocolate-tui --version           # stamped by the release workflow

# Unit tests (device tests skip when no pedal is present)
go test -count=1 -timeout 120s ./...
go test -p 1 -count=1 ./...               # serialise packages when a pedal is attached

# Device E2E tests (pedal in U mode; one process owns the port)
go test -p 1 -v -timeout 120s -run TestHeadless ./internal/midi/
go test -p 1 -v -timeout 60s -run "TestTUI" .

golangci-lint run ./...                   # lint

# Clear a stale MIDI fd / amidi process
fuser -k /dev/snd/midiC* 2>/dev/null

# Website
cd site && npm run dev                    # local docs site
cd site && npm run build                  # static build in site/dist
```

## Architecture

A Bubble Tea TUI in package `main`, backed by three internal packages:

```
tui.go                      — model/update/view, table screen, connect + read loop
mode_ui.go                  — operating-mode picker, write + verify
pedal.go                    — model-neutral Config/Device, Plus and original decoders
pedal_ui.go                 — the pedal screen (switches, banks, groups, edits)
pedal_backup.go             — export/import/compare backups
internal/sysex/protocol.go  — legacy builders, modes, checksums, known vectors
internal/sysex/image.go     — read requests, reply decoding, BuildWrite, Image
internal/sysex/plus.go      — Chocolate Plus layout and write planning
internal/midi/device.go     — ALSA I/O: raw fd reads, amidi subprocess writes
internal/detect/detect.go   — scans /proc/asound/cards for SINCO/FootCtrl/USB-Midi
site/                       — Astro Starlight documentation site (GitHub Pages)
```

The `Model` holds the table's 16 `SwitchConfig{CC int; Latching bool}` entries,
a buffered `midiMsgs chan MidiMsg` (cap 256), the partial SysEx frame
(`rxPending`), and the decoded `pedalImage`/`device`/`cfg` used by the pedal
screen. The table persists to `~/.config/mvave-chocolate-tui/config.json`;
backups go to `~/.config/mvave-chocolate-tui/backups/`.

## Critical patterns

**Read-only by default.** On connect the TUI only sends the settings read
requests. Every write path (`s`, `w`, `m`, mode change) is gated on
`m.allowWrite`, which is only true with `--write`. Connecting must never change
the pedal.

**Plan, acknowledge, read back.** Writes show the addresses and old → new
values first; each message waits for the `00 32 01 08` ACK; the configuration
is then read again and must match. Do not report success from an ACK alone.

**Async MIDI ops.** `sendAllConfig`, `requestConfig`, mode writes and pedal
writes snapshot the `*midi.Device` and run in a goroutine, reporting through
`m.midiMsgs` (or the `pedalAck` channel for writes). Never mutate `m` from a
goroutine.

**Channel drain.** Each `Update()` drains `m.midiMsgs` before handling the
message, so a full channel cannot deadlock a sender.

**amidi for writes.** `midi.Device.SendSysex` shells out to
`amidi -p hw:X,Y,0 -S <hex>`; raw ALSA writes block the UI. Reads use the raw
fd (O_RDONLY). `amidi --timeout` is in seconds (inactivity), and send + receive
must be a single invocation.

**SysEx reassembly.** A settings reply is 1,173 bytes over many reads.
`parseMidiBytes` keeps an incomplete frame in `m.rxPending` and rejoins it;
never drop a partial `F0 …` frame.

**Checksum.** For `00 32 09 49` messages the device checksums the *decoded*
message: each 4-byte group of 7-bit bytes is a 28-bit little-endian number;
sum those numbers' 8-bit bytes plus the value byte, double it, and subtract
from 660. Split the result into two 7-bit bytes. The old
`0x28A - sum - sub - value` agrees only while the address's second byte is
even. See `Checksum` and `KnownExamples`.

**Model recognition is shape-based.** `devices` tries the Plus first, then the
original. If neither matches, stay on the table screen — never write one
model's layout over another. The Plus stores message lists and groups; the
table's `s` path writes the original's sixteen-position layout.

**Switch indexing.** Positions are `[0..15]`, label `{bank+1}{A..D}`
(index 5 → `2B`). On the pedal screen the four physical switches are A–D, and
banks are per-switch.

**Mode IDs differ per model.** Do not port a mode byte between models blindly
(Plus Advanced Custom is `0x03`; the original's is `0x08`/`0x09` depending on
which source you believe — see `docs/development.md`).

## Testing on hardware

- Pedal side switch on **U**; `amidi -l` must show `SINCO MIDI 1`.
- One reader owns the raw port. Close DAWs, captures and other TUI instances.
- `go test ./...` runs packages in parallel; with a pedal attached, use `-p 1`
  or device tests can fight over the port.
- `amidi -p hw:X,0,0 -S <hex> -d --timeout 3` is the ground truth for a single
  message; send and receive in one invocation.
- Back up (`x`) before experimenting, then confirm with the dump after.

## Documentation

- `docs/development.md` — protocol orientation, hardware checklist, sources.
- `site/src/content/docs/` — the published guides (Astro Starlight).
- `docs/sysex-protocol.md` — legacy protocol notes; `reference/protocol.md` on
  the site is the current summary.

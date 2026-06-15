# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build -o mvave-chocolate-tui .       # build binary
go run .                                 # run (auto-detects device)
go run . /dev/snd/midiC5D0             # run with explicit device path

# Unit tests (no device required)
go test -v -run "TestKnown|TestBuild|TestChecksum|TestCC|TestModel|TestSend|TestFind|TestTUIMode|TestTUILog" ./...

# Device E2E tests (requires Chocolate in U mode)
go test -v -timeout 60s -run TestHeadless ./internal/midi/
go test -v -timeout 60s -run TestTUIAdvancedCustomSendAllConfig .

# Full test suite (skips device tests when no device present)
go test -count=1 -timeout 120s ./...

golangci-lint run ./...                  # lint

# Clear orphaned amidi processes (if "device busy" errors occur)
fuser -k /dev/snd/midiC* 2>/dev/null || kill -9 $(pgrep -f "amidi.*hw:5") 2>/dev/null
```

## Architecture

Single-package TUI (`tui.go` + `main`) backed by three internal packages:

```
tui.go                      — Bubbletea model/update/view + main()
tui_test.go                 — Unit + E2E tests (package main; uses teatest)
internal/midi/device.go     — ALSA MIDI I/O; reads via O_RDONLY fd, writes via amidi subprocess
internal/midi/readback_test.go — Headless device E2E tests (require physical device)
internal/sysex/protocol.go  — SysEx builders + checksum + all protocol constants
internal/detect/detect.go   — Scans /proc/asound/cards for SINCO/FootCtrl/USB-Midi
```

The `Model` struct holds 16 `SwitchConfig` entries (4 banks × 4 switches) plus a buffered `midiMsgs chan MidiMsg` (cap 256) for goroutine→UI communication.

## Critical patterns

**Async MIDI ops**: `sendAllConfig`, `sendModeChange`, `requestConfig` each snapshot the `*midi.Device` pointer and spawn a goroutine. Results return via `m.midiMsgs`. Never mutate `m` from inside those goroutines.

**Channel drain**: Every `Update()` call drains `m.midiMsgs` completely before processing the incoming message. This prevents goroutine deadlocks when the channel fills.

**amidi for writes**: `midi.Device.SendSysex` shells out to `amidi -p hw:X,Y,0 -S <hex>` for writes. Raw ALSA writes block the UI; amidi doesn't. Reads use the raw fd (O_RDONLY). amidi `--timeout` is in **seconds** (inactivity), not milliseconds.

**Checksum formula** (config messages, header `00 32 09 49`):
```
checksum = 0x28A - sum(all_bytes) - subcommand - value
```
Split into two 7-bit bytes: `low = v & 0x7F`, `high = (v >> 7) & 0x7F`. See `KnownExamples` in `protocol.go` for 18 verified test vectors.

**Switch indexing**: Switches are `[0..15]`. Bank = `idx/4`, switch-in-bank = `idx%4`. Label format: `{bank+1}{A..D}` (e.g., index 5 → `2B`).

**Mode dispatch in sendAllConfig**: After the init sequence and mode re-assertion, `sendAllConfig` dispatches by mode:
- `ModeAdvancedCustom` → `BuildAdvCustomCC` (per-switch subcmds, 3 msgs per switch)
- all other modes → `BuildCCConfig` (bank-shared subcmds, 2 msgs per switch)

## SysEx protocol reference

Vendor `00 32` (Jieli). Partial reverse-engineering at [cbix/mvave-chocolate-sysex](https://github.com/cbix/mvave-chocolate-sysex); additional protocol discovered via headless probing in this repo.

| Function | Purpose |
|---|---|
| `BuildInitSequence()` | 12-message handshake — always send before config writes |
| `BuildModeChange(mode)` | Switch operating mode (value in `ModeCustom`, `ModeAdvancedCustom`, etc.) |
| `BuildCCConfig(idx, cc, latch, chan)` | Custom CC mode: 2 msgs (CC + latch) for bank `idx/4`; use `SplitMessages` |
| `BuildAdvCustomCC(idx, cc, latch, chan)` | Advanced Custom: 3 msgs (CC#, latch, type) for per-switch `idx` 0–15 |
| `BuildReadSettings()` | Request 0D 49 config dump — **mode byte live, CC values STATIC** |
| `IsOKResponse(frame)` | Check for `00 32 01 08` ACK |
| `ParseConfigResponse(frame)` | Decode 0D 49 frame — only `Mode` field is reliable |

### Protocol constants (`internal/sysex`)

**Custom CC mode** (subcmds in `09 49` messages, byte[10]=`0x00`):
```
CustomCCSubcmdBase    = 0x02   // bank 0 CC; bank n = 0x02 + n*CustomBankStride
CustomLatchSubcmdBase = 0x03   // bank 0 latch
CustomBankStride      = 2
```

**Advanced Custom mode** (subcmds in `09 49` messages, byte[10]=`AdvCustomLiveWrite=0x0E`):
```
AdvCustomSubcmdBase    = 0x30  // switch 0, attr 0
AdvCustomSwitchStride  = 4     // 4 subcmds (attrs) per switch
AdvCustomAttrCC        = 0     // CC# value
AdvCustomAttrLatch     = 1     // latch mode (0=momentary, 1=latching)
AdvCustomAttrType      = 2     // switch type
AdvCustomLiveWrite     = 0x0E  // byte[10]: write reaches device RAM immediately
AdvCustomSwitchTypeCC  = 0x07  // switch type: outputs CC message
```

Subcmd formula: `AdvCustomSubcmdBase + switchIndex*AdvCustomSwitchStride + attr`
- Switch 0 CC: subcmd `0x30`, switch 3 CC: subcmd `0x3C`, switch 15 CC: subcmd `0x6C`

### Readback caveat (reverse-engineered)

The `0D 41` → `0D 49` response:
- **Mode byte IS live** — updates immediately when `BuildModeChange` is sent.
- **CC values are STATIC** — the 1173-byte config dump does NOT update when `09 49` CC writes are sent (confirmed via byte-diff test with CC=99). `ParseConfigResponse.CC` returns structural bytes from the dump, not the live CC assignments.
- `BuildReadSettings()` is only reliable for reading the current operating mode.
- Two read variants exist: the standard `10 7E` request (1173 bytes) and the `10 6A` variant (990 bytes, all zeros — no useful data).

### Init sequence note

`BuildInitSequence()` message 3 forces `ModeCustom (0x07)`. `sendAllConfig` re-asserts the user's chosen mode immediately after the init sequence to counteract this.

## Device requirements

- Chocolate side switch must be on **U** (USB), not HOST
- Verify with `amidi -l` — should show `SINCO MIDI 1`
- "device or resource busy": run `fuser -k /dev/snd/midiC*` to clear stale fd
- Combined send+receive: use `amidi -p dev -S <hex> -d --timeout <secs>` (one invocation); separate send then receive loses the ACK because the port closes between calls

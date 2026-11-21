# Developing for the M-VAVE Chocolate / Chocolate Plus

This is the orientation for anyone changing the protocol layer or testing on a
real pedal. The published guides live on the site (`site/src/content/docs/`);
this file is the shorter engineering companion.

## Setup

- Go **1.26+** (`go.mod`), `alsa-utils` for `amidi`.
- Build: `go build -o mvave-chocolate-tui .`
- Run: `./mvave-chocolate-tui` (reads), `./mvave-chocolate-tui --write`
  (allows writes). `--version` prints the release stamp.
- Tests: `go test ./...`; device tests skip when no pedal is found.

## The device

Both models are Jieli USB-MIDI devices (vendor `00 32`, USB ID `4353:4b4d`,
product string `SINCO`). The USB name does **not** identify the model.

- Configuration lives in **region 2**, 23,646 bytes, read with 24 `0D 41`
  requests (the official app's own requests, kept in `sysex.ReadRequests`).
- Replies are `0D 49` (`0D 79` for the last chunk), with a 7-bit packed
  payload. `DecodeReadReply` returns the address and bytes; `Image` collects
  them.
- Writes are one byte per `09 49` message. The 14-bit address is two 7-bit
  bytes. `byte[10]` is the address's high part: `0x00` is the persistent
  field, `0x0E` a live copy 1,792 bytes higher.
- Discovery is the reliable identity:
  `F0 00 32 45 00 00 00 40 7F F7` replies with a 7-bit packed ASCII name such
  as `FootCtrlPlus_010` or `FootCtrl_031`.
- Every accepted write is acknowledged with
  `F0 00 32 01 08 00 00 00 00 7F 01 F7`.

## Checksums

For `09 49` messages, checksum the *decoded* message: treat each 4-byte group
of 7-bit bytes as a 28-bit little-endian number, add those numbers' 8-bit
bytes plus the value byte, double the total, and subtract from 660. The result
is two 7-bit bytes.

The historical `0x28A - sum - subcommand - value` is equivalent only while the
address's second byte is even. Address 178 exposed the difference; the
`KnownExamples` table pins every captured vector.

## Model differences (do not port addresses)

| | Original Chocolate | Chocolate Plus |
| --- | --- | --- |
| Advanced Custom mode | `0x08` per `FootCtrl_051` disassembly; `0x09` in older notes | `0x03` |
| Switch storage | 16 slots × 4 attributes at `0x30 + 4·i` | 417-byte records: mode, two message banks, message lists |
| Groups | none | 1–8 (E/F), group count and current group |
| Messages per switch | up to 16 per event | up to 16 per bank |
| Expression jack | 1/4" TRS | 3.5 mm TRS |

The TUI's recognition is deliberately shape-based. A pedal whose configuration
does not match is left on the table screen; writing the wrong layout could
destroy its message lists. A Plus that is not recognised is a **detection**
issue, not permission to write the table.

## Hardware test checklist

1. Side switch on **U**; `amidi -l` shows `SINCO MIDI 1`.
2. Close every other reader (`fuser -v /dev/snd/midiC5D0`).
3. Read the config and save a dump; prefer a backup from the TUI (`x`).
4. Test one message at a time with
   `amidi -p hw:X,0,0 -S <hex> -d --timeout 3`.
5. Change a value, wait for the ACK, then read back and diff every byte.
6. Restore the original value and confirm the dump is byte-identical
   (ignore the device's volatile counters if a region changes on its own).
7. Use `go test -p 1` with a pedal attached; parallel packages fight over the
   raw port.

The `herdr` skill in `.opencode/skills/herdr/` explains how to run the TUI in a
second terminal pane and drive it with keys while reading the screen.

## Testing without a pedal

- Device tests skip automatically when `detect.Find()` fails.
- Golden SysEx vectors and `Image` decoding tests need no hardware.
- `snd-virmidi` (`modprobe snd-virmidi`) gives a raw MIDI port for framing
  tests; `amidi -p virtual -d` reads it.
- The TUI's test model uses `teatest`; golden files are ignored by git.

## Sources

The protocol summary on the site has the practical map. The long-form research
behind it (with confidence tags) draws on:

- [cbix/mvave-chocolate-sysex](https://github.com/cbix/mvave-chocolate-sysex) —
  captured messages, legacy checksum table, discovery, ACK.
- [majabojarska/OpenChocolate](https://github.com/majabojarska/OpenChocolate) —
  decompiled Plus app, blob layout, 24-read sequence.
- [mendong/blackboxctl](https://github.com/mendong/blackboxctl) — the `00 59`
  grammar and base-128 codec.
- M-VAVE manuals and firmware (`FootCtrl_051`, `FootCtrlPlus_012/_013`); the
  original/Plus mode tables and product specs are linked from the site's
  [operating modes](/mvave-chocolate-tui/using/modes/) and
  [use cases](/mvave-chocolate-tui/use-cases/modelers/) pages.

## Open questions

- Original Chocolate `0x08` vs `0x09` for Advanced Custom: the cbix table and
  the firmware disassembly disagree. Verify the pedal's display before
  changing the constants.
- Whether the `0x700` live copy is RAM or a second persistent region.
- The original's 16-slot space has no official documentation.
- BLE configuration uses a vendor GATT tunnel (`ae40`/`ae41`/`ae42`), not
  BLE-MIDI SysEx. This TUI is USB-only.

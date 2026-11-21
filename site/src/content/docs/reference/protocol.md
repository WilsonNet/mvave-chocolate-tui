---
title: SysEx protocol
description: The Jieli vendor protocol behind the TUI, with verified and inferred parts separated.
---

This page summarises what the TUI relies on and what the community has
established. The long-form, source-linked notes live in the repository; this is
the practical map.

The pedal is a Jieli-based USB MIDI device (vendor `00 32`). It exposes the
standard USB-MIDI class plus vendor SysEx for configuration. Configuration
lives in **region 2**, read in chunks with `0D 41` requests and written one
byte at a time with `09 49` messages.

## Acknowledgements

Successful writes and mode changes are answered with:

```text
F0 00 32 01 08 00 00 00 00 7F 01 F7
```

The TUI treats that as "the device accepted this message". It then re-reads
the configuration; an ACK alone is not proof that the value changed.

## Reading

The official app reads 23,646 bytes in 24 requests, each a `0D 41` with an
address and a length. The TUI reuses those requests (`sysex.ReadRequests`) so
they carry valid checksums. Replies are `0D 49` for every chunk except the last,
which is `0D 79`.

Each reply header contains three 28-bit words (region, address, length × 16)
followed by 7-bit packed data, two checksum bytes and `F7`. The TUI decodes a
reply with `DecodeReadReply` and collects the bytes at their addresses in an
`Image` map.

A full reply is 1,173 bytes and arrives over several reads. The read loop keeps
an incomplete frame (`rxPending`) and rejoins it with the next read; a frame cut
at any point must not be dropped.

## Writing

One `09 49` message changes one byte:

```text
F0 00 32 09 49 00 00 00 02 <addr lo> <addr hi> <byte10> 00 10 00 00 00 <value> <cs> <cs> F7
```

- The address is two 7-bit bytes, little-endian; 14 bits cover the readable
  region and the "live" copy.
- `byte[10]` selects the address's high part. `0x00` is the persistent field;
  `0x0E` is a live copy 1,792 bytes higher, which makes some changes take effect
  without a power cycle.
- The TUI's pedal screen writes both copies where the model needs it.

### Checksum

Older notes describe `0x28A - sum - subcommand - value`. That is correct only
while the address's second byte is even. The device actually checksums the
**decoded** message: each four-byte group of 7-bit bytes is a 28-bit
little-endian number, and those numbers' 8-bit bytes plus the value are summed
and doubled:

```text
value = 660 - 2 × (sum of the decoded bytes + value byte)
```

The TUI's `Checksum` uses that form, which agrees with the old formula for every
previously captured vector. Address 178 is the case that exposed it; a real
Chocolate Plus ignored the old checksum there.

## Model layouts

The two pedals do **not** share a layout, and this is the most dangerous part of
the protocol:

| | Original Chocolate | Chocolate Plus |
| --- | --- | --- |
| Advanced Custom mode | `0x08` per firmware analysis; `0x09` in older notes | `0x03` |
| Switches | 16 slots, four attributes each at `0x30 + 4·i` | 417-byte records per switch: mode, two message banks, message lists |
| Groups | none | 1–8, with a group count and a current group |
| Messages per switch | up to 16 per event | up to 16 per bank, one SysEx per event |

Writing one model's layout to the other can destroy its configuration. The TUI
only offers the pedal screen when the read *shape* matches a model, and stays on
the table when it does not.

## Discovery

```text
F0 00 32 45 00 00 00 40 7F F7
```

The reply carries a 7-bit packed ASCII name and version, for example
`FootCtrlPlus_010` or `FootCtrl_031`. Discovery is the most reliable model
identity; the USB product string is `SINCO` on both pedals.

## Open questions

- The original Chocolate's `0x08`/`0x09` mode IDs differ between the widely
  copied cbix table and the `FootCtrl_051` firmware disassembly.
- Whether the `0x700`-based "live copy" is truly RAM or a second persistent
  region.
- The classic model's `0x30…0x6F` space is consistent with four banks of four
  switches, but no official document describes the sixteen-position layout.
- BLE configuration is not BLE-MIDI SysEx: the app uses a vendor GATT service
  (`ae40`/`ae41`/`ae42`) with the same `00 59` frames. Use USB for
  configuration.

## Sources

- `docs/sysex-protocol.md` in the repository — the project's own notes.
- [cbix/mvave-chocolate-sysex](https://github.com/cbix/mvave-chocolate-sysex) —
  captured messages, checksums, discovery, ACK.
- [majabojarska/OpenChocolate](https://github.com/majabojarska/OpenChocolate) —
  decompiled Plus app, blob layout, 24-read sequence.
- [mendong/blackboxctl](https://github.com/mendong/blackboxctl) — the `00 59`
  grammar and base-128 codec shared by these Jieli devices.
- M-VAVE manuals and firmware notes; links in
  [Use cases](/mvave-chocolate-tui/use-cases/modelers/) and the repository.

Claims above are marked where they are inferred rather than verified. The TUI's
tests pin every checksum vector the project has captured.

---
title: Operating modes
description: What each M-VAVE mode sends, and how the TUI changes modes safely.
---

The pedal has one mode at a time. A mode decides what the four switches send:
MIDI notes and controllers, keyboard keys, media keys, or vendor-specific
actions. Only the MIDI modes matter to a DAW or a modeler.

Press `m` in either screen. Pick a mode, confirm, and the TUI writes the mode
byte, waits for the acknowledgement and reads it back. Without `--write`, the
picker refuses and nothing is sent.

## Chocolate Plus modes

| ID | Mode | What the switches send |
| ---: | --- | --- |
| `0x00` | Program Change A | Program Change, groups stepped with E/F |
| `0x01` | Program Change B | **CC 0–127**, not a second PC mode |
| `0x02` | Custom CC | Per-switch CC with momentary/latching behaviour |
| `0x03` | Advanced Custom | Per-switch message lists, five trigger styles, groups |
| `0x04` | Manufacturer | Internal vendor control |
| `0x05` | Touch Screen | Android swipes (HID) |
| `0x06` | Video | Rewind / forward / play-pause / loop (HID) |
| `0x07` | Keyboard A | Arrow keys (HID) |
| `0x08` | Keyboard B | Page Up / Page Down / Space / Enter (HID) |
| `0x09` | Multimedia | Previous / next / volume (HID) |
| `0x0A` | Custom Keyboard | One key or modifier combo per switch (HID) |
| `0x0B` | Mix Key | Per switch: MIDI, keyboard or tuner |
| `0x0C` | Speaker | Controls M-VAVE speaker products |

Sources: the Plus manual and OpenChocolate's decompilation. The Plus picker in
the TUI offers the modes it knows for the model; mode IDs are **not** shared
with the original Chocolate.

## Original Chocolate modes

| ID | Mode | What the switches send |
| ---: | --- | --- |
| `0x00` | Program Change A | PC 0–31 in eight groups |
| `0x01` | Program Change B | PC 0–127 in 32 groups |
| `0x02` | Touch Screen | Android swipes (HID) |
| `0x03` | Keyboard A | Arrows (HID) |
| `0x04` | Keyboard B | Page Up / Down / Space / Enter (HID) |
| `0x05` | Multimedia | Media keys (HID) |
| `0x06` | Manufacturer | Fixed CC 20–23, expression CC 7 |
| `0x07` | Custom CC | Per-switch CC, channel 1–16 |
| `0x08` | Advanced Custom | Per-switch messages, trigger styles |
| `0x09` | Video | Media playback (HID) |
| `0x0A` | Custom Keyboard | One key per switch (HID) |
| `0x0B` | Program Change C | CC 0–127 in 32 groups |

:::caution[There is an open discrepancy]
Most public notes — and the `cbix/mvave-chocolate-sysex` table this project
started from — list `0x08` as Video and `0x09` as Advanced Custom. Disassembly
of the `FootCtrl_051` firmware reports the opposite (`0x08` draws the `AD`
start screen). The contributor's constants follow the older table. If an
original Chocolate ends up in the wrong mode after `m`, this is the first thing
to check; the mode picker will be corrected once a unit confirms it.
:::

## Modes that send no MIDI

Keyboard, multimedia, video and touch-screen modes are **HID**: they type keys
or media commands over the USB HID interface, not MIDI. If a switch press does
not appear in the TUI log or in `amidi -d`, check the mode before assuming a
broken pedal.

## The display is the source of truth

The pedal's display shows the mode's start screen (`AD` for Advanced Custom,
`N4` for Custom Keyboard, and so on). After a mode change, look at the pedal,
not only at the TUI's read-back: the TUI can only report the byte it wrote.

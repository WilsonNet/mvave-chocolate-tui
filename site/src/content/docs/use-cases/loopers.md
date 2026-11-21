---
title: Loopers
description: Record, overdub, stop and clear on HX Stomp, RC-5, Ampero and Ableton.
---

Loopers want one thing from a footswitch: a **momentary** CC at 127 that the
looper maps to record, overdub, stop or clear. Latching switches cause
double-triggers.

## Line 6 HX Stomp

The HX Stomp's looper is controlled by fixed CCs:

| CC | Action |
| ---: | --- |
| 60 | Record / overdub |
| 61 | Play / stop |
| 62 | Stop |
| 63 | Clear |
| 64 | Undo / redo |
| 65 | Forward |
| 66 | Reverse |

Set the pedal's channel to the Stomp's base channel and use one switch per CC,
momentary. Use Advanced Custom on the Plus if one switch should send more than
one message.

Source: Line 6 HX Stomp 3.80 Owner's Manual, MIDI chapter.

## Boss RC-5

- **PC 1–99** recalls memories.
- The assignable **CC 80–87** can be mapped to record, play, stop, clear and
  the other looper functions in the RC-5's MIDI settings.

Because the CCs are assignable, write down the mapping you choose; a factory
reset returns them to defaults.

Source: Roland RC-5 Reference Manual, "Controlling Devices via MIDI".

## Hotone Ampero II

The official Ampero II MIDI list includes looper controls in the CC 62–72
range. Check the list for your firmware version and map those numbers directly.

Source: Hotone *Ampero II MIDI Control Information List*.

## Ableton Live

Ableton's Looper is mapped like any other device: enter MIDI Map mode, click
the Looper's record/play/clear buttons, and step on the switches. Use a
momentary switch for record and a latching one for a loop you want to hold.

## Wiring a loop station

If you use the pedal's **TRS-MIDI** output instead of USB, the pedal must be in
a MIDI mode (Custom CC or Program Change), not a keyboard/media mode, and the
TRS cable must match the pedal's wiring (Type A is the common choice). The
TUI only configures the pedal; it is not in the MIDI path during a show.

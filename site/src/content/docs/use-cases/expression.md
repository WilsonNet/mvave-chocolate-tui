---
title: Expression pedal
description: Map the TRS expression input to volume, wah or any CC, and avoid the reversed-range trap.
---

Both Chocolate models have a TRS expression input. The pedal's expression
setting decides what the jack sends; the TUI configures the switch behaviour,
not the expression calibration.

## What the jack sends

- In **Custom CC** mode, expression can send a CC (or Program Change / Channel
  Pressure on some firmware). Choose the CC and the MIDI channel in the
  official app.
- In **Manufacturer** mode, the expression jack sends CC 7 (volume) by default.
- In keyboard/media modes, expression does nothing MIDI.

The TUI does not expose expression calibration. Set it once in the official
app, then use the TUI for the switches.

## Common targets

| Target | Typical CC |
| --- | --- |
| Volume | CC 7 |
| Expression / wah | CC 11 or the modeler's own expression CC |
| Modulation | CC 1 |
| Fractal scene / expression | community recipe uses CC 16 |

The receiving device's manual wins: modelers remap expression freely.

## The reversed-range trap

On many pedals, **heel down sends the maximum value**. A switch or a setting
called "polarity" does not always fix it; check the monitor first:

```bash
amidi -p hw:5,0,0 -d
```

Move the expression pedal slowly. If the values run 127 → 0, reverse the range
in the receiving device, or swap the tip/ring wiring only if your cable and
device documentation allow it. Do not guess with a TRS cable wired for a
different pedal.

## Cables and connectors

- The original Chocolate uses a **1/4"** jack; the Plus uses **3.5 mm**. Use
  the right plug or a solid adapter.
- TRS-MIDI is a different use of the same connector on some pedals. Do not plug
  a MIDI cable into the expression jack.
- Expression pedals are potentiometers, not MIDI devices. A expression pedal
  with a TS plug may work partially or not at all; TRS is the supported wiring.

## Test before a show

1. Set the expression CC in the app.
2. Open `amidi -p hw:5,0,0 -d` and sweep the pedal.
3. Confirm the full range and direction.
4. Then map the same CC in the modeler or DAW.

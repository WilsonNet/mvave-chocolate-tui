---
title: Guitar & amp modelers
description: Preset switching with Program Change and block toggles with CC for Ampero, Valeton, HX Stomp, Kemper and plugins.
---

A modeler listens for two kinds of message: **Program Change** to recall a
preset, and **Control Change** to switch a block, tap tempo or open the tuner.
The pedal can send both; the receiving device decides which number does what.

:::note[Value pairs are not universal]
A block "on" is CC value 127 and "off" is 0 on most gear, but Kemper stomps
expect **1 and 0**, and some hosts treat any value **≥ 64** as on. A switch
that looks dead is often sending the right CC with the wrong value.
:::

## Preset switching

**In the pedal:** choose Program Change mode, then set a switch's channel and
program number.

1. Press `m` and pick **Program Change A**.
2. On the switch table or pedal screen, edit switch `1A`.
3. Set **Type** `PC`, **Channel** to the modeler's MIDI channel, **Number** to
   the preset.
4. Send the mode and the switch, then step on `1A`.

Program Change A on the original Chocolate covers PC 0–31 in eight groups;
Program Change B covers 0–127 in 32 groups. On the Plus, Program Change A
covers PC and Program Change B is actually a **CC** mode — if you expected a
second PC mode, that is the trap.

## Block toggles

**In the pedal:** Custom CC is the simplest: one CC per switch, momentary,
value 127 on press and 0 on release. Advanced Custom (Plus) is for sending
several messages from one switch.

Common starting points:

| Device | What to check first |
| --- | --- |
| **Hotone Ampero II** | Official *MIDI Control Information List* maps CCs for blocks, looper and tap; set the same channel in both. |
| **Line 6 HX Stomp** | MIDI is fully assignable in Command Center; the manual's CC 60–66 are looper controls, not a fixed block map. |
| **Kemper Profiler** | Stomps are CC 20–23 with value **1/0**; scenes CC 23 with 0–7; tuner CC 15. Community-verified. |
| **Valeton GP-5** | Listens on channel 1. CC #0 selects a preset number; block toggles use 0/127. |
| **Valeton GP-100 / GP-200** | Consult the model's own MIDI list; numbers differ by firmware. |
| **Neural DSP plugins** | Use the plugin's MIDI Learn: right-click a control, move the switch, done. |
| **BOSS GT-1000** | No fixed CC map. Map CCs under *MIDI → MIDI SETTING*; any "GT-1000 CC list" is wrong. |

Sources: Kemper MIDI documentation (May 2024); Line 6 HX Stomp 3.80 manual;
Hotone Ampero II MIDI list; Morningstar's GP-5 guide; BOSS GT-1000 guide;
Neural DSP MIDI setup guide.

## Tap tempo and tuner

Both are ordinary CCs on most modelers. Assign a switch to the CC the device
documents, momentary, and send only that switch. The TUI writes the whole
configuration, so change one value and leave the rest as read.

## Channel is part of the address

A CC is (channel, number, value). The pedal and the modeler must agree on the
channel. If a switch works in the TUI log but the modeler ignores it, change
the channel before changing the CC.

## Verified on this project's hardware?

The protocol parts (mode change, CC write, read-back) are checked on a
Chocolate Plus. The modeler CC numbers above come from manufacturer manuals or
clearly marked community notes; they were not re-tested device by device.

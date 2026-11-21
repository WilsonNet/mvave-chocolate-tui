---
title: DAWs
description: Transport, record, undo and loop control in Ableton Live, REAPER and Ardour.
---

A DAW is the easiest target: it learns whatever the pedal sends, so the pedal
can keep a simple, consistent map.

## Ableton Live

1. Preferences → **Link, Tempo & MIDI**: enable **Remote** for the pedal's
   input (track and sync can stay off).
2. Press `Cmd/Ctrl + M` to enter MIDI Map mode.
3. Click **Play**, step on a switch, then repeat for Stop and Record.
4. Exit MIDI Map mode.

Notes:

- Ableton treats a **Note On** as a toggle and CC values **≥ 64** as "on", so
  momentary 127/0 switches work for transport.
- For a switch that should stay on (a loop or a mute), use latching on the
  pedal, or a CC range that stays above 64.
- Close the TUI before mapping if Ableton complains about a busy port; both
  cannot read the raw device at once. The TUI logs presses but does not forward
  them to the DAW.

## REAPER

1. Options → Preferences → **MIDI Devices**.
2. Right-click the pedal input and enable **Enable input for control messages**.
3. Open the Action List, find an action, click **Add**, and move a switch.

REAPER's MIDI learn is per action; there is no separate "remote" switch like
Ableton's. For transport, map *Transport: Play/stop* and *Transport: Record*.

## Ardour

1. Window → **MIDI Connections** (or the Audio/MIDI setup) and connect the
   pedal's MIDI output to Ardour's **control** input.
2. Open the mixer or a plugin control, middle-click it, then move a switch.
3. Save the binding map if you want it to survive a restart.

Ardour learns on the *control* port, not on a track's MIDI input. Generic MIDI
learn also does not work on every plugin's own GUI; use Ardour's generic
controls for those.

## A practical foot map

| Switch | Message | DAW action |
| --- | --- | --- |
| A | CC 20, 127/0 | Play / stop |
| B | CC 21, 127/0 | Record |
| C | CC 22, 127/0 | Undo |
| D | CC 23, 127/0 | Metronome or loop |

Pick any numbers your DAW does not already use. The point of Custom CC mode is
that the pedal's map stays stable while each DAW learns it once.

## Bluetooth and latency

USB is the reliable path for recording. Bluetooth MIDI works for transport and
mapping, but community measurements of the Plus report roughly 90–100 ms, and
short taps can be missed. For punch-ins, use the cable.

Sources: Ableton Live 12 manual, chapter 34 (MIDI and Key Remote Control);
REAPER's MIDI device documentation; Ardour's Generic MIDI Learn and binding-map
pages.

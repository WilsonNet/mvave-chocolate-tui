---
title: First run
description: Connect the pedal, read it without writing, and learn the two screens.
---

## Before connecting

1. Connect the pedal with a USB-C **data** cable.
2. Put the side switch on **U** (USB), not HOST.
3. Check that Linux sees it:

```bash
amidi -l
# Dir Device    Name
# IO  hw:5,0,0  SINCO MIDI 1
```

`hw:5,0,0` is an example; the card number depends on your machine. The TUI
also finds the device through `/proc/asound/cards`, so you normally do not
need the path.

:::caution
Only one process can read a raw MIDI device. Close DAWs, `amidi -d` captures
and other TUI instances before starting it.
:::

## Read-only by default

```bash
mvave-chocolate-tui
```

The first thing it does is ask the pedal for its settings. It does **not**
switch modes, apply a saved configuration or write to flash. The status line
says `Connected (read-only)` and the header shows the mode it read.

If the pedal answers with a layout the TUI knows, the **pedal screen** opens;
otherwise you get the **switch table**. Both are described next.

## Allowing writes

```bash
mvave-chocolate-tui --write
```

`--write` does not send anything by itself. It enables the actions that do:
`s` in the table, `w` on the pedal screen, and `m` (mode change) everywhere.
Every one of them still asks for confirmation and reads the device back.

## Keys at a glance

| Key | Table screen | Pedal screen |
| --- | --- | --- |
| `e` / `enter` | Edit the selected switch | Edit the selected switch |
| `s` | Send the whole table | — |
| `w` | — | Write the staged edits |
| `r` | Read again | Read again |
| `m` | Change mode | Change mode |
| `x` / `i` | — | Back up / restore |
| `v` | — | Mark a read, then compare |
| `tab` / `l` | Toggle the MIDI log | Toggle the MIDI log |
| `q` | Quit | Quit |

## Pick a device explicitly

Auto-detection checks `/proc/asound/cards` for SINCO/FootCtrl/USB-Midi names.
If you have several MIDI devices, pass the raw path:

```bash
mvave-chocolate-tui --write /dev/snd/midiC5D0
```

## What is read, and what is not

The TUI reads configuration region 2 (23,646 bytes) with the same 24 requests
the official app uses. Mode, switches, groups and message lists are decoded
from that image. Some fields are deliberately not interpreted, for example
per-device names (the pedal stores none) and expression-pedal calibration.
See the [protocol reference](/mvave-chocolate-tui/reference/protocol/) for the
raw details.

---
title: Overview
description: What M-VAVE Chocolate TUI does, which pedals it supports, and the read-first safety model.
---

M-VAVE Chocolate TUI is a Linux terminal application for reading and changing
what an M-VAVE Chocolate or Chocolate Plus footswitch sends. It talks to the
pedal directly over USB MIDI using the vendor's Jieli SysEx and needs no
Windows VM, Wine, browser or account.

It is built around one rule: **opening it must never change your pedal**.

## What it does

- Reads the pedal's configuration and shows the sixteen switch positions as a
  table, or the four physical switches as a pedal screen.
- Edits what each switch sends: CC, Program Change, Note, channel, value,
  latching, and on the Plus up to sixteen messages per switch.
- Changes the operating mode, with a confirmation, an acknowledgement and a
  read-back.
- Backs up every byte it can read, restores a backup as edits, and lists what
  changed between two reads.
- Writes only when started with `--write`, one byte at a time, and only counts
  a change if the device acknowledged it and the read-back matches.

## Supported pedals

| Pedal | Read | Write | Notes |
| --- | --- | --- | --- |
| **Chocolate Plus** | Yes | Yes | The pedal screen, groups, message lists and mode switching were contributed and checked against a real Plus. |
| **Original Chocolate** | Yes | Restricted | The pedal screen for Advanced Custom is built from the same decoder and writer but has **not been checked on hardware**; the screen says so. Other modes use the switch table. |

Both models share a USB identity (`SINCO`, USB ID `4353:4b4d`), so the TUI
identifies the model from the configuration it reads, not from the USB name.
If a pedal is not recognised, the TUI keeps the generic table screen instead
of assuming a layout.

## Safety model

1. **Read on connect.** Connecting sends one settings read request, nothing
   else.
2. **Explicit writes.** `--write` enables sending, mode changes and the pedal
   screen's write key. Without it, those actions explain why they did nothing.
3. **Show the plan.** Before sending, the TUI lists each address and its
   old → new value.
4. **Acknowledgement, then read-back.** Each message must be answered, then the
   configuration is read again. A write that is not reflected does not count.

The [protocol reference](/mvave-chocolate-tui/reference/protocol/) documents
what is verified, what is inferred, and what is still open.

## Where next

- [Install](/mvave-chocolate-tui/install/) the released binary with one command.
- [First run](/mvave-chocolate-tui/first-run/) connects the pedal and explains
  the two screens.
- [Use cases](/mvave-chocolate-tui/use-cases/modelers/) has recipes for amp
  modelers, DAWs, loopers and page turning.

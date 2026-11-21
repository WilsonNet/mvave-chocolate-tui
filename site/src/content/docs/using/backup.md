---
title: Backup, restore and compare
description: Export every readable byte, restore it as edits, and diff two reads.
---

The pedal screen can save and replay what it reads. Backups are ordinary JSON
files under `~/.config/mvave-chocolate-tui/backups/`, named with the local date
and time.

## Export (`x`)

`x` writes every byte the current read collected. It does not send anything to
the pedal. Keep one before a firmware update, a big edit, or a first
experiment.

## Import (`i`)

`i` loads a backup as **edits**, not as a blind write. The screen marks the
changes, then `w` shows the byte plan and sends them with the same
acknowledgement and read-back rules. Nothing is written until you confirm.

## Compare (`v`, then `r`)

1. Press `v` to mark the current read.
2. Change something on the pedal (or in the TUI and write it).
3. Press `r` to read again.

The screen then lists every byte that differs, with the address, old → new
value, and what the model thinks it is. This is how the layouts in this
documentation were mapped; it is also the honest way to check whether a write
did what you expected.

## What a backup does not include

- Firmware, calibration and fields outside the readable configuration region.
- Switch names (the TUI stores those locally in `config.json`; the pedal has
  none).
- Anything the model does not decode. The backup stores the raw bytes, so an
  unrecognised field is still preserved on restore.

---
title: The switch table
description: The sixteen-position grid used when a pedal is in a mode the TUI does not draw as a pedal.
---

The table is the fallback screen and the editing screen for CC-style setups.
It lists sixteen positions, labelled bank + switch: `1A` through `4D`.

<figure>
  <img class="shot" src="/mvave-chocolate-tui/shots/table.png" width="999" height="1253" alt="The switch table with 16 rows, CC numbers, latch flags and press state." />
  <figcaption>The table after a read. The highlighted row is the selected switch.</figcaption>
</figure>

## What each column means

| Column | Meaning |
| --- | --- |
| **Switch** | Position `1A`…`4D`. Bank = index / 4, switch = index % 4. |
| **CC** | Controller number the switch sends in Custom CC / Advanced Custom mode. |
| **Latch** | `yes` = press toggles 127/0; `no` = momentary (127 on press, 0 on release). |
| **State** | Live press indicator while the TUI runs. |

The table shows the configuration saved by the TUI in
`~/.config/mvave-chocolate-tui/config.json`. It is what `s` sends. Use `r` to
read the pedal's current mode; decoded per-switch values are shown on the
[pedal screen](/mvave-chocolate-tui/using/pedal-screen/) when the model is
recognised.

## Edit a switch

1. Move with the arrow keys or `h`/`j`/`k`/`l`.
2. Press `e`.
3. Edit **CC** and **Latch** (`tab` or arrows to move, `enter` to save,
   `esc` to cancel).

<figure>
  <img class="shot" src="/mvave-chocolate-tui/shots/edit.png" width="1493" height="539" alt="Editing switch 1A: fields for CC (0-127) and Latch (yes/no)." />
  <figcaption>Editing one position. Values are checked before they can be saved.</figcaption>
</figure>

## Send the table

With `--write`, press `s`. The TUI:

1. switches the pedal to Advanced Custom mode,
2. writes CC and latch for all sixteen switches, and
3. saves the table to `~/.config/mvave-chocolate-tui/config.json`.

Without `--write`, `s` reports that it is read-only and sends nothing.

:::caution[Which layout does `s` use?]
The table writes this TUI's sixteen-position Advanced Custom model. That model
matches the original Chocolate's layout. A Chocolate Plus stores per-switch
message *lists* instead, so use the [pedal screen](/mvave-chocolate-tui/using/pedal-screen/)
for a Plus rather than `s`. If your Plus is not recognised and the TUI falls
back to this table, do not send: report the read instead.
:::

## The MIDI log

`tab` or `l` opens a scrolling log of every SysEx frame sent and received, and
live note/CC traffic from the switches. It is the fastest way to see whether a
press actually sends MIDI or only an HID key (keyboard, media and video modes
send no MIDI).

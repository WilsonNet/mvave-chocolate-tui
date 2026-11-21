---
title: The pedal screen
description: The four-switch view for the Chocolate Plus and for the original Chocolate in Advanced Custom mode.
---

When a complete read matches a model the TUI knows, it replaces the table with
a drawing of the pedal: four switches, the selected one in detail, and live
presses traced to the switch that sent them.

The screen is model-aware:

- **Chocolate Plus** — switch modes with trigger styles, two message banks per
  switch, up to sixteen messages each, and groups E/F step through.
- **Original Chocolate, Advanced Custom** — sixteen slots, four attributes
  each (CC, latch, switch type). This layer is built from the same decoder and
  writer as the table but has **not been checked on hardware**; the header says
  so.

## Keys

| Key | Action |
| --- | --- |
| `←` `→` / `a`–`d` | Select a switch |
| `enter` | Edit the selected switch |
| `n` | Name a switch (stored locally; the pedal has no names) |
| `w` | Write staged edits (needs `--write`) |
| `u` | Discard edits, back to the last read |
| `x` / `i` | Export a backup / import one as edits |
| `v`, then `r` | Mark this read; the next read lists what changed |
| `[` / `]` | Previous / next group (Plus) |
| `+` / `-` | How many groups E/F step through (Plus; written with `w`) |
| `p` | Put the pedal on the group being viewed (Plus; written with `w`) |
| `r` | Read again |
| `m` | Change operating mode |
| `tab` / `l` | MIDI log |
| `q` | Quit |

## Editing and writing

`enter` opens the selected switch's fields. On the Plus, each bank holds a list
of messages: type (PC, CC, Note On, Note Off), channel, number and value.
`n`/`x` add and remove messages when the model allows it.

Staged changes are marked `● unwritten edits`. Pressing `w` does not send
immediately; it shows the plan first:

```text
Write 2 byte(s) to the pedal?
  address   48    48 → 49
  address 1840    48 → 49
Each is sent on its own and must be acknowledged; the pedal is read back after.
```

`y` sends each byte in the order the pedal needs, waits for an acknowledgement
before the next, then reads everything back. If the read-back does not match,
the change is not reported as applied.

:::note
A write is not a firmware update and does not touch the pedal's mode unless
you asked for a mode change. Backup first (`x`) if you are experimenting.
:::

## Groups (Plus)

The Plus stores several groups of four switch assignments. The screen reads all
of them with the official app's own 24 read requests, shows the group count and
the group the pedal is currently on, and lets you page through them. Pressing
`p` moves the pedal to the group being viewed; the group count set with `+` /
`-` is written with `w`.

## If your Plus is not recognised

Model recognition looks at the *shape* of the configuration, not just the USB
name. A Plus whose settings differ enough from the reference layout can fail
the check; the TUI then stays on the table and says the mode was read but does
not draw a pedal. That is deliberate: writing the sixteen-position table layout
over a Plus's message lists could destroy them. Send a backup/read report rather
than pressing `s`.

The Plus layout and protocol are documented in the
[protocol reference](/mvave-chocolate-tui/reference/protocol/).

---
title: FAQ
description: Short answers about models, modes, writes and safety.
---

### Does it work if my pedal is not in Advanced Custom mode?

Yes. Reading works in any mode. The switch table is available for CC-style
setups; the pedal screen appears when a read is recognised. `m` changes the
mode, and every change is read back.

### Which pedals are supported?

The Chocolate Plus is the primary target: reading, writing, groups, the pedal
screen and mode switching were checked on real hardware. The original
Chocolate's Advanced Custom screen is built from the same decoder but has not
been verified on a unit; the header says so.

### Does it write to flash? Will a change survive a power cycle?

The write path sends the configuration bytes the model uses, including the
persistent copy, and reads back to confirm. Whether a specific field survives a
power cycle depends on the model and the address. The TUI does **not** perform
firmware updates.

### Can it brick my pedal?

It only sends the documented configuration messages, never firmware. A wrong
layout written to a different model can corrupt its settings, which is why the
TUI refuses to draw or write a layout it does not recognise. Back up first (`x`)
if you are experimenting.

### Do I need the official app?

For expression-pedal calibration and firmware updates, yes. For switch setup
and mode changes, no.

### Does it work over Bluetooth?

No. The TUI is USB-only. Bluetooth MIDI works for playing; vendor configuration
over BLE is a separate protocol.

### Why does `s` do nothing?

Without `--write` it refuses on purpose. Start with `--write`; the status line
tells you when writes are enabled.

### Where are my settings stored?

The table and local switch names: `~/.config/mvave-chocolate-tui/config.json`.
Backups: `~/.config/mvave-chocolate-tui/backups/`. The pedal stores its own
configuration; the JSON is not a copy of the device state unless you exported a
backup.

### Can I use this in a live rig?

Configure with the TUI, then quit it. During a show, the pedal talks directly
to your modeler or DAW; the TUI is not in the MIDI path.

### Is it affiliated with M-VAVE?

No. It is an independent GPL-3.0 project. M-VAVE, Chocolate and Chocolate Plus
are trademarks of their owner.

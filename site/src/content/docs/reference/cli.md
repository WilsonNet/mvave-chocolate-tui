---
title: CLI & flags
description: Every flag and path the TUI uses.
---

```text
mvave-chocolate-tui [--write] [--version] [--help] [device]
```

| Argument | Meaning |
| --- | --- |
| `--write` | Enable SysEx writes. Without it the TUI reads, shows settings and refuses every write action. |
| `--version`, `-v` | Print the version (stamped at build time) and exit. |
| `--help`, `-h` | Print usage and exit. |
| `device` | Raw ALSA MIDI path, for example `/dev/snd/midiC5D0`. Omit it to auto-detect. |

## Auto-detection

With no path, the TUI scans `/proc/asound/cards` for `SINCO`, `FootCtrl` or
`USB-Midi` and opens the matching raw MIDI node. If several match, pass the
path explicitly:

```bash
amidi -l                 # find the card
mvave-chocolate-tui --write /dev/snd/midiC5D0
```

The card number changes when you replug or reboot. Prefer auto-detection
unless you have more than one compatible device.

## Files

| Path | Contents |
| --- | --- |
| `~/.config/mvave-chocolate-tui/config.json` | The table's sixteen switch settings and local switch names. |
| `~/.config/mvave-chocolate-tui/backups/*.json` | Backups exported with `x` on the pedal screen. |

Set `XDG_CONFIG_HOME` to move the configuration root. JSON is plain text and
safe to inspect or edit while the TUI is closed.

## Exit behaviour

- `--version` and `--help` exit 0 without touching MIDI.
- No device found prints a short message and exits 1.
- A device that cannot be opened reports the error and exits 1.
- `q` closes the MIDI fd and restores the terminal. If a run is killed, the
  port can stay busy until the process exits; see
  [Troubleshooting](/mvave-chocolate-tui/reference/troubleshooting/).

## Example sessions

```bash
# inspect only — the safe default
mvave-chocolate-tui

# make a mode change, then quit without sending a table
mvave-chocolate-tui --write
```

The TUI never daemonizes and never runs in the background. When it exits, the
pedal keeps whatever was written; the program does not.

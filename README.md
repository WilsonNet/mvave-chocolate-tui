# M-Vave Chocolate TUI

A terminal UI for configuring the M-Vave Chocolate Bluetooth MIDI footswitch controller on Linux.

No more Windows VM or Wine just to change footswitch assignments. Edit CC numbers, switch modes, and send SysEx config directly from your terminal.

## Features

- **Auto-detection** — finds your Chocolate (SINCO/FootCtrl) via `/proc/asound/cards`, no hardcoded paths
- **16-switch grid** — 4 banks × 4 switches, all visible and editable
- **Real-time MIDI monitor** — see footswitch presses as they happen
- **SysEx configuration** — sends proper Jieli-manufacturer SysEx to reprogram the device
- **Mode switching** — toggle between Custom CC, Program Change, Keyboard, and more
- **Non-blocking I/O** — writes use `amidi` subprocess; reads run in background goroutine
- **Connection recovery** — clean open/close, handles unplug/replug

## Install

One command — downloads the latest release, installs it to `~/.local/bin`, and
opens the TUI:

```bash
curl -fsSL https://raw.githubusercontent.com/WilsonNet/mvave-chocolate-tui/main/install.sh | bash -s -- --run
```

Drop `--run` to only install it. Add `--write` after `--` to allow writes:

```bash
curl -fsSL https://raw.githubusercontent.com/WilsonNet/mvave-chocolate-tui/main/install.sh | bash -s -- --run -- --write
```

The script checks the release checksum, supports `--dir DIR` and
`--version vX.Y.Z`, and refuses to run anywhere but Linux. Prebuilt tarballs for
`amd64` and `arm64` are on the [Releases](https://github.com/WilsonNet/mvave-chocolate-tui/releases)
page if you would rather install by hand.

## Requirements

- **Go** 1.26+ only to build from source (managed via [asdf](https://asdf-vm.com/))
- **alsa-utils** (`amidi` command) for sending configuration
- M-Vave Chocolate in **U (USB)** mode (not HOST)

## Quick start from source

```bash
go run .
```

The TUI auto-detects your Chocolate, reads its settings and shows the switch
grid. **Connecting never writes to the device.** To send configuration, start
it with `--write`:

```bash
go run . --write
```

## Usage

| Key | Action |
|-----|--------|
| `e` | Edit highlighted switch |
| `s` | Send all config to device (needs `--write`) |
| `r` | Read current config from device |
| `m` | Change operating mode (needs `--write`; read back to verify) |
| `tab` / `l` | Toggle MIDI log view |
| `↑↓←→` / `h/j/k/l` | Navigate switch grid |
| `q` | Quit |

### Editing a switch

1. Press `e` on the switch row you want to edit
2. Tab between fields: **Type** (cc/pc/note), **CC/Note**, **Channel**, **On Value**, **Off Value**, **Latching**
3. Press `Enter` to save the edit
4. Press `s` to send all changes to the device

### Connecting to your Ampero / multi-effects

1. Set the Chocolate to **Custom CC** mode (press `m`, select "Custom CC", press `Enter`)
2. Edit each switch to match the CC numbers your Ampero expects
3. Press `s` to send (start with `--write`)
4. Connect the Chocolate MIDI OUT to your Ampero MIDI IN

## The pedal screen (Chocolate Plus, and the original in Advanced Custom)

When a read is recognised as a model the TUI knows, it switches to a pedal
screen: the four switches drawn as they sit on the pedal, the selected one in
detail, and presses as they happen, each traced to the switch that sent it.

| Key | Action |
|-----|--------|
| `←→` / `a`–`d` | Select a switch |
| `enter` | Edit it: `↑↓` field, `←→` or `-`/`+` change, digits type a value |
| `n` | Name the switch (kept locally; the pedal stores no names) |
| `w` | Write the edits: lists every byte first, then sends each and waits for the device's OK, then reads back to verify (needs `--write`) |
| `u` | Discard the edits |
| `x` / `i` | Back up every byte read / restore a backup as edits |
| `v` then `r` | Mark this read; the next read lists every byte that changed, and what it is |
| `m` | Change the operating mode (needs `--write`; read back to verify) |
| `[` / `]` | View the previous / next group (Plus) |
| `+` / `-` | Change how many groups E/F step through (Plus; written with `w`) |
| `p` | Put the pedal on the group being viewed (Plus; written with `w`) |
| `r` | Read again |

- **Chocolate Plus** -- checked against a real pedal. The whole configuration
  is read with the official app's own 24 read requests: all eight groups, the
  group count and the group the pedal is on. Groups were mapped from the
  official app's USB traffic.
- **Original Chocolate, Advanced Custom mode** -- built from this TUI's own
  decoder and writer and **not yet checked on hardware**; the screen says so.
  Reports welcome. Other modes keep the table above.

## How it works

The M-Vave Chocolate is a Jieli-based USB MIDI device (vendor `00 32`). It accepts SysEx configuration messages to reprogram what each footswitch sends. The cbix project at [cbix/mvave-chocolate-sysex](https://github.com/cbix/mvave-chocolate-sysex) documented the protocol.

- **Reads**: raw ALSA MIDI device (`/dev/snd/midiC*`) opened O_RDONLY in a goroutine
- **Writes**: `amidi -p hw:X,0,0 -S <hex>` subprocess — uses ALSA library, never blocks the UI
- **Checksum**: `0x28A - sum(all bytes) - subcommand - value` (verified against 18 known-good examples from the cbix repo)

## Testing

```bash
# All tests (unit + E2E with real device)
go test -v ./...

# Unit tests only (no device needed)
go test -v -run "TestKnown|TestBuild|TestChecksum|TestCC" ./...

# Headless E2E (requires Chocolate in U mode)
go test -v -run TestHeadless ./...

# Lint
golangci-lint run ./...
```

## Troubleshooting

**"device or resource busy"** — a previous instance didn't close cleanly. Run:
```bash
fuser -k /dev/snd/midiC*
```

**TUI can't find the device** — make sure the Chocolate side switch is on **U** (USB), not HOST. Check with:
```bash
amidi -l
```
Should show `SINCO MIDI 1`.

**Config not persisting** — the Chocolate may store config in volatile memory. The SysEx is accepted (device responds OK) but some settings revert on power loss. The TUI reads current config on startup (`r` key).

## License

GPL-3.0-or-later. See [LICENSE](LICENSE).

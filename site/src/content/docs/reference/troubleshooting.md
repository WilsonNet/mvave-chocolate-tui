---
title: Troubleshooting
description: Fix busy ports, unread modes, unrecognised pedals and writes that do not stick.
---

## `Device or resource busy`

Another process is reading the raw port. The TUI can hold it while it runs; a
crashed instance, a DAW, or a background `amidi -d` can hold it after it exits.

```bash
pgrep -af 'mvave|amidi'
fuser -v /dev/snd/midiC5D0
fuser -k /dev/snd/midiC*        # last resort; not while a DAW is recording
```

Then start the TUI again. If it happens every time you close and reopen, quit
with `q` rather than killing the terminal, so the fd is released.

## `mode not read yet`

The read request was sent but no settings reply arrived. Check:

- the pedal is in **U** mode, not HOST;
- the cable is a data cable;
- `amidi -p hw:X,0,0 -S <read request> -d --timeout 3` gets a `0D 49` reply;
- nothing else holds the port.

A reply cut off mid-frame is reassembled by the read loop; a reply that never
starts usually means the request never reached the pedal.

## The header shows `[device mode=00?]`

That is the table screen telling you it read mode `0x00` (Program Change A) but
does not draw that mode. It is not an error. Use `m` to switch to a mode the
pedal screen understands, or stay on the table for CC editing.

## My Chocolate Plus shows the table, not the pedal screen

Model recognition is shape-based: the TUI needs the decoded configuration to
look like the layout it knows. A Plus whose settings differ (or a firmware
whose layout changed) can fail that check and fall back to the table. This is
the safe outcome — do **not** press `s`, which would write the original model's
sixteen-position layout over the Plus's message lists. Export a backup (`x`)
and report the read.

## Writes report success but the pedal behaves the same

1. The pedal may need the live copy: the pedal screen writes both addresses;
   verify the read-back value changed.
2. The mode may be wrong. Keyboard, media and video modes send HID, not MIDI,
   so the receiving device sees nothing. Check the pedal's display.
3. The receiving device may use a different channel or value pair. See the
   [mode value trap](/mvave-chocolate-tui/use-cases/modelers/).
4. Firmware updates reset settings; reapply after an update.

## `amidi: command not found`

Install `alsa-utils`:

```bash
sudo pacman -S alsa-utils     # Arch
sudo apt install alsa-utils   # Debian/Ubuntu
sudo dnf install alsa-utils   # Fedora
```

Reads work without it; writes and mode changes do not.

## The pedal is not detected

```bash
lsusb | grep -i sinco         # 4353:4b4d
cat /proc/asound/cards
amidi -l
```

No card usually means the side switch is on HOST, the cable is charge-only, or
the kernel has not bound the USB MIDI class. Replug, then check `dmesg` for the
device.

## Bluetooth works for notes but not for this TUI

The TUI is USB-only by design. Vendor configuration over BLE uses a separate
GATT tunnel; use USB for configuration and Bluetooth for performance.

## Still stuck

Open an issue with the output of `mvave-chocolate-tui --version`, `amidi -l`,
the pedal model, the firmware version from its display, and what the screen
shows:

<https://github.com/WilsonNet/mvave-chocolate-tui/issues>

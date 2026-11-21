---
title: Linux & testing
description: How ALSA raw MIDI behaves, why ports go busy, and how to test without a pedal.
---

## Two ways to reach a MIDI port

Linux exposes the same hardware twice:

- **Raw MIDI** (`/dev/snd/midiC*`): a byte pipe. The TUI opens it read-only to
  receive input.
- **ALSA sequencer** (`aconnect`, `amidi -p`, DAWs): a client/port graph with
  timestamps and routing.

Both can read the jack. Mixing them is the usual cause of `Device or resource
busy`: the raw device is exclusive per direction, so an `O_RDONLY` reader blocks
another reader until it closes. A writer can coexist with a reader; two readers
cannot.

Check who owns the port:

```bash
fuser -v /dev/snd/midiC5D0
cat /proc/asound/card5/midi0
```

`fuser -k /dev/snd/midiC*` clears a stale process. Do not run it while a DAW is
using the pedal.

## amidi

`amidi` is the reference tool for this device:

```bash
amidi -l                                   # list ports
amidi -p hw:5,0,0 -d                       # dump incoming MIDI
amidi -p hw:5,0,0 -S f000320d... -d -t 2   # send and receive in one invocation
```

Two traps:

- **Send and receive in the same invocation.** With `-S` alone the port closes
  when the send finishes, and the reply is lost. `-d`/`-r` in the same command
  keeps the input open; `--timeout` is an inactivity timeout in seconds
  (fractions work).
- **Device numbers change on replug.** `hw:5,0,0` is not stable; use
  auto-detection or re-list.

The TUI writes with a short-lived `amidi -S` subprocess, because raw ALSA writes
can block the UI; reads use the raw fd.

## USB MIDI framing

USB-MIDI sends 4-byte event packets (a cable/CIN header plus three bytes).
SysEx is split into 3-byte chunks with CIN `0x4` and a final `0x5`–`0x7`; ALSA
reassembles them, so a reader sees a byte stream and must wait for `F7` rather
than assume one read is one message. A 23 KB configuration read is thousands of
packets; drain continuously.

## Permissions

The active desktop user is normally in the `audio` group. Verify:

```bash
id -nG
ls -l /dev/snd/midiC*
```

If the node is not group-readable, add a udev rule for the USB ID or fix group
membership; do not run the TUI as root.

## Testing without hardware

The repository's device tests skip when no pedal is present, and the unit tests
run anywhere:

```bash
go test ./...                 # device tests skip when no device is found
go test -p 1 ./...            # serialise packages if a pedal is attached
```

With a pedal attached, package tests can compete for the same raw port.
Running with `-p 1` avoids that.

For a synthetic port:

```bash
sudo modprobe snd-virmidi
amidi -l                       # a Virtual Raw MIDI port appears
amidi -p virtual -d            # read it
```

`snd-virmidi` bridges raw MIDI and the sequencer; it is enough to test framing
and reassembly without a Chocolate. `snd-seq-dummy` provides a "Midi Through"
port that loops a client's output back to itself.

## Bluetooth MIDI

BLE-MIDI works through BlueZ's experimental MIDI support or WirePlumber, and is
fine for notes and CC during a show. Vendor configuration over BLE is a
different GATT tunnel, and the official app is the practical way to do it. For
this TUI, use USB.

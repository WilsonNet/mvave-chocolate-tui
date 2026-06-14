# SysEx Protocol Reference

The M-Vave Chocolate uses Jieli manufacturer SysEx (vendor `00 32`). This document summarizes the known protocol from [cbix/mvave-chocolate-sysex](https://github.com/cbix/mvave-chocolate-sysex).

## Message structure

All messages follow this pattern:
```
F0 00 32 <cmd> <sub> ... <data> ... <checksum_2bytes> F7
```

The checksum is a 14-bit value packed into two 7-bit MIDI bytes (little-endian).

## Operating modes

Switch the device to different operating modes by changing the value byte at position 16 (0-indexed from F0):

| Mode | Value | Hex |
|------|-------|-----|
| Custom CC | 0x07 | `F0 00 32 09 49 00 00 00 02 00 00 00 00 10 00 00 00 07 66 03 F7` |
| Program Change A | 0x00 | `F0 00 32 09 49 00 00 00 02 00 00 00 00 10 00 00 00 00 74 03 F7` |
| Program Change B | 0x01 | `F0 00 32 09 49 00 00 00 02 00 00 00 00 10 00 00 00 01 72 03 F7` |
| Program Change C | 0x0B | `F0 00 32 09 49 00 00 00 02 00 00 00 00 10 00 00 00 0B 5E 03 F7` |
| Keyboard A | 0x03 | `F0 00 32 09 49 00 00 00 02 00 00 00 00 10 00 00 00 03 6E 03 F7` |
| Keyboard B | 0x04 | `F0 00 32 09 49 00 00 00 02 00 00 00 00 10 00 00 00 04 6C 03 F7` |
| MultiMedia | 0x05 | `F0 00 32 09 49 00 00 00 02 00 00 00 00 10 00 00 00 05 6A 03 F7` |

## Custom CC mode — switch configuration

In Custom CC mode (0x07), each of the 16 switch positions (4 banks × 4 switches) can be independently configured.

### Bank 1 (switches 1A–1D)
- CC value: subcommand `0x02`, value byte sets CC number (0–127)
- Latching: subcommand `0x03`, value byte `0x00` = momentary, `0x01` = latching

### Bank 2 (switches 2A–2D)
- CC value: subcommand `0x04`
- Latching: subcommand `0x05`

### Bank 3 (switches 3A–3D)
- CC value: subcommand `0x06`
- Latching: subcommand `0x07`

### Bank 4 (switches 4A–4D)
- CC value: subcommand `0x08`
- Latching: subcommand `0x09`

### Example: set switch 1A to CC 49, momentary

```
F0 00 32 09 49 00 00 00 02 02 00 00 00 10 00 00 00 31 <cs> F7
F0 00 32 09 49 00 00 00 02 03 00 00 00 10 00 00 00 00 <cs> F7
```

## Reading current settings

Send the read command to request the device's current configuration:

```
F0 00 32 0D 41 00 00 00 02 00 00 00 00 10 7E 00 00 07 00 F7
```

The device responds with a large SysEx dump containing the current switch assignments. Response parsing is not yet implemented in the TUI.

## Discovery

```
F0 00 32 45 00 00 00 40 7F F7
```

Response contains device identifier bytes.

## OK acknowledgment

After each configuration command, the device responds:

```
F0 00 32 01 08 00 00 00 00 7F 01 F7
```

This is the standard "OK" message. If you don't see this, the config was not accepted.

## Init sequence

For reliable configuration, send this sequence before CC config:

```
F0 00 32 0D 41 00 00 00 02 00 00 00 00 10 7E 00 00 07 00 F7
F0 00 32 0D 41 00 00 00 02 71 07 00 00 10 6A 00 00 33 01 F7
F0 00 32 09 49 ... (mode change to Custom CC)
F0 00 32 09 49 ... (bank 1 init)
F0 00 32 09 49 ... (bank 2 init)
F0 00 32 09 49 ... (bank 3 init)
F0 00 32 09 49 ... (bank 4 init)
```

## Checksum algorithm

For messages with header `00 32 09 49` (17 data bytes):

```
checksum = 0x28A - sum(all 17 bytes) - subcommand_byte - value_byte
```

The result is packed as two 7-bit bytes: `low = checksum & 0x7F`, `high = (checksum >> 7) & 0x7F`.

For OK responses (header `00 32 01 08`): `checksum = 0x13A - sum(all bytes)`.
For discovery (header `00 32 45`): `checksum = 0x136 - sum(all bytes)`.

Verification: 18 known-good test vectors in `sysex_test.go`, all passing.

---
title: Install
description: One-line installer, manual release downloads, or build from source on Linux.
---

## Requirements

- **Linux** with ALSA. The released binaries are `linux/amd64` and
  `linux/arm64`.
- **`amidi`** from `alsa-utils`. Reads use the raw MIDI device; writes shell out
  to `amidi`.
- An M-VAVE **Chocolate** or **Chocolate Plus**, connected over USB-C with the
  side switch on **U** (not HOST).

Go 1.26+ is only needed to build from source.

## One-line install

```bash
curl -fsSL https://github.com/WilsonNet/mvave-chocolate-tui/releases/latest/download/install.sh | bash -s -- --run
```

The script:

1. resolves the latest GitHub release (or the one you pin),
2. downloads the tarball for your architecture,
3. verifies it against the release's `checksums.txt` (and refuses to install if
   the checksum file or entry is missing),
4. installs `mvave-chocolate-tui` to `~/.local/bin`, and
5. with `--run`, opens it in the current terminal.

Useful options:

```bash
# install only
curl -fsSL .../install.sh | bash

# a specific version, into a directory of your choice
curl -fsSL .../install.sh | bash -s -- --version v0.2.2 --dir ~/bin

# allow writes when it opens
curl -fsSL .../install.sh | bash -s -- --run -- --write
```

If `~/.local/bin` is not on your `PATH`, the script prints the line to add.
`--skip-checksum` exists for unusual mirrors, but it turns the verification off;
prefer fixing the download.

## Manual download

Every release has two tarballs and a checksum file:

<https://github.com/WilsonNet/mvave-chocolate-tui/releases>

Verify, unpack and install by hand:

```bash
sha256sum -c checksums.txt --ignore-missing
tar -xzf mvave-chocolate-tui_0.2.0_linux_amd64.tar.gz
install -m 0755 mvave-chocolate-tui_0.2.0_linux_amd64/mvave-chocolate-tui ~/.local/bin/
```

## Build from source

```bash
git clone https://github.com/WilsonNet/mvave-chocolate-tui
cd mvave-chocolate-tui
go build -o mvave-chocolate-tui .
./mvave-chocolate-tui --version
```

## Permissions

Most desktop distributions grant the active user access to ALSA devices
through the `audio` group. If `amidi -l` shows the pedal but the TUI reports a
permission error, check your user's groups and the device node:

```bash
id -nG
ls -l /dev/snd/midiC*
```

A udev rule is only needed on unusually locked-down systems; the common
failure is another process holding the port, not permissions. See
[Troubleshooting](/mvave-chocolate-tui/reference/troubleshooting/).

## Updating

Run the installer again. It replaces the binary in place. Your saved switch
table (`~/.config/mvave-chocolate-tui/config.json`) and backups are not
touched.

## Uninstalling

```bash
rm ~/.local/bin/mvave-chocolate-tui
rm -rf ~/.config/mvave-chocolate-tui   # optional: saved table, names, backups
```

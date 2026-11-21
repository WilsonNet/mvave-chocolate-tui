# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Stack

Astro + Starlight, built as a static site under `site/` and deployed to GitHub
Pages at the project URL (`https://wilsonnet.github.io/mvave-chocolate-tui/`).
The TUI itself stays a Go program; the site is documentation, not a web app.

## Users

- **Pedal owners on Linux** — musicians and tinkerers with an M-Vave Chocolate
  or Chocolate Plus who want to change what the footswitches send without a
  Windows VM, Wine, or the vendor app. They are in a terminal, have the pedal on
  USB in U mode, and want the job done in one sitting.
- **Contributors and reverse-engineers** — people adding support for another
  model or firmware, who need the SysEx protocol, memory layout, and hardware
  test procedure written down.

## Product Purpose

Configure an M-Vave Chocolate over USB from a Linux terminal: read the pedal,
see its switches and modes, edit what they send, write the change, and verify it
was applied. The documentation site exists so a new owner can go from unboxing
to a working pedal, and so a contributor can reproduce protocol findings on real
hardware.

## Positioning

A Linux-native TUI that talks to the pedal directly over ALSA raw MIDI, with
protocol knowledge checked against a real Chocolate Plus. The original
Chocolate's pedal screen is built from the same decoder and is marked untested
on hardware in the app and docs. It reads first and refuses to write unless
started with `--write`, so opening it cannot silently replace a setup made in
the official app.

## Operating Context

- Pedal connected over USB-C with the side switch on **U**, appearing as
  `SINCO MIDI` / `FootCtrl` on an ALSA raw MIDI port.
- Writes are Jieli vendor SysEx sent through `amidi`; reads use the raw MIDI fd.
- Users compare against the official mobile/desktop app, and expect to keep
  their existing configuration.
- The project ships as tagged GitHub releases, installed with a one-line shell
  script into `~/.local/bin`.

## Capabilities and Constraints

- Read-only until `--write`; every mode change is confirmed, acknowledged, and
  read back.
- Table view for 16 switch positions (4 banks × 4 switches); pedal screen for
  the Plus and for the original in Advanced Custom mode.
- Mode switching, CC/PC/Note message editing, multiple messages per switch
  (Plus), groups, backup/restore, mark-and-compare.
- Linux only (ALSA), Go 1.26+, GPL-3.0-or-later, public repository at
  `https://github.com/WilsonNet/mvave-chocolate-tui`.
- The Plus layer was verified by its contributor on a real Plus. The original
  Chocolate's pedal screen is built from the same decoder and is still marked
  untested on hardware in the app and the docs.

## Brand Commitments

- Project name: **M-Vave Chocolate TUI**. No logo exists; the name and the
  pedal's own vocabulary (switch, bank, group, mode) are the identity.
- Voice: direct, technical, honest about what is and is not tested.

## Evidence on Hand

- `README.md`, `docs/sysex-protocol.md`, `CLAUDE.md`/`AGENTS.md`.
- Real terminal captures of the TUI against a physical Chocolate Plus during a
  read-only session.
- 98+ Go tests including device E2E tests; known-good SysEx vectors.
- No invented testimonials, benchmarks, or user counts.

## Product Principles

1. **Never surprise the pedal.** Reading is always safe; writing is explicit,
   one byte at a time, acknowledged, and read back.
2. **Hardware is the source of truth.** A finding is documented only after a
   real device showed it, and untested paths say so.
3. **The terminal is the interface.** No daemon, no account, no browser.
4. **Explain the protocol, not just the buttons.** Users and contributors get
   the same documentation.

## Accessibility & Inclusion

No product-specific requirement was established. The site should keep readable
contrast, keyboard navigation, and reduced-motion behavior regardless.

---
name: M-VAVE Chocolate TUI
description: A silkscreen bench card for a pedal configuration tool.
colors:
  paper: "#e6e7e9"
  paper-raised: "#eff0f1"
  ink: "#15171b"
  ink-secondary: "#3a3e45"
  ink-muted: "#5d626b"
  hairline: "#c3c6cb"
  hairline-soft: "#d5d7da"
  signal: "#d9442a"
  signal-deep: "#a82f1a"
  terminal: "#101216"
  terminal-deep: "#0c0e12"
  terminal-text: "#d8dade"
  connected: "#1f8455"
typography:
  display:
    fontFamily: "Archivo Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "clamp(2.55rem, 5.1vw, 4.5rem)"
    fontWeight: 680
    lineHeight: 0.98
    letterSpacing: "-0.045em"
  headline:
    fontFamily: "Archivo Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "clamp(1.9rem, 3.2vw, 2.8rem)"
    fontWeight: 660
    lineHeight: 1.03
    letterSpacing: "-0.035em"
  body:
    fontFamily: "Archivo Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "17px"
    fontWeight: 400
    lineHeight: 1.55
  label:
    fontFamily: "Spline Sans Mono Variable, ui-monospace, monospace"
    fontSize: "0.72rem"
    fontWeight: 500
    lineHeight: 1.4
    letterSpacing: "0.12em"
  mono:
    fontFamily: "Spline Sans Mono Variable, ui-monospace, monospace"
    fontSize: "0.8rem"
    fontWeight: 400
    lineHeight: 1.65
rounded:
  none: "0px"
  control: "2px"
  plate: "5px"
  pill: "999px"
spacing:
  xs: "8px"
  sm: "14px"
  md: "22px"
  lg: "28px"
  xl: "48px"
  section: "84px"
components:
  install-strip:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    rounded: "{rounded.control}"
    padding: "15px 14px"
  matrix-cell:
    backgroundColor: "{colors.paper-raised}"
    textColor: "{colors.ink}"
    rounded: "{rounded.none}"
    padding: "10px 11px 12px"
  matrix-cell-active:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
  terminal-plate:
    backgroundColor: "{colors.terminal}"
    textColor: "{colors.terminal-text}"
    rounded: "{rounded.plate}"
  primary-action:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.paper}"
    rounded: "{rounded.control}"
    padding: "12px 18px"
  copy-button:
    backgroundColor: "transparent"
    textColor: "{colors.terminal-text}"
    rounded: "{rounded.control}"
    padding: "4px 9px"
---

# Design System: M-VAVE Chocolate TUI

## Overview

**Creative North Star: "The Silkscreen Bench Card"**

The site is the quick-start card that should have shipped inside the pedal's
box: cool paper stock, one dark ink, one signal orange, and a grid of switches
printed like a technical sheet. It treats the visitor as someone about to plug
hardware in, not someone browsing a SaaS landing page. The documentation keeps
the same inks but lowers its voice: a reading surface with tabular data,
hairline rules and terminal captures as evidence.

The system is flat by intention. Depth comes from two grounds — paper for
explanations, near-black for anything the pedal would say — and from hairlines
that behave like printed rules. Rounded cards, gradients and decorative glow
are absent; the one soft shadow belongs to the terminal plate, which is a real
window onto the tool.

**Key Characteristics:**
- Cool paper gray and ink, with one signal orange doing all the pointing.
- A 4×4 switch matrix that doubles as the documentation index.
- Condensed technical labels in mono; sentences in a grotesk.
- Terminal captures framed as evidence, never as decoration.
- Flat surfaces, hairline borders, almost no radius.

## Colors

Two inks on paper, plus a terminal black and one semantic green.

### Primary

- **Signal Orange** (`#d9442a`): the selected switch, the install prompt, the
  underline on primary links, the active matrix cell's label. It is the only
  warm color on the page outside the terminal captures.
- **Deep Signal** (`#a82f1a`): link hover and the emphasized word in the
  headline; the same hue, one step darker for contrast on paper.

### Neutral

- **Paper** (`#e6e7e9`): the page ground, a cool gray that keeps the terminal
  plates from looking like holes.
- **Raised Paper** (`#eff0f1`): the facts strip and the preview panel.
- **Ink** (`#15171b`): body text, the install strip, and the active matrix cell.
- **Secondary Ink** (`#3a3e45`): prose and captions.
- **Muted Ink** (`#5d626b`): metadata, captions, footnote copy.
- **Hairline** (`#c3c6cb`) and **Soft Hairline** (`#d5d7da`): rules, cell
  borders, table separators.

### Terminal

- **Terminal Black** (`#101216`) and **Terminal Deep** (`#0c0e12`): the plate
  body and its title bar; also the code/transcript ground.
- **Terminal Text** (`#d8dade`): monospaced content inside a plate.
- **Connected Green** (`#1f8455`): only the connected dot in a terminal bar.
  A state, never an accent.

### Named Rules

**The One Signal Rule.** Orange marks what is live, selected or clickable. It
never fills a large region and never appears twice in the same glance for two
different meanings.

**The Two Grounds Rule.** Explanations live on paper; device output lives on
terminal black. Never mix the two inside one component.

## Typography

**Display Font:** Archivo Variable (with system sans fallback)
**Body Font:** Archivo Variable
**Label/Mono Font:** Spline Sans Mono Variable

**Character:** A grotesk with a slightly condensed, engineering-drawing feel,
paired with a monospace that is reserved for values, commands and device
states. The pairing reads like a spec sheet: confident sentences, exact
numbers.

### Hierarchy

- **Display** (680, `clamp(2.55rem, 5.1vw, 4.5rem)`, 0.98): the landing
  headline only; maximum width 13 characters per line.
- **Headline** (660, `clamp(1.9rem, 3.2vw, 2.8rem)`, 1.03): section headings,
  max 20 characters per line.
- **Title** (650, `0.86rem`, 1.2): matrix cell names.
- **Body** (400, `17px`, 1.55, 52–58ch): prose; 65–75ch is the ceiling in docs.
- **Label** (500, `0.72rem`, `0.12em`, uppercase): install strip headers, table
  headers and metadata.

### Named Rules

**The Numbers Are Mono Rule.** CC values, addresses, modes, commands and byte
counts are set in Spline Sans Mono. Prose is never monospaced for flavor.

## Layout

A centered container of `1180px` with `28px` gutters (`18px` below 560px).
The landing alternates a two-column split (hero, read-first, matrix) with
full-width ledgers and a two-column install block. Sections are separated by
`84px` of vertical rhythm and a single hairline; the hero is the only section
without a top rule.

The full-page grid is 4×4 at desktop and 2 columns below 560px, matching the
pedal's switch layout. The docs site uses Starlight's content column at 46rem
with a 17rem sidebar; the same hairlines and paper ground carry over.

## Elevation & Depth

The system is flat. Paper and terminal black create the only depth; borders
are 1px hairlines. The terminal plate carries one soft, downward shadow
(`0 18px 40px -24px rgba(12,14,18,0.65)`) so it reads as a physical window, not
as a floating card. No hover lift, no glass, no glow.

## Shapes

Corners are square by default. Three exceptions: a 2px radius on controls
(install strip, copy button), a 5px radius on terminal plates, and a full pill
for the version chip. Borders are always 1px and always a hairline, never a
colored left stripe.

## Components

### Install strip

- **Shape:** 2px radius, 1px ink border, flat ink fill.
- **Header:** mono label on the left, copy button on the right, separated by a
  hairline in a lighter ink.
- **Body:** command in mono with an orange `$`; wraps at `<wbr>` boundaries,
  never mid-token where a slash can take the break.
- **Copy:** label changes to `Copied` for 1.6 s; no toast, no icon.

### Terminal plate

- **Shape:** 5px radius, 1px `#2a2d34` border, deep-black title bar.
- **Bar:** a green dot and the program name on the left, device state on the
  right; both in mono at 0.72rem.
- **Content:** a real capture, full-bleed inside the plate, with a mono
  caption underneath (capture source on the left, state on the right).

### Switch matrix

- **Cell:** square, 1px hairline border, raised paper fill, min-height 104px.
  A mono position label top-left, a title bottom-left, an arrow that appears on
  hover/focus.
- **Active:** ink fill, paper text, orange position label.
- **Motion:** cells fade and rise 5px on load, 28ms apart; disabled under
  `prefers-reduced-motion`.
- **Behavior:** hover or keyboard focus updates the adjacent preview panel
  (title, description, link); the cell itself remains the link.

### Ledger table

- **Header:** mono uppercase labels above a 1px ink rule.
- **Rows:** 15px padding, hairline separators, no zebra fill.
- **Numbers:** tabular figures in mono.

### Docs asides

Starlight's aside component is re-inked: paper-tinted background, orange rule,
deep-signal title. Caution asides use a muted ochre so they do not read as
errors.

## Do's and Don'ts

- **Do** keep one orange meaning per view: the live switch, the install prompt
  or a primary link — not all three competing.
- **Do** set every address, mode byte and command in Spline Sans Mono.
- **Do** put real terminal captures on terminal black and explain them in
  paper-coloured prose.
- **Do** keep the switch matrix as the navigation; it is the product's own
  structure.
- **Don't** add rounded cards, gradient text, glass panels or colored left
  borders.
- **Don't** use the green outside a connection state.
- **Don't** invent screenshots, testimonials, benchmarks or claims that the
  hardware notes do not support.
- **Don't** let motion hide content: every entrance must end visible, and
  reduced motion must show the final state immediately.

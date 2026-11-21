---
version: 1
slug: "site-src-pages-index-astro"
primary_target: "site/src/pages/index.astro"
related_targets: []
---

# Landing page — mvave-chocolate-tui

- **Scope:** the GitHub Pages landing at `/` (Persuade), with Starlight docs under `/docs` (Read).
- **Visitor mode:** Persuade for `/`, Read for docs.
- **Audience:** Linux musicians with a Chocolate or Chocolate Plus, and developers who read protocol docs before trusting a tool.
- **Job:** decide in seconds that this is a real, careful tool, then install it with one command or open the docs.
- **Action:** copy the install command, click GitHub, or enter the docs through the switch matrix.
- **Proof:** real terminal captures, a byte-level write log, 100+ tests, hardware notes that state exactly what was and was not checked.
- **Constraints:** static Astro/Starlight, GitHub Pages project URL, no invented testimonials or benchmarks, GitHub repo always one click away.

## Direction contract

THESIS: The landing is the quick-start card that should have shipped in the pedal's box — a two-ink silkscreen bench sheet, not a SaaS hero. It refuses the centered headline + gradient + floating app mockup arrangement.

OWN-WORLD: Cool paper gray `#e6e7e9`, ink `#15171b`, one signal orange `#e04a2f` for the live switch and links, deep panel `#101216` for terminal plates. Archivo for display and UI; Spline Sans Mono for addresses, commands, mode bytes. Hairline rules, a 4×4 switch matrix, tabular data, small caps labels, no rounded cards except the terminal plate.

STORY: This tool reads before it writes, shows every byte, and was checked on real pedals — including the parts that are still untested. Install it in one line.

FIRST VIEWPORT: Left column: name, one-sentence promise, the `curl … | bash` command on an ink strip with a copy button, a read-only promise line, and two text links (Docs, GitHub). Right: the terminal plate with a real table capture, captioned with the device path and `read-only` state. A faint 16-cell grid anchors the background; the matrix becomes the docs index below.

FORM: code-led build; composition from the dealt Read-mode structures, no comp. Seed reference: seed:9e056f3122a62543.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.

# Documentation site

Astro Starlight site for M-VAVE Chocolate TUI, deployed to GitHub Pages at
<https://wilsonnet.github.io/mvave-chocolate-tui/>.

```bash
npm install
npm run dev      # http://localhost:4321/mvave-chocolate-tui/
npm run build    # static output in dist/
```

Content lives in `src/content/docs/`. The landing page is
`src/pages/index.astro`; design tokens are in `src/styles/landing.css` and the
Starlight theme in `src/styles/custom.css`.

Screenshots in `public/shots/` are real terminal captures rendered with a
pyte-based ANSI-to-SVG script; see the project's `docs/development.md` for the
hardware setup. Keep them honest: no mock screens presented as device output.

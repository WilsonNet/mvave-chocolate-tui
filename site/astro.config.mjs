// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';
import sitemap from '@astrojs/sitemap';

const site = 'https://wilsonnet.github.io';
const base = '/mvave-chocolate-tui';
const repo = 'https://github.com/WilsonNet/mvave-chocolate-tui';

export default defineConfig({
  site,
  base,
  trailingSlash: 'ignore',
  integrations: [
    starlight({
      title: 'M-VAVE Chocolate TUI',
      description:
        'Configure M-VAVE Chocolate and Chocolate Plus footswitches over USB from a Linux terminal. Read first, write on purpose, verify every byte.',
      logo: { src: './src/assets/mark.svg', alt: 'M-VAVE Chocolate TUI' },
      favicon: '/favicon.svg',
      social: [{ icon: 'github', label: 'GitHub', href: repo }],
      customCss: ['./src/styles/custom.css'],
      editLink: { baseUrl: `${repo}/edit/main/site/` },
      head: [
        { tag: 'meta', attrs: { property: 'og:type', content: 'website' } },
        { tag: 'meta', attrs: { property: 'og:site_name', content: 'M-VAVE Chocolate TUI' } },
        { tag: 'meta', attrs: { name: 'twitter:card', content: 'summary_large_image' } },
        {
          tag: 'meta',
          attrs: { property: 'og:image', content: `${site}${base}/og.png` },
        },
        {
          tag: 'meta',
          attrs: { name: 'twitter:image', content: `${site}${base}/og.png` },
        },
        { tag: 'meta', attrs: { name: 'theme-color', content: '#e6e7e9' } },
        { tag: 'link', attrs: { rel: 'alternate', type: 'text/plain', href: `${base}/llms.txt`, title: 'llms.txt' } },
      ],
      sidebar: [
        {
          label: 'Start here',
          items: [
            { label: 'Overview', slug: 'overview' },
            { label: 'Install', slug: 'install' },
            { label: 'First run', slug: 'first-run' },
          ],
        },
        {
          label: 'Using the TUI',
          items: [
            { label: 'The switch table', slug: 'using/table' },
            { label: 'The pedal screen', slug: 'using/pedal-screen' },
            { label: 'Operating modes', slug: 'using/modes' },
            { label: 'Backup, restore, compare', slug: 'using/backup' },
          ],
        },
        {
          label: 'Use cases',
          items: [
            { label: 'Guitar & amp modelers', slug: 'use-cases/modelers' },
            { label: 'DAWs', slug: 'use-cases/daws' },
            { label: 'Loopers', slug: 'use-cases/loopers' },
            { label: 'Page turning & streaming', slug: 'use-cases/page-turning' },
            { label: 'Expression pedal', slug: 'use-cases/expression' },
          ],
        },
        {
          label: 'Reference',
          items: [
            { label: 'CLI & flags', slug: 'reference/cli' },
            { label: 'SysEx protocol', slug: 'reference/protocol' },
            { label: 'Linux & testing', slug: 'reference/linux' },
            { label: 'Troubleshooting', slug: 'reference/troubleshooting' },
            { label: 'FAQ', slug: 'reference/faq' },
          ],
        },
      ],
    }),
    sitemap({
      filter: (page) => !page.includes('/404'),
    }),
  ],
});

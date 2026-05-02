import { defineConfig } from 'vitepress'
import { fileURLToPath } from 'node:url'

// Repo root, so Vite is allowed to read internal/ui/theme/palette.json
// (the shared brand palette consumed by the Go TUI and this site).
const repoRoot = fileURLToPath(new URL('../../../', import.meta.url))

export default defineConfig({
  title: 'stac-man',
  description: 'A CLI for stacked pull requests. Free, local-only, no SaaS.',
  base: '/stac-man/',
  cleanUrls: true,
  lastUpdated: true,
  lang: 'en-US',

  vite: {
    server: {
      fs: { allow: [repoRoot] },
    },
  },

  head: [
    ['meta', { name: 'theme-color', content: '#FF10F0' }],
    ['meta', { property: 'og:title', content: 'stac-man — stacked PRs, locally' }],
    ['meta', { property: 'og:description', content: 'A CLI for stacked pull requests. Free, local-only, no SaaS, no IDE plugin.' }],
    ['meta', { property: 'og:type', content: 'website' }],
  ],

  themeConfig: {
    siteTitle: 'stac-man',

    nav: [
      { text: 'Get Started', link: '/get-started/install' },
      { text: 'Concepts', link: '/concepts/' },
      { text: 'Commands', link: '/commands/' },
      { text: 'Recipes', link: '/recipes/' },
      { text: 'Troubleshooting', link: '/troubleshooting' },
      { text: 'FAQ', link: '/faq' },
    ],

    sidebar: {
      '/get-started/': [
        {
          text: 'Get Started',
          items: [
            { text: 'Install', link: '/get-started/install' },
            { text: 'Your first stack', link: '/get-started/first-stack' },
          ],
        },
      ],

      '/concepts/': [
        {
          text: 'Concepts',
          items: [
            { text: 'Overview', link: '/concepts/' },
            { text: 'Stacks', link: '/concepts/stacks' },
            { text: 'Trunk', link: '/concepts/trunk' },
            { text: 'Restack', link: '/concepts/restack' },
            { text: 'Sync', link: '/concepts/sync' },
            { text: 'Parent metadata', link: '/concepts/parent-metadata' },
          ],
        },
      ],

      '/commands/': [
        {
          text: 'Commands',
          items: [{ text: 'Overview', link: '/commands/' }],
        },
        {
          text: 'Navigation',
          items: [
            { text: 'sm checkout', link: '/commands/checkout' },
            { text: 'sm up / down / top / bottom', link: '/commands/nav' },
          ],
        },
        {
          text: 'Mutation',
          items: [
            { text: 'sm create', link: '/commands/create' },
            { text: 'sm modify', link: '/commands/modify' },
            { text: 'sm fold', link: '/commands/fold' },
            { text: 'sm split', link: '/commands/split' },
            { text: 'sm move', link: '/commands/move' },
          ],
        },
        {
          text: 'Restack engine',
          items: [
            { text: 'sm restack', link: '/commands/restack' },
            { text: 'sm continue / abort', link: '/commands/continue' },
            { text: 'sm sync', link: '/commands/sync' },
          ],
        },
        {
          text: 'PR ops',
          items: [
            { text: 'sm submit', link: '/commands/submit' },
            { text: 'sm land', link: '/commands/land' },
            { text: 'sm get', link: '/commands/get' },
          ],
        },
        {
          text: 'Inspection',
          items: [
            { text: 'sm log', link: '/commands/log' },
            { text: 'sm show', link: '/commands/show' },
            { text: 'sm parent', link: '/commands/parent' },
            { text: 'sm children', link: '/commands/children' },
            { text: 'sm doctor', link: '/commands/doctor' },
          ],
        },
        {
          text: 'Recovery',
          items: [
            { text: 'sm undo', link: '/commands/undo' },
            { text: 'sm absorb', link: '/commands/absorb' },
          ],
        },
        {
          text: 'Setup',
          items: [
            { text: 'sm track', link: '/commands/track' },
            { text: 'sm untrack', link: '/commands/untrack' },
            { text: 'sm completion', link: '/commands/completion' },
            { text: 'sm version', link: '/commands/version' },
            { text: 'sm update', link: '/commands/update' },
          ],
        },
      ],

      '/recipes/': [
        {
          text: 'Recipes',
          items: [
            { text: 'Overview', link: '/recipes/' },
            { text: 'Split a fat branch', link: '/recipes/split-fat-branch' },
            { text: 'Land the bottom of a stack', link: '/recipes/land-bottom-of-stack' },
            { text: 'Recover from a bad rebase', link: '/recipes/recover-from-bad-rebase' },
            { text: "Review someone's stack", link: '/recipes/review-someones-stack' },
            { text: 'Absorb fixups', link: '/recipes/absorb-fixups' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/bluegardenproject/stac-man' },
    ],

    editLink: {
      pattern: 'https://github.com/bluegardenproject/stac-man/edit/main/docs/site/:path',
      text: 'Edit this page on GitHub',
    },

    footer: {
      message: 'Released under a license that is currently TBD.',
      copyright: 'Built with VitePress.',
    },

    search: {
      provider: 'local',
    },

    outline: {
      level: [2, 3],
    },
  },
})

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

  // Asset paths in `head` need the configured base prefix because
  // VitePress emits them verbatim into <link>/<meta>. Asset paths in
  // `themeConfig` (logo, etc.) are resolved relative to base for us,
  // so they must NOT include the prefix.
  head: [
    ['link', { rel: 'icon', type: 'image/x-icon', href: '/stac-man/favicon.ico' }],
    ['link', { rel: 'icon', type: 'image/png', sizes: '32x32', href: '/stac-man/favicon-32x32.png' }],
    ['link', { rel: 'icon', type: 'image/png', sizes: '16x16', href: '/stac-man/favicon-16x16.png' }],
    ['link', { rel: 'apple-touch-icon', sizes: '180x180', href: '/stac-man/apple-touch-icon.png' }],
    ['link', { rel: 'manifest', href: '/stac-man/site.webmanifest' }],
    // theme-color matches --logo-pink in style.css and the logo's
    // primary highlight. Mobile browsers tint the address bar with this.
    ['meta', { name: 'theme-color', content: '#FF1493' }],
    ['meta', { property: 'og:title', content: 'stac-man — stacked PRs, locally' }],
    ['meta', { property: 'og:description', content: 'A CLI for stacked pull requests. Free, local-only, no SaaS, no IDE plugin.' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:image', content: 'https://bluegardenproject.github.io/stac-man/stac-man-logo.png' }],
    ['meta', { name: 'twitter:card', content: 'summary' }],
    ['meta', { name: 'twitter:image', content: 'https://bluegardenproject.github.io/stac-man/stac-man-logo.png' }],
  ],

  themeConfig: {
    siteTitle: 'stac-man',
    logo: '/stac-man-logo.png',

    nav: [
      { text: 'Get Started', link: '/get-started/install' },
      { text: 'Concepts', link: '/concepts/' },
      { text: 'Commands', link: '/commands/' },
      { text: 'Recipes', link: '/recipes/' },
      { text: 'Troubleshooting', link: '/troubleshooting' },
      { text: 'FAQ', link: '/faq' },
      {
        text: 'v2.x',
        items: [
          { text: 'v2.x (latest)', link: '/' },
          { text: 'v1.x', link: '/v1/' },
        ],
      },
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
            { text: 'Cockpit (TUI)', link: '/concepts/cockpit' },
            { text: 'Migrating to v2', link: '/concepts/v2-migration' },
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
            { text: 'sm status', link: '/commands/status' },
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
            { text: 'sm config', link: '/commands/config' },
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

      '/v1/get-started/': [
        {
          text: 'Get Started',
          items: [
            { text: 'Install', link: '/v1/get-started/install' },
            { text: 'Your first stack', link: '/v1/get-started/first-stack' },
          ],
        },
      ],

      '/v1/concepts/': [
        {
          text: 'Concepts',
          items: [
            { text: 'Overview', link: '/v1/concepts/' },
            { text: 'Stacks', link: '/v1/concepts/stacks' },
            { text: 'Trunk', link: '/v1/concepts/trunk' },
            { text: 'Restack', link: '/v1/concepts/restack' },
            { text: 'Sync', link: '/v1/concepts/sync' },
            { text: 'Parent metadata', link: '/v1/concepts/parent-metadata' },
            { text: 'Cockpit (TUI)', link: '/v1/concepts/cockpit' },
          ],
        },
      ],

      '/v1/commands/': [
        {
          text: 'Commands',
          items: [{ text: 'Overview', link: '/v1/commands/' }],
        },
        {
          text: 'Navigation',
          items: [
            { text: 'sm checkout', link: '/v1/commands/checkout' },
            { text: 'sm up / down / top / bottom', link: '/v1/commands/nav' },
          ],
        },
        {
          text: 'Mutation',
          items: [
            { text: 'sm create', link: '/v1/commands/create' },
            { text: 'sm modify', link: '/v1/commands/modify' },
            { text: 'sm fold', link: '/v1/commands/fold' },
            { text: 'sm split', link: '/v1/commands/split' },
            { text: 'sm move', link: '/v1/commands/move' },
          ],
        },
        {
          text: 'Restack engine',
          items: [
            { text: 'sm restack', link: '/v1/commands/restack' },
            { text: 'sm continue / abort', link: '/v1/commands/continue' },
            { text: 'sm sync', link: '/v1/commands/sync' },
          ],
        },
        {
          text: 'PR ops',
          items: [
            { text: 'sm submit', link: '/v1/commands/submit' },
            { text: 'sm land', link: '/v1/commands/land' },
            { text: 'sm get', link: '/v1/commands/get' },
          ],
        },
        {
          text: 'Inspection',
          items: [
            { text: 'sm log', link: '/v1/commands/log' },
            { text: 'sm show', link: '/v1/commands/show' },
            { text: 'sm parent', link: '/v1/commands/parent' },
            { text: 'sm children', link: '/v1/commands/children' },
            { text: 'sm doctor', link: '/v1/commands/doctor' },
          ],
        },
        {
          text: 'Recovery',
          items: [
            { text: 'sm undo', link: '/v1/commands/undo' },
            { text: 'sm absorb', link: '/v1/commands/absorb' },
          ],
        },
        {
          text: 'Setup',
          items: [
            { text: 'sm track', link: '/v1/commands/track' },
            { text: 'sm untrack', link: '/v1/commands/untrack' },
            { text: 'sm completion', link: '/v1/commands/completion' },
            { text: 'sm config', link: '/v1/commands/config' },
            { text: 'sm version', link: '/v1/commands/version' },
            { text: 'sm update', link: '/v1/commands/update' },
          ],
        },
      ],

      '/v1/recipes/': [
        {
          text: 'Recipes',
          items: [
            { text: 'Overview', link: '/v1/recipes/' },
            { text: 'Split a fat branch', link: '/v1/recipes/split-fat-branch' },
            { text: 'Land the bottom of a stack', link: '/v1/recipes/land-bottom-of-stack' },
            { text: 'Recover from a bad rebase', link: '/v1/recipes/recover-from-bad-rebase' },
            { text: "Review someone's stack", link: '/v1/recipes/review-someones-stack' },
            { text: 'Absorb fixups', link: '/v1/recipes/absorb-fixups' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/bluegardenproject/stac-man' },
    ],

    editLink: {
      pattern: 'https://github.com/bluegardenproject/stac-man/edit/main/docs/web/:path',
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

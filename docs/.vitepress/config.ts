import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Outport',
  description: 'Deterministic ports across projects and worktrees. Stable .test domains, automatic HTTPS, and .env management.',
  appearance: true,
  cleanUrls: true,
  sitemap: {
    hostname: 'https://outport.dev',
  },

  head: [
    ['link', { rel: 'icon', href: '/favicon.ico' }],
    ['link', { rel: 'icon', type: 'image/png', sizes: '32x32', href: '/favicon-32x32.png' }],
    ['link', { rel: 'icon', type: 'image/png', sizes: '16x16', href: '/favicon-16x16.png' }],
    ['link', { rel: 'apple-touch-icon', sizes: '180x180', href: '/apple-touch-icon.png' }],
    ['link', { rel: 'manifest', href: '/site.webmanifest' }],
    ['meta', { property: 'og:type', content: 'website' }],
    ['meta', { property: 'og:url', content: 'https://outport.dev' }],
    ['meta', { property: 'og:image', content: 'https://outport.dev/og-image-1280x640.png' }],
    ['meta', { property: 'og:title', content: 'Outport' }],
    ['meta', { property: 'og:description', content: 'Deterministic ports across projects and worktrees. Stable .test domains, automatic HTTPS, and .env management.' }],
    ['meta', { property: 'og:locale', content: 'en_US' }],
    ['meta', { name: 'twitter:card', content: 'summary_large_image' }],
    ['meta', { name: 'twitter:image', content: 'https://outport.dev/og-image-1280x640.png' }],
    ['link', { rel: 'preload', href: '/fonts/Barlow-Bold.ttf', as: 'font', type: 'font/ttf', crossorigin: '' }],
    ['link', { rel: 'preload', href: '/fonts/Inter.ttf', as: 'font', type: 'font/ttf', crossorigin: '' }],
  ],

  themeConfig: {
    logo: {
      light: '/logo-horizontal-color.svg',
      dark: '/logo-horizontal-white.svg',
    },
    siteTitle: false,

    nav: [
      { text: 'Guide', link: '/guide/getting-started' },
      { text: 'Reference', link: '/reference/configuration' },
    ],

    sidebar: [
      {
        text: 'Guide',
        items: [
          { text: 'Getting Started', link: '/guide/getting-started' },
          { text: 'How It Works', link: '/guide/how-it-works' },
          { text: 'Examples', link: '/guide/examples' },
          { text: 'Installation', link: '/guide/installation' },
          { text: 'Why Outport?', link: '/guide/why-outport' },
          { text: 'Dashboard', link: '/guide/dashboard' },
          { text: 'Sharing & Mobile', link: '/guide/sharing' },
          { text: 'VS Code Extension', link: '/guide/vscode' },
          { text: 'Work with AI', link: '/guide/work-with-ai' },
          { text: 'Running Your Dev Stack', link: '/guide/devstack' },
          { text: 'Tips & Troubleshooting', link: '/guide/tips' },
        ],
      },
      {
        text: 'Reference',
        items: [
          { text: 'Configuration', link: '/reference/configuration' },
          { text: 'Commands', link: '/reference/commands' },
        ],
      },
    ],

    socialLinks: [
      { icon: 'github', link: 'https://github.com/steveclarke/outport' },
      { icon: 'discord', link: 'https://discord.gg/R4SyEskf' },
    ],

    search: {
      provider: 'local',
    },
  },
})

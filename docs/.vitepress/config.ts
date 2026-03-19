import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Outport',
  description: 'Deterministic port management for multi-project development',
  appearance: false,
  cleanUrls: true,
  srcExclude: ['superpowers/**'],

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
    ['meta', { property: 'og:description', content: 'Deterministic port management for multi-project development' }],
    ['link', { rel: 'preload', href: '/fonts/Barlow-Bold.ttf', as: 'font', type: 'font/ttf', crossorigin: '' }],
    ['link', { rel: 'preload', href: '/fonts/Inter.ttf', as: 'font', type: 'font/ttf', crossorigin: '' }],
  ],

  themeConfig: {
    logo: '/logo-horizontal-color.svg',
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
          { text: 'Installation', link: '/guide/installation' },
          { text: 'Why Outport?', link: '/guide/why-outport' },
          { text: 'Examples', link: '/guide/examples' },
          { text: 'VS Code Extension', link: '/guide/vscode' },
          { text: 'Work with AI', link: '/guide/work-with-ai' },
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
    ],

    search: {
      provider: 'local',
    },
  },
})

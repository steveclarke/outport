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
    ['script', {},
      `!function(t,e){var o,n,p,r;e.__SV||(window.posthog=e,e._i=[],e.init=function(i,s,a){function g(t,e){var o=e.split(".");2==o.length&&(t=t[o[0]],e=o[1]),t[e]=function(){t.push([e].concat(Array.prototype.slice.call(arguments,0)))}}(p=t.createElement("script")).type="text/javascript",p.crossOrigin="anonymous",p.async=!0,p.src=s.api_host.replace(".i.posthog.com","-assets.i.posthog.com")+"/static/array.js",(r=t.getElementsByTagName("script")[0]).parentNode.insertBefore(p,r);var u=e;for(void 0!==a?u=e[a]=[]:a="posthog",u.people=u.people||[],u.toString=function(t){var e="posthog";return"posthog"!==a&&(e+="."+a),t||(e+=" (stub)"),e},u.people.toString=function(){return u.toString(1)+".people (stub)"},o="init capture register register_once register_for_session unregister unregister_for_session getFeatureFlag getFeatureFlagPayload isFeatureEnabled reloadFeatureFlags updateEarlyAccessFeatureEnrollment getEarlyAccessFeatures on onFeatureFlags onSessionId getSurveys getActiveMatchingSurveys renderSurvey canRenderSurvey identify setPersonProperties group resetGroups setPersonPropertiesForFlags resetPersonPropertiesForFlags setGroupPropertiesForFlags resetGroupPropertiesForFlags reset get_distinct_id getGroups get_session_id get_session_replay_url alias set_config startSessionRecording stopSessionRecording sessionRecordingStarted captureException loadToolbar get_property getSessionProperty createPersonProfile opt_in_capturing opt_out_capturing has_opted_in_capturing has_opted_out_capturing clear_opt_in_out_capturing debug getPageviewId".split(" "),n=0;n<o.length;n++)g(u,o[n]);e._i.push([i,s,a])},e.__SV=1)}(document,window.posthog||[]);posthog.init('phc_1b8Xy9sKBx4xqtzCcezTVfRQcAhXOSFxihD9RvqBIA8',{api_host:'https://us.i.posthog.com',person_profiles:'identified_only'})`
    ],
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
    ],

    search: {
      provider: 'local',
    },
  },
})

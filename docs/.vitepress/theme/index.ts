import DefaultTheme from 'vitepress/theme'
import Layout from './Layout.vue'
import './custom.css'
import posthog from 'posthog-js'

export default {
  extends: DefaultTheme,
  Layout,
  enhanceApp() {
    if (typeof window !== 'undefined') {
      posthog.init('phc_1b8Xy9sKBx4xqtzCcezTVfRQcAhXOSFxihD9RvqBIA8', {
        api_host: 'https://us.i.posthog.com',
        person_profiles: 'identified_only',
      })
    }
  },
}

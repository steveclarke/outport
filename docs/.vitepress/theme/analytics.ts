/**
 * Custom analytics events for outport.dev
 *
 * Fires to both PostHog (posthog.capture) and GA (gtag event).
 * Attaches once via event delegation on document.body — no per-element wiring.
 */

function track(event: string, props: Record<string, string> = {}) {
  // PostHog
  if (typeof window !== 'undefined' && (window as any).posthog) {
    ;(window as any).posthog.capture(event, props)
  }
  // Google Analytics
  if (typeof window !== 'undefined' && (window as any).gtag) {
    ;(window as any).gtag('event', event, props)
  }
}

function findLink(el: HTMLElement): HTMLAnchorElement | null {
  let node: HTMLElement | null = el
  while (node && node !== document.body) {
    if (node.tagName === 'A') return node as HTMLAnchorElement
    node = node.parentElement
  }
  return null
}

export function setupAnalytics() {
  if (typeof window === 'undefined') return

  // --- Click tracking via delegation ---
  document.body.addEventListener('click', (e) => {
    const link = findLink(e.target as HTMLElement)
    if (!link) return

    const href = link.getAttribute('href') || ''
    const text = link.textContent?.trim() || ''

    // CTA buttons on homepage
    if (link.classList.contains('btn-primary')) {
      track('cta_click', { button: 'get_started', href })
      return
    }
    if (link.classList.contains('btn-secondary')) {
      track('cta_click', { button: 'view_on_github', href })
      return
    }

    // Install command copy (clicking the install block)
    if (link.closest('.install-cmd') || link.closest('.install')) {
      track('install_click', { href })
      return
    }

    // GitHub link clicks
    if (href.includes('github.com/steveclarke/outport')) {
      track('github_click', { href, text })
      return
    }

    // Discord clicks
    if (href.includes('discord.gg')) {
      track('discord_click', { href })
      return
    }

    // Example repo clicks
    if (href.includes('outport-example')) {
      track('example_repo_click', { href })
      return
    }

    // "Why Outport?" / story link
    if (href.includes('why-outport')) {
      track('story_click', { href })
      return
    }

    // External links (not internal docs navigation)
    if (href.startsWith('http') && !href.includes('outport.dev')) {
      track('external_link', { href, text })
    }
  })

  // --- Install command copy detection ---
  document.addEventListener('copy', () => {
    const selection = window.getSelection()?.toString() || ''
    if (selection.includes('brew install') || selection.includes('outport.dev/install.sh')) {
      track('install_copied', { command: selection.trim().slice(0, 200) })
    }
  })

  // --- Scroll depth tracking (25%, 50%, 75%, 100%) ---
  const thresholds = [25, 50, 75, 100]
  const fired = new Set<number>()

  function checkScroll() {
    const scrollHeight = document.documentElement.scrollHeight - window.innerHeight
    if (scrollHeight <= 0) return
    const percent = Math.round((window.scrollY / scrollHeight) * 100)

    for (const t of thresholds) {
      if (percent >= t && !fired.has(t)) {
        fired.add(t)
        track('scroll_depth', { percent: `${t}`, page: window.location.pathname })
      }
    }
  }

  window.addEventListener('scroll', checkScroll, { passive: true })

  // Reset scroll tracking on SPA navigation
  const observer = new MutationObserver(() => {
    fired.clear()
  })
  const content = document.querySelector('.VPContent')
  if (content) {
    observer.observe(content, { childList: true, subtree: false })
  }
}

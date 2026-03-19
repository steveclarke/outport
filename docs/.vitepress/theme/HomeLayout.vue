<template>
  <div class="home-layout">
    <!-- HERO -->
    <section class="hero">
      <h1><code class="hero-code">myapp.test</code> <span class="accent">just works</span></h1>
      <p class="subtitle">
        Declare your services once. Every project, worktree, and checkout gets
        deterministic ports, <code>.test</code> hostnames with HTTPS, and fully
        wired <code>.env</code> files.
      </p>
      <div class="hero-ctas">
        <a href="/guide/getting-started" class="btn-primary">Get Started</a>
        <a
          href="https://github.com/steveclarke/outport"
          class="btn-secondary"
          target="_blank"
          rel="noopener"
        >View on GitHub</a>
      </div>

      <div class="terminal-pair">
        <div class="terminal-wrapper">
          <div class="terminal-label">Main checkout</div>
          <div class="terminal">
            <div class="terminal-bar">
              <div class="terminal-dot r" />
              <div class="terminal-dot y" />
              <div class="terminal-dot g" />
            </div>
            <div class="terminal-body">
              <div><span class="prompt">$</span> <span class="cmd">outport up</span></div>
              <br>
              <div><span class="success">myapp</span> <span class="dim">[main]</span></div>
              <br>
              <div>&nbsp;&nbsp;&nbsp;&nbsp;<span class="cmd">rails</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">PORT</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">&rarr;</span> <span class="value">13842</span>&nbsp;&nbsp;<span class="success">https://myapp.test</span></div>
              <div>&nbsp;&nbsp;&nbsp;&nbsp;<span class="cmd">postgres</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">DATABASE_PORT</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">&rarr;</span> <span class="value">28391</span></div>
              <div>&nbsp;&nbsp;&nbsp;&nbsp;<span class="cmd">redis</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">REDIS_PORT</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">&rarr;</span> <span class="value">19204</span></div>
              <br>
              <div><span class="success">Ports written to .env</span></div>
            </div>
          </div>
        </div>
        <div class="terminal-wrapper">
          <div class="terminal-label">Worktree</div>
          <div class="terminal">
            <div class="terminal-bar">
              <div class="terminal-dot r" />
              <div class="terminal-dot y" />
              <div class="terminal-dot g" />
            </div>
            <div class="terminal-body">
              <div><span class="prompt">$</span> <span class="cmd">outport up</span></div>
              <br>
              <div><span class="success">myapp</span> <span class="dim">[bxcf]</span></div>
              <br>
              <div>&nbsp;&nbsp;&nbsp;&nbsp;<span class="cmd">rails</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">PORT</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">&rarr;</span> <span class="value">22107</span>&nbsp;&nbsp;<span class="success">https://myapp-bxcf.test</span></div>
              <div>&nbsp;&nbsp;&nbsp;&nbsp;<span class="cmd">postgres</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">DATABASE_PORT</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">&rarr;</span> <span class="value">31842</span></div>
              <div>&nbsp;&nbsp;&nbsp;&nbsp;<span class="cmd">redis</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">REDIS_PORT</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">&rarr;</span> <span class="value">16509</span></div>
              <br>
              <div><span class="success">Ports written to .env</span></div>
            </div>
          </div>
        </div>
      </div>
      <p class="hero-callout">
        Built for parallel development — worktrees, AI agents, and multiple checkouts.
        <a href="/guide/work-with-ai">Learn more →</a>
      </p>
    </section>

    <hr class="section-divider">

    <!-- FEATURES -->
    <section class="features">
      <div class="features-heading">
        <h2>Features</h2>
      </div>
      <div class="feature-grid">
        <div class="feature-card">
          <div class="feature-icon">.e</div>
          <h3>.env Integration</h3>
          <p>Ports, URLs, and computed values written directly to your <code>.env</code> files. Your app reads them natively — no wrapper, no SDK.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">&gt;_</div>
          <h3>Multi-Instance</h3>
          <p>Worktrees, branches, AI agents — each gets isolated ports and hostnames automatically.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon computed-icon">${}</div>
          <h3>Computed Values</h3>
          <p>Services discover each other. <code>CORS_ORIGINS=${frontend.url}</code> resolves automatically across your stack.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">#</div>
          <h3>Deterministic Ports</h3>
          <p>Same project, same ports — every time, on every machine.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">.t</div>
          <h3>.test Domains with HTTPS</h3>
          <p>Real hostnames like <code>myapp.test</code> with automatic HTTPS. Built-in DNS and reverse proxy.</p>
        </div>
      </div>
    </section>

    <hr class="section-divider">

    <!-- MONOREPO EXAMPLE -->
    <section class="example">
      <h2>One config, fully wired</h2>
      <p class="example-caption">Cross-service URLs resolve automatically. Change nothing when switching between main and worktrees.</p>
      <div class="example-pair">
        <div class="example-block">
          <div class="example-label">.outport.yml</div>
          <pre class="step-code"><span class="dim"># .outport.yml</span>
name: myapp
services:
  rails:
    env_var: PORT
    hostname: myapp
    protocol: http
  frontend:
    env_var: NUXT_PORT
    hostname: myapp-frontend
    protocol: http
  postgres:
    env_var: DATABASE_PORT
computed:
  API_URL: <span class="value">${rails.url}</span>
  CORS_ORIGINS: <span class="value">${frontend.url}</span></pre>
        </div>
        <div class="example-block">
          <div class="example-label">Resulting .env</div>
          <pre class="step-code"><span class="dim"># .env</span>
<span class="dim"># Your variables are never touched</span>
SECRET_KEY=abc123
DATABASE_URL=postgres://...

<span class="dim"># --- begin outport.dev ---</span>
PORT=<span class="value">13842</span>
NUXT_PORT=<span class="value">24519</span>
DATABASE_PORT=<span class="value">28391</span>
API_URL=<span class="success">https://myapp.test</span>
CORS_ORIGINS=<span class="success">https://myapp-frontend.test</span>
<span class="dim"># --- end outport.dev ---</span></pre>
        </div>
      </div>
    </section>

    <section class="adoption-note">
      <p>
        <code>outport up</code> works without the daemon — you get deterministic ports
        and <code>.env</code> integration with zero system changes.
        Run <code>outport setup</code> to optionally enable
        <code>.test</code> hostnames and HTTPS.
      </p>
    </section>

    <hr class="section-divider">

    <!-- INSTALL -->
    <section class="install">
      <h2>Try it</h2>
      <div class="install-cmd">
        <span class="dollar">$</span> brew install steveclarke/tap/outport
      </div>
      <div class="install-cmd install-cmd-next">
        <span class="dollar">$</span> outport setup
      </div>
      <p class="install-note">macOS only. Linux support is experimental.</p>
    </section>

    <!-- FOOTER -->
    <footer class="home-footer">
      MIT License &middot;
      <a href="https://github.com/steveclarke/outport" target="_blank" rel="noopener">GitHub</a>
      &middot; Built by
      <a href="https://github.com/steveclarke" target="_blank" rel="noopener">Steve Clarke</a>
    </footer>
  </div>
</template>

<style scoped>
.home-layout {
  font-family: var(--vp-font-family-base);
  color: var(--vp-c-text-1);
  line-height: 1.6;
}

/* HERO */
.hero {
  max-width: 1100px;
  margin: 0 auto;
  padding: 4rem 2rem 3rem;
  text-align: center;
}
.hero h1 {
  font-family: 'Barlow', ui-sans-serif, system-ui, sans-serif;
  font-weight: 700;
  font-size: 3.25rem;
  color: #031C54;
  letter-spacing: -0.02em;
  line-height: 1.15;
  margin-bottom: 1rem;
}
.hero h1 .accent { color: #2E86AB; }
.hero h1 .hero-code {
  font-family: var(--vp-font-family-mono);
  font-size: 0.85em;
  background: var(--vp-c-bg-soft);
  padding: 0.05em 0.3em;
  border-radius: 6px;
}
.hero .subtitle {
  font-size: 1.2rem;
  color: var(--vp-c-text-3);
  max-width: 600px;
  margin: 0 auto 2rem;
  line-height: 1.6;
}
.hero .subtitle code {
  background: var(--vp-c-bg-soft);
  padding: 0.1em 0.4em;
  border-radius: 4px;
  font-size: 0.9em;
}
.hero-ctas {
  display: flex;
  gap: 1rem;
  justify-content: center;
  margin-bottom: 3rem;
}
.btn-primary {
  background: #2E86AB;
  color: white;
  padding: 0.75rem 1.75rem;
  border-radius: 8px;
  font-weight: 600;
  font-size: 0.95rem;
  text-decoration: none;
  transition: background 0.15s;
  border: none;
}
.btn-primary:hover { background: #3a9ac2; }
.btn-secondary {
  background: transparent;
  color: var(--vp-c-text-1);
  padding: 0.75rem 1.75rem;
  border-radius: 8px;
  font-weight: 600;
  font-size: 0.95rem;
  text-decoration: none;
  border: 1px solid var(--vp-c-divider);
  transition: border-color 0.15s;
}
.btn-secondary:hover { border-color: var(--vp-c-text-3); }

/* TERMINAL */
.terminal {
  background: #1a1b26;
  border-radius: 10px;
  overflow: hidden;
  text-align: left;
  box-shadow: 0 20px 60px rgba(3,28,84,0.12), 0 4px 16px rgba(3,28,84,0.06);
}
.terminal-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 12px 16px;
  background: #24253a;
}
.terminal-dot { width: 10px; height: 10px; border-radius: 50%; }
.terminal-dot.r { background: #ff5f57; }
.terminal-dot.y { background: #febc2e; }
.terminal-dot.g { background: #28c840; }
.terminal-body {
  padding: 1.25rem 1.5rem;
  font-family: var(--vp-font-family-mono);
  font-size: 0.82rem;
  line-height: 1.7;
  color: #a9b1d6;
  overflow-x: auto;
}
.terminal-body .prompt { color: #7aa2f7; }
.terminal-body .cmd { color: #c0caf5; }
.terminal-body .success { color: #9ece6a; }
.terminal-body .value { color: #e0af68; }
.terminal-body .dim { color: #565f89; }

/* TERMINAL PAIR (side by side) */
.terminal-pair {
  display: flex;
  gap: 1.25rem;
  max-width: 1100px;
  margin: 0 auto;
}
.terminal-wrapper {
  flex: 1;
  min-width: 0;
}
.terminal-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--vp-c-text-3);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 0.5rem;
  text-align: center;
}
.hero-callout {
  text-align: center;
  color: var(--vp-c-text-3);
  font-size: 0.9rem;
  margin-top: 2rem;
}
.hero-callout a {
  color: #2E86AB;
  text-decoration: none;
  font-weight: 500;
}
.hero-callout a:hover { text-decoration: underline; }

/* SECTION DIVIDER */
.section-divider {
  max-width: 1100px;
  margin: 0 auto;
  border: none;
  border-top: 1px solid var(--vp-c-divider);
}

/* FEATURES */
.features {
  max-width: 1100px;
  margin: 0 auto;
  padding: 4rem 2rem;
}
.features-heading {
  text-align: center;
  margin-bottom: 2.5rem;
}
.features-heading h2 {
  font-family: 'Barlow', ui-sans-serif, system-ui, sans-serif;
  font-weight: 700;
  font-size: 2rem;
  color: #031C54;
  letter-spacing: -0.01em;
}
.feature-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 1.25rem;
  justify-content: center;
}
.feature-card {
  flex: 0 1 calc(50% - 0.625rem);
  min-width: 280px;
  background: #ffffff;
  border: 1px solid var(--vp-c-divider);
  border-radius: 10px;
  padding: 1.75rem;
  transition: box-shadow 0.2s;
}
.feature-card:hover {
  box-shadow: 0 4px 16px rgba(3,28,84,0.06);
}
.feature-icon {
  width: 40px; height: 40px;
  border-radius: 8px;
  background: var(--vp-c-bg-soft);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 1rem;
  font-size: 1.25rem;
  color: #031C54;
  font-weight: 700;
  font-family: var(--vp-font-family-mono);
}
.feature-icon.computed-icon {
  font-size: 0.9rem;
}
.feature-card h3 {
  font-family: 'Barlow', ui-sans-serif, system-ui, sans-serif;
  font-weight: 700;
  font-size: 1.1rem;
  color: #031C54;
  margin-bottom: 0.5rem;
}
.feature-card p {
  color: var(--vp-c-text-3);
  font-size: 0.9rem;
  line-height: 1.6;
}
.feature-card code {
  background: var(--vp-c-bg-soft);
  padding: 0.1em 0.4em;
  border-radius: 4px;
  font-size: 0.9em;
}

/* CODE BLOCKS (shared between sections) */
.step-code {
  display: block;
  background: #1a1b26;
  color: #a9b1d6;
  padding: 0.75rem 1rem;
  border-radius: 6px;
  font-size: 0.8rem;
  font-family: var(--vp-font-family-mono);
  white-space: pre;
  overflow-x: auto;
}
.step-code .prompt { color: #7aa2f7; }
.step-code .cmd { color: #c0caf5; }
.step-code .success { color: #9ece6a; }
.step-code .value { color: #e0af68; }
.step-code .dim { color: #565f89; }

/* MONOREPO EXAMPLE */
.example {
  max-width: 1100px;
  margin: 0 auto;
  padding: 4rem 2rem;
}
.example h2 {
  font-family: 'Barlow', ui-sans-serif, system-ui, sans-serif;
  font-weight: 700;
  font-size: 2rem;
  color: #031C54;
  text-align: center;
  margin-bottom: 0.75rem;
}
.example-caption {
  text-align: center;
  color: var(--vp-c-text-3);
  font-size: 1rem;
  max-width: 600px;
  margin: 0 auto 2rem;
}
.example-pair {
  display: flex;
  gap: 1.25rem;
}
.example-block {
  flex: 1;
  min-width: 0;
}
.example-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--vp-c-text-3);
  text-transform: uppercase;
  letter-spacing: 0.05em;
  margin-bottom: 0.5rem;
}
.example-block .step-code {
  margin-top: 0;
  height: 100%;
}

/* ADOPTION NOTE */
.adoption-note {
  max-width: 600px;
  margin: 0 auto;
  padding: 0 2rem 2rem;
  text-align: center;
}
.adoption-note p {
  color: var(--vp-c-text-3);
  font-size: 0.9rem;
  line-height: 1.6;
}
.adoption-note code {
  background: var(--vp-c-bg-soft);
  padding: 0.1em 0.4em;
  border-radius: 4px;
  font-size: 0.9em;
}

/* INSTALL */
.install {
  max-width: 1100px;
  margin: 0 auto;
  padding: 4rem 2rem;
  text-align: center;
}
.install h2 {
  font-family: 'Barlow', ui-sans-serif, system-ui, sans-serif;
  font-weight: 700;
  font-size: 2rem;
  color: #031C54;
  margin-bottom: 2rem;
}
.install p {
  color: var(--vp-c-text-3);
  margin-bottom: 2rem;
}
.install-cmd {
  display: inline-flex;
  align-items: center;
  gap: 1rem;
  background: #1a1b26;
  color: #c0caf5;
  padding: 1rem 1.5rem;
  border-radius: 10px;
  font-family: var(--vp-font-family-mono);
  font-size: 0.9rem;
  box-shadow: 0 4px 16px rgba(3,28,84,0.08);
}
.install-cmd .dollar { color: #7aa2f7; }
.install-cmd-next {
  margin-top: 0.75rem;
}
.install-note {
  color: var(--vp-c-text-3);
  font-size: 0.8rem;
  margin-top: 1rem;
}

/* FOOTER */
.home-footer {
  max-width: 1100px;
  margin: 2rem auto 0;
  padding: 2rem;
  text-align: center;
  color: var(--vp-c-text-3);
  font-size: 0.85rem;
  border-top: 1px solid var(--vp-c-divider);
}
.home-footer a { color: #2E86AB; text-decoration: none; }

/* Responsive */
@media (max-width: 768px) {
  .hero h1 { font-size: 2.25rem; }
  .feature-card { flex: 0 1 100%; }
  .hero-ctas { flex-direction: column; align-items: center; }
  .terminal-pair { flex-direction: column; }
  .example-pair { flex-direction: column; }
}
</style>

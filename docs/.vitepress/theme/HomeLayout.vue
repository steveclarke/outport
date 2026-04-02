<template>
  <div class="home-layout">
    <!-- HERO -->
    <section class="hero">
      <h1>Deterministic ports across <span class="accent">projects and worktrees</span></h1>
      <p class="subtitle">
        Stable ports, <code>.test</code> hostnames, and wired
        <code>.env</code> files for every project, worktree, and agent instance.
        Declare your services once — every checkout is isolated automatically.
      </p>
      <ul class="pain-points">
        <li><span class="pain-text">Editing <code>/etc/hosts</code></span></li>
        <li><span class="pain-text">Port conflicts between projects</span></li>
        <li><span class="pain-text">Updating five <code>.env</code> files when a port changes</span></li>
        <li><span class="pain-text">Self-signed certificate warnings</span></li>
        <li><span class="pain-text">"What port was this on again?"</span></li>
      </ul>
      <div class="hero-ctas">
        <a href="/guide/getting-started" class="btn-primary">Get Started</a>
        <a
          href="https://github.com/steveclarke/outport"
          class="btn-secondary"
          target="_blank"
          rel="noopener"
        >View on GitHub</a>
      </div>
      <a href="/guide/why-outport" class="story-link">Read the story →</a>

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
        Built for the agentic era — agents, worktrees, and parallel development.
        <a href="/guide/work-with-ai">Work with AI →</a>
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
          <div class="feature-icon">#</div>
          <h3>Deterministic Ports</h3>
          <p>Same project, same ports — every time, on every machine.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">&gt;_</div>
          <h3>Multi-Instance</h3>
          <p>Worktrees, branches, AI agents — each gets isolated ports and hostnames automatically.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">.e</div>
          <h3>.env Integration</h3>
          <p>Ports, URLs, and computed values written directly to your <code>.env</code> files. Your app reads them natively — no wrapper, no SDK.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon computed-icon">${}</div>
          <h3>Computed Values</h3>
          <p>Services discover each other. <code>CORS_ORIGINS=${frontend.url}</code> resolves automatically across your stack.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">.t</div>
          <h3>.test Domains with HTTPS</h3>
          <p>Real hostnames like <code>myapp.test</code> with automatic HTTPS. Built-in DNS and reverse proxy.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon computed-icon">VS</div>
          <h3>VS Code Extension</h3>
          <p>Sidebar panel with ports, URLs, health indicators, and one-click actions. <a href="/guide/vscode">Install it →</a></p>
        </div>
      </div>
    </section>

    <hr class="section-divider">

    <!-- DASHBOARD -->
    <section class="dashboard-section">
      <h2>Live Dashboard</h2>
      <p class="dashboard-caption">
        All your projects, URLs, and service health in one place.
      </p>
      <div class="dashboard-frame">
        <img
          class="screenshot-light"
          src="/dashboard-light.png"
          alt="Outport live dashboard showing projects with .test URLs, ports, and health status"
          width="1200"
          height="800"
          loading="lazy"
        />
        <img
          class="screenshot-dark"
          src="/dashboard-dark.png"
          alt="Outport live dashboard in dark mode showing projects with .test URLs, ports, and health status"
          width="1200"
          height="800"
          loading="lazy"
        />
      </div>
      <p class="dashboard-note">
        Pin <code>https://outport.test</code> in your browser after running <code>outport setup</code>.
      </p>
    </section>

    <hr class="section-divider">

    <!-- MONOREPO EXAMPLE -->
    <section class="example">
      <h2>One config, fully wired</h2>
      <p class="example-caption">Cross-service URLs resolve automatically. Change nothing when switching between main and worktrees.</p>
      <div class="example-pair">
        <div class="example-block">
          <div class="example-label">outport.yml</div>
          <pre class="step-code"><span class="dim"># outport.yml</span>
name: myapp
services:
  rails:
    env_var: PORT
    hostname: myapp.test
  frontend:
    env_var: NUXT_PORT
    hostname: myapp-frontend.test
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
        <div><span class="dollar">$</span> brew install steveclarke/tap/outport</div>
        <div><span class="dollar">$</span> outport setup</div>
      </div>
      <p class="install-note">macOS and Linux. See <a href="/guide/installation">all install options</a>.</p>
    </section>

    <!-- FOOTER -->
    <footer class="site-footer">
      MIT License &middot;
      <a href="https://github.com/steveclarke/outport" target="_blank" rel="noopener">GitHub</a>
      &middot; Built in <a href="https://en.wikipedia.org/wiki/Newfoundland_(island)" target="_blank" rel="noopener">Newfoundland</a>
      <a href="https://en.wikipedia.org/wiki/Flag_of_Newfoundland_and_Labrador" target="_blank" rel="noopener" title="Flag of Newfoundland and Labrador"><svg class="nfld-flag" viewBox="0 0 43200 21600" xmlns="http://www.w3.org/2000/svg"><path fill="#fff" d="m0 0h43200v21600H0z"/><path fill="#003865" d="m3159 900h13547v7822zm0 19800h13547v-7822zm-1559-948v-8152h14121zm0-17904v8152h14121z"/><path fill="#ba0c2f" d="m18706 8722V900h23094zm0 1154h17894l3000 924-3000 924H18706l1600-924zm0 3002v7822h23094zm17631 6922H19606v-5667zm0-18000H19606v5667z"/><path fill="#ffa400" d="m20199 10276h16341l1701 524-1701 524H20199l907-524z"/></svg></a>
      by <a href="https://github.com/steveclarke" target="_blank" rel="noopener">Steve Clarke</a>
    </footer>
  </div>
</template>

<style scoped>
.home-layout {
  font-family: var(--vp-font-family-base);
  color: var(--vp-c-text-1);
  line-height: 1.6;
  overflow-x: hidden;
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
  color: var(--outport-heading);
  letter-spacing: -0.02em;
  line-height: 1.15;
  margin-bottom: 1rem;
}
.hero h1 .accent { color: var(--outport-accent); }
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
.pain-points {
  list-style: none;
  padding: 0;
  margin: 0 auto 2.5rem;
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 0.5rem;
  max-width: 750px;
}
.pain-points li {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.8rem;
  color: var(--vp-c-text-3);
  background: var(--vp-c-bg-soft);
  border: 1px solid var(--vp-c-divider);
  padding: 0.35rem 0.75rem;
  border-radius: 100px;
  white-space: nowrap;
  transition: border-color 0.2s;
}
.pain-points li:hover {
  border-color: var(--outport-accent);
}
.pain-points li::before {
  content: '✕';
  font-size: 0.65rem;
  font-weight: 700;
  color: var(--outport-accent);
  flex-shrink: 0;
}
.pain-points .pain-text {
  text-decoration: line-through;
  text-decoration-color: color-mix(in srgb, var(--vp-c-text-3) 40%, transparent);
}
.pain-points code {
  background: none;
  font-size: 0.9em;
  text-decoration: line-through;
  text-decoration-color: color-mix(in srgb, var(--vp-c-text-3) 40%, transparent);
}
.story-link {
  display: inline-block;
  margin-top: 0;
  margin-bottom: 2rem;
  font-size: 0.9rem;
  color: var(--vp-c-text-3);
  text-decoration: none;
}
.story-link:hover {
  text-decoration: underline;
}
.hero-ctas {
  display: flex;
  gap: 1rem;
  justify-content: center;
  margin-bottom: 3rem;
}
.btn-primary {
  background: var(--outport-accent);
  color: white;
  padding: 0.75rem 1.75rem;
  border-radius: 8px;
  font-weight: 600;
  font-size: 0.95rem;
  text-decoration: none;
  transition: background 0.15s;
  border: none;
}
.btn-primary:hover { background: var(--outport-accent-hover); }
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
  box-shadow: 0 20px 60px var(--outport-shadow), 0 4px 16px var(--outport-shadow-soft);
  min-width: 0;
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
  max-width: 100%;
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
  overflow: hidden;
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
  color: var(--outport-accent);
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
  color: var(--outport-heading);
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
  background: var(--outport-surface);
  border: 1px solid var(--vp-c-divider);
  border-radius: 10px;
  padding: 1.75rem;
  transition: box-shadow 0.2s;
}
.feature-card:hover {
  box-shadow: 0 4px 16px var(--outport-shadow-soft);
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
  color: var(--outport-heading);
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
  color: var(--outport-heading);
  margin-bottom: 0.5rem;
}
.feature-card p {
  color: var(--vp-c-text-3);
  font-size: 0.9rem;
  line-height: 1.6;
}
.feature-card a {
  color: var(--outport-accent);
  text-decoration: none;
  font-weight: 500;
  cursor: pointer;
}
.feature-card a:hover {
  text-decoration: underline;
}
.feature-card code {
  background: var(--vp-c-bg-soft);
  padding: 0.1em 0.4em;
  border-radius: 4px;
  font-size: 0.9em;
}

/* DASHBOARD */
.dashboard-section {
  max-width: 1100px;
  margin: 0 auto;
  padding: 4rem 2rem;
  text-align: center;
}
.dashboard-section h2 {
  font-family: 'Barlow', ui-sans-serif, system-ui, sans-serif;
  font-weight: 700;
  font-size: 2rem;
  color: var(--outport-heading);
  letter-spacing: -0.01em;
  margin-bottom: 0.75rem;
}
.dashboard-caption {
  color: var(--vp-c-text-3);
  font-size: 1rem;
  max-width: 500px;
  margin: 0 auto 2rem;
  line-height: 1.6;
}
.dashboard-frame {
  border-radius: 12px;
  overflow: hidden;
  box-shadow: 0 20px 60px var(--outport-shadow), 0 4px 16px var(--outport-shadow-soft);
  border: 1px solid var(--vp-c-divider);
}
.dashboard-frame img {
  width: 100%;
  height: auto;
  vertical-align: bottom;
}
.dashboard-note {
  color: var(--vp-c-text-3);
  font-size: 0.85rem;
  margin-top: 1.25rem;
}
.dashboard-note code {
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
  max-width: 100%;
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
  color: var(--outport-heading);
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
  color: var(--outport-heading);
  margin-bottom: 2rem;
}
.install p {
  color: var(--vp-c-text-3);
  margin-bottom: 2rem;
}
.install-cmd {
  display: inline-block;
  text-align: left;
  background: #1a1b26;
  color: #c0caf5;
  padding: 1rem 1.5rem;
  border-radius: 10px;
  font-family: var(--vp-font-family-mono);
  font-size: 0.9rem;
  box-shadow: 0 4px 16px var(--outport-shadow-soft);
  max-width: 100%;
  overflow-x: auto;
  box-sizing: border-box;
}
.install-cmd .dollar { color: #7aa2f7; }
.install-note {
  color: var(--vp-c-text-3);
  font-size: 0.8rem;
  margin-top: 1rem;
}

/* FOOTER — styles in custom.css (shared with doc pages) */

/* Responsive */
@media (max-width: 768px) {
  .hero { padding: 2.5rem 1rem 2rem; }
  .hero h1 { font-size: 1.75rem; }
  .hero .subtitle { font-size: 0.95rem; margin-bottom: 1.5rem; max-width: 100%; }
  .pain-points { justify-content: center; gap: 0.4rem; margin-bottom: 1.5rem; }
  .pain-points li { font-size: 0.7rem; padding: 0.25rem 0.6rem; }
  .hero-ctas { flex-direction: column; align-items: center; gap: 0.75rem; margin-bottom: 2rem; }
  .btn-primary, .btn-secondary { width: 100%; max-width: 280px; text-align: center; padding: 0.65rem 1.5rem; }
  .story-link { margin-bottom: 1.5rem; }
  .terminal-pair { flex-direction: column; gap: 1rem; }
  .terminal { border-radius: 8px; }
  .terminal-body { font-size: 0.55rem; padding: 0.75rem 0.75rem; overflow-x: auto; line-height: 1.6; }
  .terminal-bar { padding: 8px 12px; }
  .terminal-dot { width: 8px; height: 8px; }
  .features { padding: 2.5rem 1rem; }
  .features-heading h2 { font-size: 1.5rem; }
  .feature-grid { gap: 0.75rem; }
  .feature-card { flex: 0 1 100%; padding: 1.25rem; }
  .feature-card h3 { font-size: 1rem; }
  .feature-card p { font-size: 0.85rem; }
  .dashboard-section { padding: 2.5rem 1rem; }
  .dashboard-section h2 { font-size: 1.5rem; }
  .dashboard-frame { border-radius: 8px; }
  .example { padding: 2.5rem 1rem; }
  .example h2 { font-size: 1.5rem; }
  .example-caption { font-size: 0.9rem; }
  .example-pair { flex-direction: column; gap: 1rem; }
  .step-code { font-size: 0.55rem; padding: 0.6rem 0.75rem; }
  .install { padding: 2.5rem 1rem; }
  .install h2 { font-size: 1.5rem; }
  .install-cmd { font-size: 0.7rem; padding: 0.75rem 1rem; }
  .hero-callout { font-size: 0.85rem; }
  .adoption-note p { font-size: 0.85rem; }
  .site-footer { padding: 1.5rem 1rem; font-size: 0.75rem; }
}

@media (max-width: 380px) {
  .hero h1 { font-size: 1.5rem; }
  .hero .subtitle { font-size: 0.9rem; }
  .terminal-body { font-size: 0.5rem; padding: 0.5rem 0.6rem; }
  .step-code { font-size: 0.5rem; padding: 0.5rem 0.6rem; }
  .pain-points li { font-size: 0.6rem; padding: 0.2rem 0.5rem; }
  .install-cmd { font-size: 0.65rem; }
}
</style>

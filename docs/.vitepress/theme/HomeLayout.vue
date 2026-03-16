<template>
  <div class="home-layout">
    <!-- HERO -->
    <section class="hero">
      <h1>Stop fighting <span class="accent">port conflicts</span></h1>
      <p class="subtitle">
        Port orchestration for multi-project development. Deterministic ports,
        <code>.test</code> hostnames, <code>.env</code> integration, and
        multi-instance support — all from a single config file.
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

      <div class="terminal">
        <div class="terminal-bar">
          <div class="terminal-dot r" />
          <div class="terminal-dot y" />
          <div class="terminal-dot g" />
        </div>
        <div class="terminal-body">
          <div><span class="prompt">$</span> <span class="cmd">outport apply</span></div>
          <br>
          <div><span class="success">myapp</span> <span class="dim">&middot;</span> <span class="dim">main</span></div>
          <br>
          <div>&nbsp;&nbsp;<span class="dim">service</span>&nbsp;&nbsp;&nbsp;&nbsp;<span class="dim">port</span>&nbsp;&nbsp;&nbsp;<span class="dim">hostname</span></div>
          <div>&nbsp;&nbsp;<span class="cmd">rails</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="value">13842</span>&nbsp;&nbsp;<span class="success">myapp.test</span></div>
          <div>&nbsp;&nbsp;<span class="cmd">postgres</span>&nbsp;&nbsp;<span class="value">28391</span></div>
          <div>&nbsp;&nbsp;<span class="cmd">redis</span>&nbsp;&nbsp;&nbsp;&nbsp;&nbsp;<span class="value">19204</span></div>
          <br>
          <div><span class="dim">&rarr; .env updated</span></div>
        </div>
      </div>
    </section>

    <hr class="section-divider">

    <!-- FEATURES -->
    <section class="features">
      <div class="features-heading">
        <h2>What you get</h2>
      </div>
      <div class="feature-grid">
        <div class="feature-card">
          <div class="feature-icon">#</div>
          <h3>Deterministic Ports</h3>
          <p>Same project, same ports — every time, on every machine. No more "is 3000 free?" guessing games.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">.t</div>
          <h3>.test Domains</h3>
          <p>Real hostnames like <code>myapp.test</code> instead of <code>localhost:3000</code>. Built-in DNS and reverse proxy.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">&gt;_</div>
          <h3>Multi-Instance</h3>
          <p>Worktrees, branches, or multiple checkouts — each gets its own ports and hostnames. No collisions.</p>
        </div>
        <div class="feature-card">
          <div class="feature-icon">.e</div>
          <h3>.env Integration</h3>
          <p>Ports, URLs, and derived values written directly to your <code>.env</code> files. Your app reads them natively.</p>
        </div>
      </div>
    </section>

    <hr class="section-divider">

    <!-- HOW IT WORKS -->
    <section class="how-it-works">
      <h2>Three commands. Zero thinking.</h2>
      <div class="steps">
        <div class="step">
          <div class="step-number">1</div>
          <h3>Configure</h3>
          <p class="step-desc">Define your services in <code>.outport.yml</code></p>
          <pre class="step-code"><span class="dim"># .outport.yml</span>
name: myapp
services:
  rails:
    env_var: PORT
    hostname: myapp
    protocol: http
  postgres:
    env_var: PGPORT
  redis:
    env_var: REDIS_PORT</pre>
        </div>
        <div class="step">
          <div class="step-number">2</div>
          <h3>Apply</h3>
          <p class="step-desc">Run <code>outport apply</code> and ports are allocated</p>
          <pre class="step-code"><span class="prompt">$</span> <span class="cmd">outport apply</span>

<span class="dim">myapp &middot; main</span>

  rails      <span class="value">13842</span>  <span class="success">myapp.test</span>
  postgres   <span class="value">28391</span>
  redis      <span class="value">19204</span>

<span class="dim">&rarr; .env updated</span></pre>
        </div>
        <div class="step">
          <div class="step-number">3</div>
          <h3>Develop</h3>
          <p class="step-desc">Your <code>.env</code> has everything. Start coding.</p>
          <pre class="step-code"><span class="dim"># .env</span>
PORT=<span class="value">13842</span>
PGPORT=<span class="value">28391</span>
REDIS_PORT=<span class="value">19204</span>
MYAPP_URL=<span class="success">http://myapp.test</span></pre>
        </div>
      </div>
    </section>

    <hr class="section-divider">

    <!-- INSTALL -->
    <section class="install">
      <h2>Get started in seconds</h2>
      <p>One command. Single binary. No dependencies.</p>
      <div class="install-cmd">
        <span class="dollar">$</span> brew install steveclarke/tap/outport
      </div>
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
  max-width: 680px;
  margin: 0 auto;
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
}
.terminal-body .prompt { color: #7aa2f7; }
.terminal-body .cmd { color: #c0caf5; }
.terminal-body .success { color: #9ece6a; }
.terminal-body .value { color: #e0af68; }
.terminal-body .dim { color: #565f89; }

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
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 1.25rem;
}
.feature-card {
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

/* HOW IT WORKS */
.how-it-works {
  max-width: 1100px;
  margin: 0 auto;
  padding: 4rem 2rem;
}
.how-it-works h2 {
  font-family: 'Barlow', ui-sans-serif, system-ui, sans-serif;
  font-weight: 700;
  font-size: 2rem;
  color: #031C54;
  text-align: center;
  margin-bottom: 2.5rem;
}
.steps {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  grid-template-rows: auto auto auto 1fr;
  gap: 0 1.5rem;
}
.step {
  background: #ffffff;
  border: 1px solid var(--vp-c-divider);
  border-radius: 10px;
  padding: 1.75rem;
  display: grid;
  grid-row: span 4;
  grid-template-rows: subgrid;
  row-gap: 0;
}
.step-number {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px; height: 28px;
  border-radius: 50%;
  background: #031C54;
  color: white;
  font-family: 'Barlow', sans-serif;
  font-weight: 700;
  font-size: 0.85rem;
  margin-bottom: 1rem;
}
.step h3 {
  font-family: 'Barlow', ui-sans-serif, system-ui, sans-serif;
  font-weight: 700;
  color: #031C54;
  margin-bottom: 0.5rem;
}
.step-desc {
  color: var(--vp-c-text-3);
  font-size: 0.9rem;
}
.step-desc code {
  background: var(--vp-c-bg-soft);
  padding: 0.1em 0.4em;
  border-radius: 4px;
  font-size: 0.9em;
}
.step-code {
  display: block;
  background: #1a1b26;
  color: #a9b1d6;
  padding: 0.75rem 1rem;
  border-radius: 6px;
  font-size: 0.8rem;
  margin-top: 0.75rem;
  font-family: var(--vp-font-family-mono);
  white-space: pre;
  overflow-x: auto;
  align-self: stretch;
}
.step-code .prompt { color: #7aa2f7; }
.step-code .cmd { color: #c0caf5; }
.step-code .success { color: #9ece6a; }
.step-code .value { color: #e0af68; }
.step-code .dim { color: #565f89; }

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
  margin-bottom: 0.75rem;
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
  .feature-grid { grid-template-columns: 1fr; }
  .steps { grid-template-columns: 1fr; }
  .hero-ctas { flex-direction: column; align-items: center; }
}
</style>

<script>
  import { api } from '$lib/api.js';
  import { goto } from '$app/navigation';

  let activeSection = $state('intro');
  let playgroundInput = $state(`# Try editing this Hivefile!
[service.web]
image = "nginx:alpine"
replicas = 2

[service.web.ports]
"8080" = "80"

[service.web.health]
type = "http"
path = "/"
port = 80
interval = "15s"

[service.web.resources]
memory = "128M"
cpus = 0.5
`);
  let validating = $state(false);
  let validationResult = $state(null);
  let copiedId = $state('');

  const sections = [
    { id: 'intro', label: 'Getting Started' },
    { id: 'hivefile', label: 'Hivefile Basics' },
    { id: 'services', label: 'Services' },
    { id: 'env', label: 'Environment & Secrets' },
    { id: 'health', label: 'Health Checks' },
    { id: 'volumes', label: 'Volumes' },
    { id: 'resources', label: 'Resources & Scaling' },
    { id: 'deploy-strategies', label: 'Deploy Strategies' },
    { id: 'ingress', label: 'Ingress & TLS' },
    { id: 'cron', label: 'Cron Jobs' },
    { id: 'recipes', label: 'App Store Recipes' },
    { id: 'playground', label: 'Playground' },
    { id: 'reference', label: 'Quick Reference' },
  ];

  async function validate() {
    validating = true;
    try {
      validationResult = await api.validate(playgroundInput, false);
    } catch (e) {
      validationResult = { valid: false, errors: [e.message] };
    }
    validating = false;
  }

  function deployFromPlayground() {
    sessionStorage.setItem('hive_draft_toml', playgroundInput);
    goto('/deploy');
  }

  function copyBlock(id, text) {
    navigator.clipboard.writeText(text);
    copiedId = id;
    setTimeout(() => { copiedId = ''; }, 2000);
  }

  function scrollTo(id) {
    activeSection = id;
    const el = document.getElementById(id);
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }
</script>

<div class="page-header animate-in">
  <h1 class="page-title">Learn Hive</h1>
  <span class="muted" style="font-size:0.8125rem">Interactive guide to Hivefiles and TOML configuration</span>
</div>

<div class="learn-layout">
  <!-- Table of Contents -->
  <nav class="learn-toc">
    <div class="learn-toc-title">Contents</div>
    {#each sections as s}
      <a
        href="#{s.id}"
        class:active={activeSection === s.id}
        onclick={(e) => { e.preventDefault(); scrollTo(s.id); }}
      >{s.label}</a>
    {/each}
  </nav>

  <!-- Content -->
  <div>
    <!-- Getting Started -->
    <section id="intro" class="learn-section">
      <h2>Getting Started</h2>
      <p>
        Hive manages containers using <strong>Hivefiles</strong> — simple TOML configuration files that describe
        your services, their resources, health checks, and deployment strategies. If you can read a config file,
        you can use Hive.
      </p>
      <div class="callout callout-tip">
        <div class="callout-title">Why TOML?</div>
        TOML is designed to be <strong>easy to read and write</strong>. Unlike YAML, it has no significant whitespace issues.
        Unlike JSON, it supports comments and is human-friendly. Every Hivefile is valid TOML.
      </div>
      <h3>Three ways to deploy</h3>
      <ol>
        <li><strong>CLI:</strong> <code>hive deploy my-app.toml</code> — direct from terminal</li>
        <li><strong>Web Console:</strong> Use the <a href="/deploy">Deploy</a> page to paste or edit TOML</li>
        <li><strong>App Store:</strong> One-click install from the <a href="/appstore">App Store</a> catalog</li>
      </ol>
    </section>

    <!-- Hivefile Basics -->
    <section id="hivefile" class="learn-section">
      <h2>Hivefile Basics</h2>
      <p>A Hivefile defines one or more services. Each service is a TOML table under <code>[service.NAME]</code>.</p>

      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML — minimal Hivefile</span>
          <button class="code-block-copy" onclick={() => copyBlock('basic', '[service.web]\nimage = "nginx:alpine"\nreplicas = 1')}>
            {copiedId === 'basic' ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre><span class="tok-section">[service.web]</span>
<span class="tok-key">image</span> = <span class="tok-str">"nginx:alpine"</span>
<span class="tok-key">replicas</span> = <span class="tok-num">1</span></pre>
      </div>

      <p>That's a complete, deployable Hivefile. Hive fills in sensible defaults for everything else:</p>
      <ul>
        <li><code>replicas</code> defaults to <code>1</code></li>
        <li><code>restart_policy</code> defaults to <code>"on-failure"</code></li>
        <li><code>deploy.strategy</code> defaults to <code>"rolling"</code></li>
        <li>Health checks default to <code>30s</code> interval, <code>5s</code> timeout, <code>3</code> retries</li>
      </ul>
    </section>

    <!-- Services -->
    <section id="services" class="learn-section">
      <h2>Services</h2>
      <p>A service is the core deployment unit. You can define multiple services in a single Hivefile — they'll share a Docker network and discover each other automatically.</p>

      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML — multi-service stack</span>
          <button class="code-block-copy" onclick={() => copyBlock('multi', `[service.api]\nimage = "myapp/api:v1.2"\nreplicas = 3\n\n[service.api.ports]\n"3000" = "3000"\n\n[service.api.depends_on]\nservices = ["db"]\n\n[service.db]\nimage = "postgres:16-alpine"\nreplicas = 1\n\n[service.db.env]\nPOSTGRES_PASSWORD = "{{ secret:db-pass }}"`)}>
            {copiedId === 'multi' ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre><span class="tok-section">[service.api]</span>
<span class="tok-key">image</span> = <span class="tok-str">"myapp/api:v1.2"</span>
<span class="tok-key">replicas</span> = <span class="tok-num">3</span>

<span class="tok-section">[service.api.ports]</span>
<span class="tok-str">"3000"</span> = <span class="tok-str">"3000"</span>

<span class="tok-section">[service.api.depends_on]</span>
<span class="tok-key">services</span> = <span class="tok-bracket">[</span><span class="tok-str">"db"</span><span class="tok-bracket">]</span>

<span class="tok-section">[service.db]</span>
<span class="tok-key">image</span> = <span class="tok-str">"postgres:16-alpine"</span>
<span class="tok-key">replicas</span> = <span class="tok-num">1</span>

<span class="tok-section">[service.db.env]</span>
<span class="tok-key">POSTGRES_PASSWORD</span> = <span class="tok-str">"<span class="tok-placeholder">{{ secret:db-pass }}</span>"</span></pre>
      </div>

      <div class="callout callout-info">
        <div class="callout-title">Service Discovery</div>
        Services in the same deployment share a Docker network. Each service is reachable by name:
        <code>http://db:5432</code>, <code>http://api:3000</code>. Hive also injects
        <code>HIVE_SERVICE_*</code> environment variables for each peer.
      </div>
    </section>

    <!-- Environment & Secrets -->
    <section id="env" class="learn-section">
      <h2>Environment &amp; Secrets</h2>
      <p>Environment variables go under <code>[service.NAME.env]</code>. For sensitive values, use the <code>{"{{ secret:KEY }}"}</code> placeholder syntax.</p>

      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML — env vars and secrets</span>
          <button class="code-block-copy" onclick={() => copyBlock('env', `[service.app.env]\nAPP_ENV = "production"\nDATABASE_URL = "{{ secret:db-url }}"\nAPI_KEY = "{{ secret:api-key }}"`)}>
            {copiedId === 'env' ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre><span class="tok-section">[service.app.env]</span>
<span class="tok-key">APP_ENV</span> = <span class="tok-str">"production"</span>
<span class="tok-key">DATABASE_URL</span> = <span class="tok-str">"<span class="tok-placeholder">{{ secret:db-url }}</span>"</span>
<span class="tok-key">API_KEY</span> = <span class="tok-str">"<span class="tok-placeholder">{{ secret:api-key }}</span>"</span></pre>
      </div>

      <p>Secrets are stored encrypted (age/X25519) and injected at deploy time. Manage them via:</p>
      <ul>
        <li>CLI: <code>hive secret set db-url "postgres://..."</code></li>
        <li>Web: <a href="/secrets">Secrets</a> page</li>
      </ul>
    </section>

    <!-- Health Checks -->
    <section id="health" class="learn-section">
      <h2>Health Checks</h2>
      <p>Health checks let Hive know when your service is ready and detect failures. Three types are supported:</p>

      <div class="grid-3" style="margin:1rem 0">
        <div class="card" style="text-align:center; padding:1rem">
          <div style="font-size:1.5rem; margin-bottom:0.5rem">🌐</div>
          <div style="font-weight:600; font-size:0.875rem">HTTP</div>
          <div class="muted" style="font-size:0.75rem">Checks for 2xx status code</div>
        </div>
        <div class="card" style="text-align:center; padding:1rem">
          <div style="font-size:1.5rem; margin-bottom:0.5rem">🔌</div>
          <div style="font-weight:600; font-size:0.875rem">TCP</div>
          <div class="muted" style="font-size:0.75rem">Checks port connectivity</div>
        </div>
        <div class="card" style="text-align:center; padding:1rem">
          <div style="font-size:1.5rem; margin-bottom:0.5rem">⚙</div>
          <div style="font-weight:600; font-size:0.875rem">Exec</div>
          <div class="muted" style="font-size:0.75rem">Runs command, checks exit 0</div>
        </div>
      </div>

      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML — health check examples</span>
          <button class="code-block-copy" onclick={() => copyBlock('health', `# HTTP health check\n[service.web.health]\ntype = "http"\npath = "/healthz"\nport = 8080\ninterval = "15s"\ntimeout = "3s"\nretries = 3\n\n# TCP health check\n[service.db.health]\ntype = "tcp"\nport = 5432\n\n# Exec health check\n[service.worker.health]\ntype = "exec"\nexec_command = ["./check.sh"]`)}>
            {copiedId === 'health' ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre><span class="tok-comment"># HTTP health check</span>
<span class="tok-section">[service.web.health]</span>
<span class="tok-key">type</span> = <span class="tok-str">"http"</span>
<span class="tok-key">path</span> = <span class="tok-str">"/healthz"</span>
<span class="tok-key">port</span> = <span class="tok-num">8080</span>
<span class="tok-key">interval</span> = <span class="tok-str">"15s"</span>
<span class="tok-key">timeout</span> = <span class="tok-str">"3s"</span>
<span class="tok-key">retries</span> = <span class="tok-num">3</span>

<span class="tok-comment"># TCP health check</span>
<span class="tok-section">[service.db.health]</span>
<span class="tok-key">type</span> = <span class="tok-str">"tcp"</span>
<span class="tok-key">port</span> = <span class="tok-num">5432</span>

<span class="tok-comment"># Exec health check</span>
<span class="tok-section">[service.worker.health]</span>
<span class="tok-key">type</span> = <span class="tok-str">"exec"</span>
<span class="tok-key">exec_command</span> = <span class="tok-bracket">[</span><span class="tok-str">"./check.sh"</span><span class="tok-bracket">]</span></pre>
      </div>
    </section>

    <!-- Volumes -->
    <section id="volumes" class="learn-section">
      <h2>Volumes</h2>
      <p>Persistent storage uses named volumes (managed by Docker) or host bind mounts. Use <code>[[service.NAME.volumes]]</code> (double brackets = array).</p>

      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML — volumes</span>
          <button class="code-block-copy" onclick={() => copyBlock('vol', `[[service.db.volumes]]\nname = "pg-data"\ntarget = "/var/lib/postgresql/data"\n\n[[service.db.volumes]]\nname = "pg-config"\ntarget = "/etc/postgresql"\nread_only = true`)}>
            {copiedId === 'vol' ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre><span class="tok-section">[[service.db.volumes]]</span>
<span class="tok-key">name</span> = <span class="tok-str">"pg-data"</span>
<span class="tok-key">target</span> = <span class="tok-str">"/var/lib/postgresql/data"</span>

<span class="tok-section">[[service.db.volumes]]</span>
<span class="tok-key">name</span> = <span class="tok-str">"pg-config"</span>
<span class="tok-key">target</span> = <span class="tok-str">"/etc/postgresql"</span>
<span class="tok-key">read_only</span> = <span class="tok-bool">true</span></pre>
      </div>

      <div class="callout callout-warn">
        <div class="callout-title">Named volumes vs bind mounts</div>
        Named volumes (<code>name</code>) are portable across nodes. Bind mounts (<code>linux = "/host/path:/container/path"</code>)
        pin data to a specific machine and won't move during rescheduling.
      </div>
    </section>

    <!-- Resources & Scaling -->
    <section id="resources" class="learn-section">
      <h2>Resources &amp; Scaling</h2>
      <p>Control how much CPU and memory each service gets, and configure automatic scaling.</p>

      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML — resources and autoscaling</span>
          <button class="code-block-copy" onclick={() => copyBlock('res', `[service.api]\nimage = "myapp/api:latest"\nreplicas = 2\n\n[service.api.resources]\nmemory = "512M"\ncpus = 1.0\n\n[service.api.autoscale]\nmin = 2\nmax = 10\ncpu_target = 70.0\ncooldown_up = "60s"\ncooldown_down = "300s"`)}>
            {copiedId === 'res' ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre><span class="tok-section">[service.api]</span>
<span class="tok-key">image</span> = <span class="tok-str">"myapp/api:latest"</span>
<span class="tok-key">replicas</span> = <span class="tok-num">2</span>

<span class="tok-section">[service.api.resources]</span>
<span class="tok-key">memory</span> = <span class="tok-str">"512M"</span>
<span class="tok-key">cpus</span> = <span class="tok-num">1.0</span>

<span class="tok-section">[service.api.autoscale]</span>
<span class="tok-key">min</span> = <span class="tok-num">2</span>
<span class="tok-key">max</span> = <span class="tok-num">10</span>
<span class="tok-key">cpu_target</span> = <span class="tok-num">70.0</span>
<span class="tok-key">cooldown_up</span> = <span class="tok-str">"60s"</span>
<span class="tok-key">cooldown_down</span> = <span class="tok-str">"300s"</span></pre>
      </div>

      <p>You can also scale manually: <code>hive scale api 5</code> or from the Services page.</p>
    </section>

    <!-- Deploy Strategies -->
    <section id="deploy-strategies" class="learn-section">
      <h2>Deploy Strategies</h2>
      <p>Hive supports three deployment strategies:</p>

      <div class="grid-3" style="margin:1rem 0">
        <div class="card" style="padding:1rem">
          <div style="font-weight:600; margin-bottom:0.25rem; color:var(--green)">Rolling</div>
          <div class="muted" style="font-size:0.8125rem">Replaces containers one at a time. Zero downtime. <em>Default.</em></div>
        </div>
        <div class="card" style="padding:1rem">
          <div style="font-weight:600; margin-bottom:0.25rem; color:var(--cyan)">Canary</div>
          <div class="muted" style="font-size:0.8125rem">Routes a small % of traffic to the new version first.</div>
        </div>
        <div class="card" style="padding:1rem">
          <div style="font-weight:600; margin-bottom:0.25rem; color:var(--purple)">Blue-Green</div>
          <div class="muted" style="font-size:0.8125rem">Spins up full new set, then cuts over instantly.</div>
        </div>
      </div>

      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML — deploy strategy</span>
          <button class="code-block-copy" onclick={() => copyBlock('deploy', `[service.web.deploy]\nstrategy = "canary"\ncanary_weight = 10`)}>
            {copiedId === 'deploy' ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre><span class="tok-section">[service.web.deploy]</span>
<span class="tok-key">strategy</span> = <span class="tok-str">"canary"</span>
<span class="tok-key">canary_weight</span> = <span class="tok-num">10</span>     <span class="tok-comment"># 10% of traffic to new version</span></pre>
      </div>
    </section>

    <!-- Ingress & TLS -->
    <section id="ingress" class="learn-section">
      <h2>Ingress &amp; TLS</h2>
      <p>Expose services externally with automatic load balancing and optional TLS termination.</p>

      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML — ingress with TLS</span>
          <button class="code-block-copy" onclick={() => copyBlock('ingress', `[service.web.ports]\n"8080" = "80"\n\n[service.web.ingress]\nport = 8080\ntls = true`)}>
            {copiedId === 'ingress' ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre><span class="tok-section">[service.web.ports]</span>
<span class="tok-str">"8080"</span> = <span class="tok-str">"80"</span>

<span class="tok-section">[service.web.ingress]</span>
<span class="tok-key">port</span> = <span class="tok-num">8080</span>
<span class="tok-key">tls</span> = <span class="tok-bool">true</span>          <span class="tok-comment"># auto-generates self-signed cert</span></pre>
      </div>

      <p>For custom certificates, provide <code>tls_cert</code> and <code>tls_key</code> paths. Hive creates an nginx-based ingress proxy automatically.</p>
    </section>

    <!-- Cron Jobs -->
    <section id="cron" class="learn-section">
      <h2>Cron Jobs</h2>
      <p>Schedule recurring tasks inside a service using standard 5-field cron expressions.</p>

      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML — cron jobs</span>
          <button class="code-block-copy" onclick={() => copyBlock('cron', `[[service.app.cron]]\nschedule = "0 2 * * *"\ncommand = ["./cleanup.sh", "--older-than", "7d"]\n\n[[service.app.cron]]\nschedule = "*/5 * * * *"\ncommand = ["./healthcheck.sh"]`)}>
            {copiedId === 'cron' ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre><span class="tok-section">[[service.app.cron]]</span>
<span class="tok-key">schedule</span> = <span class="tok-str">"0 2 * * *"</span>        <span class="tok-comment"># daily at 2 AM</span>
<span class="tok-key">command</span> = <span class="tok-bracket">[</span><span class="tok-str">"./cleanup.sh"</span>, <span class="tok-str">"--older-than"</span>, <span class="tok-str">"7d"</span><span class="tok-bracket">]</span>

<span class="tok-section">[[service.app.cron]]</span>
<span class="tok-key">schedule</span> = <span class="tok-str">"*/5 * * * *"</span>      <span class="tok-comment"># every 5 minutes</span>
<span class="tok-key">command</span> = <span class="tok-bracket">[</span><span class="tok-str">"./healthcheck.sh"</span><span class="tok-bracket">]</span></pre>
      </div>

      <div class="callout callout-info">
        <div class="callout-title">Cron Format</div>
        <code>minute hour day-of-month month day-of-week</code> — same as standard crontab.
        Use <code>*/N</code> for intervals, <code>*</code> for wildcard, <code>1,3,5</code> for lists.
      </div>
    </section>

    <!-- Recipes -->
    <section id="recipes" class="learn-section">
      <h2>App Store Recipes</h2>
      <p>Recipes are TOML templates with metadata for the App Store. They include a <code>[recipe]</code> header with config field definitions, plus standard service blocks.</p>

      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML — recipe format</span>
          <button class="code-block-copy" onclick={() => copyBlock('recipe', `[recipe]\nid = "my-app"\nname = "My App"\ndescription = "A custom application"\nicon = "🚀"\ncategory = "devtools"\ntags = ["custom", "example"]\nimage = "myapp:latest"\nmin_memory = "128M"\n\n  [recipe.config.api_key]\n  label = "API Key"\n  type = "secret"\n  required = true\n  description = "Your API key"\n\n[service.my-app]\nimage = "myapp:latest"\nreplicas = 1\n\n[service.my-app.env]\nAPI_KEY = "{{ config:api_key }}"`)}>
            {copiedId === 'recipe' ? 'Copied!' : 'Copy'}
          </button>
        </div>
        <pre><span class="tok-section">[recipe]</span>
<span class="tok-key">id</span> = <span class="tok-str">"my-app"</span>
<span class="tok-key">name</span> = <span class="tok-str">"My App"</span>
<span class="tok-key">description</span> = <span class="tok-str">"A custom application"</span>
<span class="tok-key">icon</span> = <span class="tok-str">"🚀"</span>
<span class="tok-key">category</span> = <span class="tok-str">"devtools"</span>
<span class="tok-key">tags</span> = <span class="tok-bracket">[</span><span class="tok-str">"custom"</span>, <span class="tok-str">"example"</span><span class="tok-bracket">]</span>
<span class="tok-key">image</span> = <span class="tok-str">"myapp:latest"</span>
<span class="tok-key">min_memory</span> = <span class="tok-str">"128M"</span>

  <span class="tok-section">[recipe.config.api_key]</span>
  <span class="tok-key">label</span> = <span class="tok-str">"API Key"</span>
  <span class="tok-key">type</span> = <span class="tok-str">"secret"</span>
  <span class="tok-key">required</span> = <span class="tok-bool">true</span>
  <span class="tok-key">description</span> = <span class="tok-str">"Your API key"</span>

<span class="tok-section">[service.my-app]</span>
<span class="tok-key">image</span> = <span class="tok-str">"myapp:latest"</span>
<span class="tok-key">replicas</span> = <span class="tok-num">1</span>

<span class="tok-section">[service.my-app.env]</span>
<span class="tok-key">API_KEY</span> = <span class="tok-str">"<span class="tok-placeholder">{{ config:api_key }}</span>"</span></pre>
      </div>

      <p>Custom recipes can be added via the CLI (<code>hive app add recipe.toml</code>) or the App Store's custom app feature.</p>
    </section>

    <!-- Playground -->
    <section id="playground" class="learn-section">
      <h2>Playground</h2>
      <p>Write a Hivefile and validate it live against the Hive daemon. Edit the TOML below and hit <strong>Validate</strong>.</p>

      <div class="playground">
        <div class="playground-editor">
          <div class="playground-toolbar">
            <span class="playground-label">Editor</span>
            <div class="btn-group">
              <button class="btn btn-sm" onclick={validate} disabled={validating}>
                {validating ? 'Validating...' : 'Validate'}
              </button>
              <button class="btn btn-sm btn-primary" onclick={deployFromPlayground}>Deploy</button>
            </div>
          </div>
          <textarea
            bind:value={playgroundInput}
            spellcheck="false"
            placeholder="# Write your Hivefile here..."
          ></textarea>
        </div>
        <div class="playground-output">
          <div class="playground-toolbar" style="margin:-1rem -1rem 1rem; padding:0.5rem 1rem; border-bottom:1px solid var(--border)">
            <span class="playground-label">Validation Result</span>
          </div>
          {#if validationResult === null}
            <p class="muted" style="font-size:0.8125rem">Click "Validate" to check your TOML.</p>
          {:else if validationResult.valid}
            <div style="display:flex; align-items:center; gap:0.5rem; margin-bottom:0.75rem">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--green)" stroke-width="2.5"><polyline points="20 6 9 17 4 12"/></svg>
              <span class="text-green" style="font-weight:600">Valid Hivefile</span>
            </div>
            {#if validationResult.services?.length}
              <div style="font-size:0.8125rem; color:var(--text-muted)">
                Services found: {validationResult.services.map(s => s.name).join(', ')}
              </div>
            {/if}
            {#if validationResult.warnings?.length}
              <div style="margin-top:0.75rem">
                {#each validationResult.warnings as w}
                  <div style="font-size:0.8125rem; color:var(--yellow); margin-bottom:0.25rem">⚠ {w}</div>
                {/each}
              </div>
            {/if}
          {:else}
            <div style="display:flex; align-items:center; gap:0.5rem; margin-bottom:0.75rem">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--red)" stroke-width="2.5"><line x1="18" y1="6" x2="6" y2="18"/><line x1="6" y1="6" x2="18" y2="18"/></svg>
              <span class="text-red" style="font-weight:600">Validation Failed</span>
            </div>
            {#each (validationResult.errors || [validationResult.error || 'Unknown error']) as err}
              <div style="font-size:0.8125rem; color:var(--red); margin-bottom:0.25rem; font-family:var(--mono)">{err}</div>
            {/each}
          {/if}
        </div>
      </div>
    </section>

    <!-- Quick Reference -->
    <section id="reference" class="learn-section">
      <h2>Quick Reference</h2>
      <p>Complete field reference for Hivefile service blocks.</p>

      <div class="card" style="overflow-x:auto">
        <table>
          <thead>
            <tr>
              <th>Block</th>
              <th>Field</th>
              <th>Type</th>
              <th>Default</th>
              <th>Description</th>
            </tr>
          </thead>
          <tbody>
            <tr><td rowspan="6"><code>[service.X]</code></td><td>image</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Docker image (required)</td></tr>
            <tr><td>replicas</td><td>int</td><td>1</td><td style="font-family:var(--sans)">Number of containers</td></tr>
            <tr><td>platform</td><td>string</td><td>—</td><td style="font-family:var(--sans)">e.g. linux/amd64</td></tr>
            <tr><td>node</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Pin to specific node</td></tr>
            <tr><td>restart_policy</td><td>string</td><td>on-failure</td><td style="font-family:var(--sans)">Docker restart policy</td></tr>
            <tr><td>isolation</td><td>string</td><td>—</td><td style="font-family:var(--sans)">"strict" for network isolation</td></tr>
            <tr><td rowspan="5"><code>[service.X.health]</code></td><td>type</td><td>string</td><td>—</td><td style="font-family:var(--sans)">http, tcp, or exec</td></tr>
            <tr><td>port</td><td>int</td><td>—</td><td style="font-family:var(--sans)">Port to check</td></tr>
            <tr><td>path</td><td>string</td><td>/</td><td style="font-family:var(--sans)">HTTP path (http type only)</td></tr>
            <tr><td>interval</td><td>string</td><td>30s</td><td style="font-family:var(--sans)">Check frequency</td></tr>
            <tr><td>retries</td><td>int</td><td>3</td><td style="font-family:var(--sans)">Failures before unhealthy</td></tr>
            <tr><td><code>[service.X.resources]</code></td><td>memory</td><td>string</td><td>—</td><td style="font-family:var(--sans)">e.g. "256M", "1G"</td></tr>
            <tr><td></td><td>cpus</td><td>float</td><td>—</td><td style="font-family:var(--sans)">e.g. 0.5, 2.0</td></tr>
            <tr><td><code>[service.X.ports]</code></td><td>"host"</td><td>string</td><td>—</td><td style="font-family:var(--sans)">"host_port" = "container_port"</td></tr>
            <tr><td rowspan="3"><code>[service.X.deploy]</code></td><td>strategy</td><td>string</td><td>rolling</td><td style="font-family:var(--sans)">rolling, canary, blue-green</td></tr>
            <tr><td>max_surge</td><td>int</td><td>1</td><td style="font-family:var(--sans)">Extra replicas during rolling</td></tr>
            <tr><td>canary_weight</td><td>int</td><td>10</td><td style="font-family:var(--sans)">% traffic to canary</td></tr>
            <tr><td rowspan="3"><code>[[service.X.volumes]]</code></td><td>name</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Named volume identifier</td></tr>
            <tr><td>target</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Container mount path</td></tr>
            <tr><td>read_only</td><td>bool</td><td>false</td><td style="font-family:var(--sans)">Mount read-only</td></tr>
            <tr><td rowspan="2"><code>[service.X.ingress]</code></td><td>port</td><td>int</td><td>—</td><td style="font-family:var(--sans)">External port</td></tr>
            <tr><td>tls</td><td>bool</td><td>false</td><td style="font-family:var(--sans)">Enable HTTPS</td></tr>
          </tbody>
        </table>
      </div>
    </section>
  </div>
</div>

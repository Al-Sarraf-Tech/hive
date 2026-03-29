import{d as Ps,a as c,b as k,f as g,s as i}from"../chunks/BKzipXb4.js";import{p as Os,c as a,f as pe,t as x,g as t,b as zs,d as s,e as u,s as C,r as e,n,i as Ds}from"../chunks/D67c9Gwo.js";import{i as ve}from"../chunks/Cw8_URSI.js";import{e as me,i as he}from"../chunks/DhzCUvt5.js";import{s as Rs}from"../chunks/CKrvbhRz.js";import{s as Ws}from"../chunks/Dbb-5aXk.js";import{b as Vs}from"../chunks/BUgbhvgh.js";import{i as Bs,a as Ns}from"../chunks/DFm7loE9.js";import{g as Us}from"../chunks/6UGiqg6V.js";var Gs=g("<a> </a>"),Xs=g('<p class="muted" style="font-size:0.8125rem">Click "Validate" to check your TOML.</p>'),Ys=g('<div style="font-size:0.8125rem; color:var(--text-muted)"> </div>'),qs=g('<div style="font-size:0.8125rem; color:var(--yellow); margin-bottom:0.25rem"> </div>'),Ks=g('<div style="margin-top:0.75rem"></div>'),js=g('<div style="display:flex; align-items:center; gap:0.5rem; margin-bottom:0.75rem"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--green)" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg> <span class="text-green" style="font-weight:600">Valid Hivefile</span></div> <!> <!>',1),Fs=g('<div style="font-size:0.8125rem; color:var(--red); margin-bottom:0.25rem; font-family:var(--mono)"> </div>'),Js=g('<div style="display:flex; align-items:center; gap:0.5rem; margin-bottom:0.75rem"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--red)" stroke-width="2.5"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg> <span class="text-red" style="font-weight:600">Validation Failed</span></div> <!>',1),Qs=g(`<div class="page-header animate-in"><h1 class="page-title">Learn Hive</h1> <span class="muted" style="font-size:0.8125rem">Interactive guide to Hivefiles and TOML configuration</span></div> <div class="learn-layout"><nav class="learn-toc"><div class="learn-toc-title">Contents</div> <!></nav> <div><section id="what-is-hive" class="learn-section"><h2>What is Hive?</h2> <p>Hive is a <strong>container orchestrator</strong> — it deploys and manages Docker containers across multiple machines from one place.
        Think of it as the middle ground between Docker Compose (single machine) and Kubernetes (enterprise complexity).</p> <div class="grid-3" style="margin:1.25rem 0"><div class="card" style="text-align:center; padding:1.25rem"><div style="font-size:2rem; margin-bottom:0.5rem">📦</div> <div style="font-weight:700; font-size:0.9rem; margin-bottom:0.25rem">Deploy</div> <div class="muted" style="font-size:0.8rem">Write a TOML file, run one command. Hive pulls images, creates containers, sets up networking.</div></div> <div class="card" style="text-align:center; padding:1.25rem"><div style="font-size:2rem; margin-bottom:0.5rem">🔄</div> <div style="font-weight:700; font-size:0.9rem; margin-bottom:0.25rem">Manage</div> <div class="muted" style="font-size:0.8rem">Scale replicas, roll back, rotate secrets, monitor health — all from CLI, TUI, or this console.</div></div> <div class="card" style="text-align:center; padding:1.25rem"><div style="font-size:2rem; margin-bottom:0.5rem">🌐</div> <div style="font-weight:700; font-size:0.9rem; margin-bottom:0.25rem">Cluster</div> <div class="muted" style="font-size:0.8rem">Add machines to your cluster with one command. Hive distributes containers across nodes automatically.</div></div></div> <h3>What Hive does for you</h3> <ul><li><strong>Container lifecycle</strong> — pull images, create/start/stop/remove containers</li> <li><strong>Health monitoring</strong> — HTTP, TCP, or exec health checks with auto-restart on failure</li> <li><strong>Load balancing</strong> — built-in nginx ingress proxy with health-aware failover</li> <li><strong>Secret management</strong> — encrypted at rest (age/X25519), injected at deploy time</li> <li><strong>Multi-node scheduling</strong> — place replicas across nodes based on resources and constraints</li> <li><strong>Rolling updates</strong> — zero-downtime deploys with health checks between each replica</li> <li><strong>App Store</strong> — 35+ pre-configured apps ready to deploy in one click</li> <li><strong>Encrypted mesh</strong> — optional WireGuard overlay for secure node-to-node communication</li></ul> <div class="callout callout-info"><div class="callout-title">Who is Hive for?</div> If you run 1-20 machines (homelab, small team, staging environment) and want to manage containers
        without the complexity of Kubernetes, Hive is for you. It works on Linux and Windows, needs no cloud provider,
        and runs on a Raspberry Pi or a datacenter server equally well.</div></section> <section id="how-it-works" class="learn-section"><h2>How It Works</h2> <p>Hive has four components that work together:</p> <div class="grid-2" style="margin:1rem 0"><div class="card" style="padding:1rem"><div style="font-weight:700; color:var(--cyan); margin-bottom:0.25rem">hived (daemon)</div> <div class="muted" style="font-size:0.8rem">Runs on every node. Manages containers via Docker API, participates in gossip mesh, serves gRPC + HTTP APIs. Written in Go.</div></div> <div class="card" style="padding:1rem"><div style="font-weight:700; color:var(--green); margin-bottom:0.25rem">hive (CLI)</div> <div class="muted" style="font-size:0.8rem">Command-line tool for deploying, scaling, managing services. Connects to hived via gRPC. Written in Rust.</div></div> <div class="card" style="padding:1rem"><div style="font-weight:700; color:var(--purple); margin-bottom:0.25rem">hivetop (TUI)</div> <div class="muted" style="font-size:0.8rem">Real-time terminal dashboard with 4 tabs: overview, nodes, services, logs. Written in Rust with ratatui.</div></div> <div class="card" style="padding:1rem"><div style="font-weight:700; color:var(--yellow); margin-bottom:0.25rem">Console (Web UI)</div> <div class="muted" style="font-size:0.8rem">This web interface. Built with SvelteKit, embedded in hived. 18 pages for full cluster management.</div></div></div> <h3>The deploy flow</h3> <ol><li>You write a <strong>Hivefile</strong> (TOML) describing your services — or pick one from the App Store</li> <li>Hive <strong>parses</strong> the file, validates it, resolves secret placeholders</li> <li>The <strong>scheduler</strong> picks which node(s) should run each replica based on resources and constraints</li> <li>Hive <strong>pulls the image</strong> (with registry auth if configured), creates a Docker network, starts containers</li> <li>Health checks begin immediately — if a container fails, Hive auto-restarts it</li> <li>If ingress is configured, an nginx proxy container is created for load balancing</li></ol> <h3>Networking</h3> <p>Nodes discover each other via <strong>SWIM gossip</strong> (UDP port 7946) — no central coordinator. State is eventually consistent across all nodes. For secure communication, Hive uses:</p> <ul><li><strong>mTLS</strong> — mutual TLS on the mesh gRPC port (7948) with auto-generated certificates</li> <li><strong>WireGuard</strong> — optional encrypted overlay network (userspace, no root required)</li> <li><strong>Gossip encryption</strong> — optional AES-256 for cluster membership traffic</li></ul></section> <section id="intro" class="learn-section"><h2>Getting Started</h2> <p>Hive manages containers using <strong>Hivefiles</strong> — simple TOML configuration files that describe
        your services, their resources, health checks, and deployment strategies. If you can read a config file,
        you can use Hive.</p> <div class="callout callout-tip"><div class="callout-title">Why TOML?</div> TOML is designed to be <strong>easy to read and write</strong>. Unlike YAML, it has no significant whitespace issues.
        Unlike JSON, it supports comments and is human-friendly. Every Hivefile is valid TOML.</div> <h3>Install Hive</h3> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">Shell — one-line install</span> <button class="code-block-copy"> </button></div> <pre>curl -fsSL https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.sh | bash</pre></div> <p>This installs <code>hived</code>, <code>hive</code>, and <code>hivetop</code> to <code>~/.local/bin</code>.</p> <h3>Start the daemon</h3> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">Shell — start hived</span> <button class="code-block-copy"> </button></div> <pre>hive setup                           <span class="tok-comment"># interactive first-time wizard</span>
<span class="tok-comment"># OR manually:</span>
hived --data-dir /var/lib/hive --log-level info</pre></div> <h3>Three ways to deploy</h3> <ol><li><strong>CLI:</strong> <code>hive deploy my-app.toml</code> — direct from terminal</li> <li><strong>Web Console:</strong> Use the <a href="/deploy">Deploy</a> page to paste or edit TOML</li> <li><strong>App Store:</strong> One-click install from the <a href="/appstore">App Store</a> — 35 apps ready to go</li></ol></section> <section id="clustering" class="learn-section"><h2>Clustering</h2> <p>Hive clusters are peer-to-peer — every node is equal. There's no control plane to manage or single point of failure.</p> <h3>Create a cluster</h3> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">Shell — cluster setup</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-comment"># On the first node:</span>
hive init --name my-cluster
<span class="tok-comment"># Output: Join Code: HIVE-AB12-CD34</span>

<span class="tok-comment"># On other nodes:</span>
hive setup --join HIVE-AB12-CD34</pre></div> <p>When a node joins, Hive automatically:</p> <ul><li>Exchanges gossip metadata (CPU, memory, disk, capabilities)</li> <li>Generates a TLS certificate signed by the cluster CA</li> <li>Sets up WireGuard mesh overlay (if enabled)</li> <li>Begins participating in container scheduling</li></ul> <h3>Node labels and constraints</h3> <p>Label nodes to control where services run:</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">Shell — labels</span></div> <pre>hive node label add worker-01 gpu=true
hive node label add worker-01 region=us-east</pre></div> <p>Then use constraints in your Hivefile:</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — placement constraints</span></div> <pre><span class="tok-section">[service.ml-model]</span>
<span class="tok-key">image</span> = <span class="tok-str">"myapp/model:latest"</span>
<span class="tok-key">constraints</span> = <span class="tok-bracket">[</span><span class="tok-str">"gpu=true"</span>, <span class="tok-str">"region=us-east"</span><span class="tok-bracket">]</span></pre></div></section> <section id="hivefile" class="learn-section"><h2>Hivefile Basics</h2> <p>A Hivefile defines one or more services. Each service is a TOML table under <code>[service.NAME]</code>.</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — minimal Hivefile</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[service.web]</span>
<span class="tok-key">image</span> = <span class="tok-str">"nginx:alpine"</span>
<span class="tok-key">replicas</span> = <span class="tok-num">1</span></pre></div> <p>That's a complete, deployable Hivefile. Hive fills in sensible defaults for everything else:</p> <ul><li><code>replicas</code> defaults to <code>1</code></li> <li><code>restart_policy</code> defaults to <code>"on-failure"</code></li> <li><code>deploy.strategy</code> defaults to <code>"rolling"</code></li> <li>Health checks default to <code>30s</code> interval, <code>5s</code> timeout, <code>3</code> retries</li></ul></section> <section id="services" class="learn-section"><h2>Services</h2> <p>A service is the core deployment unit. You can define multiple services in a single Hivefile — they'll share a Docker network and discover each other automatically.</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — multi-service stack</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[service.api]</span>
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
<span class="tok-key">POSTGRES_PASSWORD</span> = <span class="tok-str">"<span class="tok-placeholder"></span>"</span></pre></div> <div class="callout callout-info"><div class="callout-title">Service Discovery</div> Services in the same deployment share a Docker network. Each service is reachable by name: <code>http://db:5432</code>, <code>http://api:3000</code>. Hive also injects <code>HIVE_SERVICE_*</code> environment variables for each peer.</div></section> <section id="env" class="learn-section"><h2>Environment &amp; Secrets</h2> <p>Environment variables go under <code>[service.NAME.env]</code>. For sensitive values, use the <code></code> placeholder syntax.</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — env vars and secrets</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[service.app.env]</span>
<span class="tok-key">APP_ENV</span> = <span class="tok-str">"production"</span>
<span class="tok-key">DATABASE_URL</span> = <span class="tok-str">"<span class="tok-placeholder"></span>"</span>
<span class="tok-key">API_KEY</span> = <span class="tok-str">"<span class="tok-placeholder"></span>"</span></pre></div> <p>Secrets are stored encrypted (age/X25519) and injected at deploy time. Manage them via:</p> <ul><li>CLI: <code>hive secret set db-url "postgres://..."</code></li> <li>Web: <a href="/secrets">Secrets</a> page</li></ul></section> <section id="health" class="learn-section"><h2>Health Checks</h2> <p>Health checks let Hive know when your service is ready and detect failures. Three types are supported:</p> <div class="grid-3" style="margin:1rem 0"><div class="card" style="text-align:center; padding:1rem"><div style="font-size:1.5rem; margin-bottom:0.5rem">🌐</div> <div style="font-weight:600; font-size:0.875rem">HTTP</div> <div class="muted" style="font-size:0.75rem">Checks for 2xx status code</div></div> <div class="card" style="text-align:center; padding:1rem"><div style="font-size:1.5rem; margin-bottom:0.5rem">🔌</div> <div style="font-weight:600; font-size:0.875rem">TCP</div> <div class="muted" style="font-size:0.75rem">Checks port connectivity</div></div> <div class="card" style="text-align:center; padding:1rem"><div style="font-size:1.5rem; margin-bottom:0.5rem">⚙</div> <div style="font-weight:600; font-size:0.875rem">Exec</div> <div class="muted" style="font-size:0.75rem">Runs command, checks exit 0</div></div></div> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — health check examples</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-comment"># HTTP health check</span>
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
<span class="tok-key">exec_command</span> = <span class="tok-bracket">[</span><span class="tok-str">"./check.sh"</span><span class="tok-bracket">]</span></pre></div></section> <section id="volumes" class="learn-section"><h2>Volumes</h2> <p>Persistent storage uses named volumes (managed by Docker) or host bind mounts. Use <code>[[service.NAME.volumes]]</code> (double brackets = array).</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — volumes</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[[service.db.volumes]]</span>
<span class="tok-key">name</span> = <span class="tok-str">"pg-data"</span>
<span class="tok-key">target</span> = <span class="tok-str">"/var/lib/postgresql/data"</span>

<span class="tok-section">[[service.db.volumes]]</span>
<span class="tok-key">name</span> = <span class="tok-str">"pg-config"</span>
<span class="tok-key">target</span> = <span class="tok-str">"/etc/postgresql"</span>
<span class="tok-key">read_only</span> = <span class="tok-bool">true</span></pre></div> <div class="callout callout-warn"><div class="callout-title">Named volumes vs bind mounts</div> Named volumes (<code>name</code>) are portable across nodes. Bind mounts (<code>linux = "/host/path:/container/path"</code>)
        pin data to a specific machine and won't move during rescheduling.</div></section> <section id="resources" class="learn-section"><h2>Resources &amp; Scaling</h2> <p>Control how much CPU and memory each service gets, and configure automatic scaling.</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — resources and autoscaling</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[service.api]</span>
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
<span class="tok-key">cooldown_down</span> = <span class="tok-str">"300s"</span></pre></div> <p>You can also scale manually: <code>hive scale api 5</code> or from the Services page.</p></section> <section id="deploy-strategies" class="learn-section"><h2>Deploy Strategies</h2> <p>Hive supports three deployment strategies:</p> <div class="grid-3" style="margin:1rem 0"><div class="card" style="padding:1rem"><div style="font-weight:600; margin-bottom:0.25rem; color:var(--green)">Rolling</div> <div class="muted" style="font-size:0.8125rem">Replaces containers one at a time. Zero downtime. <em>Default.</em></div></div> <div class="card" style="padding:1rem"><div style="font-weight:600; margin-bottom:0.25rem; color:var(--cyan)">Canary</div> <div class="muted" style="font-size:0.8125rem">Routes a small % of traffic to the new version first.</div></div> <div class="card" style="padding:1rem"><div style="font-weight:600; margin-bottom:0.25rem; color:var(--purple)">Blue-Green</div> <div class="muted" style="font-size:0.8125rem">Spins up full new set, then cuts over instantly.</div></div></div> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — deploy strategy</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[service.web.deploy]</span>
<span class="tok-key">strategy</span> = <span class="tok-str">"canary"</span>
<span class="tok-key">canary_weight</span> = <span class="tok-num">10</span>     <span class="tok-comment"># 10% of traffic to new version</span></pre></div></section> <section id="ingress" class="learn-section"><h2>Ingress &amp; TLS</h2> <p>Expose services externally with automatic load balancing and optional TLS termination.</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — ingress with TLS</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[service.web.ports]</span>
<span class="tok-str">"8080"</span> = <span class="tok-str">"80"</span>

<span class="tok-section">[service.web.ingress]</span>
<span class="tok-key">port</span> = <span class="tok-num">8080</span>
<span class="tok-key">tls</span> = <span class="tok-bool">true</span>          <span class="tok-comment"># auto-generates self-signed cert</span></pre></div> <p>For custom certificates, provide <code>tls_cert</code> and <code>tls_key</code> paths. Hive creates an nginx-based ingress proxy automatically.</p></section> <section id="cron" class="learn-section"><h2>Cron Jobs</h2> <p>Schedule recurring tasks inside a service using standard 5-field cron expressions.</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — cron jobs</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[[service.app.cron]]</span>
<span class="tok-key">schedule</span> = <span class="tok-str">"0 2 * * *"</span>        <span class="tok-comment"># daily at 2 AM</span>
<span class="tok-key">command</span> = <span class="tok-bracket">[</span><span class="tok-str">"./cleanup.sh"</span>, <span class="tok-str">"--older-than"</span>, <span class="tok-str">"7d"</span><span class="tok-bracket">]</span>

<span class="tok-section">[[service.app.cron]]</span>
<span class="tok-key">schedule</span> = <span class="tok-str">"*/5 * * * *"</span>      <span class="tok-comment"># every 5 minutes</span>
<span class="tok-key">command</span> = <span class="tok-bracket">[</span><span class="tok-str">"./healthcheck.sh"</span><span class="tok-bracket">]</span></pre></div> <div class="callout callout-info"><div class="callout-title">Cron Format</div> <code>minute hour day-of-month month day-of-week</code> — same as standard crontab.
        Use <code>*/N</code> for intervals, <code>*</code> for wildcard, <code>1,3,5</code> for lists.</div></section> <section id="appstore" class="learn-section"><h2>App Store</h2> <p>The App Store has 35 pre-configured apps you can deploy in one click — no Hivefile writing needed.</p> <h3>Browse and install</h3> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">Shell — app store commands</span> <button class="code-block-copy"> </button></div> <pre>hive app ls                                    <span class="tok-comment"># browse all 35 apps</span>
hive app ls --category database                 <span class="tok-comment"># filter by category</span>
hive app search grafana                          <span class="tok-comment"># search by name/tag</span>
hive app info postgres                           <span class="tok-comment"># see details + config fields</span>
hive app install postgres --config db_password=secret  <span class="tok-comment"># deploy!</span></pre></div> <p>Or use the <a href="/appstore">App Store page</a> in this console — browse, configure, and install without writing any TOML.</p> <div class="callout callout-tip"><div class="callout-title">No sign-in needed to browse</div> The App Store is publicly accessible. Sign in only when you're ready to deploy. You can explore all 35 apps,
        read their configs, and preview the generated TOML without an account.</div> <h3>Categories</h3> <div style="display:flex; gap:0.5rem; flex-wrap:wrap; margin:0.75rem 0"><span class="badge">🗃 database</span> <span class="badge">⚡ cache</span> <span class="badge">🌐 webserver</span> <span class="badge">📊 monitoring</span> <span class="badge">🔀 proxy</span> <span class="badge">✉ messaging</span> <span class="badge">💾 storage</span> <span class="badge">🔧 devtools</span> <span class="badge">🎬 media</span> <span class="badge">📋 productivity</span> <span class="badge">🛡 security</span> <span class="badge">🌍 networking</span> <span class="badge">▶ automation</span></div></section> <section id="cli" class="learn-section"><h2>CLI Commands</h2> <p>The <code>hive</code> CLI is the primary way to interact with your cluster. Here are the most important commands:</p> <h3>Cluster management</h3> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">Shell</span></div> <pre>hive setup                    <span class="tok-comment"># interactive first-run wizard</span>
hive init --name my-cluster   <span class="tok-comment"># create a new cluster</span>
hive join --code HIVE-XXXX    <span class="tok-comment"># join an existing cluster</span>
hive status                   <span class="tok-comment"># cluster health summary</span>
hive nodes                    <span class="tok-comment"># list all nodes</span></pre></div> <h3>Deploy and manage</h3> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">Shell</span></div> <pre>hive deploy app.toml          <span class="tok-comment"># deploy from Hivefile</span>
hive ps                       <span class="tok-comment"># list running services</span>
hive logs web -f              <span class="tok-comment"># stream service logs</span>
hive scale web 5              <span class="tok-comment"># scale to 5 replicas</span>
hive stop web                 <span class="tok-comment"># stop a service</span>
hive restart web              <span class="tok-comment"># rolling restart</span>
hive rollback web             <span class="tok-comment"># revert to previous version</span>
hive exec web "ls -la"        <span class="tok-comment"># run command in container</span></pre></div> <h3>Updates and secrets</h3> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">Shell</span></div> <pre>hive update web --image nginx:1.27   <span class="tok-comment"># update image with rolling restart</span>
hive update web --env API_KEY=new    <span class="tok-comment"># update env var</span>
hive secret set db-pass              <span class="tok-comment"># set a secret (reads from stdin)</span>
hive secret rotate db-pass           <span class="tok-comment"># rotate + restart referencing services</span>
hive secret ls                       <span class="tok-comment"># list secrets</span></pre></div> <h3>Validation and preview</h3> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">Shell</span></div> <pre>hive validate app.toml        <span class="tok-comment"># check for errors without deploying</span>
hive diff app.toml            <span class="tok-comment"># preview what would change</span></pre></div> <h3>Docker registries</h3> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">Shell</span></div> <pre>hive registry login ghcr.io   <span class="tok-comment"># store credentials (encrypted)</span>
hive registry ls              <span class="tok-comment"># list configured registries</span>
hive registry rm ghcr.io      <span class="tok-comment"># remove credentials</span></pre></div> <div class="callout callout-info"><div class="callout-title">Global flags</div> All commands accept <code>--addr host:port</code> (default: 127.0.0.1:7947) and <code>--ca-cert path</code> for TLS connections. Set <code>HIVE_ADDR</code> and <code>HIVE_CA_CERT</code> env vars to avoid repeating them.</div></section> <section id="recipes" class="learn-section"><h2>Custom Recipes</h2> <p>Recipes are TOML templates with metadata for the App Store. They include a <code>[recipe]</code> header with config field definitions, plus standard service blocks.</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — recipe format</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[recipe]</span>
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
<span class="tok-key">API_KEY</span> = <span class="tok-str">"<span class="tok-placeholder"></span>"</span></pre></div> <p>Custom recipes can be added via the CLI (<code>hive app add recipe.toml</code>) or the App Store's custom app feature.</p></section> <section id="playground" class="learn-section"><h2>Playground</h2> <p>Write a Hivefile and validate it live against the Hive daemon. Edit the TOML below and hit <strong>Validate</strong>.</p> <div class="playground"><div class="playground-editor"><div class="playground-toolbar"><span class="playground-label">Editor</span> <div class="btn-group"><button class="btn btn-sm"> </button> <button class="btn btn-sm btn-primary">Deploy</button></div></div> <textarea spellcheck="false" placeholder="# Write your Hivefile here..."></textarea></div> <div class="playground-output"><div class="playground-toolbar" style="margin:-1rem -1rem 1rem; padding:0.5rem 1rem; border-bottom:1px solid var(--border)"><span class="playground-label">Validation Result</span></div> <!></div></div></section> <section id="reference" class="learn-section"><h2>Quick Reference</h2> <p>Complete field reference for Hivefile service blocks.</p> <div class="card" style="overflow-x:auto"><table><thead><tr><th>Block</th><th>Field</th><th>Type</th><th>Default</th><th>Description</th></tr></thead><tbody><tr><td rowspan="6"><code>[service.X]</code></td><td>image</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Docker image (required)</td></tr><tr><td>replicas</td><td>int</td><td>1</td><td style="font-family:var(--sans)">Number of containers</td></tr><tr><td>platform</td><td>string</td><td>—</td><td style="font-family:var(--sans)">e.g. linux/amd64</td></tr><tr><td>node</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Pin to specific node</td></tr><tr><td>restart_policy</td><td>string</td><td>on-failure</td><td style="font-family:var(--sans)">Docker restart policy</td></tr><tr><td>isolation</td><td>string</td><td>—</td><td style="font-family:var(--sans)">"strict" for network isolation</td></tr><tr><td rowspan="5"><code>[service.X.health]</code></td><td>type</td><td>string</td><td>—</td><td style="font-family:var(--sans)">http, tcp, or exec</td></tr><tr><td>port</td><td>int</td><td>—</td><td style="font-family:var(--sans)">Port to check</td></tr><tr><td>path</td><td>string</td><td>/</td><td style="font-family:var(--sans)">HTTP path (http type only)</td></tr><tr><td>interval</td><td>string</td><td>30s</td><td style="font-family:var(--sans)">Check frequency</td></tr><tr><td>retries</td><td>int</td><td>3</td><td style="font-family:var(--sans)">Failures before unhealthy</td></tr><tr><td><code>[service.X.resources]</code></td><td>memory</td><td>string</td><td>—</td><td style="font-family:var(--sans)">e.g. "256M", "1G"</td></tr><tr><td></td><td>cpus</td><td>float</td><td>—</td><td style="font-family:var(--sans)">e.g. 0.5, 2.0</td></tr><tr><td><code>[service.X.ports]</code></td><td>"host"</td><td>string</td><td>—</td><td style="font-family:var(--sans)">"host_port" = "container_port"</td></tr><tr><td rowspan="3"><code>[service.X.deploy]</code></td><td>strategy</td><td>string</td><td>rolling</td><td style="font-family:var(--sans)">rolling, canary, blue-green</td></tr><tr><td>max_surge</td><td>int</td><td>1</td><td style="font-family:var(--sans)">Extra replicas during rolling</td></tr><tr><td>canary_weight</td><td>int</td><td>10</td><td style="font-family:var(--sans)">% traffic to canary</td></tr><tr><td rowspan="3"><code>[[service.X.volumes]]</code></td><td>name</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Named volume identifier</td></tr><tr><td>target</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Container mount path</td></tr><tr><td>read_only</td><td>bool</td><td>false</td><td style="font-family:var(--sans)">Mount read-only</td></tr><tr><td rowspan="2"><code>[service.X.ingress]</code></td><td>port</td><td>int</td><td>—</td><td style="font-family:var(--sans)">External port</td></tr><tr><td>tls</td><td>bool</td><td>false</td><td style="font-family:var(--sans)">Enable HTTPS</td></tr></tbody></table></div></section></div></div>`,1);function ra(es,ss){Os(ss,!0);let ke=C("what-is-hive"),T=C(`# Try editing this Hivefile!
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
`),S=C(!1),v=C(null),l=C("");const as=[{id:"what-is-hive",label:"What is Hive?"},{id:"how-it-works",label:"How It Works"},{id:"intro",label:"Getting Started"},{id:"clustering",label:"Clustering"},{id:"hivefile",label:"Hivefile Basics"},{id:"services",label:"Services"},{id:"env",label:"Environment & Secrets"},{id:"health",label:"Health Checks"},{id:"volumes",label:"Volumes"},{id:"resources",label:"Resources & Scaling"},{id:"deploy-strategies",label:"Deploy Strategies"},{id:"ingress",label:"Ingress & TLS"},{id:"cron",label:"Cron Jobs"},{id:"appstore",label:"App Store"},{id:"cli",label:"CLI Commands"},{id:"recipes",label:"Custom Recipes"},{id:"playground",label:"Playground"},{id:"reference",label:"Quick Reference"}];async function ts(){if(!Bs()){u(v,{valid:!1,errors:["Sign in to validate against the live daemon. The TOML syntax looks correct based on structure."]},!0);return}u(S,!0);try{u(v,await Ns.validate(t(T),!1),!0)}catch(o){u(v,{valid:!1,errors:[o.message]},!0)}u(S,!1)}function ns(){sessionStorage.setItem("hive_draft_toml",t(T)),Us("/deploy")}function d(o,r){navigator.clipboard.writeText(r),u(l,o,!0),setTimeout(()=>{u(l,"")},2e3)}function os(o){u(ke,o,!0);const r=document.getElementById(o);r&&r.scrollIntoView({behavior:"smooth",block:"start"})}var ue=Qs(),ge=a(pe(ue),2),H=s(ge),is=a(s(H),2);me(is,17,()=>as,he,(o,r)=>{var m=Gs();let b;var _=s(m,!0);e(m),x(()=>{Rs(m,"href",`#${t(r).id??""}`),b=Ws(m,1,"",null,b,{active:t(ke)===t(r).id}),i(_,t(r).label)}),c("click",m,y=>{y.preventDefault(),os(t(r).id)}),k(o,m)}),e(H);var ye=a(H,2),A=a(s(ye),4),L=a(s(A),8),be=s(L),I=a(s(be),2),rs=s(I,!0);e(I),e(be),n(2),e(L);var fe=a(L,6),_e=s(fe),E=a(s(_e),2),cs=s(E,!0);e(E),e(_e),n(2),e(fe),n(4),e(A);var M=a(A,2),we=a(s(M),6),xe=s(we),P=a(s(xe),2),ls=s(P,!0);e(P),e(xe),n(2),e(we),n(14),e(M);var O=a(M,2),Ce=a(s(O),4),Te=s(Ce),z=a(s(Te),2),ds=s(z,!0);e(z),e(Te),n(2),e(Ce),n(4),e(O);var D=a(O,2),Se=a(s(D),4),R=s(Se),W=a(s(R),2),ps=s(W,!0);e(W),e(R);var He=a(R,2),Ae=a(s(He),38),vs=a(s(Ae));vs.textContent="{{ secret:db-pass }}",n(),e(Ae),e(He),e(Se),n(2),e(D);var V=a(D,2),B=a(s(V),2),ms=a(s(B),3);ms.textContent="{{ secret:KEY }}",n(),e(B);var Le=a(B,2),N=s(Le),U=a(s(N),2),hs=s(U,!0);e(U),e(N);var Ie=a(N,2),G=a(s(Ie),8),ks=a(s(G));ks.textContent="{{ secret:db-url }}",n(),e(G);var Ee=a(G,4),us=a(s(Ee));us.textContent="{{ secret:api-key }}",n(),e(Ee),e(Ie),e(Le),n(4),e(V);var X=a(V,2),Me=a(s(X),6),Pe=s(Me),Y=a(s(Pe),2),gs=s(Y,!0);e(Y),e(Pe),n(2),e(Me),e(X);var q=a(X,2),Oe=a(s(q),4),ze=s(Oe),K=a(s(ze),2),ys=s(K,!0);e(K),e(ze),n(2),e(Oe),n(2),e(q);var j=a(q,2),De=a(s(j),4),Re=s(De),F=a(s(Re),2),bs=s(F,!0);e(F),e(Re),n(2),e(De),n(2),e(j);var J=a(j,2),We=a(s(J),6),Ve=s(We),Q=a(s(Ve),2),fs=s(Q,!0);e(Q),e(Ve),n(2),e(We),e(J);var Z=a(J,2),Be=a(s(Z),4),Ne=s(Be),$=a(s(Ne),2),_s=s($,!0);e($),e(Ne),n(2),e(Be),n(2),e(Z);var ee=a(Z,2),Ue=a(s(ee),4),Ge=s(Ue),se=a(s(Ge),2),ws=s(se,!0);e(se),e(Ge),n(2),e(Ue),n(2),e(ee);var ae=a(ee,2),Xe=a(s(ae),6),Ye=s(Xe),te=a(s(Ye),2),xs=s(te,!0);e(te),e(Ye),n(2),e(Xe),n(8),e(ae);var ne=a(ae,4),qe=a(s(ne),4),oe=s(qe),ie=a(s(oe),2),Cs=s(ie,!0);e(ie),e(oe);var Ke=a(oe,2),je=a(s(Ke),70),Ts=a(s(je));Ts.textContent="{{ config:api_key }}",n(),e(je),e(Ke),e(qe),n(2),e(ne);var Fe=a(ne,2),Je=a(s(Fe),4),re=s(Je),ce=s(re),Qe=a(s(ce),2),f=s(Qe),Ss=s(f,!0);e(f);var Hs=a(f,2);e(Qe),e(ce);var Ze=a(ce,2);Ds(Ze),e(re);var $e=a(re,2),As=a(s($e),2);{var Ls=o=>{var r=Xs();k(o,r)},Is=o=>{var r=js(),m=a(pe(r),2);{var b=h=>{var p=Ys(),le=s(p);e(p),x(w=>i(le,`Services found: ${w??""}`),[()=>t(v).services.map(w=>w.name).join(", ")]),k(h,p)};ve(m,h=>{var p;(p=t(v).services)!=null&&p.length&&h(b)})}var _=a(m,2);{var y=h=>{var p=Ks();me(p,21,()=>t(v).warnings,he,(le,w)=>{var de=qs(),Ms=s(de);e(de),x(()=>i(Ms,`⚠ ${t(w)??""}`)),k(le,de)}),e(p),k(h,p)};ve(_,h=>{var p;(p=t(v).warnings)!=null&&p.length&&h(y)})}k(o,r)},Es=o=>{var r=Js(),m=a(pe(r),2);me(m,17,()=>t(v).errors||[t(v).error||"Unknown error"],he,(b,_)=>{var y=Fs(),h=s(y,!0);e(y),x(()=>i(h,t(_))),k(b,y)}),k(o,r)};ve(As,o=>{t(v)===null?o(Ls):t(v).valid?o(Is,1):o(Es,-1)})}e($e),e(Je),e(Fe),n(2),e(ye),e(ge),x(()=>{i(rs,t(l)==="install"?"Copied!":"Copy"),i(cs,t(l)==="start"?"Copied!":"Copy"),i(ls,t(l)==="cluster"?"Copied!":"Copy"),i(ds,t(l)==="basic"?"Copied!":"Copy"),i(ps,t(l)==="multi"?"Copied!":"Copy"),i(hs,t(l)==="env"?"Copied!":"Copy"),i(gs,t(l)==="health"?"Copied!":"Copy"),i(ys,t(l)==="vol"?"Copied!":"Copy"),i(bs,t(l)==="res"?"Copied!":"Copy"),i(fs,t(l)==="deploy"?"Copied!":"Copy"),i(_s,t(l)==="ingress"?"Copied!":"Copy"),i(ws,t(l)==="cron"?"Copied!":"Copy"),i(xs,t(l)==="appstore-cmds"?"Copied!":"Copy"),i(Cs,t(l)==="recipe"?"Copied!":"Copy"),f.disabled=t(S),i(Ss,t(S)?"Validating...":"Validate")}),c("click",I,()=>d("install","curl -fsSL https://raw.githubusercontent.com/Al-Sarraf-Tech/hive/main/install.sh | bash")),c("click",E,()=>d("start","hive setup")),c("click",P,()=>d("cluster",`# On the first node:
hive init --name my-cluster
# Output: Join Code: HIVE-AB12-CD34

# On other nodes:
hive setup --join HIVE-AB12-CD34`)),c("click",z,()=>d("basic",`[service.web]
image = "nginx:alpine"
replicas = 1`)),c("click",W,()=>d("multi",`[service.api]
image = "myapp/api:v1.2"
replicas = 3

[service.api.ports]
"3000" = "3000"

[service.api.depends_on]
services = ["db"]

[service.db]
image = "postgres:16-alpine"
replicas = 1

[service.db.env]
POSTGRES_PASSWORD = "{{ secret:db-pass }}"`)),c("click",U,()=>d("env",`[service.app.env]
APP_ENV = "production"
DATABASE_URL = "{{ secret:db-url }}"
API_KEY = "{{ secret:api-key }}"`)),c("click",Y,()=>d("health",`# HTTP health check
[service.web.health]
type = "http"
path = "/healthz"
port = 8080
interval = "15s"
timeout = "3s"
retries = 3

# TCP health check
[service.db.health]
type = "tcp"
port = 5432

# Exec health check
[service.worker.health]
type = "exec"
exec_command = ["./check.sh"]`)),c("click",K,()=>d("vol",`[[service.db.volumes]]
name = "pg-data"
target = "/var/lib/postgresql/data"

[[service.db.volumes]]
name = "pg-config"
target = "/etc/postgresql"
read_only = true`)),c("click",F,()=>d("res",`[service.api]
image = "myapp/api:latest"
replicas = 2

[service.api.resources]
memory = "512M"
cpus = 1.0

[service.api.autoscale]
min = 2
max = 10
cpu_target = 70.0
cooldown_up = "60s"
cooldown_down = "300s"`)),c("click",Q,()=>d("deploy",`[service.web.deploy]
strategy = "canary"
canary_weight = 10`)),c("click",$,()=>d("ingress",`[service.web.ports]
"8080" = "80"

[service.web.ingress]
port = 8080
tls = true`)),c("click",se,()=>d("cron",`[[service.app.cron]]
schedule = "0 2 * * *"
command = ["./cleanup.sh", "--older-than", "7d"]

[[service.app.cron]]
schedule = "*/5 * * * *"
command = ["./healthcheck.sh"]`)),c("click",te,()=>d("appstore-cmds",`hive app ls                                    # browse all apps
hive app ls --category database                 # filter by category
hive app search grafana                          # search by name
hive app info postgres                           # see config fields
hive app install postgres --config db_password=secret  # install`)),c("click",ie,()=>d("recipe",`[recipe]
id = "my-app"
name = "My App"
description = "A custom application"
icon = "🚀"
category = "devtools"
tags = ["custom", "example"]
image = "myapp:latest"
min_memory = "128M"

  [recipe.config.api_key]
  label = "API Key"
  type = "secret"
  required = true
  description = "Your API key"

[service.my-app]
image = "myapp:latest"
replicas = 1

[service.my-app.env]
API_KEY = "{{ config:api_key }}"`)),c("click",f,ts),c("click",Hs,ns),Vs(Ze,()=>t(T),o=>u(T,o)),k(es,ue),zs()}Ps(["click"]);export{ra as component};

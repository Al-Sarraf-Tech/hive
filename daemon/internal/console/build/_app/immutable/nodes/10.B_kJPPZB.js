import{d as ue,a as l,b as y,f as u,s as c}from"../chunks/Blf7IsOJ.js";import{p as he,s as a,f as ts,t as w,g as t,a as ge,c as e,b as h,d as T,r as s,n,i as be}from"../chunks/Cl5EXWOd.js";import{i as ns}from"../chunks/VuCxtS5p.js";import{e as os,i as rs}from"../chunks/DspTH_g6.js";import{s as fe}from"../chunks/Bhv17BdX.js";import{s as _e}from"../chunks/BoaqluJ8.js";import{b as xe}from"../chunks/DL1sB78A.js";import{a as Ns}from"../chunks/DFm7loE9.js";import{g as we}from"../chunks/7Vtn33E-.js";var Te=u("<a> </a>"),Ce=u('<p class="muted" style="font-size:0.8125rem">Click "Validate" to check your TOML.</p>'),Se=u('<div style="font-size:0.8125rem; color:var(--text-muted)"> </div>'),Ae=u('<div style="font-size:0.8125rem; color:var(--yellow); margin-bottom:0.25rem"> </div>'),Me=u('<div style="margin-top:0.75rem"></div>'),Ee=u('<div style="display:flex; align-items:center; gap:0.5rem; margin-bottom:0.75rem"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--green)" stroke-width="2.5"><polyline points="20 6 9 17 4 12"></polyline></svg> <span class="text-green" style="font-weight:600">Valid Hivefile</span></div> <!> <!>',1),He=u('<div style="font-size:0.8125rem; color:var(--red); margin-bottom:0.25rem; font-family:var(--mono)"> </div>'),Le=u('<div style="display:flex; align-items:center; gap:0.5rem; margin-bottom:0.75rem"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--red)" stroke-width="2.5"><line x1="18" y1="6" x2="6" y2="18"></line><line x1="6" y1="6" x2="18" y2="18"></line></svg> <span class="text-red" style="font-weight:600">Validation Failed</span></div> <!>',1),Pe=u(`<div class="page-header animate-in"><h1 class="page-title">Learn Hive</h1> <span class="muted" style="font-size:0.8125rem">Interactive guide to Hivefiles and TOML configuration</span></div> <div class="learn-layout"><nav class="learn-toc"><div class="learn-toc-title">Contents</div> <!></nav> <div><section id="intro" class="learn-section"><h2>Getting Started</h2> <p>Hive manages containers using <strong>Hivefiles</strong> — simple TOML configuration files that describe
        your services, their resources, health checks, and deployment strategies. If you can read a config file,
        you can use Hive.</p> <div class="callout callout-tip"><div class="callout-title">Why TOML?</div> TOML is designed to be <strong>easy to read and write</strong>. Unlike YAML, it has no significant whitespace issues.
        Unlike JSON, it supports comments and is human-friendly. Every Hivefile is valid TOML.</div> <h3>Three ways to deploy</h3> <ol><li><strong>CLI:</strong> <code>hive deploy my-app.toml</code> — direct from terminal</li> <li><strong>Web Console:</strong> Use the <a href="/deploy">Deploy</a> page to paste or edit TOML</li> <li><strong>App Store:</strong> One-click install from the <a href="/appstore">App Store</a> catalog</li></ol></section> <section id="hivefile" class="learn-section"><h2>Hivefile Basics</h2> <p>A Hivefile defines one or more services. Each service is a TOML table under <code>[service.NAME]</code>.</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — minimal Hivefile</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[service.web]</span>
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
<span class="tok-key">API_KEY</span> = <span class="tok-str">"<span class="tok-placeholder"> </span>"</span></pre></div> <p>Secrets are stored encrypted (age/X25519) and injected at deploy time. Manage them via:</p> <ul><li>CLI: <code>hive secret set db-url "postgres://..."</code></li> <li>Web: <a href="/secrets">Secrets</a> page</li></ul></section> <section id="health" class="learn-section"><h2>Health Checks</h2> <p>Health checks let Hive know when your service is ready and detect failures. Three types are supported:</p> <div class="grid-3" style="margin:1rem 0"><div class="card" style="text-align:center; padding:1rem"><div style="font-size:1.5rem; margin-bottom:0.5rem">🌐</div> <div style="font-weight:600; font-size:0.875rem">HTTP</div> <div class="muted" style="font-size:0.75rem">Checks for 2xx status code</div></div> <div class="card" style="text-align:center; padding:1rem"><div style="font-size:1.5rem; margin-bottom:0.5rem">🔌</div> <div style="font-weight:600; font-size:0.875rem">TCP</div> <div class="muted" style="font-size:0.75rem">Checks port connectivity</div></div> <div class="card" style="text-align:center; padding:1rem"><div style="font-size:1.5rem; margin-bottom:0.5rem">⚙</div> <div style="font-weight:600; font-size:0.875rem">Exec</div> <div class="muted" style="font-size:0.75rem">Runs command, checks exit 0</div></div></div> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — health check examples</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-comment"># HTTP health check</span>
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
        Use <code>*/N</code> for intervals, <code>*</code> for wildcard, <code>1,3,5</code> for lists.</div></section> <section id="recipes" class="learn-section"><h2>App Store Recipes</h2> <p>Recipes are TOML templates with metadata for the App Store. They include a <code>[recipe]</code> header with config field definitions, plus standard service blocks.</p> <div class="code-block"><div class="code-block-header"><span class="code-block-lang">TOML — recipe format</span> <button class="code-block-copy"> </button></div> <pre><span class="tok-section">[recipe]</span>
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
<span class="tok-key">API_KEY</span> = <span class="tok-str">"<span class="tok-placeholder"></span>"</span></pre></div> <p>Custom recipes can be added via the CLI (<code>hive app add recipe.toml</code>) or the App Store's custom app feature.</p></section> <section id="playground" class="learn-section"><h2>Playground</h2> <p>Write a Hivefile and validate it live against the Hive daemon. Edit the TOML below and hit <strong>Validate</strong>.</p> <div class="playground"><div class="playground-editor"><div class="playground-toolbar"><span class="playground-label">Editor</span> <div class="btn-group"><button class="btn btn-sm"> </button> <button class="btn btn-sm btn-primary">Deploy</button></div></div> <textarea spellcheck="false" placeholder="# Write your Hivefile here..."></textarea></div> <div class="playground-output"><div class="playground-toolbar" style="margin:-1rem -1rem 1rem; padding:0.5rem 1rem; border-bottom:1px solid var(--border)"><span class="playground-label">Validation Result</span></div> <!></div></div></section> <section id="reference" class="learn-section"><h2>Quick Reference</h2> <p>Complete field reference for Hivefile service blocks.</p> <div class="card" style="overflow-x:auto"><table><thead><tr><th>Block</th><th>Field</th><th>Type</th><th>Default</th><th>Description</th></tr></thead><tbody><tr><td rowspan="6"><code>[service.X]</code></td><td>image</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Docker image (required)</td></tr><tr><td>replicas</td><td>int</td><td>1</td><td style="font-family:var(--sans)">Number of containers</td></tr><tr><td>platform</td><td>string</td><td>—</td><td style="font-family:var(--sans)">e.g. linux/amd64</td></tr><tr><td>node</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Pin to specific node</td></tr><tr><td>restart_policy</td><td>string</td><td>on-failure</td><td style="font-family:var(--sans)">Docker restart policy</td></tr><tr><td>isolation</td><td>string</td><td>—</td><td style="font-family:var(--sans)">"strict" for network isolation</td></tr><tr><td rowspan="5"><code>[service.X.health]</code></td><td>type</td><td>string</td><td>—</td><td style="font-family:var(--sans)">http, tcp, or exec</td></tr><tr><td>port</td><td>int</td><td>—</td><td style="font-family:var(--sans)">Port to check</td></tr><tr><td>path</td><td>string</td><td>/</td><td style="font-family:var(--sans)">HTTP path (http type only)</td></tr><tr><td>interval</td><td>string</td><td>30s</td><td style="font-family:var(--sans)">Check frequency</td></tr><tr><td>retries</td><td>int</td><td>3</td><td style="font-family:var(--sans)">Failures before unhealthy</td></tr><tr><td><code>[service.X.resources]</code></td><td>memory</td><td>string</td><td>—</td><td style="font-family:var(--sans)">e.g. "256M", "1G"</td></tr><tr><td></td><td>cpus</td><td>float</td><td>—</td><td style="font-family:var(--sans)">e.g. 0.5, 2.0</td></tr><tr><td><code>[service.X.ports]</code></td><td>"host"</td><td>string</td><td>—</td><td style="font-family:var(--sans)">"host_port" = "container_port"</td></tr><tr><td rowspan="3"><code>[service.X.deploy]</code></td><td>strategy</td><td>string</td><td>rolling</td><td style="font-family:var(--sans)">rolling, canary, blue-green</td></tr><tr><td>max_surge</td><td>int</td><td>1</td><td style="font-family:var(--sans)">Extra replicas during rolling</td></tr><tr><td>canary_weight</td><td>int</td><td>10</td><td style="font-family:var(--sans)">% traffic to canary</td></tr><tr><td rowspan="3"><code>[[service.X.volumes]]</code></td><td>name</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Named volume identifier</td></tr><tr><td>target</td><td>string</td><td>—</td><td style="font-family:var(--sans)">Container mount path</td></tr><tr><td>read_only</td><td>bool</td><td>false</td><td style="font-family:var(--sans)">Mount read-only</td></tr><tr><td rowspan="2"><code>[service.X.ingress]</code></td><td>port</td><td>int</td><td>—</td><td style="font-family:var(--sans)">External port</td></tr><tr><td>tls</td><td>bool</td><td>false</td><td style="font-family:var(--sans)">Enable HTTPS</td></tr></tbody></table></div></section></div></div>`,1);function Ue(Ys,Us){he(Us,!0);let cs=T("intro"),C=T(`# Try editing this Hivefile!
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
`),S=T(!1),v=T(null),p=T("");const qs=[{id:"intro",label:"Getting Started"},{id:"hivefile",label:"Hivefile Basics"},{id:"services",label:"Services"},{id:"env",label:"Environment & Secrets"},{id:"health",label:"Health Checks"},{id:"volumes",label:"Volumes"},{id:"resources",label:"Resources & Scaling"},{id:"deploy-strategies",label:"Deploy Strategies"},{id:"ingress",label:"Ingress & TLS"},{id:"cron",label:"Cron Jobs"},{id:"recipes",label:"App Store Recipes"},{id:"playground",label:"Playground"},{id:"reference",label:"Quick Reference"}];async function Xs(){h(S,!0);try{h(v,await Ns.validate(t(C),!1),!0)}catch(o){h(v,{valid:!1,errors:[o.message]},!0)}h(S,!1)}function Fs(){sessionStorage.setItem("hive_draft_toml",t(C)),we("/deploy")}function k(o,r){navigator.clipboard.writeText(r),h(p,o,!0),setTimeout(()=>{h(p,"")},2e3)}function Ks(o){h(cs,o,!0);const r=document.getElementById(o);r&&r.scrollIntoView({behavior:"smooth",block:"start"})}var is=Pe(),ls=a(ts(is),2),A=e(ls),Ws=a(e(A),2);os(Ws,17,()=>qs,rs,(o,r)=>{var d=Te();let b;var _=e(d,!0);s(d),w(()=>{fe(d,"href",`#${t(r).id??""}`),b=_e(d,1,"",null,b,{active:t(cs)===t(r).id}),c(_,t(r).label)}),l("click",d,g=>{g.preventDefault(),Ks(t(r).id)}),y(o,d)}),s(A);var ps=a(A,2),M=a(e(ps),2),ds=a(e(M),4),vs=e(ds),E=a(e(vs),2),Gs=e(E,!0);s(E),s(vs),n(2),s(ds),n(4),s(M);var H=a(M,2),ks=a(e(H),4),L=e(ks),P=a(e(L),2),js=e(P,!0);s(P),s(L);var ms=a(L,2),ys=a(e(ms),38),Js=a(e(ys));Js.textContent={secret:db-pass},n(),s(ys),s(ms),s(ks),n(2),s(H);var O=a(H,2),I=a(e(O),2),Qs=a(e(I),3);Qs.textContent="{{ secret:KEY }}",n(),s(I);var us=a(I,2),R=e(us),z=a(e(R),2),Zs=e(z,!0);s(z),s(R);var hs=a(R,2),D=a(e(hs),8),$s=a(e(D));$s.textContent={secret:db-url},n(),s(D);var gs=a(D,4),bs=a(e(gs)),se=e(bs,!0);s(bs),n(),s(gs),s(hs),s(us),n(4),s(O);var V=a(O,2),fs=a(e(V),6),_s=e(fs),B=a(e(_s),2),ee=e(B,!0);s(B),s(_s),n(2),s(fs),s(V);var N=a(V,2),xs=a(e(N),4),ws=e(xs),Y=a(e(ws),2),ae=e(Y,!0);s(Y),s(ws),n(2),s(xs),n(2),s(N);var U=a(N,2),Ts=a(e(U),4),Cs=e(Ts),q=a(e(Cs),2),te=e(q,!0);s(q),s(Cs),n(2),s(Ts),n(2),s(U);var X=a(U,2),Ss=a(e(X),6),As=e(Ss),F=a(e(As),2),ne=e(F,!0);s(F),s(As),n(2),s(Ss),s(X);var K=a(X,2),Ms=a(e(K),4),Es=e(Ms),W=a(e(Es),2),oe=e(W,!0);s(W),s(Es),n(2),s(Ms),n(2),s(K);var G=a(K,2),Hs=a(e(G),4),Ls=e(Hs),j=a(e(Ls),2),re=e(j,!0);s(j),s(Ls),n(2),s(Hs),n(2),s(G);var J=a(G,2),Ps=a(e(J),4),Q=e(Ps),Z=a(e(Q),2),ce=e(Z,!0);s(Z),s(Q);var Os=a(Q,2),Is=a(e(Os),70),ie=a(e(Is));ie.textContent={config:api_key},n(),s(Is),s(Os),s(Ps),n(2),s(J);var Rs=a(J,2),zs=a(e(Rs),4),$=e(zs),ss=e($),Ds=a(e(ss),2),f=e(Ds),le=e(f,!0);s(f);var pe=a(f,2);s(Ds),s(ss);var Vs=a(ss,2);be(Vs),s($);var Bs=a($,2),de=a(e(Bs),2);{var ve=o=>{var r=Ce();y(o,r)},ke=o=>{var r=Ee(),d=a(ts(r),2);{var b=m=>{var i=Se(),es=e(i);s(i),w(x=>c(es,`Services found: ${x??""}`),[()=>t(v).services.map(x=>x.name).join(", ")]),y(m,i)};ns(d,m=>{var i;(i=t(v).services)!=null&&i.length&&m(b)})}var _=a(d,2);{var g=m=>{var i=Me();os(i,21,()=>t(v).warnings,rs,(es,x)=>{var as=Ae(),ye=e(as);s(as),w(()=>c(ye,`⚠ ${t(x)??""}`)),y(es,as)}),s(i),y(m,i)};ns(_,m=>{var i;(i=t(v).warnings)!=null&&i.length&&m(g)})}y(o,r)},me=o=>{var r=Le(),d=a(ts(r),2);os(d,17,()=>t(v).errors||[t(v).error||"Unknown error"],rs,(b,_)=>{var g=He(),m=e(g,!0);s(g),w(()=>c(m,t(_))),y(b,g)}),y(o,r)};ns(de,o=>{t(v)===null?o(ve):t(v).valid?o(ke,1):o(me,-1)})}s(Bs),s(zs),s(Rs),n(2),s(ps),s(ls),w(()=>{c(Gs,t(p)==="basic"?"Copied!":"Copy"),c(js,t(p)==="multi"?"Copied!":"Copy"),c(Zs,t(p)==="env"?"Copied!":"Copy"),c(se,{secret:Ns-key}),c(ee,t(p)==="health"?"Copied!":"Copy"),c(ae,t(p)==="vol"?"Copied!":"Copy"),c(te,t(p)==="res"?"Copied!":"Copy"),c(ne,t(p)==="deploy"?"Copied!":"Copy"),c(oe,t(p)==="ingress"?"Copied!":"Copy"),c(re,t(p)==="cron"?"Copied!":"Copy"),c(ce,t(p)==="recipe"?"Copied!":"Copy"),f.disabled=t(S),c(le,t(S)?"Validating...":"Validate")}),l("click",E,()=>k("basic",`[service.web]
image = "nginx:alpine"
replicas = 1`)),l("click",P,()=>k("multi",`[service.api]
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
POSTGRES_PASSWORD = "{{ secret:db-pass }}"`)),l("click",z,()=>k("env",`[service.app.env]
APP_ENV = "production"
DATABASE_URL = "{{ secret:db-url }}"
API_KEY = "{{ secret:api-key }}"`)),l("click",B,()=>k("health",`# HTTP health check
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
exec_command = ["./check.sh"]`)),l("click",Y,()=>k("vol",`[[service.db.volumes]]
name = "pg-data"
target = "/var/lib/postgresql/data"

[[service.db.volumes]]
name = "pg-config"
target = "/etc/postgresql"
read_only = true`)),l("click",q,()=>k("res",`[service.api]
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
cooldown_down = "300s"`)),l("click",F,()=>k("deploy",`[service.web.deploy]
strategy = "canary"
canary_weight = 10`)),l("click",W,()=>k("ingress",`[service.web.ports]
"8080" = "80"

[service.web.ingress]
port = 8080
tls = true`)),l("click",j,()=>k("cron",`[[service.app.cron]]
schedule = "0 2 * * *"
command = ["./cleanup.sh", "--older-than", "7d"]

[[service.app.cron]]
schedule = "*/5 * * * *"
command = ["./healthcheck.sh"]`)),l("click",Z,()=>k("recipe",`[recipe]
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
API_KEY = "{{ config:api_key }}"`)),l("click",f,Xs),l("click",pe,Fs),xe(Vs,()=>t(C),o=>h(C,o)),y(Ys,is),ge()}ue(["click"]);export{Ue as component};

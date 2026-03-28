<script>
  import { api } from '$lib/api.js';

  const templates = {
    blank: '',
    nginx: `[service.nginx]
image = "nginx:alpine"
replicas = 2
restart_policy = "on-failure"

[service.nginx.ports]
"8080" = "80"

[service.nginx.health]
type = "http"
port = 80
path = "/"
interval = "30s"
timeout = "5s"
retries = 3

[service.nginx.resources]
memory = "128M"
cpus = 0.5
`,
    postgres: `[service.postgres]
image = "postgres:16-alpine"
replicas = 1
restart_policy = "always"

[service.postgres.ports]
"5432" = "5432"

[service.postgres.env]
POSTGRES_PASSWORD = "{{ secret:pg-password }}"

[service.postgres.health]
type = "tcp"
port = 5432
interval = "30s"
timeout = "5s"
retries = 5

[service.postgres.resources]
memory = "512M"
cpus = 1.0
`,
    redis: `[service.redis]
image = "redis:7-alpine"
replicas = 1
restart_policy = "always"

[service.redis.ports]
"6379" = "6379"

[service.redis.health]
type = "tcp"
port = 6379
interval = "30s"
timeout = "5s"
retries = 3

[service.redis.resources]
memory = "256M"
cpus = 0.5
`,
  };

  let toml = $state(templates.nginx);
  let result = $state(null);
  let error = $state(null);
  let deploying = $state(false);
  let selectedTemplate = $state('nginx');
  let validationResult = $state(null);
  let validating = $state(false);

  function selectTemplate(name) {
    selectedTemplate = name;
    toml = templates[name];
    result = null;
    error = null;
  }

  async function validate() {
    if (!toml.trim()) { error = 'Hivefile cannot be empty'; return; }
    validating = true;
    validationResult = null;
    error = null;
    try {
      validationResult = await api.validate(toml, true);
    } catch (e) {
      error = e.message;
    } finally {
      validating = false;
    }
  }

  async function deploy() {
    if (!toml.trim()) { error = 'Hivefile cannot be empty'; return; }
    deploying = true;
    error = null;
    result = null;
    validationResult = null;
    try {
      result = await api.deploy(toml);
    } catch (e) {
      error = e.message;
    } finally {
      deploying = false;
    }
  }

  function handleFile(e) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => { toml = reader.result; selectedTemplate = 'blank'; };
    reader.readAsText(file);
  }
</script>

<div class="page-header">
  <h1 class="page-title">Deploy</h1>
</div>

<div style="display:flex; gap:0.5rem; margin-bottom:1rem; flex-wrap:wrap">
  <span class="muted" style="align-self:center; font-size:0.8125rem">Template:</span>
  {#each Object.keys(templates).filter(t => t !== 'blank') as name}
    <button
      class="btn btn-sm"
      class:btn-primary={selectedTemplate === name}
      onclick={() => selectTemplate(name)}
    >{name}</button>
  {/each}
  <label class="btn btn-sm" style="cursor:pointer">
    Upload .toml
    <input type="file" accept=".toml,.txt" onchange={handleFile} style="display:none" />
  </label>
</div>

<div class="card" style="margin-bottom:1rem">
  <div class="card-title">Hivefile (TOML)</div>
  <textarea
    bind:value={toml}
    rows="20"
    style="margin-top:0.5rem; resize:vertical"
    spellcheck="false"
  ></textarea>
  <div style="margin-top:1rem; display:flex; gap:0.5rem; align-items:center">
    <button class="btn btn-primary" onclick={deploy} disabled={deploying || validating}>
      {deploying ? 'Deploying...' : 'Deploy'}
    </button>
    <button class="btn btn-sm" onclick={validate} disabled={validating || deploying}>
      {validating ? 'Validating...' : 'Validate'}
    </button>
    {#if deploying}
      <span class="muted">Pulling images and starting containers...</span>
    {/if}
    {#if validating}
      <span class="muted">Validating hivefile...</span>
    {/if}
  </div>
</div>

{#if validationResult}
  {#if validationResult.valid && (!validationResult.issues || validationResult.issues.length === 0)}
    <div class="card" style="border-color: var(--green); margin-bottom:1rem">
      <span class="badge badge-green">Valid</span>
      <span class="muted" style="margin-left:0.5rem">Hivefile passed all checks</span>
    </div>
  {:else if validationResult.issues?.length}
    <div class="card" style="border-color: var(--yellow); margin-bottom:1rem">
      <div class="card-title">Validation Issues</div>
      <table style="margin-top:0.5rem">
        <thead><tr><th>Severity</th><th>Service</th><th>Field</th><th>Message</th></tr></thead>
        <tbody>
          {#each validationResult.issues as issue}
            <tr>
              <td>
                {#if issue.severity === 'VALIDATION_SEVERITY_ERROR'}
                  <span class="badge badge-red">error</span>
                {:else if issue.severity === 'VALIDATION_SEVERITY_WARNING'}
                  <span class="badge badge-yellow">warning</span>
                {:else}
                  <span class="badge">info</span>
                {/if}
              </td>
              <td>{issue.service || '-'}</td>
              <td class="muted">{issue.field || '-'}</td>
              <td>{issue.message}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}
{/if}

{#if error}
  <div class="card" style="border-color: var(--red)">
    <div class="card-title text-red">Deploy Failed</div>
    <pre class="exec-output" style="color:var(--red)">{error}</pre>
  </div>
{/if}

{#if result?.services?.length}
  <div class="card" style="border-color: var(--green)">
    <div class="card-title text-green">Deployed Successfully</div>
    <table style="margin-top:0.5rem">
      <thead>
        <tr><th>Service</th><th>Image</th><th>Replicas</th><th>ID</th></tr>
      </thead>
      <tbody>
        {#each result.services as svc}
          <tr>
            <td><a href="/services/{svc.name}">{svc.name}</a></td>
            <td class="muted">{svc.image}</td>
            <td>{svc.replicasRunning ?? 0}/{svc.replicasDesired ?? 0}</td>
            <td class="muted">{svc.id?.substring(0, 12) || '-'}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

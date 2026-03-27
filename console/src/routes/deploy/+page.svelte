<script>
  import { api } from '$lib/api.js';

  let toml = $state(`[service.web]
image = "nginx:latest"
replicas = 2
restart_policy = "on-failure"

[service.web.ports]
"8080" = "80"

[service.web.health]
type = "http"
port = 8080
path = "/"
`);
  let result = $state(null);
  let error = $state(null);
  let deploying = $state(false);

  async function deploy() {
    deploying = true;
    error = null;
    result = null;
    try {
      result = await api.deploy(toml);
    } catch (e) {
      error = e.message;
    } finally {
      deploying = false;
    }
  }
</script>

<div class="page-header">
  <h1 class="page-title">Deploy</h1>
</div>

<div class="card" style="margin-bottom:1rem">
  <div class="card-title">Hivefile (TOML)</div>
  <textarea
    bind:value={toml}
    rows="18"
    style="margin-top:0.5rem; resize:vertical"
    spellcheck="false"
  ></textarea>
  <div style="margin-top:1rem; display:flex; gap:0.5rem; align-items:center">
    <button class="btn btn-primary" onclick={deploy} disabled={deploying}>
      {deploying ? 'Deploying...' : 'Deploy'}
    </button>
    {#if deploying}
      <span class="muted">Pulling images and starting containers...</span>
    {/if}
  </div>
</div>

{#if error}
  <div class="card" style="border-color: var(--red)">
    <div class="card-title text-red">Deploy Failed</div>
    <pre style="white-space:pre-wrap; font-family:var(--mono); font-size:0.8125rem; margin-top:0.5rem; color:var(--red)">{error}</pre>
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
            <td>{svc.name}</td>
            <td class="muted">{svc.image}</td>
            <td>{svc.replicasRunning}/{svc.replicasDesired}</td>
            <td class="muted">{svc.id?.substring(0, 12) || '-'}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';

  let services = $state([]);
  let error = $state(null);

  async function refresh() {
    try {
      const data = await api.listServices();
      services = data.services || [];
      error = null;
    } catch (e) { error = e.message; }
  }

  async function stop(name) {
    if (!confirm(`Stop service "${name}"? This will remove all containers.`)) return;
    try { await api.stopService(name); await refresh(); } catch (e) { alert(e.message); }
  }

  async function rollback(name) {
    if (!confirm(`Rollback "${name}" to previous version?`)) return;
    try { await api.rollbackService(name); await refresh(); } catch (e) { alert(e.message); }
  }

  async function scale(name) {
    const count = prompt(`Scale "${name}" to how many replicas?`);
    if (!count) return;
    try { await api.scaleService(name, parseInt(count)); await refresh(); } catch (e) { alert(e.message); }
  }

  function statusBadge(s) {
    switch (s) {
      case 'SERVICE_STATUS_RUNNING': return { text: 'running', cls: 'badge-green' };
      case 'SERVICE_STATUS_DEGRADED': return { text: 'degraded', cls: 'badge-yellow' };
      case 'SERVICE_STATUS_STOPPED': return { text: 'stopped', cls: 'badge-red' };
      default: return { text: s?.replace('SERVICE_STATUS_', '').toLowerCase() || 'unknown', cls: '' };
    }
  }

  onMount(() => { refresh(); const i = setInterval(refresh, 5000); return () => clearInterval(i); });
</script>

<div class="page-header">
  <h1 class="page-title">Services</h1>
  <div class="flex gap-sm">
    <a href="/deploy" class="btn btn-primary btn-sm">Deploy</a>
    <button class="btn btn-sm" onclick={refresh}>Refresh</button>
  </div>
</div>

{#if error}
  <p class="text-red">{error}</p>
{:else if services.length === 0}
  <div class="card">
    <p class="muted">No services running.</p>
    <p class="mt-1"><a href="/deploy">Deploy your first service</a></p>
  </div>
{:else}
  <div class="card">
    <table>
      <thead>
        <tr>
          <th>Status</th>
          <th>Name</th>
          <th>Image</th>
          <th>Replicas</th>
          <th>Node</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each services as svc}
          {@const badge = statusBadge(svc.status)}
          <tr>
            <td><span class="badge {badge.cls}">{badge.text}</span></td>
            <td>{svc.name}</td>
            <td class="muted">{svc.image}</td>
            <td>{svc.replicasRunning}<span class="muted">/{svc.replicasDesired}</span></td>
            <td class="muted">{svc.nodeConstraint || 'any'}</td>
            <td>
              <div class="flex gap-sm">
                <button class="btn btn-sm" onclick={() => scale(svc.name)}>Scale</button>
                <button class="btn btn-sm" onclick={() => rollback(svc.name)}>Rollback</button>
                <button class="btn btn-sm btn-danger" onclick={() => stop(svc.name)}>Stop</button>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

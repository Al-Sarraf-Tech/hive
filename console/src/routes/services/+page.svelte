<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { serviceBadge, timeAgo } from '$lib/utils.js';

  let services = $state([]);
  let error = $state(null);
  let loading = $state(true);

  async function refresh() {
    try {
      const data = await api.listServices();
      services = data.services || [];
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  async function stop(name) {
    if (!confirm(`Stop service "${name}"? This will remove all containers.`)) return;
    try { await api.stopService(name); await refresh(); } catch (e) { alert(e.message); }
  }

  async function restart(name) {
    if (!confirm(`Restart "${name}"?`)) return;
    try { await api.restartService(name); await refresh(); } catch (e) { alert(e.message); }
  }

  async function rollback(name) {
    if (!confirm(`Rollback "${name}" to previous version?`)) return;
    try { await api.rollbackService(name); await refresh(); } catch (e) { alert(e.message); }
  }

  async function scale(name) {
    const count = prompt(`Scale "${name}" to how many replicas?`);
    if (!count) return;
    const n = parseInt(count, 10);
    if (isNaN(n) || n < 1) { alert('Must be a positive integer'); return; }
    try { await api.scaleService(name, n); await refresh(); } catch (e) { alert(e.message); }
  }

  onMount(() => { refresh(); const i = setInterval(refresh, 5000); return () => clearInterval(i); });
</script>

<div class="page-header">
  <h1 class="page-title">Services</h1>
  <div class="btn-group">
    <a href="/deploy" class="btn btn-primary btn-sm">Deploy</a>
    <button class="btn btn-sm" onclick={refresh}>Refresh</button>
  </div>
</div>

{#if error}
  <p class="text-red">{error}</p>
{:else if loading}
  <p class="muted">Loading...</p>
{:else if services.length === 0}
  <div class="card empty-state">
    <div class="empty-state-icon">&gt;</div>
    <p>No services deployed</p>
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
          <th>Strategy</th>
          <th>Health</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each services as svc}
          {@const badge = serviceBadge(svc.status)}
          <tr>
            <td><span class="badge {badge.cls}">{badge.text}</span></td>
            <td><a href="/services/{svc.name}">{svc.name}</a></td>
            <td class="muted">{svc.image}</td>
            <td>{svc.replicasRunning ?? 0}<span class="muted">/{svc.replicasDesired ?? 0}</span></td>
            <td class="muted">{svc.deployStrategy?.replace('DEPLOY_STRATEGY_', '').toLowerCase() || 'rolling'}</td>
            <td>
              {#if svc.healthCheck?.type}
                <span class="badge badge-cyan">{svc.healthCheck.type.replace('HEALTH_CHECK_TYPE_', '').toLowerCase()}</span>
              {:else}
                <span class="muted">-</span>
              {/if}
            </td>
            <td>
              <div class="btn-group">
                <button class="btn btn-sm" onclick={() => scale(svc.name)}>Scale</button>
                <button class="btn btn-sm" onclick={() => restart(svc.name)}>Restart</button>
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

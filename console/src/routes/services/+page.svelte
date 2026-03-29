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

  // Quick-update modal
  let editSvc = $state(null);
  let editImage = $state('');
  let editReplicas = $state(1);
  let editEnv = $state('');
  let updating = $state(false);
  let updateError = $state('');

  function openEdit(svc) {
    editSvc = svc;
    editImage = svc.image || '';
    editReplicas = svc.replicasDesired || 1;
    editEnv = Object.entries(svc.env || {}).map(([k, v]) => `${k}=${v}`).join('\n');
    updateError = '';
  }

  async function submitUpdate() {
    updating = true;
    updateError = '';
    try {
      const updates = {};
      if (editImage !== editSvc.image) updates.image = editImage;
      if (editReplicas !== editSvc.replicasDesired) updates.replicas = editReplicas;
      const envLines = editEnv.trim().split('\n').filter(l => l.includes('='));
      if (envLines.length > 0) {
        const env = {};
        for (const line of envLines) {
          const idx = line.indexOf('=');
          env[line.substring(0, idx)] = line.substring(idx + 1);
        }
        updates.env = env;
      }
      await api.updateService(editSvc.name, updates);
      editSvc = null;
      await refresh();
    } catch (e) { updateError = e.message; }
    finally { updating = false; }
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
                <button class="btn btn-sm btn-primary" onclick={() => openEdit(svc)}>Edit</button>
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

{#if editSvc}
  <div class="modal-overlay" onclick={() => editSvc = null} role="presentation">
    <div class="modal animate-in" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="modal-title">Update Service: {editSvc.name}</div>
      {#if updateError}
        <div class="callout callout-warn" style="margin-bottom:1rem; padding:0.5rem 0.75rem">
          <p style="margin:0; font-size:0.8125rem; color:var(--red)">{updateError}</p>
        </div>
      {/if}
      <div class="form-group">
        <label>Image</label>
        <input type="text" bind:value={editImage} placeholder="nginx:alpine" />
      </div>
      <div class="form-group">
        <label>Replicas</label>
        <input type="number" bind:value={editReplicas} min="1" max="100" style="max-width:100px" />
      </div>
      <div class="form-group">
        <label>Environment Variables (KEY=VALUE, one per line)</label>
        <textarea bind:value={editEnv} rows="5" placeholder="KEY=value" style="font-family:var(--mono); font-size:0.8125rem"></textarea>
      </div>
      <div class="modal-actions">
        <button class="btn" onclick={() => editSvc = null}>Cancel</button>
        <button class="btn btn-primary" onclick={submitUpdate} disabled={updating}>
          {updating ? 'Updating...' : 'Apply Changes'}
        </button>
      </div>
    </div>
  </div>
{/if}

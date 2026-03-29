<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { timeAgo } from '$lib/utils.js';

  let secrets = $state([]);
  let error = $state(null);
  let loading = $state(true);
  let newKey = $state('');
  let newValue = $state('');
  let adding = $state(false);

  async function refresh() {
    try {
      const data = await api.listSecrets();
      secrets = data.secrets || [];
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  async function addSecret() {
    if (!newKey.trim()) return;
    if (!newValue) { alert('Secret value cannot be empty'); return; }
    adding = true;
    try {
      await api.setSecret(newKey.trim(), newValue);
      newKey = '';
      newValue = '';
      await refresh();
    } catch (e) { alert(e.message); }
    finally { adding = false; }
  }

  let rotateKey = $state(null);
  let rotateValue = $state('');
  let rotating = $state(false);
  let rotateResult = $state(null);

  async function remove(key) {
    if (!confirm(`Delete secret "${key}"? Services using this secret will fail.`)) return;
    try { await api.deleteSecret(key); await refresh(); } catch (e) { alert(e.message); }
  }

  async function rotate() {
    if (!rotateValue) return;
    rotating = true;
    rotateResult = null;
    try {
      const resp = await api.rotateSecret(rotateKey, rotateValue);
      rotateResult = resp;
      rotateValue = '';
      await refresh();
    } catch (e) { alert(e.message); }
    finally { rotating = false; }
  }

  onMount(() => {
    refresh();
    const i = setInterval(refresh, 10000);
    return () => clearInterval(i);
  });
</script>

<div class="page-header">
  <h1 class="page-title">Secrets</h1>
  <button class="btn btn-sm" onclick={refresh}>Refresh</button>
</div>

<div class="card" style="margin-bottom:1.5rem">
  <div class="card-title">Add Secret</div>
  <div style="display:grid; grid-template-columns:1fr 2fr auto; gap:0.5rem; margin-top:0.5rem; align-items:end">
    <div>
      <label for="secret-key">Key</label>
      <input id="secret-key" bind:value={newKey} placeholder="SECRET_NAME" />
    </div>
    <div>
      <label for="secret-value">Value</label>
      <input
        id="secret-value"
        bind:value={newValue}
        type="password"
        placeholder="secret value"
        onkeydown={(e) => { if (e.key === 'Enter') addSecret(); }}
      />
    </div>
    <button class="btn btn-primary" onclick={addSecret} disabled={adding}>
      {adding ? 'Setting...' : 'Set'}
    </button>
  </div>
  <p class="muted" style="font-size:0.75rem; margin-top:0.5rem">
    Reference in Hivefiles: <code style="color:var(--accent)">{'{{ secret:KEY }}'}</code>
  </p>
</div>

{#if error}
  <p class="text-red mb-1">{error}</p>
{/if}

{#if loading}
  <p class="muted">Loading...</p>
{:else if secrets.length === 0}
  <div class="card empty-state">
    <div class="empty-state-icon">*</div>
    <p>No secrets stored</p>
  </div>
{:else}
  <div class="card">
    <table>
      <thead>
        <tr>
          <th>Key</th>
          <th>Created</th>
          <th>Updated</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each secrets as s}
          <tr>
            <td>{s.key}</td>
            <td class="muted">{s.createdAtUnix ? timeAgo({ seconds: s.createdAtUnix }) : '-'}</td>
            <td class="muted">{s.updatedAtUnix ? timeAgo({ seconds: s.updatedAtUnix }) : '-'}</td>
            <td>
              <div class="btn-group">
                <button class="btn btn-sm" onclick={() => { rotateKey = s.key; rotateResult = null; }}>Rotate</button>
                <button class="btn btn-sm btn-danger" onclick={() => remove(s.key)}>Delete</button>
              </div>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

{#if rotateKey}
  <div class="modal-overlay" onclick={() => rotateKey = null} role="presentation">
    <div class="modal animate-in" onclick={(e) => e.stopPropagation()} role="dialog">
      <div class="modal-title">Rotate Secret: {rotateKey}</div>
      <p class="muted" style="margin-bottom:1rem; font-size:0.8125rem">
        Set a new value and automatically rolling-restart all services that reference this secret.
      </p>
      <div class="form-group">
        <label>New Value</label>
        <input type="password" bind:value={rotateValue} placeholder="New secret value"
          onkeydown={(e) => { if (e.key === 'Enter') rotate(); }} />
      </div>
      {#if rotateResult}
        <div class="callout callout-tip" style="margin-bottom:1rem">
          <div class="callout-title">Rotation Complete</div>
          {#if rotateResult.restartedServices?.length}
            <p style="margin:0; font-size:0.8125rem">Restarted services: {rotateResult.restartedServices.join(', ')}</p>
          {:else}
            <p style="margin:0; font-size:0.8125rem">No services reference this secret.</p>
          {/if}
        </div>
      {/if}
      <div class="modal-actions">
        <button class="btn" onclick={() => rotateKey = null}>Close</button>
        <button class="btn btn-primary" onclick={rotate} disabled={rotating || !rotateValue}>
          {rotating ? 'Rotating...' : 'Rotate & Restart'}
        </button>
      </div>
    </div>
  </div>
{/if}

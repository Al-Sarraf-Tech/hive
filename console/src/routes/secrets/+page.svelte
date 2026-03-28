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

  async function remove(key) {
    if (!confirm(`Delete secret "${key}"? Services using this secret will fail.`)) return;
    try { await api.deleteSecret(key); await refresh(); } catch (e) { alert(e.message); }
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
              <button class="btn btn-sm btn-danger" onclick={() => remove(s.key)}>Delete</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

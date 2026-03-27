<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';

  let secrets = $state([]);
  let error = $state(null);
  let newKey = $state('');
  let newValue = $state('');

  async function refresh() {
    try {
      const data = await api.listSecrets();
      secrets = data.secrets || [];
      error = null;
    } catch (e) { error = e.message; }
  }

  async function addSecret() {
    if (!newKey.trim()) return;
    try {
      await api.setSecret(newKey.trim(), newValue);
      newKey = '';
      newValue = '';
      await refresh();
    } catch (e) { alert(e.message); }
  }

  async function remove(key) {
    if (!confirm(`Delete secret "${key}"?`)) return;
    try { await api.deleteSecret(key); await refresh(); } catch (e) { alert(e.message); }
  }

  onMount(refresh);
</script>

<div class="page-header">
  <h1 class="page-title">Secrets</h1>
</div>

<div class="card" style="margin-bottom:1rem">
  <div class="card-title">Add Secret</div>
  <div style="display:grid; grid-template-columns:1fr 2fr auto; gap:0.5rem; margin-top:0.5rem; align-items:end">
    <div>
      <label class="muted" style="font-size:0.75rem">Key</label>
      <input bind:value={newKey} placeholder="SECRET_NAME" />
    </div>
    <div>
      <label class="muted" style="font-size:0.75rem">Value</label>
      <input bind:value={newValue} type="password" placeholder="secret value" />
    </div>
    <button class="btn btn-primary" onclick={addSecret}>Set</button>
  </div>
</div>

{#if error}
  <p class="text-red">{error}</p>
{:else if secrets.length === 0}
  <div class="card"><p class="muted">No secrets stored.</p></div>
{:else}
  <div class="card">
    <table>
      <thead><tr><th>Key</th><th>Actions</th></tr></thead>
      <tbody>
        {#each secrets as s}
          <tr>
            <td>{s.key}</td>
            <td><button class="btn btn-sm btn-danger" onclick={() => remove(s.key)}>Delete</button></td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

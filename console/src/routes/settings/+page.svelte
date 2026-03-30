<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';

  let registries = $state([]);
  let loading = $state(true);
  let error = $state(null);

  let newUrl = $state('');
  let newUsername = $state('');
  let newPassword = $state('');
  let addError = $state(null);
  let adding = $state(false);

  async function refresh() {
    try {
      const data = await api.listRegistries();
      registries = data.registries || [];
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  onMount(() => { refresh(); });

  async function addRegistry() {
    if (!newUrl || !newUsername || !newPassword) {
      addError = 'All fields are required';
      return;
    }
    adding = true;
    addError = null;
    try {
      await api.registryLogin(newUrl, newUsername, newPassword);
      newUrl = '';
      newUsername = '';
      newPassword = '';
      await refresh();
    } catch (e) {
      addError = e.message;
    } finally {
      adding = false;
    }
  }

  async function removeReg(url) {
    try {
      await api.removeRegistry(url);
      await refresh();
    } catch (e) { alert(e.message); }
  }
</script>

<div class="page-header">
  <h1 class="page-title">Settings</h1>
</div>

<div class="card" style="padding:1.5rem; margin-bottom:1.5rem">
  <div class="card-title" style="margin-bottom:0.5rem">Docker Registry Credentials</div>
  <p class="muted" style="font-size:0.8rem; margin-bottom:0.25rem">
    These are your <strong>Docker Hub / GHCR / private registry</strong> credentials for pulling container images.
  </p>
  <p style="font-size:0.7rem; color:var(--text-muted); margin-bottom:1rem">
    This is separate from your Hive login. Add credentials here so Hive can pull images from private registries when you deploy apps.
  </p>

  {#if loading}
    <p class="muted">Loading...</p>
  {:else if error}
    <p class="text-red">{error}</p>
  {:else}
    {#if registries.length > 0}
      <table style="margin-bottom:1.5rem">
        <thead>
          <tr><th>URL</th><th>Username</th><th></th></tr>
        </thead>
        <tbody>
          {#each registries as reg}
            <tr>
              <td class="mono">{reg.url}</td>
              <td>{reg.username}</td>
              <td>
                <button class="btn btn-sm" style="color:var(--red)" onclick={() => removeReg(reg.url)}>Remove</button>
              </td>
            </tr>
          {/each}
        </tbody>
      </table>
    {:else}
      <p class="muted" style="margin-bottom:1rem">No registries configured.</p>
    {/if}
  {/if}

  <h4 style="margin-bottom:0.5rem">Add Registry</h4>
  <div style="display:flex; gap:0.375rem; margin-bottom:0.75rem; flex-wrap:wrap">
    <button class="btn btn-sm" style="font-size:0.65rem" onclick={() => newUrl = 'docker.io'}>Docker Hub</button>
    <button class="btn btn-sm" style="font-size:0.65rem" onclick={() => newUrl = 'ghcr.io'}>GitHub (GHCR)</button>
    <button class="btn btn-sm" style="font-size:0.65rem" onclick={() => newUrl = 'registry.gitlab.com'}>GitLab</button>
    <button class="btn btn-sm" style="font-size:0.65rem" onclick={() => newUrl = 'lscr.io'}>LinuxServer.io</button>
  </div>
  <div style="display:flex; gap:0.5rem; flex-wrap:wrap; align-items:flex-end">
    <div>
      <label class="muted" style="font-size:0.7rem; display:block">Registry URL</label>
      <input type="text" bind:value={newUrl} placeholder="docker.io, ghcr.io, etc." style="padding:0.4rem 0.75rem; background:var(--bg); border:1px solid var(--border); border-radius:8px; color:var(--fg); width:200px" />
    </div>
    <div>
      <label class="muted" style="font-size:0.7rem; display:block">Username</label>
      <input type="text" bind:value={newUsername} placeholder="username" style="padding:0.4rem 0.75rem; background:var(--bg); border:1px solid var(--border); border-radius:8px; color:var(--fg); width:140px" />
    </div>
    <div>
      <label class="muted" style="font-size:0.7rem; display:block">Password</label>
      <input type="password" bind:value={newPassword} placeholder="token or password" style="padding:0.4rem 0.75rem; background:var(--bg); border:1px solid var(--border); border-radius:8px; color:var(--fg); width:180px" />
    </div>
    <button class="btn btn-primary btn-sm" onclick={addRegistry} disabled={adding}>
      {adding ? 'Adding...' : 'Add'}
    </button>
  </div>
  {#if addError}
    <p class="text-red" style="margin-top:0.5rem; font-size:0.8rem">{addError}</p>
  {/if}
</div>

<div class="card" style="padding:1.5rem">
  <div class="card-title" style="margin-bottom:0.75rem">About</div>
  <div style="display:grid; grid-template-columns:1fr 1fr; gap:0.5rem; font-size:0.85rem">
    <span class="muted">Version</span><span>Hive v2.6.0</span>
    <span class="muted">Console</span><span>SvelteKit 5</span>
    <span class="muted">Daemon</span><span>Go</span>
    <span class="muted">CLI</span><span>Rust</span>
  </div>
</div>

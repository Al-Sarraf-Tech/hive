<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, isAuthenticated } from '$lib/api.js';

  let containers = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let adopting = $state(null);
  let adoptName = $state('');
  let adoptStop = $state(true);
  let adoptError = $state(null);

  async function refresh() {
    try {
      const data = await api.discoverContainers();
      containers = data.containers || [];
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  onMount(() => { refresh(); });

  function startAdopt(c) {
    adopting = c;
    adoptName = c.name.replace(/[^a-zA-Z0-9_-]/g, '-').replace(/^-+|-+$/g, '');
    adoptStop = true;
    adoptError = null;
  }

  async function confirmAdopt() {
    if (!adoptName.trim()) { adoptError = 'Service name is required'; return; }
    adoptError = null;
    try {
      await api.adoptContainer(adopting.id, adoptName.trim(), adoptStop);
      adopting = null;
      goto(`/services`);
    } catch (e) { adoptError = e.message; }
  }

  function shortId(id) { return id?.substring(0, 12) || ''; }
</script>

<div class="page-header">
  <h1 class="page-title">Discover Containers</h1>
  <button class="btn btn-sm" onclick={refresh}>Refresh</button>
</div>

<p class="muted" style="margin-bottom:1rem">
  Docker containers running on this node that are <strong>not managed by Hive</strong>.
  Adopt them to bring under Hive management with health checks, scaling, and ingress.
</p>

{#if error === 'unauthorized'}
  <div class="card" style="padding:2rem; text-align:center">
    <div style="font-size:2rem; margin-bottom:0.75rem">🔒</div>
    <h3 style="margin:0 0 0.5rem">Sign in to discover containers</h3>
    <p class="muted" style="margin-bottom:1rem">Container discovery requires authentication.</p>
    <a href="/login" class="btn btn-primary" style="text-decoration:none">Sign In</a>
  </div>
{:else if error}
  <div class="card" style="border-color:var(--red)"><p class="text-red">{error}</p></div>
{:else if loading}
  <p class="muted">Scanning Docker...</p>
{:else if containers.length === 0}
  <div class="card" style="padding:2rem; text-align:center">
    <div style="font-size:2rem; margin-bottom:0.75rem">✅</div>
    <h3 style="margin:0 0 0.5rem">All containers are managed</h3>
    <p class="muted">No unmanaged Docker containers found on this node.</p>
  </div>
{:else}
  <div class="card">
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Image</th>
          <th>Status</th>
          <th>Ports</th>
          <th></th>
        </tr>
      </thead>
      <tbody>
        {#each containers as c}
          <tr>
            <td>
              <div style="font-weight:600">{c.name}</div>
              <div class="muted mono" style="font-size:0.7rem">{shortId(c.id)}</div>
            </td>
            <td class="mono" style="font-size:0.8rem">{c.image}</td>
            <td>
              <span class="badge" class:badge-green={c.status === 'running'} class:badge-yellow={c.status !== 'running'}>
                {c.status}
              </span>
            </td>
            <td class="mono muted" style="font-size:0.75rem">
              {#each Object.entries(c.ports || {}) as [host, container]}
                {host}→{container}{' '}
              {/each}
            </td>
            <td>
              <button class="btn btn-primary btn-sm" onclick={() => startAdopt(c)}>Adopt</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

{#if adopting}
  <div style="position:fixed; inset:0; background:rgba(0,0,0,0.6); display:flex; align-items:center; justify-content:center; z-index:100"
       onclick={() => adopting = null} role="dialog">
    <div class="card" style="padding:1.5rem; max-width:450px; width:90%" onclick={(e) => e.stopPropagation()} role="document">
      <h3 style="margin:0 0 0.5rem">Adopt Container</h3>
      <p class="muted" style="margin-bottom:1rem">
        Import <strong>{adopting.name}</strong> (<code class="mono">{adopting.image}</code>) into Hive.
        Hive will generate a service definition from its running config.
      </p>

      <div style="margin-bottom:1rem">
        <label style="display:block; font-size:0.8rem; margin-bottom:0.25rem; color:var(--text-muted)">Service Name</label>
        <input type="text" bind:value={adoptName} placeholder="my-service"
          style="width:100%; padding:0.5rem 0.75rem; background:var(--bg); border:1px solid var(--border); border-radius:8px; color:var(--fg)"
          onkeydown={(e) => { if (e.key === 'Enter') confirmAdopt(); }}
        />
      </div>

      <label style="display:flex; align-items:center; gap:0.5rem; margin-bottom:1rem; font-size:0.85rem">
        <input type="checkbox" bind:checked={adoptStop} />
        Stop original container after adopting
      </label>

      {#if adoptError}
        <p class="text-red" style="margin-bottom:0.75rem; font-size:0.85rem">{adoptError}</p>
      {/if}

      <div style="display:flex; gap:0.5rem">
        <button class="btn btn-primary" onclick={confirmAdopt}>Adopt into Hive</button>
        <button class="btn btn-sm" onclick={() => adopting = null}>Cancel</button>
      </div>
    </div>
  </div>
{/if}

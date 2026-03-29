<script>
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';

  let app = $state(null);
  let loading = $state(true);
  let error = $state(null);
  let installing = $state(false);
  let installed = $state(false);
  let installError = $state(null);
  let serviceName = $state('');
  let configValues = $state({});

  const id = $derived($page.params.id);

  async function loadApp() {
    try {
      app = await api.getApp(id);
      serviceName = id;
      // Set defaults
      for (const f of (app.configFields || [])) {
        configValues[f.key] = f.defaultValue || '';
      }
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  onMount(() => { loadApp(); });

  async function install() {
    // Validate required fields
    for (const f of (app.configFields || [])) {
      if (f.required && !configValues[f.key]) {
        installError = `${f.label} is required`;
        return;
      }
    }

    installing = true;
    installError = null;
    try {
      await api.installApp(id, serviceName, configValues);
      installed = true;
    } catch (e) {
      installError = e.message;
    } finally {
      installing = false;
    }
  }
</script>

<div class="page-header">
  <h1 class="page-title">
    <a href="/appstore" style="color:var(--text-muted); text-decoration:none">App Store</a>
    <span class="muted" style="margin:0 0.5rem">/</span>
    {app?.name || id}
  </h1>
</div>

{#if loading}
  <p class="muted">Loading...</p>
{:else if error}
  <div class="card" style="border-color:var(--red)">
    <p class="text-red">{error}</p>
  </div>
{:else if installed}
  <div class="card" style="border-color:var(--green)">
    <div style="display:flex; align-items:center; gap:0.75rem; margin-bottom:1rem">
      <span style="font-size:2.5rem">{app.icon}</span>
      <div>
        <h2 style="margin:0; color:var(--green)">Installed!</h2>
        <p class="muted" style="margin:0">{app.name} deployed as "{serviceName}"</p>
      </div>
    </div>
    <div style="display:flex; gap:0.75rem">
      <button class="btn btn-primary" onclick={() => goto(`/services/${serviceName}`)}>View Service</button>
      <button class="btn btn-sm" onclick={() => goto('/appstore')}>Back to Store</button>
    </div>
  </div>
{:else if app}
  <div class="card" style="padding:1.5rem">
    <div style="display:flex; align-items:flex-start; gap:1rem; margin-bottom:1.5rem">
      <span style="font-size:3rem">{app.icon}</span>
      <div>
        <h2 style="margin:0">{app.name}</h2>
        <p class="muted" style="margin:0.25rem 0">{app.description}</p>
        <div style="display:flex; gap:0.5rem; margin-top:0.5rem; flex-wrap:wrap">
          <span class="badge">{app.category}</span>
          {#each (app.tags || []) as tag}
            <span style="font-size:0.6rem; padding:0.15rem 0.5rem; border:1px solid var(--border); border-radius:999px; color:var(--text-muted)">{tag}</span>
          {/each}
        </div>
      </div>
    </div>

    <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:1rem; margin-bottom:1.5rem; padding:1rem; background:var(--bg); border-radius:8px">
      <div>
        <div class="muted" style="font-size:0.7rem">Image</div>
        <div class="mono" style="font-size:0.85rem">{app.image}</div>
      </div>
      <div>
        <div class="muted" style="font-size:0.7rem">Version</div>
        <div style="font-size:0.85rem">{app.version || 'latest'}</div>
      </div>
      <div>
        <div class="muted" style="font-size:0.7rem">Min Memory</div>
        <div style="font-size:0.85rem">{app.minMemory || 'N/A'}</div>
      </div>
    </div>

    <h3 style="margin-bottom:1rem">Install</h3>

    <div style="margin-bottom:1rem">
      <label style="display:block; font-size:0.8rem; margin-bottom:0.25rem; color:var(--text-muted)">Service Name</label>
      <input
        type="text"
        bind:value={serviceName}
        placeholder="my-service"
        style="width:100%; max-width:300px; padding:0.5rem 0.75rem; background:var(--bg); border:1px solid var(--border); border-radius:8px; color:var(--fg)"
      />
    </div>

    {#each (app.configFields || []) as field}
      <div style="margin-bottom:1rem">
        <label style="display:block; font-size:0.8rem; margin-bottom:0.25rem; color:var(--text-muted)">
          {field.label}
          {#if field.required}<span style="color:var(--red)"> *</span>{/if}
        </label>
        {#if field.description}
          <p class="muted" style="font-size:0.7rem; margin:0 0 0.25rem">{field.description}</p>
        {/if}
        {#if field.type === 'secret'}
          <input
            type="password"
            bind:value={configValues[field.key]}
            placeholder={field.defaultValue || ''}
            style="width:100%; max-width:300px; padding:0.5rem 0.75rem; background:var(--bg); border:1px solid var(--border); border-radius:8px; color:var(--fg)"
          />
        {:else if field.type === 'bool'}
          <label style="display:flex; align-items:center; gap:0.5rem">
            <input type="checkbox" bind:checked={configValues[field.key]} />
            Enabled
          </label>
        {:else}
          <input
            type="text"
            bind:value={configValues[field.key]}
            placeholder={field.defaultValue || ''}
            style="width:100%; max-width:300px; padding:0.5rem 0.75rem; background:var(--bg); border:1px solid var(--border); border-radius:8px; color:var(--fg)"
          />
        {/if}
      </div>
    {/each}

    {#if installError}
      <p class="text-red" style="margin-bottom:0.75rem">{installError}</p>
    {/if}

    <button class="btn btn-primary" onclick={install} disabled={installing}>
      {installing ? 'Installing...' : 'Install'}
    </button>
  </div>
{/if}

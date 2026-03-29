<script>
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { goto } from '$app/navigation';
  import { api, isAuthenticated } from '$lib/api.js';

  let app = $state(null);
  let loading = $state(true);
  let error = $state(null);
  let installing = $state(false);
  let installed = $state(false);
  let installError = $state(null);
  let serviceName = $state('');
  let configValues = $state({});
  let tab = $state('overview');
  let tomlPreview = $state('');
  let authed = $state(false);

  const id = $derived($page.params.id);

  async function loadApp() {
    try {
      authed = isAuthenticated();
      app = authed ? await api.getApp(id) : await api.publicGetApp(id);
      serviceName = id;
      for (const f of (app.configFields || [])) {
        configValues[f.key] = f.defaultValue || '';
      }
      generateToml();
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  onMount(() => { loadApp(); });

  function generateToml() {
    if (!app) return;
    let lines = [`# Hivefile generated from App Store: ${app.name}`, ''];
    lines.push(`[service.${serviceName || app.id}]`);
    lines.push(`image = "${app.image}"`);
    lines.push('replicas = 1');
    lines.push('');
    if (app.configFields?.length) {
      lines.push(`[service.${serviceName || app.id}.env]`);
      for (const f of app.configFields) {
        const val = configValues[f.key] || f.defaultValue || '';
        if (f.type === 'secret') {
          lines.push(`${f.key.toUpperCase()} = "{{ secret:${f.key} }}"`);
        } else {
          lines.push(`${f.key.toUpperCase()} = "${val}"`);
        }
      }
      lines.push('');
    }
    if (app.minMemory) {
      lines.push(`[service.${serviceName || app.id}.resources]`);
      lines.push(`memory = "${app.minMemory}"`);
      lines.push('');
    }
    tomlPreview = lines.join('\n');
  }

  // Regenerate preview when config changes
  $effect(() => {
    void serviceName;
    void configValues;
    generateToml();
  });

  async function install() {
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

  function copyToml() {
    navigator.clipboard.writeText(tomlPreview);
  }
</script>

<div class="breadcrumb animate-in">
  <a href="/appstore">App Store</a>
  <span class="sep">/</span>
  <span class="current">{app?.name || id}</span>
</div>

{#if loading}
  <div class="card" style="padding:2rem">
    <div class="flex items-center gap-md">
      <div class="skeleton skeleton-avatar"></div>
      <div style="flex:1">
        <div class="skeleton skeleton-title"></div>
        <div class="skeleton skeleton-text"></div>
      </div>
    </div>
  </div>
{:else if error}
  <div class="card" style="border-color:var(--red)">
    <p class="text-red">{error}</p>
    <button class="btn btn-sm mt-1" onclick={() => goto('/appstore')}>Back to Store</button>
  </div>
{:else if installed}
  <div class="card animate-in" style="border-color:var(--green); padding:2rem">
    <div style="display:flex; align-items:center; gap:1rem; margin-bottom:1.5rem">
      <div class="app-card-icon" style="width:56px; height:56px; font-size:2rem; background:rgba(52,211,153,0.1)">
        {app.icon}
      </div>
      <div>
        <h2 style="color:var(--green); margin:0; font-size:1.25rem">Successfully Installed</h2>
        <p class="muted" style="margin:0.25rem 0 0">{app.name} deployed as <code class="mono" style="color:var(--cyan)">{serviceName}</code></p>
      </div>
    </div>
    <div class="btn-group">
      <button class="btn btn-primary" onclick={() => goto(`/services/${serviceName}`)}>
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="9 18 15 12 9 6"/></svg>
        View Service
      </button>
      <button class="btn" onclick={() => goto('/appstore')}>Back to Store</button>
    </div>
  </div>
{:else if app}
  <!-- App Header -->
  <div class="card animate-in" style="padding:1.5rem; margin-bottom:1.5rem">
    <div style="display:flex; align-items:flex-start; gap:1.25rem">
      <div class="app-card-icon" style="width:64px; height:64px; font-size:2.25rem; border-radius:14px">
        {app.icon}
      </div>
      <div style="flex:1">
        <h1 style="font-size:1.5rem; font-weight:700; margin:0">{app.name}</h1>
        <p class="muted" style="margin:0.375rem 0 0.75rem; font-size:0.9375rem; line-height:1.5">{app.description}</p>
        <div style="display:flex; gap:0.5rem; flex-wrap:wrap; align-items:center">
          <span class="badge badge-accent">{app.category}</span>
          {#each (app.tags || []) as tag}
            <span class="app-card-tag">{tag}</span>
          {/each}
        </div>
      </div>
      {#if authed}
        <button class="btn btn-primary" onclick={() => { tab = 'install'; }} style="flex-shrink:0">
          Install App
        </button>
      {:else}
        <a href="/login" class="btn btn-primary" style="flex-shrink:0; text-decoration:none">
          Sign In to Install
        </a>
      {/if}
    </div>
  </div>

  <!-- Tabs -->
  <div class="tabs">
    <button class="tab" class:active={tab === 'overview'} onclick={() => tab = 'overview'}>Overview</button>
    <button class="tab" class:active={tab === 'install'} onclick={() => tab = 'install'}>Install</button>
    <button class="tab" class:active={tab === 'toml'} onclick={() => tab = 'toml'}>TOML Preview</button>
  </div>

  <!-- Overview Tab -->
  {#if tab === 'overview'}
    <div class="grid-3 animate-in">
      <div class="card">
        <div class="card-title">Image</div>
        <div class="mono" style="font-size:0.9375rem; word-break:break-all">{app.image}</div>
      </div>
      <div class="card">
        <div class="card-title">Version</div>
        <div style="font-size:0.9375rem">{app.version || 'latest'}</div>
      </div>
      <div class="card">
        <div class="card-title">Min Memory</div>
        <div style="font-size:0.9375rem">{app.minMemory || 'No minimum'}</div>
      </div>
    </div>

    {#if app.configFields?.length}
      <div class="section mt-2">
        <h3 class="section-title">Configuration Fields</h3>
        <div class="card">
          <table>
            <thead>
              <tr>
                <th>Field</th>
                <th>Type</th>
                <th>Required</th>
                <th>Default</th>
                <th>Description</th>
              </tr>
            </thead>
            <tbody>
              {#each app.configFields as f}
                <tr>
                  <td>{f.label}</td>
                  <td><span class="badge badge-cyan">{f.type}</span></td>
                  <td>{f.required ? 'Yes' : 'No'}</td>
                  <td>{f.defaultValue || '-'}</td>
                  <td style="font-family:var(--sans); color:var(--text-muted)">{f.description || '-'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        </div>
      </div>
    {/if}

    <div class="section mt-2">
      <h3 class="section-title">Quick Start</h3>
      <div class="callout callout-tip">
        <div class="callout-title">CLI Install</div>
        <code class="mono" style="font-size:0.8125rem">hive app install {app.id} --name my-{app.id}</code>
      </div>
    </div>
  {/if}

  <!-- Install Tab -->
  {#if tab === 'install'}
    {#if !authed}
      <div class="card animate-in" style="padding:2rem; max-width:500px; text-align:center">
        <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="var(--text-muted)" stroke-width="1.5" style="margin-bottom:1rem">
          <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0110 0v4"/>
        </svg>
        <h3 style="margin-bottom:0.5rem">Sign in to install</h3>
        <p class="muted" style="font-size:0.875rem; margin-bottom:1.25rem">
          You need to be authenticated to deploy services. Sign in with your Hive bearer token.
        </p>
        <a href="/login" class="btn btn-primary" style="text-decoration:none">Sign In</a>
      </div>
    {:else}
    <div class="card animate-in" style="padding:1.5rem; max-width:600px">
      <h3 style="margin-bottom:1.25rem; font-size:1rem">Configure &amp; Deploy</h3>

      <div class="form-group">
        <label>Service Name <span class="text-red">*</span></label>
        <input
          type="text"
          bind:value={serviceName}
          placeholder="my-service"
          style="max-width:100%"
        />
        <div class="form-hint">Unique name for this service instance</div>
      </div>

      {#each (app.configFields || []) as field}
        <div class="form-group">
          <label>
            {field.label}
            {#if field.required}<span class="text-red"> *</span>{/if}
          </label>
          {#if field.description}
            <div class="form-hint" style="margin-bottom:0.375rem">{field.description}</div>
          {/if}
          {#if field.type === 'secret'}
            <input
              type="password"
              bind:value={configValues[field.key]}
              placeholder={field.defaultValue || 'Enter secret value'}
              style="max-width:100%"
            />
          {:else if field.type === 'bool'}
            <label style="display:flex; align-items:center; gap:0.5rem; font-size:0.875rem; color:var(--text)">
              <input type="checkbox" bind:checked={configValues[field.key]} style="width:auto" />
              Enabled
            </label>
          {:else}
            <input
              type="text"
              bind:value={configValues[field.key]}
              placeholder={field.defaultValue || ''}
              style="max-width:100%"
            />
          {/if}
        </div>
      {/each}

      {#if installError}
        <div class="callout callout-warn" style="margin-bottom:1rem">
          <p class="text-red" style="margin:0">{installError}</p>
        </div>
      {/if}

      <div class="btn-group">
        <button class="btn btn-primary" onclick={install} disabled={installing}>
          {#if installing}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="animation: spin 1s linear infinite"><circle cx="12" cy="12" r="10" stroke-dasharray="32" stroke-dashoffset="12"/></svg>
            Installing...
          {:else}
            Deploy Service
          {/if}
        </button>
        <button class="btn" onclick={() => tab = 'toml'}>Preview TOML</button>
      </div>
    </div>
    {/if}
  {/if}

  <!-- TOML Preview Tab -->
  {#if tab === 'toml'}
    <div class="animate-in">
      <div class="callout callout-info" style="margin-bottom:1rem">
        <div class="callout-title">Generated Hivefile</div>
        This TOML is generated from your configuration above. You can copy it and deploy manually via CLI or the Deploy page.
      </div>
      <div class="code-block">
        <div class="code-block-header">
          <span class="code-block-lang">TOML</span>
          <button class="code-block-copy" onclick={copyToml}>Copy</button>
        </div>
        <pre>{tomlPreview}</pre>
      </div>
      <div class="btn-group mt-1">
        <button class="btn btn-primary" onclick={() => tab = 'install'}>Install with this config</button>
        <button class="btn" onclick={() => goto('/deploy')}>Open in Deploy Editor</button>
      </div>
    </div>
  {/if}
{/if}

<style>
  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>

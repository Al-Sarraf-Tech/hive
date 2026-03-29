<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, isAuthenticated } from '$lib/api.js';

  let apps = $state([]);
  let installed = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let search = $state('');
  let category = $state('');

  const categories = [
    { key: '', label: 'All', icon: 'grid' },
    { key: 'database', label: 'Database', icon: 'db' },
    { key: 'cache', label: 'Cache', icon: 'zap' },
    { key: 'media', label: 'Media', icon: 'media' },
    { key: 'monitoring', label: 'Monitoring', icon: 'chart' },
    { key: 'webserver', label: 'Web Server', icon: 'globe' },
    { key: 'proxy', label: 'Proxy', icon: 'shuffle' },
    { key: 'messaging', label: 'Messaging', icon: 'mail' },
    { key: 'storage', label: 'Storage', icon: 'hdd' },
    { key: 'devtools', label: 'DevTools', icon: 'wrench' },
    { key: 'networking', label: 'Networking', icon: 'network' },
    { key: 'security', label: 'Security', icon: 'shield' },
    { key: 'productivity', label: 'Productivity', icon: 'productivity' },
    { key: 'automation', label: 'Automation', icon: 'play' },
  ];

  const featured = ['postgres', 'jellyfin', 'grafana', 'traefik', 'nextcloud'];

  let installedSet = $derived(new Set(installed.map(a => a.appId || a.app_id)));

  let authed = $state(false);

  async function refresh() {
    try {
      loading = true;
      authed = isAuthenticated();
      // Use public API when not authenticated, authenticated API otherwise
      const listFn = authed ? (search ? api.searchApps : (cat) => api.listApps(cat)) : (search ? api.publicSearchApps : (cat) => api.publicListApps(cat));
      const appData = search ? await (authed ? api.searchApps(search) : api.publicSearchApps(search))
                             : await (authed ? api.listApps(category) : api.publicListApps(category));
      apps = appData.apps || [];
      // Installed apps only available when authenticated
      if (authed) {
        const installedData = await api.listInstalledApps().catch(() => ({ apps: [] }));
        installed = installedData.apps || installedData.instances || [];
      } else {
        installed = [];
      }
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  onMount(() => { refresh(); });

  function selectCategory(cat) {
    category = cat;
    search = '';
    refresh();
  }

  function onSearch() {
    category = '';
    refresh();
  }

  function handleSearchKey(e) {
    if (e.key === 'Enter') onSearch();
  }

  let featuredApps = $derived(apps.filter(a => featured.includes(a.id)));
  let regularApps = $derived(
    category || search ? apps : apps.filter(a => !featured.includes(a.id))
  );

  function categoryIcon(key) {
    const map = {
      '': '⬡', database: '🗃', cache: '⚡', media: '🎬', monitoring: '📊',
      webserver: '🌐', proxy: '🔀', messaging: '✉', storage: '💾',
      devtools: '🔧', networking: '🌍', security: '🛡', productivity: '📋',
      automation: '▶',
    };
    return map[key] || '⬡';
  }
</script>

<div class="appstore-hero animate-in">
  <h1>App Store</h1>
  <p>Deploy production-ready services in one click. Browse curated recipes or bring your own.</p>
  <div class="appstore-search">
    <input
      type="text"
      placeholder="Search apps, tags, categories..."
      bind:value={search}
      onkeydown={handleSearchKey}
    />
    <button class="btn btn-primary" onclick={onSearch}>Search</button>
  </div>
</div>

<div class="category-pills">
  {#each categories as cat}
    <button
      class="category-pill"
      class:active={cat.key === category}
      onclick={() => selectCategory(cat.key)}
    >
      <span>{categoryIcon(cat.key)}</span>
      {cat.label}
    </button>
  {/each}
</div>

{#if loading}
  <div class="app-grid">
    {#each Array(6) as _}
      <div class="skeleton skeleton-card"></div>
    {/each}
  </div>
{:else if error}
  <div class="card" style="border-color:var(--red)">
    <p class="text-red">{error}</p>
    <button class="btn btn-sm mt-1" onclick={refresh}>Retry</button>
  </div>
{:else}
  {#if featuredApps.length > 0 && !category && !search}
    <div class="section">
      <div class="flex items-center justify-between mb-1">
        <h2 style="font-size:1rem; font-weight:600">Popular</h2>
        <span class="muted" style="font-size:0.75rem">{apps.length} apps available</span>
      </div>
      <div class="featured-grid">
        {#each featuredApps as app}
          <div
            class="featured-card"
            onclick={() => goto(`/appstore/${app.id}`)}
            role="button"
            tabindex="0"
            onkeydown={(e) => { if (e.key === 'Enter') goto(`/appstore/${app.id}`); }}
          >
            <div class="featured-label">Popular</div>
            <div class="app-card-header">
              <div class="app-card-icon">{app.icon}</div>
              <div class="app-card-meta">
                <div class="app-card-name">{app.name}</div>
                <div class="app-card-category">{app.category}</div>
              </div>
              {#if installedSet.has(app.id)}
                <span class="app-card-installed">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline points="20 6 9 17 4 12"/></svg>
                  Installed
                </span>
              {/if}
            </div>
            <p class="app-card-desc">{app.description}</p>
          </div>
        {/each}
      </div>
    </div>
  {/if}

  {#if regularApps.length === 0 && featuredApps.length === 0}
    <div class="empty-state">
      <div class="empty-state-icon">⬡</div>
      <p>No apps found{search ? ` for "${search}"` : category ? ` in ${category}` : ''}.</p>
      {#if search || category}
        <button class="btn btn-sm mt-1" onclick={() => { search = ''; category = ''; refresh(); }}>Clear filters</button>
      {/if}
    </div>
  {:else}
    <div class="section">
      {#if !category && !search}
        <h2 style="font-size:1rem; font-weight:600; margin-bottom:1rem">All Apps</h2>
      {:else}
        <h2 style="font-size:1rem; font-weight:600; margin-bottom:1rem">
          {search ? `Results for "${search}"` : categories.find(c => c.key === category)?.label || 'Apps'}
          <span class="muted" style="font-weight:400; font-size:0.875rem"> ({regularApps.length})</span>
        </h2>
      {/if}
      <div class="app-grid">
        {#each regularApps as app}
          <div
            class="app-card"
            onclick={() => goto(`/appstore/${app.id}`)}
            role="button"
            tabindex="0"
            onkeydown={(e) => { if (e.key === 'Enter') goto(`/appstore/${app.id}`); }}
          >
            <div class="app-card-header">
              <div class="app-card-icon">{app.icon}</div>
              <div class="app-card-meta">
                <div class="app-card-name">{app.name}</div>
                <div class="app-card-category">{app.category}</div>
              </div>
              {#if installedSet.has(app.id)}
                <span class="app-card-installed">
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3"><polyline points="20 6 9 17 4 12"/></svg>
                  Installed
                </span>
              {/if}
            </div>
            <p class="app-card-desc">{app.description}</p>
            <div class="app-card-footer">
              <div class="app-card-tags">
                {#each (app.tags || []).slice(0, 3) as tag}
                  <span class="app-card-tag">{tag}</span>
                {/each}
              </div>
              <span class="mono muted" style="font-size:0.7rem">
                {#if (app.tags || []).includes('linuxserver.io')}
                  <span class="badge badge-purple" style="font-size:0.6rem; margin-right:0.25rem">LSIO</span>
                {/if}
                {app.version || 'latest'}
              </span>
            </div>
          </div>
        {/each}
      </div>
    </div>
  {/if}
{/if}

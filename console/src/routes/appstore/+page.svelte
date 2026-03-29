<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';

  let apps = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let search = $state('');
  let category = $state('');

  const categories = ['', 'database', 'cache', 'monitoring', 'webserver', 'proxy', 'messaging', 'storage', 'devtools'];

  async function refresh() {
    try {
      loading = true;
      if (search) {
        const data = await api.searchApps(search);
        apps = data.apps || [];
      } else {
        const data = await api.listApps(category);
        apps = data.apps || [];
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
</script>

<div class="page-header">
  <h1 class="page-title">App Store</h1>
  <div style="display:flex; gap:0.5rem; align-items:center">
    <input
      type="text"
      placeholder="Search apps..."
      bind:value={search}
      onkeydown={(e) => { if (e.key === 'Enter') onSearch(); }}
      style="padding:0.4rem 0.75rem; background:var(--bg-card); border:1px solid var(--border); border-radius:8px; color:var(--fg); width:200px"
    />
    <button class="btn btn-sm" onclick={onSearch}>Search</button>
  </div>
</div>

<div style="display:flex; gap:0.5rem; margin-bottom:1.5rem; flex-wrap:wrap">
  {#each categories as cat}
    <button
      class="btn btn-sm"
      style={cat === category ? 'background:var(--ring); color:#fff' : ''}
      onclick={() => selectCategory(cat)}
    >
      {cat || 'All'}
    </button>
  {/each}
</div>

{#if loading}
  <p class="muted">Loading catalog...</p>
{:else if error}
  <div class="card" style="border-color:var(--red)">
    <p class="text-red">{error}</p>
  </div>
{:else if apps.length === 0}
  <p class="muted">No apps found.</p>
{:else}
  <div style="display:grid; grid-template-columns:repeat(auto-fill, minmax(280px, 1fr)); gap:1rem">
    {#each apps as app}
      <div
        class="card clickable"
        style="cursor:pointer; padding:1.25rem"
        onclick={() => goto(`/appstore/${app.id}`)}
        role="button"
        tabindex="0"
        onkeydown={(e) => { if (e.key === 'Enter') goto(`/appstore/${app.id}`); }}
      >
        <div style="display:flex; align-items:center; gap:0.75rem; margin-bottom:0.75rem">
          <span style="font-size:2rem">{app.icon}</span>
          <div>
            <div style="font-weight:600; font-size:1.1rem">{app.name}</div>
            <span class="badge" style="font-size:0.65rem">{app.category}</span>
          </div>
        </div>
        <p class="muted" style="font-size:0.85rem; margin:0">{app.description}</p>
        <div style="margin-top:0.75rem; display:flex; gap:0.5rem; flex-wrap:wrap">
          {#each (app.tags || []).slice(0, 3) as tag}
            <span style="font-size:0.6rem; padding:0.15rem 0.5rem; border:1px solid var(--border); border-radius:999px; color:var(--text-muted)">{tag}</span>
          {/each}
        </div>
      </div>
    {/each}
  </div>
{/if}

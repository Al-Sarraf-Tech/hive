<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, isAuthenticated } from '$lib/api.js';
  import { nodeBadge, eventIcon, timeAgo, fmtBytes } from '$lib/utils.js';

  let status = $state(null);
  let error = $state(null);
  let loading = $state(true);
  let featuredApps = $state([]);
  let authed = $state(false);

  async function refresh() {
    try {
      status = await api.getStatus();
      error = null;
    } catch (e) {
      error = e.message;
    } finally {
      loading = false;
    }
  }

  async function loadFeaturedApps() {
    try {
      const fn = isAuthenticated() ? api.listApps : api.publicListApps;
      const data = await fn('');
      const popular = ['postgres', 'redis', 'grafana', 'nginx', 'jellyfin', 'traefik'];
      featuredApps = (data.apps || []).filter(a => popular.includes(a.id)).slice(0, 6);
    } catch { /* silent — app store is optional */ }
  }

  onMount(() => {
    authed = isAuthenticated();
    refresh();
    loadFeaturedApps();
    const interval = setInterval(refresh, 3000);
    return () => clearInterval(interval);
  });
</script>

<div class="page-header">
  <h1 class="page-title">
    Cluster Overview
    <span style="display:inline-block; width:8px; height:8px; border-radius:50%; background:var(--green); margin-left:0.5rem; animation:pulse-dot 2s ease infinite"></span>
  </h1>
  <div class="btn-group">
    <a href="/deploy" class="btn btn-primary btn-sm" style="text-decoration:none">Deploy</a>
    <a href="/appstore" class="btn btn-sm" style="text-decoration:none">App Store</a>
    <button class="btn btn-sm" onclick={refresh}>Refresh</button>
  </div>
</div>

{#if loading}
  <p class="muted">Connecting to hived...</p>
{:else if error}
  <div class="card" style="border-color: var(--border); padding:2rem; text-align:center">
    {#if error.includes('unauthorized') || error.includes('401')}
      <div style="font-size:2rem; margin-bottom:0.75rem">🔒</div>
      <h3 style="margin:0 0 0.5rem">Sign in to view cluster data</h3>
      <p class="muted" style="margin-bottom:1rem">The dashboard shows live cluster stats, services, and nodes. Sign in to access.</p>
      <a href="/login" class="btn btn-primary" style="text-decoration:none">Sign In</a>
    {:else}
      <p class="text-red">Failed to connect: {error}</p>
      <p class="muted mt-1">Is hived running with --http-port 7949?</p>
    {/if}
  </div>
{:else if status}
  <div class="stats-grid">
    <div class="card">
      <div class="card-title">Healthy Nodes</div>
      <div class="card-value text-cyan">
        {status.healthyNodes ?? 0}<span class="muted" style="font-size:1rem">/{status.totalNodes ?? 0}</span>
      </div>
    </div>
    <div class="card">
      <div class="card-title">Services</div>
      <div class="card-value text-green">{status.totalServices ?? 0}</div>
    </div>
    <div class="card">
      <div class="card-title">Containers</div>
      <div class="card-value text-yellow">{status.runningContainers ?? 0}</div>
    </div>
  </div>

  {#if status.containersPerNode && Object.keys(status.containersPerNode).length}
    <div class="card">
      <div class="card-title" style="margin-bottom:1rem">Containers per Node</div>
      <table>
        <thead><tr><th>Node</th><th>Containers</th></tr></thead>
        <tbody>
          {#each Object.entries(status.containersPerNode) as [node, count]}
            <tr class="clickable" onclick={() => goto(`/nodes/${node}`)}>
              <td>{node}</td>
              <td>{count}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  {/if}

  <div class="grid-2">
    <div class="card">
      <div class="card-title" style="margin-bottom:1rem">Nodes</div>
      {#if status.nodes?.length}
        <table>
          <thead>
            <tr>
              <th>Status</th>
              <th>Name</th>
              <th>Address</th>
              <th>CPU</th>
              <th>Memory</th>
            </tr>
          </thead>
          <tbody>
            {#each status.nodes as node}
              {@const badge = nodeBadge(node.status)}
              <tr class="clickable" onclick={() => goto(`/nodes/${node.name}`)}>
                <td><span class="badge {badge.cls}">{badge.text}</span></td>
                <td>{node.name}</td>
                <td class="muted">{node.advertiseAddr || '-'}</td>
                <td>{node.resources?.cpuCores || '-'}</td>
                <td>{fmtBytes(node.resources?.memoryAvailableBytes)} <span class="muted">/ {fmtBytes(node.resources?.memoryTotalBytes)}</span></td>
              </tr>
            {/each}
          </tbody>
        </table>
      {:else}
        <p class="muted">No nodes</p>
      {/if}
    </div>

    <div class="card">
      <div class="card-title" style="margin-bottom:1rem">Recent Events</div>
      {#if status.recentEvents?.length}
        {#each status.recentEvents as evt}
          {@const ei = eventIcon(evt.type)}
          <div class="event-item">
            <span class="event-icon {ei.cls}">{ei.icon}</span>
            <span class="event-msg">{evt.message}</span>
            <span class="event-time">{timeAgo(evt.timestamp)}</span>
          </div>
        {/each}
      {:else}
        <p class="muted">No recent events</p>
      {/if}
    </div>
  </div>

  <!-- Quick Deploy from App Store -->
  {#if featuredApps.length > 0}
    <div class="card" style="margin-top:1.5rem">
      <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:1rem">
        <div class="card-title">Quick Deploy</div>
        <div style="display:flex; gap:0.5rem; align-items:center">
          {#if authed}
            <span style="font-size:0.7rem; color:var(--green)">● Signed in — click to deploy</span>
          {:else}
            <span style="font-size:0.7rem; color:var(--text-muted)">○ <a href="/login" style="color:var(--cyan)">Sign in</a> to deploy</span>
          {/if}
          <a href="/appstore" class="btn btn-sm" style="text-decoration:none; font-size:0.7rem">Browse all {featuredApps.length > 0 ? '35' : ''} apps →</a>
        </div>
      </div>
      <div style="display:grid; grid-template-columns:repeat(auto-fill, minmax(160px, 1fr)); gap:0.75rem">
        {#each featuredApps as app}
          <div
            class="card clickable"
            style="padding:0.75rem; cursor:pointer; border:1px solid var(--border)"
            onclick={() => goto(`/appstore/${app.id}`)}
            role="button"
            tabindex="0"
            onkeydown={(e) => { if (e.key === 'Enter') goto(`/appstore/${app.id}`); }}
          >
            <div style="display:flex; align-items:center; gap:0.5rem; margin-bottom:0.375rem">
              <span style="font-size:1.25rem">{app.icon}</span>
              <span style="font-weight:600; font-size:0.85rem">{app.name}</span>
            </div>
            <p class="muted" style="font-size:0.7rem; margin:0; line-height:1.3">
              {app.description.length > 60 ? app.description.slice(0, 57) + '...' : app.description}
            </p>
          </div>
        {/each}
      </div>
    </div>
  {/if}
{/if}

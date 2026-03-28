<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { containerBadge, shortId, timeAgo } from '$lib/utils.js';

  let containers = $state([]);
  let services = $state([]);
  let nodes = $state([]);
  let filterService = $state('');
  let filterNode = $state('');
  let error = $state(null);
  let loading = $state(true);

  async function refresh() {
    try {
      const data = await api.listContainers(filterService, filterNode);
      containers = data.containers || [];
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  async function loadFilters() {
    try {
      const [sData, nData] = await Promise.all([api.listServices(), api.listNodes()]);
      services = sData.services || [];
      nodes = nData.nodes || [];
    } catch (_) {}
  }

  onMount(() => {
    loadFilters();
    refresh();
    const i = setInterval(refresh, 5000);
    return () => clearInterval(i);
  });
</script>

<div class="page-header">
  <h1 class="page-title">Containers</h1>
  <button class="btn btn-sm" onclick={refresh}>Refresh</button>
</div>

<div class="filter-bar">
  <select bind:value={filterService} onchange={refresh}>
    <option value="">All services</option>
    {#each services as svc}
      <option value={svc.name}>{svc.name}</option>
    {/each}
  </select>
  <select bind:value={filterNode} onchange={refresh}>
    <option value="">All nodes</option>
    {#each nodes as n}
      <option value={n.name}>{n.name}</option>
    {/each}
  </select>
</div>

{#if error}
  <p class="text-red">{error}</p>
{:else if loading}
  <p class="muted">Loading...</p>
{:else if containers.length === 0}
  <div class="card empty-state">
    <div class="empty-state-icon">=</div>
    <p>No containers running</p>
  </div>
{:else}
  <div class="card">
    <table>
      <thead>
        <tr>
          <th>Status</th>
          <th>ID</th>
          <th>Service</th>
          <th>Node</th>
          <th>Image</th>
          <th>Started</th>
          <th>Ports</th>
        </tr>
      </thead>
      <tbody>
        {#each containers as c}
          {@const badge = containerBadge(c.status)}
          <tr>
            <td><span class="badge {badge.cls}">{badge.text}</span></td>
            <td>{shortId(c.id)}</td>
            <td><a href="/services/{c.serviceName}">{c.serviceName}</a></td>
            <td><a href="/nodes/{c.nodeId}">{c.nodeId}</a></td>
            <td class="muted">{c.image}</td>
            <td class="muted">{timeAgo(c.startedAt)}</td>
            <td class="muted">
              {#if c.ports && Object.keys(c.ports).length}
                {Object.entries(c.ports).map(([h, p]) => `${h}→${p}`).join(', ')}
              {:else}
                -
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

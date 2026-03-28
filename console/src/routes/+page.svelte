<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { nodeBadge, eventIcon, timeAgo, fmtBytes } from '$lib/utils.js';

  let status = $state(null);
  let error = $state(null);
  let loading = $state(true);

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

  onMount(() => {
    refresh();
    const interval = setInterval(refresh, 3000);
    return () => clearInterval(interval);
  });
</script>

<div class="page-header">
  <h1 class="page-title">Cluster Overview</h1>
  <button class="btn btn-sm" onclick={refresh}>Refresh</button>
</div>

{#if loading}
  <p class="muted">Connecting to hived...</p>
{:else if error}
  <div class="card" style="border-color: var(--red)">
    <p class="text-red">Failed to connect: {error}</p>
    <p class="muted mt-1">Is hived running with --http-port 7949?</p>
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
{/if}

<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';

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

  function statusBadge(s) {
    switch (s) {
      case 'NODE_STATUS_READY': return { text: 'ready', cls: 'badge-green' };
      case 'NODE_STATUS_DRAINING': return { text: 'draining', cls: 'badge-yellow' };
      case 'NODE_STATUS_DOWN': return { text: 'down', cls: 'badge-red' };
      default: return { text: 'unknown', cls: '' };
    }
  }
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
      <div class="card-title">Nodes</div>
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

  {#if status.nodes?.length}
    <div class="card">
      <div class="card-title" style="margin-bottom:1rem">Nodes</div>
      <table>
        <thead>
          <tr>
            <th>Status</th>
            <th>Name</th>
            <th>Address</th>
            <th>Runtime</th>
            <th>CPU</th>
            <th>Memory</th>
          </tr>
        </thead>
        <tbody>
          {#each status.nodes as node}
            {@const badge = statusBadge(node.status)}
            <tr>
              <td><span class="badge {badge.cls}">{badge.text}</span></td>
              <td>{node.name}</td>
              <td class="muted">{node.advertiseAddr || '-'}:{node.grpcPort || '-'}</td>
              <td class="muted">{node.capabilities?.containerRuntime || '-'}</td>
              <td>{node.resources?.cpuCores || '-'} cores</td>
              <td>
                {#if node.resources?.memoryTotalBytes}
                  {Math.round(Number(node.resources.memoryAvailableBytes) / 1073741824)}
                  <span class="muted">/ {Math.round(Number(node.resources.memoryTotalBytes) / 1073741824)} GB</span>
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
{/if}

<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';

  let nodes = $state([]);
  let error = $state(null);

  async function refresh() {
    try {
      const data = await api.listNodes();
      nodes = data.nodes || [];
      error = null;
    } catch (e) { error = e.message; }
  }

  function statusBadge(s) {
    switch (s) {
      case 'NODE_STATUS_READY': return { text: 'ready', cls: 'badge-green' };
      case 'NODE_STATUS_DRAINING': return { text: 'draining', cls: 'badge-yellow' };
      case 'NODE_STATUS_DOWN': return { text: 'down', cls: 'badge-red' };
      default: return { text: 'unknown', cls: '' };
    }
  }

  function fmtBytes(b) {
    if (!b) return '-';
    const gb = Number(b) / 1073741824;
    return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(Number(b) / 1048576).toFixed(0)} MB`;
  }

  onMount(() => { refresh(); const i = setInterval(refresh, 5000); return () => clearInterval(i); });
</script>

<div class="page-header">
  <h1 class="page-title">Nodes</h1>
  <button class="btn btn-sm" onclick={refresh}>Refresh</button>
</div>

{#if error}
  <p class="text-red">{error}</p>
{:else if nodes.length === 0}
  <div class="card"><p class="muted">No nodes in cluster.</p></div>
{:else}
  <div class="card">
    <table>
      <thead>
        <tr>
          <th>Status</th>
          <th>Name</th>
          <th>Address</th>
          <th>OS/Arch</th>
          <th>Runtime</th>
          <th>CPU</th>
          <th>Memory</th>
          <th>Disk</th>
        </tr>
      </thead>
      <tbody>
        {#each nodes as node}
          {@const badge = statusBadge(node.status)}
          {@const r = node.resources || {}}
          {@const c = node.capabilities || {}}
          <tr>
            <td><span class="badge {badge.cls}">{badge.text}</span></td>
            <td>{node.name}</td>
            <td class="muted">{node.advertiseAddr || '-'}</td>
            <td>{c.os || '-'}/{c.arch || '-'}</td>
            <td>{c.containerRuntime || '-'}</td>
            <td>{r.cpuCores || '-'}</td>
            <td>
              {#if r.memoryTotalBytes}
                {fmtBytes(r.memoryAvailableBytes)} <span class="muted">/ {fmtBytes(r.memoryTotalBytes)}</span>
              {:else}-{/if}
            </td>
            <td>
              {#if r.diskTotalBytes}
                {fmtBytes(r.diskAvailableBytes)} <span class="muted">/ {fmtBytes(r.diskTotalBytes)}</span>
              {:else}-{/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

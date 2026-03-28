<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api } from '$lib/api.js';
  import { nodeBadge, fmtBytes, pct } from '$lib/utils.js';

  let nodes = $state([]);
  let error = $state(null);
  let loading = $state(true);

  async function refresh() {
    try {
      const data = await api.listNodes();
      nodes = data.nodes || [];
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  async function drain(name) {
    if (!confirm(`Drain node "${name}"?`)) return;
    try { await api.drainNode(name); await refresh(); } catch (e) { alert(e.message); }
  }

  function barColor(p) { return p > 90 ? 'red' : p > 70 ? 'yellow' : 'green'; }

  onMount(() => { refresh(); const i = setInterval(refresh, 5000); return () => clearInterval(i); });
</script>

<div class="page-header">
  <h1 class="page-title">Nodes</h1>
  <button class="btn btn-sm" onclick={refresh}>Refresh</button>
</div>

{#if error}
  <p class="text-red">{error}</p>
{:else if loading}
  <p class="muted">Loading...</p>
{:else if nodes.length === 0}
  <div class="card empty-state">
    <div class="empty-state-icon">#</div>
    <p>No nodes in cluster</p>
  </div>
{:else}
  <div class="card">
    <table>
      <thead>
        <tr>
          <th>Status</th>
          <th>Name</th>
          <th>Address</th>
          <th>Mesh IP</th>
          <th>OS/Arch</th>
          <th>Runtime</th>
          <th>CPU</th>
          <th>Memory</th>
          <th>Disk</th>
          <th>Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each nodes as node}
          {@const badge = nodeBadge(node.status)}
          {@const r = node.resources || {}}
          {@const c = node.capabilities || {}}
          {@const memPct = pct(Number(r.memoryTotalBytes) - Number(r.memoryAvailableBytes), r.memoryTotalBytes)}
          {@const diskPct = pct(Number(r.diskTotalBytes) - Number(r.diskAvailableBytes), r.diskTotalBytes)}
          <tr class="clickable" onclick={(e) => { if (!e.target.closest('button')) goto(`/nodes/${node.name}`); }}>
            <td><span class="badge {badge.cls}">{badge.text}</span></td>
            <td>{node.name}</td>
            <td class="muted">{node.advertiseAddr || '-'}</td>
            <td class="muted">{node.wgAddr || '-'}</td>
            <td>{c.os || '-'}/{c.arch || '-'}</td>
            <td>{c.containerRuntime || '-'}</td>
            <td>{r.cpuCores || '-'}</td>
            <td>
              <div>{fmtBytes(r.memoryAvailableBytes)} <span class="muted">/ {fmtBytes(r.memoryTotalBytes)}</span></div>
              <div class="resource-bar"><div class="resource-bar-fill {barColor(memPct)}" style="width:{memPct}%"></div></div>
            </td>
            <td>
              <div>{fmtBytes(r.diskAvailableBytes)} <span class="muted">/ {fmtBytes(r.diskTotalBytes)}</span></div>
              <div class="resource-bar"><div class="resource-bar-fill {barColor(diskPct)}" style="width:{diskPct}%"></div></div>
            </td>
            <td>
              {#if node.status !== 'NODE_STATUS_DRAINING'}
                <button class="btn btn-sm btn-danger" onclick={() => drain(node.name)}>Drain</button>
              {:else}
                <span class="badge badge-yellow">draining</span>
              {/if}
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

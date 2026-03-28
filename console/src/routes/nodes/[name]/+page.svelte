<script>
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api.js';
  import { nodeBadge, containerBadge, shortId, timeAgo, fmtBytes, pct } from '$lib/utils.js';

  const name = $derived($page.params.name);
  let node = $state(null);
  let containers = $state([]);
  let error = $state(null);

  async function refresh() {
    try {
      const [nData, cData] = await Promise.all([
        api.listNodes(),
        api.listContainers('', name)
      ]);
      node = (nData.nodes || []).find(n => n.name === name) || null;
      containers = cData.containers || [];
      error = null;
    } catch (e) { error = e.message; }
  }

  async function drain() {
    if (!confirm(`Drain node "${name}"? New workloads will not be scheduled here.`)) return;
    try { await api.drainNode(name); await refresh(); } catch (e) { alert(e.message); }
  }

  function barColor(pctVal) {
    if (pctVal > 90) return 'red';
    if (pctVal > 70) return 'yellow';
    return 'green';
  }

  onMount(() => {
    refresh();
    const i = setInterval(refresh, 5000);
    return () => clearInterval(i);
  });
</script>

<div class="breadcrumb">
  <a href="/nodes">Nodes</a>
  <span class="sep">/</span>
  <span class="current">{name}</span>
</div>

{#if error}
  <p class="text-red">{error}</p>
{:else if !node}
  <p class="muted">Loading...</p>
{:else}
  {@const badge = nodeBadge(node.status)}
  {@const r = node.resources || {}}
  {@const c = node.capabilities || {}}
  {@const memUsedPct = pct(Number(r.memoryTotalBytes) - Number(r.memoryAvailableBytes), r.memoryTotalBytes)}
  {@const diskUsedPct = pct(Number(r.diskTotalBytes) - Number(r.diskAvailableBytes), r.diskTotalBytes)}

  <div class="page-header">
    <h1 class="page-title">
      <span class="badge {badge.cls}" style="margin-right:0.5rem">{badge.text}</span>
      {name}
    </h1>
    {#if node.status !== 'NODE_STATUS_DRAINING'}
      <button class="btn btn-sm btn-danger" onclick={drain}>Drain Node</button>
    {/if}
  </div>

  <div class="detail-grid">
    <div class="card">
      <div class="card-title">System Info</div>
      <div class="detail-row"><span class="detail-label">Address</span><span class="detail-value">{node.advertiseAddr || '-'}</span></div>
      <div class="detail-row"><span class="detail-label">gRPC Port</span><span class="detail-value">{node.grpcPort || '-'}</span></div>
      <div class="detail-row"><span class="detail-label">OS / Arch</span><span class="detail-value">{c.os || '-'} / {c.arch || '-'}</span></div>
      <div class="detail-row"><span class="detail-label">Runtime</span><span class="detail-value">{c.containerRuntime || '-'}</span></div>
      {#if c.platforms?.length}
        <div class="detail-row"><span class="detail-label">Platforms</span><span class="detail-value">{c.platforms.join(', ')}</span></div>
      {/if}
      <div class="detail-row"><span class="detail-label">Joined</span><span class="detail-value">{timeAgo(node.joinedAt)}</span></div>
      <div class="detail-row"><span class="detail-label">Last Seen</span><span class="detail-value">{timeAgo(node.lastSeen)}</span></div>
      {#if node.wgAddr}
        <div class="detail-row"><span class="detail-label">WireGuard IP</span><span class="detail-value">{node.wgAddr}</span></div>
        <div class="detail-row"><span class="detail-label">WireGuard Key</span><span class="detail-value" style="font-size:0.7rem">{node.wgPubKey?.substring(0, 20)}...</span></div>
      {/if}
    </div>

    <div class="card">
      <div class="card-title">Resources</div>

      <div style="margin-bottom:1rem">
        <div class="resource-label">
          <span>CPU</span>
          <span>{r.cpuCores || '-'} cores{r.cpuUsagePercent ? ` (${r.cpuUsagePercent.toFixed(1)}%)` : ''}</span>
        </div>
        {#if r.cpuUsagePercent != null}
          {@const cpuPct = Math.round(r.cpuUsagePercent)}
          <div class="resource-bar"><div class="resource-bar-fill {barColor(cpuPct)}" style="width:{cpuPct}%"></div></div>
        {/if}
      </div>

      <div style="margin-bottom:1rem">
        <div class="resource-label">
          <span>Memory</span>
          <span>{fmtBytes(Number(r.memoryTotalBytes) - Number(r.memoryAvailableBytes))} / {fmtBytes(r.memoryTotalBytes)} ({memUsedPct}%)</span>
        </div>
        <div class="resource-bar"><div class="resource-bar-fill {barColor(memUsedPct)}" style="width:{memUsedPct}%"></div></div>
      </div>

      <div>
        <div class="resource-label">
          <span>Disk</span>
          <span>{fmtBytes(Number(r.diskTotalBytes) - Number(r.diskAvailableBytes))} / {fmtBytes(r.diskTotalBytes)} ({diskUsedPct}%)</span>
        </div>
        <div class="resource-bar"><div class="resource-bar-fill {barColor(diskUsedPct)}" style="width:{diskUsedPct}%"></div></div>
      </div>

      {#if r.temperatureCelsius}
        <div class="detail-row" style="margin-top:1rem"><span class="detail-label">Temperature</span><span class="detail-value">{r.temperatureCelsius.toFixed(1)}°C</span></div>
      {/if}
    </div>
  </div>

  <div class="section">
    <h2 class="section-title">Containers on this Node ({containers.length})</h2>
    {#if containers.length === 0}
      <div class="card"><p class="muted">No containers on this node</p></div>
    {:else}
      <div class="card">
        <table>
          <thead><tr><th>Status</th><th>ID</th><th>Service</th><th>Image</th><th>Started</th></tr></thead>
          <tbody>
            {#each containers as ct}
              {@const cb = containerBadge(ct.status)}
              <tr>
                <td><span class="badge {cb.cls}">{cb.text}</span></td>
                <td>{shortId(ct.id)}</td>
                <td><a href="/services/{ct.serviceName}">{ct.serviceName}</a></td>
                <td class="muted">{ct.image}</td>
                <td class="muted">{timeAgo(ct.startedAt)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </div>

  {#if node.labels && Object.keys(node.labels).length}
    <div class="section">
      <h2 class="section-title">Labels</h2>
      <div class="card">
        <div class="kv-grid">
          {#each Object.entries(node.labels) as [k, v]}
            <span class="kv-key">{k}</span>
            <span class="kv-val">{v}</span>
          {/each}
        </div>
      </div>
    </div>
  {/if}
{/if}

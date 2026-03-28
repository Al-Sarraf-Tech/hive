<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';

  let logs = $state([]);
  let services = $state([]);
  let filterService = $state('');
  let lineCount = $state(200);
  let autoRefresh = $state(true);
  let error = $state(null);
  let logEl;
  let evtSource = null;

  async function refresh() {
    try {
      const data = filterService
        ? await api.getServiceLogs(filterService, lineCount)
        : await api.getLogs(lineCount);
      logs = data ? (Array.isArray(data) ? data : (data.entries || data.logs || [])) : [];
      error = null;
      if (autoRefresh && logEl) {
        requestAnimationFrame(() => { logEl.scrollTop = logEl.scrollHeight; });
      }
    } catch (e) { error = e.message; }
  }

  function startStream() {
    if (evtSource) evtSource.close();
    const token = typeof sessionStorage !== 'undefined' ? sessionStorage.getItem('hive_token') : '';
    const params = new URLSearchParams();
    if (filterService) params.set('service', filterService);
    if (token) params.set('token', token);
    evtSource = new EventSource(`/api/v1/logs/stream?${params}`);
    evtSource.onmessage = (e) => {
      const entry = JSON.parse(e.data);
      logs = [...logs, entry];
      if (logs.length > lineCount) logs = logs.slice(-lineCount);
      error = null;
      if (autoRefresh && logEl) {
        requestAnimationFrame(() => { logEl.scrollTop = logEl.scrollHeight; });
      }
    };
    evtSource.onerror = () => {
      // EventSource reconnects automatically; surface a transient error
      error = 'Stream disconnected — reconnecting...';
    };
  }

  function stopStream() {
    if (evtSource) {
      evtSource.close();
      evtSource = null;
    }
  }

  async function loadServices() {
    try {
      const data = await api.listServices();
      services = data.services || [];
    } catch (_) {}
  }

  function fmtTs(ts) {
    if (!ts) return '';
    const d = new Date(ts);
    if (isNaN(d)) return '';
    return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
  }

  function onFilterChange() {
    refresh().then(() => {
      if (autoRefresh) startStream();
    });
  }

  onMount(() => {
    loadServices();
    refresh().then(() => {
      if (autoRefresh) startStream();
    });
    return () => stopStream();
  });
</script>

<div class="page-header">
  <h1 class="page-title">Logs</h1>
  <div class="btn-group">
    <button class="btn btn-sm" class:btn-primary={autoRefresh} onclick={() => { autoRefresh = !autoRefresh; if (autoRefresh) startStream(); else stopStream(); }}>
      {autoRefresh ? 'Live' : 'Paused'}
    </button>
    <button class="btn btn-sm" onclick={refresh}>Refresh</button>
  </div>
</div>

<div class="log-toolbar">
  <select bind:value={filterService} onchange={onFilterChange} style="width:auto; min-width:150px">
    <option value="">All services</option>
    {#each services as svc}
      <option value={svc.name}>{svc.name}</option>
    {/each}
  </select>
  <select value={lineCount} onchange={(e) => { lineCount = parseInt(e.target.value, 10); onFilterChange(); }} style="width:auto; min-width:100px">
    <option value={100}>100 lines</option>
    <option value={200}>200 lines</option>
    <option value={500}>500 lines</option>
    <option value={1000}>1000 lines</option>
  </select>
  <span class="muted" style="font-size:0.75rem">{logs.length} entries</span>
</div>

{#if error}
  <p class="text-red mb-1">{error}</p>
{/if}

<div class="log-viewer" bind:this={logEl}>
  {#if logs.length === 0}
    <p class="muted">No logs available</p>
  {:else}
    {#each logs as entry}
      <div class="log-line" class:log-stderr={entry.stream === 'stderr'}>
        <span class="log-ts">{fmtTs(entry.timestamp)}</span>
        <span class="log-svc">{entry.service_name || '-'}</span>
        <span class="log-msg">{entry.line}</span>
      </div>
    {/each}
  {/if}
</div>

<script>
  import { onMount } from 'svelte';
  import { page } from '$app/stores';
  import { api } from '$lib/api.js';
  import { serviceBadge, containerBadge, shortId, timeAgo } from '$lib/utils.js';

  const sensitivePattern = /secret|password|token|key/i;
  function maskSensitive(key, value) {
    return sensitivePattern.test(key) ? '••••••' : value;
  }

  const name = $derived($page.params.name);
  let service = $state(null);
  let containers = $state([]);
  let logs = $state([]);
  let error = $state(null);
  let tab = $state('overview');

  // Health state
  let healthEvents = $state([]);
  let healthLoading = $state(false);
  let healthStatus = $state(null);

  // Exec state
  let execCmd = $state('');
  let execResult = $state(null);
  let execRunning = $state(false);

  async function refresh() {
    try {
      const [sData, cData] = await Promise.all([
        api.listServices(),
        api.listContainers(name)
      ]);
      service = (sData.services || []).find(s => s.name === name) || null;
      containers = cData.containers || [];
      error = null;
    } catch (e) { error = e.message; }
  }

  async function loadLogs() {
    try {
      const data = await api.getServiceLogs(name, 100);
      logs = data ? (Array.isArray(data) ? data : (data.entries || data.logs || [])) : [];
    } catch (_) {}
  }

  async function doExec() {
    if (!execCmd.trim()) return;
    execRunning = true;
    execResult = null;
    try {
      const parts = execCmd.trim().split(/\s+/);
      execResult = await api.execCommand(name, parts);
    } catch (e) {
      execResult = { exitCode: -1, stderr: e.message, stdout: '' };
    } finally {
      execRunning = false;
    }
  }

  async function stop() {
    if (!confirm(`Stop service "${name}"? All containers will be removed.`)) return;
    try { await api.stopService(name); await refresh(); } catch (e) { alert(e.message); }
  }

  async function restart() {
    if (!confirm(`Restart service "${name}"?`)) return;
    try { await api.restartService(name); await refresh(); } catch (e) { alert(e.message); }
  }

  async function scale() {
    const count = prompt(`Scale "${name}" to how many replicas?`);
    if (!count) return;
    const n = parseInt(count, 10);
    if (isNaN(n) || n < 1) { alert('Invalid replica count'); return; }
    try { await api.scaleService(name, n); await refresh(); } catch (e) { alert(e.message); }
  }

  async function rollback() {
    if (!confirm(`Rollback "${name}" to previous version?`)) return;
    try { await api.rollbackService(name); await refresh(); } catch (e) { alert(e.message); }
  }

  function fmtTs(ts) {
    if (!ts) return '';
    const d = new Date(ts);
    if (isNaN(d)) return '';
    return d.toLocaleTimeString('en-US', { hour12: false });
  }

  onMount(() => {
    refresh();
    const i = setInterval(refresh, 5000);
    return () => clearInterval(i);
  });

  async function loadHealth() {
    healthLoading = true;
    try {
      const data = await api.getServiceHealth(name);
      healthEvents = data.events || [];
      healthStatus = {
        currentlyHealthy: data.currentlyHealthy ?? null,
        consecutiveFailures: data.consecutiveFailures ?? 0,
      };
    } catch (_) {
      healthEvents = [];
      healthStatus = null;
    } finally {
      healthLoading = false;
    }
  }

  $effect(() => { if (tab === 'logs') loadLogs(); });

  $effect(() => {
    if (tab === 'health') {
      loadHealth();
      const i = setInterval(loadHealth, 10000);
      return () => clearInterval(i);
    }
  });
</script>

<div class="breadcrumb">
  <a href="/services">Services</a>
  <span class="sep">/</span>
  <span class="current">{name}</span>
</div>

{#if error}
  <p class="text-red">{error}</p>
{:else if !service}
  <p class="muted">Loading...</p>
{:else}
  {@const badge = serviceBadge(service.status)}
  <div class="page-header">
    <h1 class="page-title">
      <span class="badge {badge.cls}" style="margin-right:0.5rem">{badge.text}</span>
      {name}
    </h1>
    <div class="btn-group">
      <button class="btn btn-sm" onclick={scale}>Scale</button>
      <button class="btn btn-sm" onclick={restart}>Restart</button>
      <button class="btn btn-sm" onclick={rollback}>Rollback</button>
      <button class="btn btn-sm btn-danger" onclick={stop}>Stop</button>
    </div>
  </div>

  <div class="tabs">
    <button class="tab" class:active={tab === 'overview'} onclick={() => tab = 'overview'}>Overview</button>
    <button class="tab" class:active={tab === 'containers'} onclick={() => tab = 'containers'}>Containers ({containers.length})</button>
    <button class="tab" class:active={tab === 'config'} onclick={() => tab = 'config'}>Config</button>
    <button class="tab" class:active={tab === 'health'} onclick={() => tab = 'health'}>Health</button>
    <button class="tab" class:active={tab === 'logs'} onclick={() => tab = 'logs'}>Logs</button>
    <button class="tab" class:active={tab === 'exec'} onclick={() => tab = 'exec'}>Exec</button>
  </div>

  {#if tab === 'overview'}
    <div class="detail-grid">
      <div class="card">
        <div class="card-title">Service Info</div>
        <div class="detail-row"><span class="detail-label">Image</span><span class="detail-value">{service.image}</span></div>
        <div class="detail-row"><span class="detail-label">Replicas</span><span class="detail-value">{service.replicasRunning ?? 0}/{service.replicasDesired ?? 0}</span></div>
        <div class="detail-row"><span class="detail-label">Platform</span><span class="detail-value">{service.platform || 'any'}</span></div>
        <div class="detail-row"><span class="detail-label">Node</span><span class="detail-value">{service.nodeConstraint || 'any'}</span></div>
        <div class="detail-row"><span class="detail-label">Strategy</span><span class="detail-value">{service.deployStrategy?.replace('DEPLOY_STRATEGY_', '').toLowerCase() || 'rolling'}</span></div>
      </div>
      <div class="card">
        <div class="card-title">Health Check</div>
        {#if service.healthCheck?.type}
          <div class="detail-row"><span class="detail-label">Type</span><span class="detail-value">{service.healthCheck.type.replace('HEALTH_CHECK_TYPE_', '').toLowerCase()}</span></div>
          {#if service.healthCheck.path}<div class="detail-row"><span class="detail-label">Path</span><span class="detail-value">{service.healthCheck.path}</span></div>{/if}
          <div class="detail-row"><span class="detail-label">Port</span><span class="detail-value">{service.healthCheck.port}</span></div>
          <div class="detail-row"><span class="detail-label">Interval</span><span class="detail-value">{service.healthCheck.interval || '30s'}</span></div>
          <div class="detail-row"><span class="detail-label">Timeout</span><span class="detail-value">{service.healthCheck.timeout || '5s'}</span></div>
          <div class="detail-row"><span class="detail-label">Retries</span><span class="detail-value">{service.healthCheck.retries || 3}</span></div>
        {:else}
          <p class="muted">No health check configured</p>
        {/if}
      </div>
    </div>

    {#if service.resourceSpec}
      <div class="card section">
        <div class="card-title">Resources</div>
        {#if service.resourceSpec.memoryLimit}<div class="detail-row"><span class="detail-label">Memory Limit</span><span class="detail-value">{service.resourceSpec.memoryLimit}</span></div>{/if}
        {#if service.resourceSpec.cpuLimit}<div class="detail-row"><span class="detail-label">CPU Limit</span><span class="detail-value">{service.resourceSpec.cpuLimit}</span></div>{/if}
      </div>
    {/if}

  {:else if tab === 'containers'}
    {#if containers.length === 0}
      <div class="card empty-state"><p>No containers running for this service</p></div>
    {:else}
      <div class="card">
        <table>
          <thead><tr><th>Status</th><th>ID</th><th>Node</th><th>Image</th><th>Started</th><th>Ports</th></tr></thead>
          <tbody>
            {#each containers as c}
              {@const cb = containerBadge(c.status)}
              <tr>
                <td><span class="badge {cb.cls}">{cb.text}</span></td>
                <td>{shortId(c.id)}</td>
                <td>{c.nodeId || '-'}</td>
                <td class="muted">{c.image}</td>
                <td class="muted">{timeAgo(c.startedAt)}</td>
                <td class="muted">{c.ports ? Object.entries(c.ports).map(([h, p]) => `${h}→${p}`).join(', ') : '-'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

  {:else if tab === 'config'}
    <div class="detail-grid">
      <div class="card">
        <div class="card-title">Environment Variables</div>
        {#if service.env && Object.keys(service.env).length}
          <div class="kv-grid" style="margin-top:0.5rem">
            {#each Object.entries(service.env) as [k, v]}
              <span class="kv-key">{k}</span>
              <span class="kv-val">{maskSensitive(k, v)}</span>
            {/each}
          </div>
        {:else}
          <p class="muted">No environment variables</p>
        {/if}
      </div>
      <div class="card">
        <div class="card-title">Ports</div>
        {#if service.ports && Object.keys(service.ports).length}
          <div class="kv-grid" style="margin-top:0.5rem">
            {#each Object.entries(service.ports) as [host, container]}
              <span class="kv-key">{host}</span>
              <span class="kv-val">→ {container}</span>
            {/each}
          </div>
        {:else}
          <p class="muted">No port mappings</p>
        {/if}
      </div>
    </div>

    {#if service.volumes?.length}
      <div class="card section">
        <div class="card-title">Volumes</div>
        <table>
          <thead><tr><th>Name</th><th>Target</th><th>Mode</th></tr></thead>
          <tbody>
            {#each service.volumes as vol}
              <tr>
                <td>{vol.name || '-'}</td>
                <td>{vol.target || '-'}</td>
                <td class="muted">{vol.readOnly ? 'ro' : 'rw'}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}

    {#if service.dependsOn?.length}
      <div class="card section">
        <div class="card-title">Dependencies</div>
        <div style="display:flex; gap:0.5rem; flex-wrap:wrap; margin-top:0.5rem">
          {#each service.dependsOn as dep}
            <a href="/services/{dep}" class="badge badge-cyan">{dep}</a>
          {/each}
        </div>
      </div>
    {/if}

  {:else if tab === 'health'}
    <div class="card">
      <div class="card-title">Health Status</div>
      {#if healthLoading && !healthStatus}
        <p class="muted">Loading health data...</p>
      {:else if !healthStatus}
        <p class="muted">No health data available</p>
      {:else}
        <div style="display:flex; align-items:center; gap:1rem; margin-top:0.5rem; margin-bottom:1rem">
          {#if healthStatus.currentlyHealthy}
            <span class="badge badge-green">healthy</span>
          {:else}
            <span class="badge badge-red">unhealthy</span>
          {/if}
          {#if healthStatus.consecutiveFailures > 0}
            <span class="muted">Consecutive failures: <strong class="text-red">{healthStatus.consecutiveFailures}</strong></span>
          {/if}
        </div>

        {#if healthEvents.length === 0}
          <p class="muted">No health events recorded</p>
        {:else}
          <table>
            <thead><tr><th>Timestamp</th><th>Status</th><th>Message</th><th>Duration</th></tr></thead>
            <tbody>
              {#each healthEvents as evt}
                <tr>
                  <td class="muted">{fmtTs(evt.timestamp)}</td>
                  <td>
                    {#if evt.healthy}
                      <span style="color:var(--green)">&#9679;</span>
                    {:else}
                      <span style="color:var(--red)">&#9679;</span>
                    {/if}
                  </td>
                  <td>{evt.message || '-'}</td>
                  <td class="muted">{evt.durationMs != null ? evt.durationMs + ' ms' : '-'}</td>
                </tr>
              {/each}
            </tbody>
          </table>
        {/if}
      {/if}
    </div>

  {:else if tab === 'logs'}
    <div class="log-viewer">
      {#if logs.length === 0}
        <p class="muted">No logs available</p>
      {:else}
        {#each logs as entry}
          <div class="log-line" class:log-stderr={entry.stream === 'stderr'}>
            <span class="log-ts">{fmtTs(entry.timestamp)}</span>
            <span class="log-msg">{entry.line}</span>
          </div>
        {/each}
      {/if}
    </div>
    <button class="btn btn-sm mt-1" onclick={loadLogs}>Refresh Logs</button>

  {:else if tab === 'exec'}
    <div class="card">
      <div class="card-title">Execute Command</div>
      <div style="display:flex; gap:0.5rem; margin-top:0.5rem">
        <input
          bind:value={execCmd}
          placeholder="e.g. ls -la /app"
          onkeydown={(e) => { if (e.key === 'Enter') doExec(); }}
        />
        <button class="btn btn-primary" onclick={doExec} disabled={execRunning}>
          {execRunning ? 'Running...' : 'Run'}
        </button>
      </div>
      {#if execResult}
        <div class="exec-output" style="margin-top:0.75rem">
          {#if execResult.stdout}<span>{execResult.stdout}</span>{/if}
          {#if execResult.stderr}<span class="text-red">{execResult.stderr}</span>{/if}
          <div class="muted" style="margin-top:0.5rem; border-top:1px solid var(--border); padding-top:0.5rem">exit code: {execResult.exitCode}</div>
        </div>
      {/if}
    </div>
  {/if}
{/if}

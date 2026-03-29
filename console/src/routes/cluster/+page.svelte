<script>
  import { api } from '$lib/api.js';
  import { goto } from '$app/navigation';

  let tab = $state('init');
  let clusterName = $state('');
  let joinAddresses = $state('');
  let joinToken = $state('');
  let loading = $state(false);
  let result = $state(null);
  let error = $state(null);

  async function init() {
    if (!clusterName.trim()) { error = 'Cluster name is required'; return; }
    loading = true;
    error = null;
    try {
      result = await api.initCluster(clusterName.trim());
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  async function join() {
    if (!joinAddresses.trim()) { error = 'Seed address is required'; return; }
    if (!joinToken.trim()) { error = 'Join token is required'; return; }
    loading = true;
    error = null;
    try {
      result = await api.joinCluster(joinAddresses.trim().split(',').map(s => s.trim()), joinToken.trim());
      setTimeout(() => goto('/'), 2000);
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }
</script>

<div class="page-header">
  <h1 class="page-title">Cluster Setup</h1>
</div>

<div class="tabs">
  <button class="tab" class:active={tab === 'init'} onclick={() => { tab = 'init'; result = null; error = null; }}>Initialize Cluster</button>
  <button class="tab" class:active={tab === 'join'} onclick={() => { tab = 'join'; result = null; error = null; }}>Join Cluster</button>
</div>

{#if error}
  <div class="callout callout-warn" style="margin-bottom:1rem">
    <p style="margin:0; color:var(--red)">{error}</p>
  </div>
{/if}

{#if tab === 'init'}
  <div class="card animate-in" style="padding:1.5rem; max-width:600px">
    <h3 style="margin-bottom:0.75rem">Create a New Cluster</h3>
    <p class="muted" style="font-size:0.8125rem; margin-bottom:1.25rem">
      Initialize this node as the first member of a new Hive cluster. A Certificate Authority will be generated for mTLS.
    </p>

    {#if result}
      <div class="callout callout-tip" style="margin-bottom:1rem">
        <div class="callout-title">Cluster Initialized</div>
        <p style="margin:0.25rem 0; font-size:0.8125rem">
          Cluster <strong>{clusterName}</strong> is ready.
          {#if result.joinCode}
            <br>Join code: <code class="mono" style="color:var(--accent); font-size:0.9rem">{result.joinCode}</code>
          {/if}
        </p>
      </div>
      <button class="btn btn-primary" onclick={() => goto('/')}>Go to Dashboard</button>
    {:else}
      <div class="form-group">
        <label>Cluster Name</label>
        <input type="text" bind:value={clusterName} placeholder="my-cluster"
          onkeydown={(e) => { if (e.key === 'Enter') init(); }} />
      </div>
      <button class="btn btn-primary" onclick={init} disabled={loading}>
        {loading ? 'Initializing...' : 'Initialize Cluster'}
      </button>
    {/if}
  </div>
{:else}
  <div class="card animate-in" style="padding:1.5rem; max-width:600px">
    <h3 style="margin-bottom:0.75rem">Join Existing Cluster</h3>
    <p class="muted" style="font-size:0.8125rem; margin-bottom:1.25rem">
      Connect this node to an existing Hive cluster. You'll need the seed address and join token from the cluster leader.
    </p>

    {#if result}
      <div class="callout callout-tip" style="margin-bottom:1rem">
        <div class="callout-title">Joined Successfully</div>
        <p style="margin:0; font-size:0.8125rem">
          Connected to {result.nodesJoined || 0} node(s). Redirecting to dashboard...
        </p>
      </div>
    {:else}
      <div class="form-group">
        <label>Seed Address(es)</label>
        <input type="text" bind:value={joinAddresses} placeholder="192.168.1.10:7946" />
        <div class="form-hint">Comma-separated gossip addresses of existing nodes</div>
      </div>
      <div class="form-group">
        <label>Join Token</label>
        <input type="password" bind:value={joinToken} placeholder="Cluster join token"
          onkeydown={(e) => { if (e.key === 'Enter') join(); }} />
      </div>
      <button class="btn btn-primary" onclick={join} disabled={loading}>
        {loading ? 'Joining...' : 'Join Cluster'}
      </button>
    {/if}
  </div>
{/if}

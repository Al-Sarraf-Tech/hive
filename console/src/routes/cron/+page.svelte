<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';
  import { fmtTimestamp } from '$lib/utils.js';

  let jobs = $state([]);
  let error = $state(null);
  let loading = $state(true);

  async function refresh() {
    try {
      const data = await api.listCronJobs();
      jobs = data.jobs || [];
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  onMount(() => {
    refresh();
    const i = setInterval(refresh, 10000);
    return () => clearInterval(i);
  });
</script>

<div class="page-header">
  <h1 class="page-title">Cron Jobs</h1>
  <button class="btn btn-sm" onclick={refresh}>Refresh</button>
</div>

{#if error}
  <p class="text-red">{error}</p>
{:else if loading}
  <p class="muted">Loading...</p>
{:else if jobs.length === 0}
  <div class="card empty-state">
    <div class="empty-state-icon">@</div>
    <p>No cron jobs configured</p>
    <p class="muted mt-1">Add cron schedules to your Hivefile to see them here</p>
  </div>
{:else}
  <div class="card">
    <table>
      <thead>
        <tr>
          <th>Name</th>
          <th>Schedule</th>
          <th>Service</th>
          <th>Command</th>
          <th>Next Run</th>
          <th>Last Run</th>
        </tr>
      </thead>
      <tbody>
        {#each jobs as job}
          <tr>
            <td>{job.name}</td>
            <td><code style="color:var(--accent)">{job.schedule}</code></td>
            <td><a href="/services/{job.service}">{job.service}</a></td>
            <td class="muted">{Array.isArray(job.command) ? job.command.join(' ') : job.command || '-'}</td>
            <td class="muted">{fmtTimestamp(job.nextRun)}</td>
            <td class="muted">{job.lastRun ? fmtTimestamp(job.lastRun) : 'never'}</td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

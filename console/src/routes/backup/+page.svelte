<script>
  import { api } from '$lib/api.js';

  let exporting = $state(false);
  let exportData = $state(null);
  let importData = $state('');
  let importing = $state(false);
  let importResult = $state(null);
  let overwrite = $state(false);
  let error = $state(null);

  async function doExport() {
    exporting = true;
    error = null;
    try {
      const resp = await api.exportCluster();
      exportData = resp;
    } catch (e) { error = e.message; }
    finally { exporting = false; }
  }

  function downloadExport() {
    if (!exportData?.data) return;
    const blob = new Blob([typeof exportData.data === 'string' ? exportData.data : JSON.stringify(exportData)], { type: 'application/octet-stream' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `hive-backup-${new Date().toISOString().split('T')[0]}.json`;
    a.click();
    URL.revokeObjectURL(url);
  }

  function handleFile(e) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => { importData = reader.result; };
    reader.readAsText(file);
  }

  async function doImport() {
    if (!importData.trim()) { error = 'No backup data provided'; return; }
    if (!confirm(overwrite ? 'This will OVERWRITE existing cluster state. Continue?' : 'Import backup? Existing data will be preserved.')) return;
    importing = true;
    error = null;
    importResult = null;
    try {
      importResult = await api.importCluster(importData, overwrite);
    } catch (e) { error = e.message; }
    finally { importing = false; }
  }
</script>

<div class="page-header">
  <h1 class="page-title">Backup &amp; Restore</h1>
</div>

<div class="grid-2">
  <div class="card" style="padding:1.5rem">
    <h3 style="margin-bottom:0.75rem">Export Cluster</h3>
    <p class="muted" style="font-size:0.8125rem; margin-bottom:1rem">
      Export the entire cluster state — services, secrets, volumes, and configuration — as an encrypted backup.
    </p>
    <button class="btn btn-primary" onclick={doExport} disabled={exporting}>
      {exporting ? 'Exporting...' : 'Export Backup'}
    </button>
    {#if exportData}
      <div class="callout callout-tip" style="margin-top:1rem">
        <div class="callout-title">Backup Ready</div>
        <p style="margin:0.5rem 0; font-size:0.8125rem">
          Cluster state exported successfully.
        </p>
        <button class="btn btn-sm btn-primary" onclick={downloadExport}>Download .json</button>
      </div>
    {/if}
  </div>

  <div class="card" style="padding:1.5rem">
    <h3 style="margin-bottom:0.75rem">Import / Restore</h3>
    <p class="muted" style="font-size:0.8125rem; margin-bottom:1rem">
      Restore cluster state from a backup file. Services will be redeployed.
    </p>
    <div class="form-group">
      <label class="btn btn-sm" style="cursor:pointer; display:inline-flex">
        Upload Backup File
        <input type="file" accept=".json,.bin" onchange={handleFile} style="display:none" />
      </label>
      {#if importData}
        <span class="muted" style="font-size:0.75rem; margin-left:0.5rem">File loaded ({(importData.length / 1024).toFixed(1)} KB)</span>
      {/if}
    </div>
    <div class="form-group">
      <label style="display:flex; align-items:center; gap:0.5rem; font-size:0.8125rem; color:var(--text)">
        <input type="checkbox" bind:checked={overwrite} style="width:auto" />
        Overwrite existing state
      </label>
      <div class="form-hint">When enabled, replaces all current cluster data</div>
    </div>
    <button class="btn btn-primary" onclick={doImport} disabled={importing || !importData}>
      {importing ? 'Importing...' : 'Restore Backup'}
    </button>
    {#if importResult}
      <div class="callout callout-tip" style="margin-top:1rem">
        <div class="callout-title">Restore Complete</div>
        <p style="margin:0; font-size:0.8125rem">
          Cluster state restored. Services: {importResult.servicesRestored ?? 0}, Secrets: {importResult.secretsRestored ?? 0}
        </p>
      </div>
    {/if}
  </div>
</div>

{#if error}
  <div class="callout callout-warn mt-1">
    <p style="margin:0; color:var(--red)">{error}</p>
  </div>
{/if}

<script>
  import { onMount } from 'svelte';
  import { api } from '$lib/api.js';

  let users = $state([]);
  let loading = $state(true);
  let error = $state(null);
  let showCreate = $state(false);
  let newUsername = $state('');
  let newPassword = $state('');
  let newRole = $state('operator');
  let createError = $state('');
  let creating = $state(false);

  async function refresh() {
    try {
      loading = true;
      const data = await api.authListUsers();
      users = data.users || [];
      error = null;
    } catch (e) { error = e.message; }
    finally { loading = false; }
  }

  onMount(() => { refresh(); });

  async function createUser() {
    if (!newUsername.trim() || !newPassword) { createError = 'Fill all fields'; return; }
    if (newPassword.length < 8) { createError = 'Min 8 characters'; return; }
    creating = true;
    createError = '';
    try {
      await api.authCreateUser(newUsername.trim(), newPassword, newRole);
      newUsername = ''; newPassword = ''; showCreate = false;
      await refresh();
    } catch (e) { createError = e.message; }
    finally { creating = false; }
  }

  async function deleteUser(username) {
    if (!confirm(`Delete user "${username}"?`)) return;
    try {
      await api.authDeleteUser(username);
      await refresh();
    } catch (e) { alert(e.message); }
  }

  async function changeRole(username, role) {
    try {
      await api.authSetRole(username, role);
      await refresh();
    } catch (e) { alert(e.message); }
  }

  function roleBadge(role) {
    switch (role) {
      case 'admin': return 'badge-accent';
      case 'operator': return 'badge-cyan';
      case 'viewer': return 'badge-purple';
      default: return '';
    }
  }
</script>

<div class="page-header">
  <h1 class="page-title">Users</h1>
  <button class="btn btn-primary" onclick={() => showCreate = !showCreate}>
    {showCreate ? 'Cancel' : 'Add User'}
  </button>
</div>

{#if showCreate}
  <div class="card animate-in" style="padding:1.5rem; max-width:500px; margin-bottom:1.5rem">
    <h3 style="margin-bottom:1rem; font-size:1rem">Create User</h3>
    {#if createError}
      <div class="callout callout-warn" style="margin-bottom:1rem; padding:0.5rem 0.75rem">
        <p style="margin:0; font-size:0.8125rem; color:var(--red)">{createError}</p>
      </div>
    {/if}
    <div class="form-group">
      <label>Username</label>
      <input type="text" bind:value={newUsername} placeholder="username" />
    </div>
    <div class="form-group">
      <label>Password</label>
      <input type="password" bind:value={newPassword} placeholder="Min 8 characters" />
    </div>
    <div class="form-group">
      <label>Role</label>
      <select bind:value={newRole}>
        <option value="admin">Admin — full access</option>
        <option value="operator">Operator — deploy and manage</option>
        <option value="viewer">Viewer — read-only</option>
      </select>
    </div>
    <button class="btn btn-primary" onclick={createUser} disabled={creating}>
      {creating ? 'Creating...' : 'Create User'}
    </button>
  </div>
{/if}

{#if loading}
  <p class="muted">Loading users...</p>
{:else if error}
  <div class="card" style="border-color:var(--red)">
    <p class="text-red">{error}</p>
  </div>
{:else}
  <div class="card">
    <table>
      <thead>
        <tr>
          <th>Username</th>
          <th>Role</th>
          <th>Created</th>
          <th>Last Login</th>
          <th>Status</th>
          <th style="text-align:right">Actions</th>
        </tr>
      </thead>
      <tbody>
        {#each users as user}
          <tr>
            <td style="font-weight:600">{user.username}</td>
            <td>
              <select
                value={user.role}
                style="width:auto; padding:0.2rem 0.5rem; font-size:0.75rem"
                onchange={(e) => changeRole(user.username, e.target.value)}
              >
                <option value="admin">admin</option>
                <option value="operator">operator</option>
                <option value="viewer">viewer</option>
              </select>
            </td>
            <td style="font-family:var(--sans); color:var(--text-muted); font-size:0.8125rem">
              {new Date(user.created_at).toLocaleDateString()}
            </td>
            <td style="font-family:var(--sans); color:var(--text-muted); font-size:0.8125rem">
              {user.last_login ? new Date(user.last_login).toLocaleString() : 'Never'}
            </td>
            <td>
              {#if user.disabled}
                <span class="badge badge-red">Disabled</span>
              {:else}
                <span class="badge badge-green">Active</span>
              {/if}
            </td>
            <td style="text-align:right">
              <button class="btn btn-sm btn-danger" onclick={() => deleteUser(user.username)}>Delete</button>
            </td>
          </tr>
        {/each}
      </tbody>
    </table>
  </div>
{/if}

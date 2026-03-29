<script>
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { api, storeTokens } from '$lib/api.js';

  let mode = $state('loading'); // loading, setup, login, token
  let username = $state('');
  let password = $state('');
  let confirmPassword = $state('');
  let token = $state('');
  let error = $state('');
  let submitting = $state(false);

  onMount(async () => {
    try {
      const status = await fetch('/api/v1/auth/status').then(r => r.json());
      if (status.auth_enabled && status.needs_setup) {
        mode = 'setup';
      } else if (status.auth_enabled) {
        mode = 'login';
      } else {
        mode = 'token';
      }
    } catch {
      mode = 'token'; // fallback to legacy token mode
    }
  });

  async function handleSetup() {
    if (!username.trim() || !password) { error = 'Username and password required'; return; }
    if (password.length < 8) { error = 'Password must be at least 8 characters'; return; }
    if (password !== confirmPassword) { error = 'Passwords do not match'; return; }

    submitting = true;
    error = '';
    try {
      const data = await api.authSetup(username.trim(), password);
      storeTokens(data.access_token, data.refresh_token);
      goto('/');
    } catch (e) { error = e.message; }
    finally { submitting = false; }
  }

  async function handleLogin() {
    if (!username.trim() || !password) { error = 'Username and password required'; return; }

    submitting = true;
    error = '';
    try {
      const data = await api.authLogin(username.trim(), password);
      storeTokens(data.access_token, data.refresh_token);
      goto('/');
    } catch (e) { error = e.message; }
    finally { submitting = false; }
  }

  async function handleTokenLogin() {
    if (!token.trim()) { error = 'Token is required'; return; }

    submitting = true;
    error = '';
    sessionStorage.setItem('hive_token', token.trim());
    try {
      await api.getStatus();
      goto('/');
    } catch (e) {
      sessionStorage.removeItem('hive_token');
      error = e.message.includes('unauthorized') ? 'Invalid token' : `Connection failed: ${e.message}`;
    } finally { submitting = false; }
  }

  function switchToToken() { mode = 'token'; error = ''; }
  function switchToLogin() { mode = 'login'; error = ''; }
</script>

<div class="login-page">
  {#if mode === 'loading'}
    <div class="login-card animate-in" style="text-align:center; padding:3rem">
      <p class="muted">Connecting...</p>
    </div>
  {:else}
    <div class="login-card animate-in">
      <div class="login-logo">
        <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="var(--accent)" stroke-width="2.5">
          <polygon points="12 2 22 8.5 22 15.5 12 22 2 15.5 2 8.5 12 2"/>
          <line x1="12" y1="2" x2="12" y2="22" opacity="0.3"/>
          <line x1="2" y1="8.5" x2="22" y2="8.5" opacity="0.3"/>
        </svg>
        <span>Hive</span>
      </div>

      {#if mode === 'setup'}
        <p class="muted" style="margin-bottom:1.25rem">Create your admin account to get started</p>
        <div class="badge badge-accent" style="margin-bottom:1rem">First-time setup</div>

        {#if error}
          <div class="callout callout-warn" style="margin-bottom:1rem; padding:0.5rem 0.75rem">
            <p style="margin:0; font-size:0.8125rem; color:var(--red)">{error}</p>
          </div>
        {/if}

        <div class="form-group">
          <label>Username</label>
          <input type="text" bind:value={username} placeholder="admin" autocomplete="username"
            onkeydown={(e) => { if (e.key === 'Enter') document.getElementById('setup-pass').focus(); }} />
        </div>
        <div class="form-group">
          <label>Password</label>
          <input id="setup-pass" type="password" bind:value={password} placeholder="Min 8 characters" autocomplete="new-password"
            onkeydown={(e) => { if (e.key === 'Enter') document.getElementById('setup-confirm').focus(); }} />
        </div>
        <div class="form-group">
          <label>Confirm Password</label>
          <input id="setup-confirm" type="password" bind:value={confirmPassword} placeholder="Re-enter password" autocomplete="new-password"
            onkeydown={(e) => { if (e.key === 'Enter') handleSetup(); }} />
        </div>
        <button class="btn btn-primary" style="width:100%" onclick={handleSetup} disabled={submitting}>
          {submitting ? 'Creating...' : 'Create Admin Account'}
        </button>

      {:else if mode === 'login'}
        <p class="muted" style="margin-bottom:1.25rem">Sign in to your cluster</p>

        {#if error}
          <div class="callout callout-warn" style="margin-bottom:1rem; padding:0.5rem 0.75rem">
            <p style="margin:0; font-size:0.8125rem; color:var(--red)">{error}</p>
          </div>
        {/if}

        <div class="form-group">
          <label>Username</label>
          <input type="text" bind:value={username} placeholder="Username" autocomplete="username"
            onkeydown={(e) => { if (e.key === 'Enter') document.getElementById('login-pass').focus(); }} />
        </div>
        <div class="form-group">
          <label>Password</label>
          <input id="login-pass" type="password" bind:value={password} placeholder="Password" autocomplete="current-password"
            onkeydown={(e) => { if (e.key === 'Enter') handleLogin(); }} />
        </div>
        <button class="btn btn-primary" style="width:100%" onclick={handleLogin} disabled={submitting}>
          {submitting ? 'Signing in...' : 'Sign In'}
        </button>
        <div style="margin-top:1rem; text-align:center">
          <button class="btn-ghost" style="font-size:0.75rem; border:none; background:none; color:var(--text-muted); cursor:pointer" onclick={switchToToken}>
            Use bearer token instead
          </button>
        </div>

      {:else}
        <p class="muted" style="margin-bottom:1.25rem">Enter your API bearer token</p>

        {#if error}
          <div class="callout callout-warn" style="margin-bottom:1rem; padding:0.5rem 0.75rem">
            <p style="margin:0; font-size:0.8125rem; color:var(--red)">{error}</p>
          </div>
        {/if}

        <div class="form-group">
          <label>Bearer Token</label>
          <input type="password" bind:value={token} placeholder="Enter API token" autocomplete="current-password"
            onkeydown={(e) => { if (e.key === 'Enter') handleTokenLogin(); }} />
          <div class="form-hint">Set with: <code class="mono" style="color:var(--cyan)">hived --http-token secret</code></div>
        </div>
        <button class="btn btn-primary" style="width:100%" onclick={handleTokenLogin} disabled={submitting}>
          {submitting ? 'Verifying...' : 'Sign In'}
        </button>
        <div style="margin-top:1rem; text-align:center">
          <button class="btn-ghost" style="font-size:0.75rem; border:none; background:none; color:var(--text-muted); cursor:pointer" onclick={switchToLogin}>
            Sign in with username/password
          </button>
        </div>
      {/if}

      <div style="margin-top:1.25rem; text-align:center">
        <a href="/appstore" style="font-size:0.8125rem; color:var(--text-muted)">Browse App Store</a>
      </div>
    </div>
  {/if}
</div>

<style>
  .login-page {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    background: var(--bg);
    background-image: radial-gradient(ellipse at 50% 0%, rgba(240, 192, 64, 0.04) 0%, transparent 60%);
  }
  .login-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 14px;
    padding: 2rem;
    width: 100%;
    max-width: 400px;
    box-shadow: 0 8px 40px rgba(0, 0, 0, 0.3);
  }
  .login-logo {
    display: flex;
    align-items: center;
    gap: 0.625rem;
    font-size: 1.5rem;
    font-weight: 700;
    margin-bottom: 0.5rem;
    color: var(--accent);
  }
</style>

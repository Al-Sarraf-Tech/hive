<script>
  import { goto } from '$app/navigation';

  let token = $state('');
  let error = $state('');

  function login() {
    if (!token.trim()) {
      error = 'Token is required';
      return;
    }
    sessionStorage.setItem('hive_token', token.trim());
    goto('/');
  }
</script>

<div class="login-page">
  <div class="login-card">
    <div class="login-logo">
      <span class="mono">⬡</span>
      <span>Hive</span>
    </div>
    <p class="muted">Enter your API token to continue</p>
    {#if error}
      <p class="text-red" style="margin-bottom:0.5rem">{error}</p>
    {/if}
    <input
      type="password"
      bind:value={token}
      placeholder="Bearer token"
      onkeydown={(e) => { if (e.key === 'Enter') login(); }}
    />
    <button class="btn btn-primary" style="width:100%; margin-top:0.75rem" onclick={login}>
      Sign In
    </button>
    <p class="muted" style="margin-top:1rem; font-size:0.7rem">
      Set a token with: hived --http-port 7949 --http-token your-secret
    </p>
  </div>
</div>

<style>
  .login-page {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
    background: var(--bg);
  }
  .login-card {
    background: var(--bg-card);
    border: 1px solid var(--border);
    border-radius: 12px;
    padding: 2rem;
    width: 100%;
    max-width: 360px;
  }
  .login-logo {
    display: flex;
    align-items: center;
    gap: 0.5rem;
    font-size: 1.5rem;
    font-weight: 700;
    margin-bottom: 1rem;
  }
</style>

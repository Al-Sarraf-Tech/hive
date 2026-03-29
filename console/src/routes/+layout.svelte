<script>
  import '../app.css';
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';

  let { children } = $props();

  // Auth guard: redirect to /login if no token (skip on login page itself)
  onMount(() => {
    if ($page.url.pathname !== '/login') {
      const token = sessionStorage.getItem('hive_token');
      if (!token) {
        goto('/login');
      }
    }
  });

  function logout() {
    sessionStorage.removeItem('hive_token');
    goto('/login');
  }

  const nav = [
    { href: '/', label: 'Overview', icon: '~' },
    { href: '/services', label: 'Services', icon: '>' },
    { href: '/nodes', label: 'Nodes', icon: '#' },
    { href: '/containers', label: 'Containers', icon: '=' },
    { href: '/logs', label: 'Logs', icon: '|' },
    { href: '/cron', label: 'Cron', icon: '@' },
    { href: '/deploy', label: 'Deploy', icon: '+' },
    { href: '/secrets', label: 'Secrets', icon: '*' },
    { href: '/appstore', label: 'App Store', icon: '□' },
    { href: '/settings', label: 'Settings', icon: '⚙' },
  ];

  function isActive(href, pathname) {
    if (href === '/') return pathname === '/';
    return pathname === href || pathname.startsWith(href + '/');
  }
</script>

{#if $page.url.pathname === '/login'}
  {@render children()}
{:else}
  <div class="app-layout">
    <aside class="sidebar">
      <div class="sidebar-logo">
        <span class="mono">⬡</span>
        <span>Hive</span>
      </div>
      <div class="nav-section">Cluster</div>
      {#each nav.slice(0, 3) as item}
        <a
          href={item.href}
          class="nav-link"
          class:active={isActive(item.href, $page.url.pathname)}
        >
          <span class="mono muted" style="margin-right:0.5rem">{item.icon}</span>
          {item.label}
        </a>
      {/each}
      <div class="nav-section">Observe</div>
      {#each nav.slice(3, 6) as item}
        <a
          href={item.href}
          class="nav-link"
          class:active={isActive(item.href, $page.url.pathname)}
        >
          <span class="mono muted" style="margin-right:0.5rem">{item.icon}</span>
          {item.label}
        </a>
      {/each}
      <div class="nav-section">Manage</div>
      {#each nav.slice(6, 8) as item}
        <a
          href={item.href}
          class="nav-link"
          class:active={isActive(item.href, $page.url.pathname)}
        >
          <span class="mono muted" style="margin-right:0.5rem">{item.icon}</span>
          {item.label}
        </a>
      {/each}
      <div class="nav-section">Store</div>
      {#each nav.slice(8) as item}
        <a
          href={item.href}
          class="nav-link"
          class:active={isActive(item.href, $page.url.pathname)}
        >
          <span class="mono muted" style="margin-right:0.5rem">{item.icon}</span>
          {item.label}
        </a>
      {/each}
      <div style="margin-top:auto; padding:0.75rem 1rem; border-top:1px solid var(--border);">
        <span class="mono muted" style="font-size:0.65rem">Hive v2.5.0</span>
      </div>
      <button class="nav-link" style="color:var(--text-muted); border:none; background:none; cursor:pointer; width:100%; text-align:left;" onclick={logout}>
        <span class="mono muted" style="margin-right:0.5rem">←</span>
        Logout
      </button>
    </aside>
    <main class="main-content">
      {@render children()}
    </main>
  </div>
{/if}

<script>
  import '../app.css';
  import { page } from '$app/stores';

  let { children } = $props();

  const nav = [
    { href: '/', label: 'Overview', icon: '~' },
    { href: '/services', label: 'Services', icon: '>' },
    { href: '/nodes', label: 'Nodes', icon: '#' },
    { href: '/containers', label: 'Containers', icon: '=' },
    { href: '/logs', label: 'Logs', icon: '|' },
    { href: '/cron', label: 'Cron', icon: '@' },
    { href: '/deploy', label: 'Deploy', icon: '+' },
    { href: '/secrets', label: 'Secrets', icon: '*' },
  ];

  function isActive(href, pathname) {
    if (href === '/') return pathname === '/';
    return pathname === href || pathname.startsWith(href + '/');
  }
</script>

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
    {#each nav.slice(6) as item}
      <a
        href={item.href}
        class="nav-link"
        class:active={isActive(item.href, $page.url.pathname)}
      >
        <span class="mono muted" style="margin-right:0.5rem">{item.icon}</span>
        {item.label}
      </a>
    {/each}
  </aside>
  <main class="main-content">
    {@render children()}
  </main>
</div>

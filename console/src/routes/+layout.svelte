<script>
  import '../app.css';
  let { children } = $props();

  const nav = [
    { href: '/', label: 'Overview', icon: '~' },
    { href: '/services', label: 'Services', icon: '>' },
    { href: '/nodes', label: 'Nodes', icon: '#' },
    { href: '/deploy', label: 'Deploy', icon: '+' },
    { href: '/secrets', label: 'Secrets', icon: '*' },
  ];
</script>

<div class="app-layout">
  <aside class="sidebar">
    <div class="sidebar-logo">
      <span>Hive</span>
    </div>
    <div class="nav-section">Cluster</div>
    {#each nav as item}
      <a
        href={item.href}
        class="nav-link"
        class:active={false}
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

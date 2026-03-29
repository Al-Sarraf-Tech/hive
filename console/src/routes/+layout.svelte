<script>
  import '../app.css';
  import { page } from '$app/stores';
  import { onMount } from 'svelte';
  import { goto } from '$app/navigation';
  import { clearTokens } from '$lib/api.js';

  let { children } = $props();
  let authenticated = $state(false);

  onMount(() => {
    authenticated = !!sessionStorage.getItem('hive_token');
    // No redirect — all pages are browsable without login.
    // Login is available for users who want to deploy/manage.
  });

  function logout() {
    clearTokens();
    authenticated = false;
  }

  const sections = [
    {
      label: 'Cluster',
      items: [
        { href: '/', label: 'Overview', icon: 'grid' },
        { href: '/services', label: 'Services', icon: 'layers' },
        { href: '/nodes', label: 'Nodes', icon: 'server' },
      ]
    },
    {
      label: 'Observe',
      items: [
        { href: '/containers', label: 'Containers', icon: 'box' },
        { href: '/logs', label: 'Logs', icon: 'terminal' },
        { href: '/cron', label: 'Cron', icon: 'clock' },
      ]
    },
    {
      label: 'Manage',
      items: [
        { href: '/deploy', label: 'Deploy', icon: 'upload' },
        { href: '/secrets', label: 'Secrets', icon: 'lock' },
        { href: '/backup', label: 'Backup', icon: 'download' },
        { href: '/cluster', label: 'Cluster', icon: 'hexagon' },
      ]
    },
    {
      label: 'Store',
      items: [
        { href: '/appstore', label: 'App Store', icon: 'store' },
        { href: '/users', label: 'Users', icon: 'users' },
        { href: '/settings', label: 'Settings', icon: 'settings' },
      ]
    },
    {
      label: 'Learn',
      items: [
        { href: '/learn', label: 'Tutorial', icon: 'book' },
      ]
    },
  ];

  function isActive(href, pathname) {
    if (href === '/') return pathname === '/';
    return pathname === href || pathname.startsWith(href + '/');
  }
</script>

<!-- SVG icon sprites (hidden) -->
<svg style="display:none" xmlns="http://www.w3.org/2000/svg">
  <symbol id="icon-grid" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <rect x="3" y="3" width="7" height="7"/><rect x="14" y="3" width="7" height="7"/><rect x="14" y="14" width="7" height="7"/><rect x="3" y="14" width="7" height="7"/>
  </symbol>
  <symbol id="icon-layers" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <polygon points="12 2 2 7 12 12 22 7 12 2"/><polyline points="2 17 12 22 22 17"/><polyline points="2 12 12 17 22 12"/>
  </symbol>
  <symbol id="icon-server" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/><rect x="2" y="14" width="20" height="8" rx="2" ry="2"/><line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/>
  </symbol>
  <symbol id="icon-box" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z"/><polyline points="3.27 6.96 12 12.01 20.73 6.96"/><line x1="12" y1="22.08" x2="12" y2="12"/>
  </symbol>
  <symbol id="icon-terminal" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <polyline points="4 17 10 11 4 5"/><line x1="12" y1="19" x2="20" y2="19"/>
  </symbol>
  <symbol id="icon-clock" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/>
  </symbol>
  <symbol id="icon-upload" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <polyline points="16 16 12 12 8 16"/><line x1="12" y1="12" x2="12" y2="21"/><path d="M20.39 18.39A5 5 0 0018 9h-1.26A8 8 0 103 16.3"/>
  </symbol>
  <symbol id="icon-lock" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <rect x="3" y="11" width="18" height="11" rx="2" ry="2"/><path d="M7 11V7a5 5 0 0110 0v4"/>
  </symbol>
  <symbol id="icon-store" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M3 9l9-7 9 7v11a2 2 0 01-2 2H5a2 2 0 01-2-2z"/><polyline points="9 22 9 12 15 12 15 22"/>
  </symbol>
  <symbol id="icon-settings" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 010 2.83 2 2 0 01-2.83 0l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 01-2.83-2.83l.06-.06A1.65 1.65 0 004.68 15a1.65 1.65 0 00-1.51-1H3a2 2 0 010-4h.09A1.65 1.65 0 004.6 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 012.83-2.83l.06.06A1.65 1.65 0 009 4.68a1.65 1.65 0 001-1.51V3a2 2 0 014 0v.09a1.65 1.65 0 001 1.51 1.65 1.65 0 001.82-.33l.06-.06a2 2 0 012.83 2.83l-.06.06A1.65 1.65 0 0019.4 9a1.65 1.65 0 001.51 1H21a2 2 0 010 4h-.09a1.65 1.65 0 00-1.51 1z"/>
  </symbol>
  <symbol id="icon-book" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M4 19.5A2.5 2.5 0 016.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 014 19.5v-15A2.5 2.5 0 016.5 2z"/>
  </symbol>
  <symbol id="icon-download" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/>
  </symbol>
  <symbol id="icon-hexagon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M21 16V8a2 2 0 00-1-1.73l-7-4a2 2 0 00-2 0l-7 4A2 2 0 003 8v8a2 2 0 001 1.73l7 4a2 2 0 002 0l7-4A2 2 0 0021 16z"/>
  </symbol>
  <symbol id="icon-users" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M17 21v-2a4 4 0 00-4-4H5a4 4 0 00-4 4v2"/><circle cx="9" cy="7" r="4"/><path d="M23 21v-2a4 4 0 00-3-3.87"/><path d="M16 3.13a4 4 0 010 7.75"/>
  </symbol>
  <symbol id="icon-logout" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
    <path d="M9 21H5a2 2 0 01-2-2V5a2 2 0 012-2h4"/><polyline points="16 17 21 12 16 7"/><line x1="21" y1="12" x2="9" y2="12"/>
  </symbol>
</svg>

{#if $page.url.pathname === '/login'}
  {@render children()}
{:else}
  <div class="app-layout">
    <aside class="sidebar">
      <a href="/" class="sidebar-logo" style="text-decoration:none">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5">
          <polygon points="12 2 22 8.5 22 15.5 12 22 2 15.5 2 8.5 12 2" stroke="var(--accent)"/>
          <line x1="12" y1="2" x2="12" y2="22" stroke="var(--accent)" opacity="0.3"/>
          <line x1="2" y1="8.5" x2="22" y2="8.5" stroke="var(--accent)" opacity="0.3"/>
        </svg>
        <span>Hive</span>
      </a>
      {#each sections as section}
        <div class="nav-section">{section.label}</div>
        {#each section.items as item}
          <a
            href={item.href}
            class="nav-link"
            class:active={isActive(item.href, $page.url.pathname)}
          >
            <svg width="16" height="16"><use href="#icon-{item.icon}"/></svg>
            <span class="nav-label">{item.label}</span>
          </a>
        {/each}
      {/each}
      <div style="margin-top:auto; padding:0.75rem 1.25rem; border-top:1px solid var(--border);">
        <span class="mono muted" style="font-size:0.65rem">Hive v2.5.1</span>
      </div>
      {#if authenticated}
        <button class="nav-link" style="border:none; background:none; cursor:pointer; width:100%; text-align:left;" onclick={logout}>
          <svg width="16" height="16"><use href="#icon-logout"/></svg>
          <span class="nav-label">Logout</span>
        </button>
      {/if}
    </aside>
    <main class="main-content">
      {@render children()}
    </main>
  </div>
{/if}

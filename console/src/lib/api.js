const BASE = '/api/v1';

function getToken() {
  return typeof sessionStorage !== 'undefined' ? sessionStorage.getItem('hive_token') : null;
}

async function request(path, opts = {}) {
  const token = getToken();
  const headers = { 'Content-Type': 'application/json', ...opts.headers };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(`${BASE}${path}`, {
    ...opts,
    headers,
  });
  if (res.status === 204) return {};
  if (res.status === 401) throw new Error('unauthorized');
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

// Public (unauthenticated) requests — for app store browsing without login
async function publicRequest(path) {
  const res = await fetch(`${BASE}/public${path}`);
  if (res.status === 204) return {};
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

export function isAuthenticated() {
  return !!getToken();
}

export const api = {
  // Cluster
  getStatus: () => request('/status'),

  // Nodes
  listNodes: () => request('/nodes'),
  drainNode: (name) => request(`/nodes/${encodeURIComponent(name)}/drain`, { method: 'POST' }),

  // Services
  listServices: () => request('/services'),
  deploy: (hivefileToml) => request('/deploy', { method: 'POST', body: JSON.stringify({ hivefile_toml: hivefileToml }) }),
  stopService: (name) => request(`/services/${encodeURIComponent(name)}/stop`, { method: 'POST' }),
  scaleService: (name, replicas) => request(`/services/${encodeURIComponent(name)}/scale`, { method: 'POST', body: JSON.stringify({ replicas }) }),
  rollbackService: (name) => request(`/services/${encodeURIComponent(name)}/rollback`, { method: 'POST' }),
  restartService: (name) => request(`/services/${encodeURIComponent(name)}/restart`, { method: 'POST' }),

  // Containers
  listContainers: (service = '', node = '') => {
    const params = new URLSearchParams();
    if (service) params.set('service', service);
    if (node) params.set('node', node);
    const qs = params.toString();
    return request(`/containers${qs ? '?' + qs : ''}`);
  },

  // Exec
  execCommand: (service, command) => request(`/services/${encodeURIComponent(service)}/exec`, { method: 'POST', body: JSON.stringify({ command }) }),

  // Logs
  getLogs: (lines = 200) => request(`/logs?lines=${lines}`),
  getServiceLogs: (service, lines = 200) => request(`/logs/${encodeURIComponent(service)}?lines=${lines}`),

  // Secrets
  listSecrets: () => request('/secrets'),
  setSecret: (key, value) => request(`/secrets/${encodeURIComponent(key)}`, { method: 'POST', body: JSON.stringify({ value }) }),
  deleteSecret: (key) => request(`/secrets/${encodeURIComponent(key)}`, { method: 'DELETE' }),

  // Validate
  validate: (hivefileToml, serverChecks = false) =>
    request('/validate', { method: 'POST', body: JSON.stringify({ hivefile_toml: hivefileToml, server_checks: serverChecks }) }),

  // Diff (deploy preview)
  diff: (hivefileToml) =>
    request('/diff', { method: 'POST', body: JSON.stringify({ hivefile_toml: hivefileToml }) }),

  // Health
  getServiceHealth: (name, limit = 100) =>
    request(`/services/${encodeURIComponent(name)}/health?limit=${limit}`),

  // Cron
  listCronJobs: () => request('/cron'),

  // App Store
  listApps: (category) => request(`/apps${category ? '?category=' + encodeURIComponent(category) : ''}`),
  getApp: (id) => request(`/apps/${encodeURIComponent(id)}`),
  searchApps: (q) => request(`/apps/search?q=${encodeURIComponent(q)}`),
  installApp: (id, serviceName, config) => request(`/apps/${encodeURIComponent(id)}/install`, {
    method: 'POST', body: JSON.stringify({ service_name: serviceName, config })
  }),
  listInstalledApps: () => request('/apps/installed'),

  // Node labels
  getNode: (name) => request(`/nodes/${encodeURIComponent(name)}`),
  setNodeLabel: (name, key, value) => request(`/nodes/${encodeURIComponent(name)}/labels`, {
    method: 'POST', body: JSON.stringify({ key, value })
  }),
  removeNodeLabel: (name, key) => request(`/nodes/${encodeURIComponent(name)}/labels/${encodeURIComponent(key)}`, { method: 'DELETE' }),

  // Secret rotation
  rotateSecret: (key, value) => request(`/secrets/${encodeURIComponent(key)}/rotate`, {
    method: 'POST', body: JSON.stringify({ value })
  }),

  // Backup / Restore
  exportCluster: () => request('/backup/export'),
  importCluster: (data, overwrite = false) => request('/backup/import', {
    method: 'POST', body: JSON.stringify({ data, overwrite })
  }),

  // Cluster init / join
  initCluster: (name) => request('/cluster/init', { method: 'POST', body: JSON.stringify({ name }) }),
  joinCluster: (addresses, token) => request('/cluster/join', { method: 'POST', body: JSON.stringify({ addresses, token }) }),

  // Stack deploy
  deployStack: (name, files) => request('/deploy/stack', { method: 'POST', body: JSON.stringify({ name, files }) }),

  // Service detail
  getServiceDetail: (name) => request(`/services/${encodeURIComponent(name)}`),

  // Update service (image, replicas, env)
  updateService: (name, updates) => request(`/services/${encodeURIComponent(name)}`, {
    method: 'PATCH', body: JSON.stringify(updates)
  }),

  // Discovery
  discoverContainers: () => request('/discover'),
  adoptContainer: (id, serviceName, stopOriginal) => request(`/discover/${encodeURIComponent(id)}/adopt`, {
    method: 'POST', body: JSON.stringify({ service_name: serviceName, stop_original: stopOriginal })
  }),

  // Disks
  listDisks: () => request('/disks'),

  // Registry
  registryLogin: (url, username, password) => request('/registries', {
    method: 'POST', body: JSON.stringify({ url, username, password })
  }),
  listRegistries: () => request('/registries'),
  removeRegistry: (url) => request(`/registries/${encodeURIComponent(url)}`, { method: 'DELETE' }),

  // Public App Store (no auth required — read-only catalog)
  publicListApps: (category) => publicRequest(`/apps${category ? '?category=' + encodeURIComponent(category) : ''}`),
  publicGetApp: (id) => publicRequest(`/apps/${encodeURIComponent(id)}`),
  publicSearchApps: (q) => publicRequest(`/apps/search?q=${encodeURIComponent(q)}`),

  // Authentication
  authStatus: () => publicRequest('/auth/status'.replace('/public', '')).catch(() =>
    fetch(`${BASE}/auth/status`).then(r => r.json())
  ),
  authSetup: (username, password) => fetch(`${BASE}/auth/setup`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password })
  }).then(r => r.json().then(d => { if (!r.ok) throw new Error(d.error); return d; })),
  authLogin: (username, password) => fetch(`${BASE}/auth/login`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password })
  }).then(r => r.json().then(d => { if (!r.ok) throw new Error(d.error); return d; })),
  authRefresh: (refreshToken) => fetch(`${BASE}/auth/refresh`, {
    method: 'POST', headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ refresh_token: refreshToken })
  }).then(r => r.json().then(d => { if (!r.ok) throw new Error(d.error); return d; })),
  authMe: () => request('/auth/me'),
  authChangePassword: (oldPassword, newPassword) => request('/auth/password', {
    method: 'PUT', body: JSON.stringify({ old_password: oldPassword, new_password: newPassword })
  }),
  authListUsers: () => request('/auth/users'),
  authCreateUser: (username, password, role) => request('/auth/users', {
    method: 'POST', body: JSON.stringify({ username, password, role })
  }),
  authDeleteUser: (username) => request(`/auth/users/${encodeURIComponent(username)}`, { method: 'DELETE' }),
  authSetRole: (username, role) => request(`/auth/users/${encodeURIComponent(username)}/role`, {
    method: 'PUT', body: JSON.stringify({ role })
  }),
};

// Store and retrieve JWT tokens
export function storeTokens(access, refresh) {
  sessionStorage.setItem('hive_token', access);
  if (refresh) sessionStorage.setItem('hive_refresh_token', refresh);
}

export function clearTokens() {
  sessionStorage.removeItem('hive_token');
  sessionStorage.removeItem('hive_refresh_token');
}

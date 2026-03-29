const BASE = '/api/v1';

async function request(path, opts = {}) {
  const token = typeof sessionStorage !== 'undefined' ? sessionStorage.getItem('hive_token') : null;
  const headers = { 'Content-Type': 'application/json', ...opts.headers };
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }
  const res = await fetch(`${BASE}${path}`, {
    ...opts,
    headers,
  });
  if (res.status === 204) return {};
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
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

  // Registry
  registryLogin: (url, username, password) => request('/registries', {
    method: 'POST', body: JSON.stringify({ url, username, password })
  }),
  listRegistries: () => request('/registries'),
  removeRegistry: (url) => request(`/registries/${encodeURIComponent(url)}`, { method: 'DELETE' }),
};

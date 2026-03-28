const BASE = '/api/v1';

async function request(path, opts = {}) {
  const res = await fetch(`${BASE}${path}`, {
    ...opts,
    headers: { 'Content-Type': 'application/json', ...opts.headers },
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

  // Health
  getServiceHealth: (name, limit = 100) =>
    request(`/services/${encodeURIComponent(name)}/health?limit=${limit}`),

  // Cron
  listCronJobs: () => request('/cron'),
};

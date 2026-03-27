const BASE = '/api/v1';

async function request(path, opts = {}) {
  const res = await fetch(`${BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...opts.headers },
    ...opts
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || `HTTP ${res.status}`);
  return data;
}

export const api = {
  getStatus: () => request('/status'),
  listNodes: () => request('/nodes'),
  listServices: () => request('/services'),
  listContainers: (service = '') =>
    request(`/containers${service ? `?service=${service}` : ''}`),
  listSecrets: () => request('/secrets'),

  deploy: (hivefileToml) =>
    request('/deploy', {
      method: 'POST',
      body: JSON.stringify({ hivefile_toml: hivefileToml })
    }),

  stopService: (name) =>
    request(`/services/${name}/stop`, { method: 'POST' }),

  scaleService: (name, replicas) =>
    request(`/services/${name}/scale`, {
      method: 'POST',
      body: JSON.stringify({ replicas })
    }),

  rollbackService: (name) =>
    request(`/services/${name}/rollback`, { method: 'POST' }),

  execCommand: (service, command) =>
    request(`/services/${service}/exec`, {
      method: 'POST',
      body: JSON.stringify({ command })
    }),

  setSecret: (key, value) =>
    request(`/secrets/${key}`, {
      method: 'POST',
      body: JSON.stringify({ value })
    }),

  deleteSecret: (key) =>
    request(`/secrets/${key}`, { method: 'DELETE' })
};

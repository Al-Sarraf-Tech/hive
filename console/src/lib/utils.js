export function fmtBytes(b) {
  if (b == null || b === '') return '-';
  const n = Number(b);
  if (isNaN(n)) return '-';
  if (n >= 1073741824) return `${(n / 1073741824).toFixed(1)} GB`;
  if (n >= 1048576) return `${(n / 1048576).toFixed(0)} MB`;
  if (n >= 1024) return `${(n / 1024).toFixed(0)} KB`;
  return `${n} B`;
}

export function nodeBadge(s) {
  switch (s) {
    case 'NODE_STATUS_READY': return { text: 'ready', cls: 'badge-green' };
    case 'NODE_STATUS_DRAINING': return { text: 'draining', cls: 'badge-yellow' };
    case 'NODE_STATUS_DOWN': return { text: 'down', cls: 'badge-red' };
    default: return { text: 'unknown', cls: '' };
  }
}

export function serviceBadge(s) {
  switch (s) {
    case 'SERVICE_STATUS_RUNNING': return { text: 'running', cls: 'badge-green' };
    case 'SERVICE_STATUS_DEGRADED': return { text: 'degraded', cls: 'badge-yellow' };
    case 'SERVICE_STATUS_STOPPED': return { text: 'stopped', cls: 'badge-red' };
    case 'SERVICE_STATUS_DEPLOYING': return { text: 'deploying', cls: 'badge-cyan' };
    case 'SERVICE_STATUS_ROLLING_BACK': return { text: 'rolling back', cls: 'badge-yellow' };
    default: return { text: s?.replace('SERVICE_STATUS_', '').toLowerCase() || 'unknown', cls: '' };
  }
}

export function containerBadge(s) {
  switch (s) {
    case 'CONTAINER_STATUS_RUNNING': return { text: 'running', cls: 'badge-green' };
    case 'CONTAINER_STATUS_STOPPED': return { text: 'stopped', cls: 'badge-red' };
    case 'CONTAINER_STATUS_RESTARTING': return { text: 'restarting', cls: 'badge-yellow' };
    case 'CONTAINER_STATUS_FAILED': return { text: 'failed', cls: 'badge-red' };
    default: return { text: s?.replace('CONTAINER_STATUS_', '').toLowerCase() || 'unknown', cls: '' };
  }
}

export function eventIcon(type) {
  switch (type) {
    case 'EVENT_TYPE_NODE_JOINED': return { icon: '+', cls: 'text-green' };
    case 'EVENT_TYPE_NODE_LEFT': return { icon: '-', cls: 'text-yellow' };
    case 'EVENT_TYPE_NODE_FAILED': return { icon: '!', cls: 'text-red' };
    case 'EVENT_TYPE_SERVICE_DEPLOYED': return { icon: '>', cls: 'text-green' };
    case 'EVENT_TYPE_SERVICE_STOPPED': return { icon: 'x', cls: 'text-red' };
    case 'EVENT_TYPE_SERVICE_SCALED': return { icon: '#', cls: 'text-cyan' };
    case 'EVENT_TYPE_CONTAINER_STARTED': return { icon: '+', cls: 'text-green' };
    case 'EVENT_TYPE_CONTAINER_STOPPED': return { icon: '-', cls: 'text-red' };
    case 'EVENT_TYPE_CONTAINER_RESTARTED': return { icon: '~', cls: 'text-yellow' };
    case 'EVENT_TYPE_CONTAINER_FAILED': return { icon: '!', cls: 'text-red' };
    case 'EVENT_TYPE_HEALTH_CHECK_FAILED': return { icon: '!', cls: 'text-red' };
    case 'EVENT_TYPE_SECRET_UPDATED': return { icon: '*', cls: 'text-cyan' };
    default: return { icon: '?', cls: 'muted' };
  }
}

export function timeAgo(ts) {
  if (!ts) return '-';
  const d = typeof ts === 'string' ? new Date(ts) : new Date(ts.seconds ? ts.seconds * 1000 : ts);
  if (isNaN(d.getTime())) return '-';
  const diff = (Date.now() - d.getTime()) / 1000;
  if (diff < 0) return 'just now';
  if (diff < 60) return `${Math.floor(diff)}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

export function fmtTimestamp(ts) {
  if (!ts) return '-';
  const d = typeof ts === 'string' ? new Date(ts) : new Date(ts.seconds ? ts.seconds * 1000 : ts);
  return d.toLocaleString();
}

export function pct(used, total) {
  if (!total || !used) return 0;
  return Math.round((Number(used) / Number(total)) * 100);
}

export function shortId(id) {
  return id ? id.substring(0, 12) : '-';
}

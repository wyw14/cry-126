const api = async (path, options = {}) => {
  const response = await fetch(path, {headers: {'Content-Type': 'application/json'}, ...options});
  const body = await response.json();
  if (!response.ok) throw new Error(body.error || `HTTP ${response.status}`);
  return body;
};
const text = value => value === undefined || value === null || value === '' ? '—' : String(value);
const badge = value => {
  const v = String(value || 'unknown');
  const kind = ['sealed', 'allowed', 'draining', 'closed', 'ok'].includes(v) ? 'ok' : ['failed', 'critical', 'blocked'].includes(v) ? 'bad' : 'warn';
  return `<span class="status ${kind}">${v}</span>`;
};
const setMessage = (id, value, error = false) => {
  const node = document.getElementById(id);
  if (!node) return;
  node.textContent = value;
  node.style.color = error ? '#ff9e9e' : '#9fb2b4';
};

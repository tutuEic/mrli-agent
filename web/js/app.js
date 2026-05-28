// ==================== API Helper ====================
const API = '/api';

async function api(path, opts = {}) {
  const res = await fetch(API + path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts
  });
  const data = await res.json();
  if (!res.ok) throw new Error(data.error || 'Request failed');
  return data;
}

function toast(msg, type = 'success') {
  const el = document.createElement('div');
  el.className = 'toast toast-' + type;
  el.textContent = msg;
  document.body.appendChild(el);
  setTimeout(() => el.remove(), 3000);
}

function escapeHTML(str) {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}

// ==================== Page Switching ====================
let currentPage = 'dashboard';

function showPage(page) {
  // Hide all sections
  document.querySelectorAll('.page-section').forEach(s => s.classList.remove('active'));

  // Remove active from all buttons
  document.querySelectorAll('.nav-btn').forEach(b => b.classList.remove('active'));

  // Show target section
  const section = document.getElementById('page-' + page);
  if (section) section.classList.add('active');

  // Add active to clicked button
  const btn = document.querySelector('.nav-btn[data-page="' + page + '"]');
  if (btn) btn.classList.add('active');

  currentPage = page;

  // Load data for the page
  switch (page) {
    case 'dashboard': loadDashboard(); break;
    case 'roles': loadRoles(); break;
    case 'agents': loadAgents(); loadRoleOptions(); break;
    case 'skills': loadSkills(); break;
    case 'projects': loadProjects(); break;
    case 'tasks': loadTasks(); break;
    case 'keys': loadAPIKeys(); break;
    case 'chat': loadChatAgents(); break;
    case 'audit': loadTokenUsage(); break;
  }
}

// Init sidebar buttons
document.querySelectorAll('.nav-btn').forEach(btn => {
  btn.addEventListener('click', () => showPage(btn.dataset.page));
});

// ==================== Dashboard ====================
async function loadDashboard() {
  try {
    const stats = await api('/dashboard');
    const agents = (await api('/agents')) || [];

    document.getElementById('dashboard-stats').innerHTML = [
      { label: 'Agents', val: stats.agent_count, sub: stats.online_count + ' online' },
      { label: 'Projects', val: stats.project_count, sub: 'Total' },
      { label: 'API Keys', val: stats.api_key_count, sub: 'Configured' },
      { label: 'Tasks', val: stats.task_count, sub: stats.done_count + ' done' },
      { label: 'Skills', val: stats.skill_count || 0, sub: (stats.skill_uses || 0) + ' uses' },
      { label: 'Recoveries', val: stats.recover_count || 0, sub: 'Auto-healed' },
    ].map(s =>
      '<div class="stat-card">' +
      '<div class="stat-label">' + s.label + '</div>' +
      '<div class="stat-val">' + s.val + '</div>' +
      '<div class="stat-sub">' + s.sub + '</div>' +
      '</div>'
    ).join('');

    const agentsDiv = document.getElementById('dashboard-agents');
    if (agents.length === 0) {
      agentsDiv.innerHTML = '<div class="empty">No agents configured.</div>';
    } else {
      agentsDiv.innerHTML =
        '<div style="font-size:13px;font-weight:600;color:var(--text2);margin-bottom:10px">Agent Status</div>' +
        '<div class="agent-grid">' +
        agents.map(a =>
          '<div class="agent-card">' +
          '<div class="agent-dot ' + a.status + '"></div>' +
          '<div><div style="font-weight:600">' + a.name + '</div>' +
          '<div style="font-size:12px;color:var(--muted)">' + a.type + ' · ' + a.status + '</div></div>' +
          '</div>'
        ).join('') + '</div>';
    }
  } catch (e) { toast(e.message, 'error'); }
}

// ==================== Agents ====================
async function loadAgents() {
  try {
    const agents = (await api('/agents')) || [];
    const roles = (await api('/roles')) || [];
    const tbody = document.getElementById('agents-body');
    const empty = document.getElementById('agents-empty');

    if (agents.length === 0) { tbody.innerHTML = ''; empty.style.display = 'block'; return; }
    empty.style.display = 'none';

    // Build role lookup
    const roleMap = {};
    roles.forEach(r => roleMap[r.id] = r.name);

    tbody.innerHTML = agents.map(a => {
      const roleName = a.role_name || (a.role_id ? roleMap[a.role_id] : null) || '-';
      const lastSeen = a.last_seen ? new Date(a.last_seen).toLocaleString() : '-';
      const statusDot = getStatusColor(a.status);
      return '<tr><td>' + a.id + '</td>' +
      '<td><strong>' + a.name + '</strong></td>' +
      '<td>' + a.type + '</td>' +
      '<td>' + roleName + '</td>' +
      '<td class="mono">' + (a.endpoint || '-') + '</td>' +
      '<td><span class="status-dot" style="background:' + statusDot + '"></span>' +
      ' <span class="badge badge-' + a.status + '">' + a.status + '</span></td>' +
      '<td style="font-size:11px">' + lastSeen + '</td>' +
      '<td>' + (a.recover_count || 0) + '</td>' +
      '<td class="actions-cell">' +
      '<select class="form-input" style="width:110px;display:inline-block;font-size:11px;padding:3px 6px" onchange="assignRole(' + a.id + ',this.value)">' +
      '<option value="">-- No Role --</option>' +
      roles.map(r => '<option value="' + r.id + '"' + (a.role_id == r.id ? ' selected' : '') + '>' + r.name + '</option>').join('') +
      '</select> ' +
      '<button class="btn btn-sm btn-lifecycle" onclick="agentPing(' + a.id + ')" title="Ping">📡</button> ' +
      '<button class="btn btn-sm btn-lifecycle btn-wake" onclick="agentWake(' + a.id + ')" title="Wake">⚡</button> ' +
      '<button class="btn btn-sm btn-lifecycle btn-restart" onclick="agentRestart(' + a.id + ')" title="Restart">🔄</button> ' +
      '<button class="btn btn-sm btn-lifecycle btn-stop" onclick="agentStop(' + a.id + ')" title="Stop">⏹</button> ' +
      '<button class="btn btn-danger btn-sm" onclick="deleteAgent(' + a.id + ')">✕</button></td></tr>';
    }).join('');
  } catch (e) { toast(e.message, 'error'); }
}

async function assignRole(agentId, roleId) {
  if (!roleId) {
    try { await api('/agents/' + agentId + '/role', { method: 'DELETE' }); toast('Role removed'); loadAgents(); } catch (e) { toast(e.message, 'error'); }
    return;
  }
  try { await api('/agents/' + agentId + '/role', { method: 'POST', body: JSON.stringify({ role_id: parseInt(roleId) }) }); toast('Role assigned'); loadAgents(); } catch (e) { toast(e.message, 'error'); }
}

async function addAgent() {
  const data = {
    name: document.getElementById('a-name').value.trim(),
    type: document.getElementById('a-type').value,
    endpoint: document.getElementById('a-endpoint').value.trim(),
    binary_path: document.getElementById('a-binary').value.trim(),
    args: document.getElementById('a-args').value.trim() || '{}'
  };
  if (!data.name) { toast('Name is required', 'error'); return; }
  try {
    const agent = await api('/agents', { method: 'POST', body: JSON.stringify(data) });
    // Assign role if selected
    const roleId = document.getElementById('a-role').value;
    if (roleId && agent.id) {
      await api('/agents/' + agent.id + '/role', { method: 'POST', body: JSON.stringify({ role_id: parseInt(roleId) }) });
    }
    toast('Agent added');
    ['a-name', 'a-endpoint', 'a-binary', 'a-args'].forEach(id => document.getElementById(id).value = '');
    document.getElementById('a-role').value = '';
    loadAgents();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteAgent(id) {
  if (!confirm('Delete this agent?')) return;
  try { await api('/agents/' + id, { method: 'DELETE' }); toast('Agent deleted'); loadAgents(); } catch (e) { toast(e.message, 'error'); }
}

// ==================== Agent Lifecycle ====================
function getStatusColor(status) {
  switch (status) {
    case 'online': return '#27ae60';
    case 'busy': return '#f39c12';
    case 'starting':
    case 'stopping':
    case 'recovering': return '#3498db';
    case 'unhealthy': return '#e74c3c';
    case 'offline':
    default: return '#95a5a6';
  }
}

async function agentPing(id) {
  try {
    const r = await api('/agents/' + id + '/ping', { method: 'POST' });
    toast('Ping: ' + (r.success ? 'OK' : 'FAIL') + ' (' + (r.latency_ms||0) + 'ms) — ' + (r.message||''));
    loadAgents();
  } catch (e) { toast(e.message, 'error'); }
}

async function agentWake(id) {
  try {
    const r = await api('/agents/' + id + '/wake', { method: 'POST' });
    toast('Wake: ' + (r.success ? 'OK' : 'failed') + ' — status: ' + r.status);
    loadAgents();
  } catch (e) { toast(e.message, 'error'); }
}

async function agentRestart(id) {
  if (!confirm('Restart this agent?')) return;
  try {
    const r = await api('/agents/' + id + '/restart', { method: 'POST' });
    toast('Restart: ' + (r.success ? 'OK' : 'failed') + ' — status: ' + r.status);
    loadAgents();
  } catch (e) { toast(e.message, 'error'); }
}

async function agentStop(id) {
  if (!confirm('Stop this agent?')) return;
  try {
    const r = await api('/agents/' + id + '/stop', { method: 'POST' });
    toast('Stop: ' + (r.success ? 'OK' : 'failed'));
    loadAgents();
  } catch (e) { toast(e.message, 'error'); }
}

// ==================== Skills ====================
async function loadSkills() {
  try {
    const skills = (await api('/skills')) || [];
    const tbody = document.getElementById('skills-body');
    const empty = document.getElementById('skills-empty');

    // Load stats
    try {
      const stats = await api('/skills/stats');
      document.getElementById('skills-stats').innerHTML = [
        { label: 'Total Skills', val: stats.total || 0, sub: (stats.enabled || 0) + ' enabled' },
        { label: 'Total Uses', val: stats.total_uses || 0, sub: stats.most_used ? 'Top: ' + stats.most_used : '-' },
      ].map(s =>
        '<div class="stat-card">' +
        '<div class="stat-label">' + s.label + '</div>' +
        '<div class="stat-val">' + s.val + '</div>' +
        '<div class="stat-sub">' + s.sub + '</div>' +
        '</div>'
      ).join('');
    } catch (e) {}

    if (skills.length === 0) { tbody.innerHTML = ''; empty.style.display = 'block'; return; }
    empty.style.display = 'none';

    tbody.innerHTML = skills.map(s =>
      '<tr><td>' + s.id + '</td>' +
      '<td><strong>' + escapeHTML(s.name) + '</strong></td>' +
      '<td><span class="badge badge-info">' + escapeHTML(s.category) + '</span></td>' +
      '<td style="max-width:300px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">' + escapeHTML(s.description) + '</td>' +
      '<td>' + (s.use_count || 0) + '</td>' +
      '<td>' + (s.enabled ? '<span class="badge badge-online">enabled</span>' : '<span class="badge badge-offline">disabled</span>') + '</td>' +
      '<td>' +
      '<button class="btn btn-sm btn-secondary" onclick="toggleSkill(' + s.id + ',' + !s.enabled + ')">' + (s.enabled ? 'Disable' : 'Enable') + '</button> ' +
      '<button class="btn btn-danger btn-sm" onclick="deleteSkill(' + s.id + ')">Delete</button></td></tr>'
    ).join('');
  } catch (e) { toast(e.message, 'error'); }
}

async function addSkill() {
  const data = {
    name: document.getElementById('sk-name').value.trim(),
    category: document.getElementById('sk-category').value,
    description: document.getElementById('sk-desc').value.trim(),
    content: document.getElementById('sk-content').value.trim(),
  };
  if (!data.name) { toast('Name is required', 'error'); return; }
  try {
    await api('/skills', { method: 'POST', body: JSON.stringify(data) });
    toast('Skill created');
    ['sk-name', 'sk-desc', 'sk-content'].forEach(id => document.getElementById(id).value = '');
    loadSkills();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteSkill(id) {
  if (!confirm('Delete this skill?')) return;
  try { await api('/skills/' + id, { method: 'DELETE' }); toast('Skill deleted'); loadSkills(); } catch (e) { toast(e.message, 'error'); }
}

async function toggleSkill(id, enabled) {
  try {
    const skill = await api('/skills/' + id);
    skill.enabled = enabled;
    await api('/skills/' + id, { method: 'PUT', body: JSON.stringify(skill) });
    toast('Skill ' + (enabled ? 'enabled' : 'disabled'));
    loadSkills();
  } catch (e) { toast(e.message, 'error'); }
}

// ==================== Projects ====================
async function loadProjects() {
  try {
    const projects = (await api('/projects')) || [];
    const tbody = document.getElementById('projects-body');
    const empty = document.getElementById('projects-empty');

    if (projects.length === 0) { tbody.innerHTML = ''; empty.style.display = 'block'; return; }
    empty.style.display = 'none';

    tbody.innerHTML = projects.map(p =>
      '<tr><td>' + p.id + '</td>' +
      '<td><strong>' + p.name + '</strong></td>' +
      '<td class="mono">' + (p.git_path || '-') + '</td>' +
      '<td>' + (p.branch || 'main') + '</td>' +
      '<td>' + new Date(p.created_at).toLocaleDateString() + '</td>' +
      '<td><button class="btn btn-danger btn-sm" onclick="deleteProject(' + p.id + ')">Delete</button></td></tr>'
    ).join('');
  } catch (e) { toast(e.message, 'error'); }
}

async function addProject() {
  const data = {
    name: document.getElementById('p-name').value.trim(),
    git_path: document.getElementById('p-git').value.trim(),
    branch: document.getElementById('p-branch').value.trim() || 'main'
  };
  if (!data.name) { toast('Name is required', 'error'); return; }
  try {
    await api('/projects', { method: 'POST', body: JSON.stringify(data) });
    toast('Project added');
    ['p-name', 'p-git'].forEach(id => document.getElementById(id).value = '');
    loadProjects();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteProject(id) {
  if (!confirm('Delete this project?')) return;
  try { await api('/projects/' + id, { method: 'DELETE' }); toast('Project deleted'); loadProjects(); } catch (e) { toast(e.message, 'error'); }
}

// ==================== Tasks ====================
const TASK_STATUSES = ['todo', 'running', 'review', 'done', 'failed'];
const STATUS_LABELS = { todo: 'To Do', running: 'Running', review: 'Review', done: 'Done', failed: 'Failed' };
const STATUS_COLORS = { todo: 'var(--muted)', running: 'var(--gold)', review: '#9b59b6', done: 'var(--success)', failed: 'var(--error)' };

async function loadTasks() {
  try {
    const tasks = (await api('/tasks')) || [];
    const projects = (await api('/projects')) || [];

    const sel = document.getElementById('t-project');
    sel.innerHTML = projects.map(p => '<option value="' + p.id + '">' + p.name + '</option>').join('');

    const grouped = {};
    TASK_STATUSES.forEach(s => grouped[s] = []);
    tasks.forEach(t => { if (grouped[t.status]) grouped[t.status].push(t); });

    const board = document.getElementById('task-board');
    const empty = document.getElementById('tasks-empty');

    if (tasks.length === 0) { board.innerHTML = ''; empty.style.display = 'block'; return; }
    empty.style.display = 'none';

    board.innerHTML = TASK_STATUSES.map(status => {
      const items = grouped[status];
      return '<div class="kanban-col">' +
        '<div class="kanban-header"><span style="color:' + STATUS_COLORS[status] + '">' + STATUS_LABELS[status] + '</span><span class="kanban-count">' + items.length + '</span></div>' +
        items.map(t =>
          '<div class="kanban-card"><div class="kanban-title">' + t.title + '</div>' +
          '<div class="kanban-meta">' + (t.description || '-') + '</div>' +
          '<div class="kanban-actions">' +
          (status !== 'done' && status !== 'failed' ? '<button class="kanban-btn" onclick="moveTask(' + t.id + ',\'' + getNext(status) + '\')">' + getNextLabel(status) + '</button>' : '') +
          '<button class="kanban-btn" onclick="deleteTask(' + t.id + ')" style="color:var(--error)">Delete</button></div></div>'
        ).join('') + '</div>';
    }).join('');
  } catch (e) { toast(e.message, 'error'); }
}

function getNext(s) { return { todo: 'running', running: 'review', review: 'done' }[s] || s; }
function getNextLabel(s) { return { todo: 'Start', running: 'Review', review: 'Complete' }[s] || ''; }

async function addTask() {
  const pid = document.getElementById('t-project').value;
  const title = document.getElementById('t-title').value.trim();
  const desc = document.getElementById('t-desc').value.trim();
  if (!pid) { toast('Select a project', 'error'); return; }
  if (!title) { toast('Title is required', 'error'); return; }
  try {
    await api('/tasks', { method: 'POST', body: JSON.stringify({ project_id: parseInt(pid), title, description: desc }) });
    toast('Task added');
    document.getElementById('t-title').value = '';
    document.getElementById('t-desc').value = '';
    loadTasks();
  } catch (e) { toast(e.message, 'error'); }
}

async function moveTask(id, status) {
  try { await api('/tasks/' + id + '/status', { method: 'PUT', body: JSON.stringify({ status }) }); toast('Task moved'); loadTasks(); } catch (e) { toast(e.message, 'error'); }
}

async function deleteTask(id) {
  if (!confirm('Delete this task?')) return;
  try { await api('/tasks/' + id, { method: 'DELETE' }); toast('Task deleted'); loadTasks(); } catch (e) { toast(e.message, 'error'); }
}

// ==================== API Keys ====================
async function loadAPIKeys() {
  try {
    const keys = (await api('/api-keys')) || [];
    const tbody = document.getElementById('keys-body');
    const empty = document.getElementById('keys-empty');

    if (keys.length === 0) { tbody.innerHTML = ''; empty.style.display = 'block'; return; }
    empty.style.display = 'none';

    tbody.innerHTML = keys.map(k =>
      '<tr><td>' + k.id + '</td>' +
      '<td><strong>' + k.provider + '</strong></td>' +
      '<td>' + (k.name || '-') + '</td>' +
      '<td class="mono">' + k.api_key + '</td>' +
      '<td class="mono">' + (k.endpoint || '-') + '</td>' +
      '<td><button class="btn btn-danger btn-sm" onclick="deleteAPIKey(' + k.id + ')">Delete</button></td></tr>'
    ).join('');
  } catch (e) { toast(e.message, 'error'); }
}

async function addAPIKey() {
  const data = {
    provider: document.getElementById('k-provider').value,
    name: document.getElementById('k-name').value.trim(),
    api_key: document.getElementById('k-key').value.trim(),
    endpoint: document.getElementById('k-endpoint').value.trim()
  };
  if (!data.api_key) { toast('API Key is required', 'error'); return; }
  try {
    await api('/api-keys', { method: 'POST', body: JSON.stringify(data) });
    toast('API key added');
    ['k-name', 'k-key', 'k-endpoint'].forEach(id => document.getElementById(id).value = '');
    loadAPIKeys();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteAPIKey(id) {
  if (!confirm('Delete this API key?')) return;
  try { await api('/api-keys/' + id, { method: 'DELETE' }); toast('API key deleted'); loadAPIKeys(); } catch (e) { toast(e.message, 'error'); }
}

// ==================== Chat ====================
async function loadChatAgents() {
  try {
    const agents = (await api('/agents')) || [];
    const sel = document.getElementById('chat-agent');
    sel.innerHTML = agents.map(a => '<option value="' + a.id + '">' + a.name + ' (' + a.type + ')</option>').join('');
    if (agents.length > 0) loadChat();
  } catch (e) { toast(e.message, 'error'); }
}

async function loadChat() {
  const agentID = document.getElementById('chat-agent').value;
  if (!agentID) return;
  try {
    const messages = (await api('/chat/' + agentID)) || [];
    const container = document.getElementById('chat-messages');
    if (messages.length === 0) { container.innerHTML = '<div class="empty">No messages yet.</div>'; return; }
    container.innerHTML = messages.map(m =>
      '<div class="chat-msg ' + m.role + '"><div class="chat-bubble">' + escapeHTML(m.content) + '</div>' +
      '<div class="chat-time">' + new Date(m.created_at).toLocaleTimeString() + '</div></div>'
    ).join('');
    container.scrollTop = container.scrollHeight;
  } catch (e) { toast(e.message, 'error'); }
}

async function sendChat() {
  const agentID = document.getElementById('chat-agent').value;
  const input = document.getElementById('chat-input');
  const content = input.value.trim();
  if (!agentID) { toast('Select an agent', 'error'); return; }
  if (!content) return;
  try {
    await api('/chat/' + agentID, { method: 'POST', body: JSON.stringify({ content }) });
    input.value = '';
    loadChat();
  } catch (e) { toast(e.message, 'error'); }
}

async function clearChat() {
  const agentID = document.getElementById('chat-agent').value;
  if (!agentID) return;
  if (!confirm('Clear chat history?')) return;
  try { await api('/chat/' + agentID, { method: 'DELETE' }); toast('Chat cleared'); loadChat(); } catch (e) { toast(e.message, 'error'); }
}

// ==================== Token Usage ====================
async function loadTokenUsage() {
  try {
    const summary = await api('/tokens/summary');
    const records = (await api('/tokens')) || [];
    const byModel = summary.by_model || [];

    document.getElementById('token-summary').innerHTML = [
      { label: 'Total Cost', val: '$' + (summary.total_cost || 0).toFixed(4), sub: 'All models' },
      { label: 'Total Tokens', val: (summary.total_tokens || 0).toLocaleString(), sub: 'All models' },
      { label: 'Models', val: byModel.length, sub: 'Unique' },
      { label: 'Calls', val: byModel.reduce((s, m) => s + m.total_calls, 0), sub: 'Total' },
    ].map(s =>
      '<div class="stat-card"><div class="stat-label">' + s.label + '</div><div class="stat-val">' + s.val + '</div><div class="stat-sub">' + s.sub + '</div></div>'
    ).join('');

    const tbody = document.getElementById('tokens-body');
    const empty = document.getElementById('tokens-empty');
    if (records.length === 0) { tbody.innerHTML = ''; empty.style.display = 'block'; return; }
    empty.style.display = 'none';

    tbody.innerHTML = records.map(u =>
      '<tr><td>' + u.id + '</td><td><strong>' + u.model + '</strong></td>' +
      '<td>' + u.prompt_tokens.toLocaleString() + '</td>' +
      '<td>' + u.completion_tokens.toLocaleString() + '</td>' +
      '<td>' + u.total_tokens.toLocaleString() + '</td>' +
      '<td>$' + u.cost.toFixed(4) + '</td>' +
      '<td style="font-size:11px">' + new Date(u.created_at).toLocaleString() + '</td></tr>'
    ).join('');
  } catch (e) { toast(e.message, 'error'); }
}

// ==================== Roles ====================

async function loadRoles() {
  try {
    const roles = (await api('/roles')) || [];
    const tbody = document.getElementById('roles-body');
    const empty = document.getElementById('roles-empty');

    if (roles.length === 0) { tbody.innerHTML = ''; empty.style.display = 'block'; return; }
    empty.style.display = 'none';

    tbody.innerHTML = roles.map(r =>
      '<tr><td>' + r.id + '</td>' +
      '<td><strong>' + r.name + '</strong></td>' +
      '<td>' + (r.description || '-') + '</td>' +
      '<td>' + r.priority + '</td>' +
      '<td><span class="badge badge-' + (r.agent_count > 0 ? 'online' : 'offline') + '">' + r.agent_count + ' agents</span></td>' +
      '<td><span class="badge badge-' + (r.enabled ? 'online' : 'offline') + '">' + (r.enabled ? 'Active' : 'Disabled') + '</span></td>' +
      '<td><button class="btn btn-danger btn-sm" onclick="deleteRole(' + r.id + ')">Delete</button></td></tr>'
    ).join('');
  } catch (e) { toast(e.message, 'error'); }
}

async function addRole() {
  const name = document.getElementById('r-name').value.trim();
  const desc = document.getElementById('r-desc').value.trim();
  const priority = parseInt(document.getElementById('r-priority').value) || 0;

  if (!name) { toast('Name is required', 'error'); return; }

  try {
    await api('/roles', { method: 'POST', body: JSON.stringify({ name, description: desc, priority }) });
    toast('Role created');
    document.getElementById('r-name').value = '';
    document.getElementById('r-desc').value = '';
    document.getElementById('r-priority').value = '0';
    document.getElementById('r-name-select').value = '';
    loadRoles();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteRole(id) {
  if (!confirm('Delete this role?')) return;
  try { await api('/roles/' + id, { method: 'DELETE' }); toast('Role deleted'); loadRoles(); } catch (e) { toast(e.message, 'error'); }
}

async function loadRoleOptions() {
  try {
    const roles = (await api('/roles')) || [];
    const sel = document.getElementById('a-role');
    if (sel) {
      sel.innerHTML = '<option value="">-- No Role --</option>' +
        roles.map(r => '<option value="' + r.id + '">' + r.name + '</option>').join('');
    }
  } catch {}
}

// ==================== WebSocket ====================
let ws = null;

function connectWebSocket() {
  const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
  ws = new WebSocket(protocol + '//' + location.host + '/ws');

  ws.onopen = () => {
    document.getElementById('ws-dot').classList.add('live');
    document.getElementById('ws-label').textContent = 'Live';
  };

  ws.onmessage = (ev) => {
    try {
      const event = JSON.parse(ev.data);
      // Only update current section
      switch (event.type) {
        case 'agent.status':
        case 'dashboard.updated':
          if (currentPage === 'dashboard') loadDashboard();
          if (currentPage === 'agents') loadAgents();
          break;
        case 'task.created':
        case 'task.updated':
          if (currentPage === 'tasks') loadTasks();
          break;
        case 'chat.message':
          if (currentPage === 'chat') loadChat();
          break;
      }
    } catch {}
  };

  ws.onclose = () => {
    document.getElementById('ws-dot').classList.remove('live');
    document.getElementById('ws-label').textContent = 'Reconnecting...';
    setTimeout(connectWebSocket, 2000);
  };
}

// ==================== Init ====================
loadDashboard();
connectWebSocket();

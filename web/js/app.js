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

// ==================== Skills (Enterprise Hub) ====================
let allSkills = [];
let currentSkillFilter = 'all';
let selectedSkillId = null;

async function loadSkills() {
  try {
    allSkills = (await api('/skills')) || [];
    renderSkillStats();
    renderSkillCards();
    renderAgentSkillsMap();
    if (selectedSkillId) showSkillDetail(selectedSkillId);
  } catch (e) { toast(e.message, 'error'); }
}

function renderSkillStats() {
  const bar = document.getElementById('skills-stats-bar');
  const total = allSkills.length;
  const enabled = allSkills.filter(s => s.enabled).length;
  const uses = allSkills.reduce((a, s) => a + (s.use_count || 0), 0);
  const favs = allSkills.filter(s => s.favorite).length;
  bar.innerHTML =
    '<div class="stat-card"><div class="stat-label">Total</div><div class="stat-val">' + total + '</div></div>' +
    '<div class="stat-card"><div class="stat-label">Enabled</div><div class="stat-val">' + enabled + '</div></div>' +
    '<div class="stat-card"><div class="stat-label">Uses</div><div class="stat-val">' + uses + '</div></div>' +
    '<div class="stat-card"><div class="stat-label">Favorites</div><div class="stat-val">' + favs + '</div></div>';
}

function filterSkills() { renderSkillCards(); }

function setSkillFilter(el, filter) {
  currentSkillFilter = filter;
  document.querySelectorAll('.skills-filter-item').forEach(i => i.classList.remove('active'));
  el.classList.add('active');
  renderSkillCards();
}

function renderSkillCards() {
  const search = (document.getElementById('skill-search')?.value || '').toLowerCase();
  const catFilter = document.getElementById('skill-cat-filter')?.value || 'all';
  const grid = document.getElementById('skills-card-grid');
  const empty = document.getElementById('skills-empty');

  let filtered = allSkills.filter(s => {
    if (search && !s.name.toLowerCase().includes(search) && !s.description.toLowerCase().includes(search)) return false;
    if (catFilter !== 'all' && s.category !== catFilter) return false;
    if (currentSkillFilter === 'favorite') return s.favorite;
    if (currentSkillFilter === 'enabled') return s.enabled;
    if (currentSkillFilter === 'disabled') return !s.enabled;
    if (currentSkillFilter === 'github') return s.source === 'GitHub';
    if (currentSkillFilter === 'team') return s.source === 'Team Shared';
    if (currentSkillFilter === 'recent') return s.use_count > 0;
    return true;
  });

  if (filtered.length === 0) { grid.innerHTML = ''; empty.style.display = 'block'; return; }
  empty.style.display = 'none';

  grid.innerHTML = filtered.map(s => {
    const tags = tryParseJSON(s.tags, []);
    const sourceClass = 'skill-source-' + (s.source || 'Manual').replace(/\s/g, '');
    return '<div class="skill-card" onclick="showSkillDetail(' + s.id + ')">' +
      '<div class="skill-card-header">' +
      '<span class="skill-name">' + escapeHTML(s.name) + '</span>' +
      '<span class="skill-source ' + sourceClass + '">' + escapeHTML(s.source || 'Manual') + '</span>' +
      '<span class="skill-fav" onclick="event.stopPropagation();toggleSkillFav(' + s.id + ')">' + (s.favorite ? '&#9733;' : '&#9734;') + '</span>' +
      '</div>' +
      '<div class="skill-card-desc">' + escapeHTML(s.description || 'No description') + '</div>' +
      '<div class="skill-card-meta">' +
      '<span class="badge badge-info">' + escapeHTML(s.category) + '</span>' +
      '<span>v' + escapeHTML(s.version || '1.0') + '</span>' +
      '<span>&#128260; ' + (s.use_count || 0) + ' uses</span>' +
      '<span>' + (s.enabled ? '&#9989; Active' : '&#128683; Disabled') + '</span>' +
      '</div>' +
      (tags.length ? '<div class="skill-card-tags">' + tags.map(t => '<span class="skill-tag">' + escapeHTML(t) + '</span>').join('') + '</div>' : '') +
      '</div>';
  }).join('');
}

function tryParseJSON(str, fallback) { try { return JSON.parse(str); } catch { return fallback; } }

async function showSkillDetail(id) {
  selectedSkillId = id;
  const panel = document.getElementById('skills-detail');
  try {
    const s = await api('/skills/' + id);
    const logs = (await api('/skills/' + id + '/logs')) || [];

    const tags = tryParseJSON(s.tags, []);
    panel.innerHTML =
      '<div class="detail-section">' +
      '<div style="display:flex;justify-content:space-between;align-items:center">' +
      '<div style="font-weight:700;font-size:16px">' + escapeHTML(s.name) + '</div>' +
      '<span class="badge ' + (s.enabled ? 'badge-online' : 'badge-offline') + '">' + (s.enabled ? 'Active' : 'Disabled') + '</span>' +
      '</div>' +
      '<div style="font-size:12px;color:var(--text2);margin-top:6px">' + escapeHTML(s.description || '') + '</div>' +
      '</div>' +

      '<div class="detail-section"><div class="detail-section-title">Info</div>' +
      '<div class="detail-row"><span class="label">Category</span><span class="value">' + escapeHTML(s.category) + '</span></div>' +
      '<div class="detail-row"><span class="label">Source</span><span class="value">' + escapeHTML(s.source || 'Manual') + '</span></div>' +
      '<div class="detail-row"><span class="label">Version</span><span class="value">' + escapeHTML(s.version || '1.0') + '</span></div>' +
      '<div class="detail-row"><span class="label">Uses</span><span class="value">' + (s.use_count || 0) + '</span></div>' +
      (s.input_params ? '<div class="detail-row"><span class="label">Input</span><span class="value">' + escapeHTML(s.input_params) + '</span></div>' : '') +
      (s.output_format ? '<div class="detail-row"><span class="label">Output</span><span class="value">' + escapeHTML(s.output_format) + '</span></div>' : '') +
      (s.github_url ? '<div class="detail-row"><span class="label">GitHub</span><span class="value"><a href="' + escapeHTML(s.github_url) + '" target="_blank" style="color:var(--gold)">Link</a></span></div>' : '') +
      '</div>' +

      (tags.length ? '<div class="detail-section"><div class="detail-section-title">Tags</div><div class="skill-card-tags">' + tags.map(t => '<span class="skill-tag">' + escapeHTML(t) + '</span>').join('') + '</div></div>' : '') +

      (s.content ? '<div class="detail-section"><div class="detail-section-title">Prompt</div><div style="font-size:11px;background:var(--bg);padding:8px;border-radius:6px;max-height:120px;overflow-y:auto;white-space:pre-wrap">' + escapeHTML(s.content) + '</div></div>' : '') +

      '<div class="detail-section"><div class="detail-section-title">Actions</div>' +
      '<div style="display:flex;gap:6px;flex-wrap:wrap">' +
      '<button class="btn btn-sm btn-secondary" onclick="toggleSkillEnabled(' + s.id + ',' + !s.enabled + ')">' + (s.enabled ? 'Disable' : 'Enable') + '</button>' +
      '<button class="btn btn-sm btn-danger" onclick="deleteSkill(' + s.id + ')">Delete</button>' +
      '</div></div>' +

      '<div class="detail-section"><div class="detail-section-title">Recent Calls (' + logs.length + ')</div>' +
      (logs.length === 0 ? '<div style="font-size:11px;color:var(--muted)">No calls recorded</div>' :
        logs.slice(0, 10).map(l =>
          '<div class="detail-log-item">' +
          '<span class="' + (l.success ? 'log-success' : 'log-fail') + '">' + (l.success ? '&#10003;' : '&#10007;') + '</span> ' +
          new Date(l.created_at).toLocaleString() +
          (l.input ? ' <span style="color:var(--muted)">' + escapeHTML(l.input).substring(0, 40) + '</span>' : '') +
          '</div>'
        ).join('')) +
      '</div>';
  } catch (e) { panel.innerHTML = '<div class="empty">Error loading skill</div>'; }
}

async function renderAgentSkillsMap() {
  const panel = document.getElementById('skills-agent-map');
  try {
    const agents = (await api('/agents')) || [];
    const map = await api('/skills/agent-map');
    panel.innerHTML = '<div class="detail-section-title" style="margin-bottom:8px">Agent Skills</div>' +
      agents.map(a => {
        const skills = map[a.id] || [];
        return '<div style="margin-bottom:10px">' +
          '<div style="font-size:12px;font-weight:600;display:flex;align-items:center;gap:4px"><span class="status-dot" style="background:' + getStatusColor(a.status) + '"></span>' + escapeHTML(a.name) + '</div>' +
          (skills.length ? '<div style="margin-top:4px">' + skills.map(s => '<span class="skill-mini" style="cursor:pointer" onclick="showSkillDetail(' + s.id + ')">' + escapeHTML(s.name) + '</span>').join('') + '</div>' :
            '<div style="font-size:10px;color:var(--muted);margin-top:2px">No skills bound</div>') +
          '</div>';
      }).join('') +
      '<hr style="border:none;border-top:1px solid var(--border);margin:12px 0">';
  } catch (e) {}
}

function showCreateSkillModal() {
  document.getElementById('modal-create-skill').style.display = 'flex';
  document.getElementById('cs-name').focus();
}

async function createSkill() {
  const tagsStr = document.getElementById('cs-tags').value.trim();
  const tags = tagsStr ? JSON.stringify(tagsStr.split(',').map(t => t.trim()).filter(Boolean)) : '[]';
  const data = {
    name: document.getElementById('cs-name').value.trim(),
    category: document.getElementById('cs-category').value,
    description: document.getElementById('cs-desc').value.trim(),
    content: document.getElementById('cs-content').value.trim(),
    tags: tags,
    version: document.getElementById('cs-version').value.trim() || '1.0',
    input_params: document.getElementById('cs-input').value.trim(),
    output_format: document.getElementById('cs-output').value.trim(),
    source: 'Manual',
  };
  if (!data.name) { toast('Name is required', 'error'); return; }
  try {
    await api('/skills', { method: 'POST', body: JSON.stringify(data) });
    toast('Skill created');
    document.getElementById('modal-create-skill').style.display = 'none';
    ['cs-name', 'cs-desc', 'cs-tags', 'cs-input', 'cs-output', 'cs-content'].forEach(id => document.getElementById(id).value = '');
    loadSkills();
  } catch (e) { toast(e.message, 'error'); }
}

async function deleteSkill(id) {
  if (!confirm('Delete this skill?')) return;
  try { await api('/skills/' + id, { method: 'DELETE' }); toast('Skill deleted'); selectedSkillId = null; loadSkills(); } catch (e) { toast(e.message, 'error'); }
}

async function toggleSkillEnabled(id, enabled) {
  try {
    const skill = await api('/skills/' + id);
    skill.enabled = enabled;
    await api('/skills/' + id, { method: 'PUT', body: JSON.stringify(skill) });
    toast('Skill ' + (enabled ? 'enabled' : 'disabled'));
    loadSkills();
  } catch (e) { toast(e.message, 'error'); }
}

async function toggleSkillFav(id) {
  try { await api('/skills/' + id + '/favorite', { method: 'POST' }); loadSkills(); } catch (e) { toast(e.message, 'error'); }
}

// ==================== GitHub Import ====================
function showGitHubImportModal() {
  document.getElementById('modal-github-import').style.display = 'flex';
  document.getElementById('gh-query').focus();
}

async function searchGitHub() {
  const q = document.getElementById('gh-query').value.trim();
  if (!q) return;
  const results = document.getElementById('gh-results');
  results.innerHTML = '<div style="text-align:center;padding:20px;color:var(--muted)">Searching...</div>';
  try {
    const resp = await fetch('https://api.github.com/search/repositories?q=' + encodeURIComponent(q) + '&sort=stars&per_page=10');
    const data = await resp.json();
    if (!data.items || data.items.length === 0) {
      results.innerHTML = '<div class="empty">No results found</div>';
      return;
    }
    results.innerHTML = data.items.map(r =>
      '<div class="gh-result-item">' +
      '<div class="gh-result-header"><span class="gh-result-name">' + escapeHTML(r.full_name) + '</span><span class="gh-result-stars">&#9733; ' + (r.stargazers_count || 0) + '</span></div>' +
      '<div class="gh-result-desc">' + escapeHTML(r.description || 'No description') + '</div>' +
      '<div class="gh-result-meta">' +
      (r.language ? '<span>' + escapeHTML(r.language) + '</span>' : '') +
      '<span>Updated: ' + new Date(r.updated_at).toLocaleDateString() + '</span>' +
      '</div>' +
      '<div style="margin-top:8px;display:flex;gap:6px">' +
      '<button class="btn btn-sm btn-primary" onclick="importGitHubSkill(\'' + escapeHTML(r.full_name) + '\',\'' + escapeHTML(r.description || '') + '\',\'' + escapeHTML(r.html_url) + '\')">Import</button>' +
      '<a class="btn btn-sm btn-secondary" href="' + escapeHTML(r.html_url) + '" target="_blank">View Repo</a>' +
      '</div>' +
      '</div>'
    ).join('');
  } catch (e) { results.innerHTML = '<div class="empty">Search failed: ' + e.message + '</div>'; }
}

async function importGitHubSkill(name, desc, url) {
  const skillName = name.split('/').pop() || name;
  const data = {
    name: skillName,
    category: 'DevOps',
    description: desc || 'Imported from ' + name,
    content: '',
    source: 'GitHub',
    github_url: url,
    tags: JSON.stringify(['github', 'imported']),
    version: '1.0',
  };
  try {
    await api('/skills', { method: 'POST', body: JSON.stringify(data) });
    toast('Imported: ' + skillName);
    document.getElementById('modal-github-import').style.display = 'none';
    loadSkills();
  } catch (e) { toast(e.message, 'error'); }
}

// ==================== Projects (New Panel) ====================
let allProjects = [];
let currentProjectId = null;
let currentProjectView = 'list';

async function loadProjects() {
  try {
    allProjects = (await api('/projects')) || [];
    renderProjectList();
    if (currentProjectId) {
      selectProject(currentProjectId);
    }
  } catch (e) { toast(e.message, 'error'); }
}

function renderProjectList() {
  const search = (document.getElementById('project-search')?.value || '').toLowerCase();
  const filter = document.getElementById('project-filter')?.value || 'all';
  const list = document.getElementById('project-list');
  if (!list) return;

  let filtered = allProjects.filter(p => {
    if (search && !p.name.toLowerCase().includes(search)) return false;
    if (filter === 'favorite') return p.favorite;
    if (filter !== 'all' && p.status !== filter) return false;
    return true;
  });

  if (filtered.length === 0) {
    list.innerHTML = '<div style="padding:20px;text-align:center;color:var(--muted);font-size:12px">No projects found</div>';
    return;
  }

  list.innerHTML = filtered.map(p => {
    const statusColor = { Draft: '#95a5a6', Active: '#27ae60', Running: '#3498db', Blocked: '#e74c3c', Completed: '#2ecc71', Archived: '#7f8c8d' }[p.status] || '#95a5a6';
    return '<div class="project-list-item' + (p.id === currentProjectId ? ' active' : '') + '" onclick="selectProject(' + p.id + ')">' +
      '<span class="proj-fav" onclick="event.stopPropagation();toggleFavorite(' + p.id + ')">' + (p.favorite ? '&#9733;' : '&#9734;') + '</span>' +
      '<span class="proj-name">' + escapeHTML(p.name) + '</span>' +
      '<span class="proj-status badge" style="background:' + statusColor + '20;color:' + statusColor + '">' + p.status + '</span>' +
      '</div>';
  }).join('');
}

function filterProjects() { renderProjectList(); }

async function selectProject(id) {
  currentProjectId = id;
  renderProjectList();
  document.getElementById('project-empty').style.display = 'none';
  document.getElementById('project-detail').style.display = 'block';

  try {
    const p = await api('/projects/' + id);
    const stats = await api('/projects/' + id + '/stats');
    const tasks = (await api('/tasks?project_id=' + id)) || [];
    const agents = (await api('/agents')) || [];
    const skills = (await api('/projects/' + id + '/skills')) || [];

    renderProjectHeader(p, stats);
    renderProjectActions(p);
    renderProjectTasks(tasks);
    renderProjectKanban(tasks);
    renderProjectStats(stats);
    renderProjectAgents(agents);
    renderProjectSkills(skills);
  } catch (e) { toast(e.message, 'error'); }
}

function renderProjectHeader(p, stats) {
  const h = document.getElementById('project-header');
  const rate = Math.round(stats.completion_rate || 0);
  const statusColors = { Draft: '#95a5a6', Active: '#27ae60', Running: '#3498db', Blocked: '#e74c3c', Completed: '#2ecc71', Archived: '#7f8c8d' };
  h.innerHTML =
    '<div style="display:flex;align-items:center;gap:12px">' +
    '<h2>' + escapeHTML(p.name) + '</h2>' +
    '<span class="badge" style="background:' + (statusColors[p.status] || '#95a5a6') + '20;color:' + (statusColors[p.status] || '#95a5a6') + '">' + p.status + '</span>' +
    '<span class="badge badge-info">P' + p.priority + '</span>' +
    '</div>' +
    (p.description ? '<div style="font-size:13px;color:var(--text2);margin:6px 0">' + escapeHTML(p.description) + '</div>' : '') +
    '<div class="project-meta">' +
    (p.owner ? '<span class="project-meta-item">&#128100; ' + escapeHTML(p.owner) + '</span>' : '') +
    '<span class="project-meta-item">&#128202; ' + (stats.total_tasks || 0) + ' tasks</span>' +
    '<span class="project-meta-item">&#9989; ' + rate + '% complete</span>' +
    '<span class="project-meta-item">&#129302; ' + (stats.online_agents || 0) + ' agents online</span>' +
    '<span class="project-meta-item">&#128197; ' + new Date(p.created_at).toLocaleDateString() + '</span>' +
    '</div>';
}

function renderProjectActions(p) {
  const a = document.getElementById('project-actions');
  const isPaused = p.status === 'Blocked';
  a.innerHTML =
    '<button class="btn btn-sm btn-primary" onclick="showNewTaskModal(' + p.id + ')">+ New Task</button>' +
    (isPaused
      ? '<button class="btn btn-sm btn-wake" onclick="resumeProject(' + p.id + ')">&#9654; Resume</button>'
      : '<button class="btn btn-sm btn-stop" onclick="pauseProject(' + p.id + ')">&#9646;&#9646; Pause</button>') +
    '<button class="btn btn-sm btn-secondary" onclick="deleteProject(' + p.id + ')">&#128465; Delete</button>';
}

function renderProjectTasks(tasks) {
  const c = document.getElementById('project-task-list');
  if (tasks.length === 0) {
    c.innerHTML = '<div class="empty">No tasks yet. Click "+ New Task" to create one.</div>';
    return;
  }
  const statusColors = { pending: '#95a5a6', assigned: '#3498db', running: '#f39c12', waiting: '#9b59b6', review: '#e67e22', failed: '#e74c3c', completed: '#27ae60' };
  c.innerHTML = tasks.map(t => {
    const color = statusColors[t.status] || '#95a5a6';
    return '<div class="task-row">' +
      '<span style="width:8px;height:8px;border-radius:50%;background:' + color + ';flex-shrink:0"></span>' +
      '<span class="task-title">' + escapeHTML(t.title) + '</span>' +
      '<span class="badge" style="background:' + color + '20;color:' + color + ';font-size:10px">' + t.status + '</span>' +
      '<div class="task-progress"><div class="progress-bar"><div class="progress-fill" style="width:' + t.progress + '%;background:' + color + '"></div></div></div>' +
      '<span style="font-size:11px;color:var(--muted);width:35px">' + t.progress + '%</span>' +
      '</div>';
  }).join('');
}

function renderProjectKanban(tasks) {
  const c = document.getElementById('project-kanban');
  const cols = [
    { key: 'pending', label: 'To Do', color: '#95a5a6' },
    { key: 'assigned', label: 'Assigned', color: '#3498db' },
    { key: 'running', label: 'In Progress', color: '#f39c12' },
    { key: 'review', label: 'Review', color: '#e67e22' },
    { key: 'completed', label: 'Done', color: '#27ae60' },
    { key: 'failed', label: 'Failed', color: '#e74c3c' },
  ];
  c.innerHTML = cols.map(col => {
    const colTasks = tasks.filter(t => t.status === col.key);
    return '<div class="kanban-col">' +
      '<div class="kanban-col-header"><span style="color:' + col.color + '">' + col.label + '</span><span class="badge" style="font-size:10px">' + colTasks.length + '</span></div>' +
      '<div class="kanban-col-body">' +
      colTasks.map(t =>
        '<div class="kanban-card">' +
        '<div style="font-weight:600;margin-bottom:4px">' + escapeHTML(t.title) + '</div>' +
        '<div style="display:flex;justify-content:space-between;color:var(--muted)"><span>P' + t.priority + '</span><span>' + t.progress + '%</span></div>' +
        '</div>'
      ).join('') +
      '</div></div>';
  }).join('');
}

function renderProjectStats(stats) {
  const s = document.getElementById('project-stats');
  s.innerHTML = '<div class="panel-title">Statistics</div>' +
    '<div class="stat-card"><div class="stat-label">Total Tasks</div><div class="stat-val">' + (stats.total_tasks || 0) + '</div></div>' +
    '<div class="stat-card"><div class="stat-label">Completion</div><div class="stat-val">' + Math.round(stats.completion_rate || 0) + '%</div><div class="progress-bar" style="margin-top:4px"><div class="progress-fill" style="width:' + Math.round(stats.completion_rate || 0) + '%;background:var(--success)"></div></div></div>' +
    '<div class="stat-card"><div class="stat-label">Running</div><div class="stat-val">' + (stats.running_tasks || 0) + '</div></div>' +
    '<div class="stat-card"><div class="stat-label">Failed</div><div class="stat-val" style="color:var(--error)">' + (stats.failed_tasks || 0) + '</div></div>';
}

function renderProjectAgents(agents) {
  const c = document.getElementById('project-agents-panel');
  c.innerHTML = '<div class="panel-title">Agents</div>' +
    agents.map(a => {
      const dot = getStatusColor(a.status);
      return '<div class="agent-mini"><span class="status-dot" style="background:' + dot + '"></span><span>' + escapeHTML(a.name) + '</span><span style="color:var(--muted);margin-left:auto;font-size:10px">' + a.status + '</span></div>';
    }).join('');
}

function renderProjectSkills(skills) {
  const c = document.getElementById('project-skills-panel');
  c.innerHTML = '<div class="panel-title">Skills</div>' +
    (skills.length === 0
      ? '<div style="font-size:11px;color:var(--muted)">No skills bound</div>'
      : skills.map(s => '<span class="skill-mini">' + escapeHTML(s.name) + '</span>').join(''));
}

function switchProjectView(view) {
  currentProjectView = view;
  document.querySelectorAll('.view-btn').forEach(b => b.classList.toggle('active', b.dataset.view === view));
  document.getElementById('project-task-list').style.display = view === 'list' ? 'block' : 'none';
  document.getElementById('project-kanban').style.display = view === 'kanban' ? 'flex' : 'none';
}

function showNewProjectModal() {
  document.getElementById('modal-new-project').style.display = 'flex';
  document.getElementById('np-name').focus();
}

async function createProject() {
  const data = {
    name: document.getElementById('np-name').value.trim(),
    description: document.getElementById('np-desc').value.trim(),
    git_path: document.getElementById('np-git').value.trim(),
    branch: document.getElementById('np-branch').value.trim() || 'main',
    priority: parseInt(document.getElementById('np-priority').value) || 0,
    owner: document.getElementById('np-owner').value.trim(),
  };
  if (!data.name) { toast('Name is required', 'error'); return; }
  try {
    await api('/projects', { method: 'POST', body: JSON.stringify(data) });
    toast('Project created');
    document.getElementById('modal-new-project').style.display = 'none';
    ['np-name', 'np-desc', 'np-git', 'np-owner'].forEach(id => document.getElementById(id).value = '');
    loadProjects();
  } catch (e) { toast(e.message, 'error'); }
}

function showNewTaskModal(projectId) {
  const title = prompt('Task title:');
  if (!title) return;
  const desc = prompt('Description (optional):') || '';
  const priority = parseInt(prompt('Priority (0-5):') || '0');
  api('/tasks', { method: 'POST', body: JSON.stringify({ project_id: projectId, title: title, description: desc, priority: priority, status: 'pending' }) })
    .then(() => { toast('Task created'); selectProject(projectId); })
    .catch(e => toast(e.message, 'error'));
}

async function pauseProject(id) {
  try { await api('/projects/' + id + '/pause', { method: 'POST' }); toast('Project paused'); loadProjects(); } catch (e) { toast(e.message, 'error'); }
}

async function resumeProject(id) {
  try { await api('/projects/' + id + '/resume', { method: 'POST' }); toast('Project resumed'); loadProjects(); } catch (e) { toast(e.message, 'error'); }
}

async function toggleFavorite(id) {
  try { await api('/projects/' + id + '/favorite', { method: 'POST' }); loadProjects(); } catch (e) { toast(e.message, 'error'); }
}

async function deleteProject(id) {
  if (!confirm('Delete this project and all its tasks?')) return;
  try {
    await api('/projects/' + id, { method: 'DELETE' });
    toast('Project deleted');
    if (currentProjectId === id) {
      currentProjectId = null;
      document.getElementById('project-empty').style.display = 'flex';
      document.getElementById('project-detail').style.display = 'none';
    }
    loadProjects();
  } catch (e) { toast(e.message, 'error'); }
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
  const btn = document.querySelector('#page-chat .btn-primary');
  const content = input.value.trim();
  if (!agentID) { toast('Select an agent', 'error'); return; }
  if (!content) return;

  // Show user message immediately
  const container = document.getElementById('chat-messages');
  container.innerHTML += '<div class="chat-msg user"><div class="chat-bubble">' + escapeHTML(content) + '</div></div>';
  container.scrollTop = container.scrollHeight;

  // Show loading indicator
  container.innerHTML += '<div id="chat-loading" class="chat-msg assistant"><div class="chat-bubble" style="opacity:0.6"><em>Thinking...</em></div></div>';
  container.scrollTop = container.scrollHeight;

  input.value = '';
  btn.disabled = true;
  btn.textContent = 'Waiting...';

  try {
    await api('/chat/' + agentID, { method: 'POST', body: JSON.stringify({ content }) });
    loadChat();
  } catch (e) {
    // Remove loading indicator on error
    const loading = document.getElementById('chat-loading');
    if (loading) loading.remove();
    toast(e.message, 'error');
  } finally {
    btn.disabled = false;
    btn.textContent = 'Send';
  }
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

/**
 * OmniRAG Agent UI — app.js
 * Conversational chat with Web / My Library source toggles, conversation memory,
 * and a past-conversations sidebar. JSON request/response (SSE deferred).
 */

'use strict';

// ─── Lucide inline icons (stroke-based, 16-18px, stroke-width 2.2) ───────────────
const ICONS = {
  user: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="4"/><path d="M4 20c0-4 3.6-7 8-7s8 3 8 7"/></svg>',
  agent: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 8V4H8"/><rect width="16" height="12" x="4" y="8" rx="2"/><path d="M2 14h2"/><path d="M20 14h2"/><path d="M15 13v2"/><path d="M9 13v2"/></svg>',
  helpCircle: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="12" r="10"/><path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/><path d="M12 17h.01"/></svg>',
  book: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5v-15A2.5 2.5 0 0 1 6.5 2H20v20H6.5a2.5 2.5 0 0 1 0-5H20"/></svg>',
  externalLink: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/></svg>',
  database: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M3 5V19A9 3 0 0 0 21 19V5"/><path d="M3 12A9 3 0 0 0 21 12"/></svg>',
  msg: '<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>',
  trash: '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M3 6h18"/><path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/></svg>',
  loader: '<svg class="spin" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>',
  arrowUp: '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="m5 12 7-7 7 7"/><path d="M12 19V5"/></svg>',
};

const SOURCE_META = {
  pdf: { label: 'My Library', icon: ICONS.book },
  web: { label: 'Web', icon: ICONS.externalLink },
  mongo: { label: 'Database', icon: ICONS.database },
  model: { label: "the model's own knowledge", icon: ICONS.agent },
};

// ─── DOM refs ──────────────────────────────────────────────────────────────────
const agentInput = document.getElementById('agent-input');
const agentBtn = document.getElementById('agent-btn');
const btnIcon = document.getElementById('btn-icon');
const thread = document.getElementById('thread');
const hero = document.getElementById('hero');
const errorBanner = document.getElementById('error-banner');
const errorText = document.getElementById('error-text');
const newChatBtn = document.getElementById('new-chat-btn');
const toggleWeb = document.getElementById('toggle-web');
const toggleLibrary = document.getElementById('toggle-library');
const convoList = document.getElementById('convo-list');
const convoEmpty = document.getElementById('convo-empty');

// ─── State ───────────────────────────────────────────────────────────────────
let isLoading = false;
let conversationId = '';
let useWeb = false;
let useLibrary = false;

// ─── Toggle handling ───────────────────────────────────────────────────────────
function paintToggle(btn, on) {
  btn.dataset.on = String(on);
  if (on) {
    btn.classList.add('border-emerald-500', 'bg-emerald-500/15', 'text-emerald-300');
    btn.classList.remove('border-slate-700', 'text-slate-300');
  } else {
    btn.classList.remove('border-emerald-500', 'bg-emerald-500/15', 'text-emerald-300');
    btn.classList.add('border-slate-700', 'text-slate-300');
  }
}
toggleWeb.addEventListener('click', () => { useWeb = !useWeb; paintToggle(toggleWeb, useWeb); });
toggleLibrary.addEventListener('click', () => { useLibrary = !useLibrary; paintToggle(toggleLibrary, useLibrary); });

// ─── Button / loading state ──────────────────────────────────────────────────
function setLoading(on) {
  isLoading = on;
  agentBtn.disabled = on;
  btnIcon.innerHTML = on ? ICONS.loader : ICONS.arrowUp;
}

// ─── Error banner ──────────────────────────────────────────────────────────────
function showError(message) { errorText.textContent = message; errorBanner.classList.remove('hidden'); }
function hideError() { errorBanner.classList.add('hidden'); errorText.textContent = ''; }

// ─── Bubble builders ─────────────────────────────────────────────────────────
function userBubble(text) {
  const el = document.createElement('div');
  el.className = 'flex items-start gap-3 animate-slideUp';
  el.innerHTML =
    '<div class="mt-0.5 flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full border border-slate-700 bg-slate-900 text-slate-400">' + ICONS.user + '</div>' +
    '<div class="flex-1 rounded-2xl border border-slate-800 bg-slate-900/40 px-4 py-3"><p class="text-sm font-medium leading-relaxed text-white"></p></div>';
  el.querySelector('p').textContent = text;
  return el;
}

function sourceTags(sources) {
  const list = (sources && sources.length) ? sources : ['model'];
  const tags = list.map(s => {
    const m = SOURCE_META[s];
    if (!m) return '';
    return '<span class="inline-flex items-center gap-1 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-2 py-0.5 text-[11px] font-medium text-emerald-300">' +
      m.icon + 'from ' + m.label + '</span>';
  }).join('');
  return '<div class="mt-3 flex flex-wrap gap-2">' + tags + '</div>';
}

function citationLinks(citations) {
  if (!citations || !citations.length) return '';
  const link = '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"><path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/></svg>';
  const chips = citations.map(c => {
    let host = c.url;
    try { host = new URL(c.url).hostname.replace(/^www\./, ''); } catch (e) {}
    const tip = (c.title || c.url).replace(/"/g, '&quot;');
    return '<a href="' + c.url + '" target="_blank" rel="noopener noreferrer" title="' + tip + '" ' +
      'class="inline-flex max-w-[240px] items-center gap-1 truncate rounded-full border border-sky-500/30 bg-sky-500/10 px-2 py-0.5 text-[11px] font-medium text-sky-300 transition hover:bg-sky-500/20 hover:text-sky-200">' +
      link + host + '</a>';
  }).join('');
  return '<div class="mt-2 flex flex-wrap gap-2"><span class="text-[11px] text-slate-500">Sources:</span>' + chips + '</div>';
}

function agentBubble(answer, sources, needsClarification, citations) {
  const el = document.createElement('div');
  el.className = 'flex items-start gap-3 animate-slideUp';
  const accent = needsClarification ? '#f59e0b' : '#10b981';
  const ring = needsClarification ? 'border-amber-500/40 bg-amber-500/15' : 'border-emerald-500/40 bg-emerald-500/20';
  const headerIcon = needsClarification ? ICONS.helpCircle : ICONS.agent;
  const headerText = needsClarification ? 'Needs your input' : 'Agent';
  const headerColor = needsClarification ? 'text-amber-400' : 'text-emerald-400';
  el.innerHTML =
    '<div class="mt-0.5 flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full border ' + ring + '" style="color:' + accent + '">' + headerIcon + '</div>' +
    '<div class="min-w-0 flex-1 rounded-2xl border border-slate-800 bg-slate-900/50 px-4 py-3 shadow-glow">' +
      '<p class="mb-2 text-xs font-semibold ' + headerColor + '">' + headerText + '</p>' +
      '<div class="answer-text text-sm text-slate-200"></div>' +
      sourceTags(sources) +
      citationLinks(citations) +
    '</div>';
  el.querySelector('.answer-text').textContent = answer;
  return el;
}

function appendToThread(node) {
  if (hero && !hero.classList.contains('hidden')) hero.classList.add('hidden');
  thread.appendChild(node);
  thread.scrollTop = thread.scrollHeight;
}

// ─── Main agent handler ──────────────────────────────────────────────────────
async function runAgent() {
  const query = agentInput.value.trim();
  if (!query || isLoading) return;

  hideError();
  setLoading(true);
  appendToThread(userBubble(query));
  agentInput.value = '';

  // Placeholder agent bubble while loading.
  const loadingLabel = useLibrary ? 'Searching My Library…' : (useWeb ? 'Searching the web…' : 'Thinking…');
  const placeholder = agentBubble(loadingLabel, [], false);
  appendToThread(placeholder);

  try {
    const resp = await fetch('/api/agent/query', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        query,
        conversation_id: conversationId,
        use_web: useWeb,
        use_library: useLibrary,
      }),
    });

    if (!resp.ok) throw new Error(await resp.text());
    const data = await resp.json();

    conversationId = data.conversation_id || conversationId;

    const finished = agentBubble(data.answer, data.sources, data.needs_clarification, data.citations);
    thread.replaceChild(finished, placeholder);
    thread.scrollTop = thread.scrollHeight;

    loadConversations();
  } catch (err) {
    placeholder.remove();
    showError(err.message || 'An error occurred');
    console.error('[Agent] error:', err);
  } finally {
    setLoading(false);
    agentInput.focus();
  }
}

// ─── New chat ──────────────────────────────────────────────────────────────────
function newChat() {
  conversationId = '';
  thread.querySelectorAll('.animate-slideUp').forEach(n => n.remove());
  if (hero) hero.classList.remove('hidden');
  hideError();
  agentInput.focus();
}

// ─── Sidebar ───────────────────────────────────────────────────────────────────
async function loadConversations() {
  try {
    const resp = await fetch('/api/conversations');
    if (!resp.ok) return;
    const convos = await resp.json();
    renderConversations(convos || []);
  } catch (err) {
    console.error('[Sidebar] load error:', err);
  }
}

function renderConversations(convos) {
  convoList.innerHTML = '';
  if (!convos.length) {
    convoList.appendChild(convoEmpty);
    convoEmpty.classList.remove('hidden');
    return;
  }
  convos.forEach(c => {
    const item = document.createElement('div');
    const active = c.id === conversationId;
    item.className = 'group flex items-center gap-2 rounded-lg border px-2.5 py-2 text-sm transition cursor-pointer ' +
      (active ? 'border-emerald-500/50 bg-emerald-500/10 text-emerald-200' : 'border-transparent text-slate-300 hover:bg-slate-900');
    item.innerHTML =
      '<span class="flex-shrink-0 text-slate-500">' + ICONS.msg + '</span>' +
      '<span class="flex-1 truncate"></span>' +
      '<button class="del-btn flex-shrink-0 text-slate-600 opacity-0 transition group-hover:opacity-100 hover:text-red-400" title="Delete">' + ICONS.trash + '</button>';
    item.querySelector('span.flex-1').textContent = c.title || 'Untitled';
    item.addEventListener('click', () => openConversation(c.id));
    item.querySelector('.del-btn').addEventListener('click', (e) => { e.stopPropagation(); deleteConversation(c.id); });
    convoList.appendChild(item);
  });
}

async function openConversation(id) {
  try {
    const resp = await fetch('/api/conversations/' + encodeURIComponent(id));
    if (!resp.ok) throw new Error(await resp.text());
    const data = await resp.json();

    conversationId = id;
    thread.querySelectorAll('.animate-slideUp').forEach(n => n.remove());
    if (hero) hero.classList.add('hidden');

    (data.messages || []).forEach(m => {
      if (m.role === 'user') {
        appendToThread(userBubble(m.content));
      } else {
        appendToThread(agentBubble(m.content, m.sources, false, m.citations));
      }
    });
    loadConversations();
    agentInput.focus();
  } catch (err) {
    showError(err.message || 'Could not open conversation');
  }
}

async function deleteConversation(id) {
  try {
    const resp = await fetch('/api/conversations/' + encodeURIComponent(id), { method: 'DELETE' });
    if (!resp.ok && resp.status !== 204) throw new Error(await resp.text());
    if (id === conversationId) newChat();
    loadConversations();
  } catch (err) {
    showError(err.message || 'Could not delete conversation');
  }
}

// ─── Bindings ────────────────────────────────────────────────────────────────
agentBtn.addEventListener('click', runAgent);
newChatBtn.addEventListener('click', newChat);
agentInput.addEventListener('keydown', e => {
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) { e.preventDefault(); runAgent(); }
});
agentInput.addEventListener('input', () => {
  agentInput.style.height = 'auto';
  agentInput.style.height = Math.min(agentInput.scrollHeight, 160) + 'px';
});

// ─── Init ────────────────────────────────────────────────────────────────────
paintToggle(toggleWeb, useWeb);
paintToggle(toggleLibrary, useLibrary);
loadConversations();
agentInput.focus();


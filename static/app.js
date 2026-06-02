/**
 * OmniRAG Agent UI — app.js
 * Handles agent queries, displays reasoning results with streaming support.
 */

'use strict';

// ─── DOM refs ──────────────────────────────────────────────────────────────────
const agentInput = document.getElementById('agent-input');
const agentBtn = document.getElementById('agent-btn');
const btnIcon = document.getElementById('btn-icon');
const btnLabel = document.getElementById('btn-label');
const reasoning = document.getElementById('reasoning');
const stepsList = document.getElementById('steps-list');
const resultsEl = document.getElementById('results');
const errorBanner = document.getElementById('error-banner');
const errorText = document.getElementById('error-text');
const hero = document.getElementById('hero');

// ─── State ────────────────────────────────────────────────────────────────────
let isLoading = false;

// ─── Button state helpers ─────────────────────────────────────────────────────
function setLoading(on) {
  isLoading = on;
  agentBtn.disabled = on;

  if (on) {
    btnIcon.innerHTML = '<svg class="w-4 h-4 animate-spin" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg>';
    btnLabel.textContent = 'Reasoning…';
  } else {
    btnIcon.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><path d="m21 21-4.35-4.35"/></svg>';
    btnLabel.textContent = 'Ask Agent';
  }
}

// ─── Error banner ─────────────────────────────────────────────────────────────
function showError(message) {
  errorText.textContent = message;
  errorBanner.classList.remove('hidden');
}

function hideError() {
  errorBanner.classList.add('hidden');
  errorText.textContent = '';
}

// ─── Chat card creation ───────────────────────────────────────────────────────
function createAnswerCard(query) {
  const card = document.createElement('div');
  card.className = 'chat-entry rounded-2xl border border-slate-800 bg-slate-900/50 p-5 shadow-glow';

  const html = '<div class="pb-4 border-b border-slate-800"><div class="flex items-start gap-3"><div class="w-7 h-7 rounded-full bg-slate-900 border border-slate-700 flex items-center justify-center flex-shrink-0 mt-0.5"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#94a3b8" stroke-width="2.2" stroke-linecap="round"><circle cx="12" cy="8" r="4"/><path d="M4 20c0-4 3.6-7 8-7s8 3 8 7"/></svg></div><p class="text-sm text-white font-medium leading-relaxed query-text"></p></div></div><div class="pt-4"><div class="flex items-start gap-3"><div class="w-7 h-7 rounded-full bg-emerald-500/20 border border-emerald-500/40 flex items-center justify-center flex-shrink-0 mt-0.5"><svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="#10b981" stroke-width="2.2" stroke-linecap="round"><path d="M12 2a7 7 0 0 1 7 7c0 5-7 13-7 13S5 14 5 9a7 7 0 0 1 7-7z"/><circle cx="12" cy="9" r="2.5"/></svg></div><div class="flex-1 min-w-0"><p class="text-xs font-semibold text-emerald-400 mb-2">Agent Answer</p><div id="answer-text" class="text-sm text-slate-200 cursor"></div></div></div></div>';

  card.innerHTML = html;
  card.querySelector('.query-text').textContent = query;
  resultsEl.prepend(card);
  return {
    el: card,
    textEl: card.querySelector('#answer-text'),
  };
}

// ─── Main agent handler ───────────────────────────────────────────────────────
async function runAgent() {
  const query = agentInput.value.trim();
  if (!query || isLoading) return;

  hideError();
  setLoading(true);
  hero.classList.add('hidden');
  reasoning.classList.remove('hidden');
  stepsList.innerHTML = '';
  resultsEl.innerHTML = '';

  const card = createAnswerCard(query);
  let fullAnswer = '';

  try {
    const resp = await fetch('/api/agent/query', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ query }),
    });

    if (!resp.ok) {
      const err = await resp.text();
      throw new Error(err);
    }

    const data = await resp.json();
    fullAnswer = data.answer;

    card.textEl.textContent = fullAnswer;
    card.textEl.classList.remove('cursor');

    stepsList.innerHTML = '<div class="text-xs text-slate-500">ReAct reasoning complete</div>';

  } catch (err) {
    showError(err.message || 'An error occurred');
    if (card) card.textEl.classList.remove('cursor');
    console.error('[Agent] error:', err);
  } finally {
    setLoading(false);
    agentInput.value = '';
    agentInput.focus();
    agentInput.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
  }
}

// ─── Input bindings ───────────────────────────────────────────────────────────
agentBtn.addEventListener('click', runAgent);

agentInput.addEventListener('keydown', e => {
  // Cmd+Enter / Ctrl+Enter to submit
  if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
    e.preventDefault();
    runAgent();
  }
});

// ─── Init ─────────────────────────────────────────────────────────────────────
agentInput.focus();

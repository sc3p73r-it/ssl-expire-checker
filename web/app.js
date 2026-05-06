import { createClient } from 'https://esm.sh/@supabase/supabase-js@2.49.1';

let supabase = null;
let expiryThreshold = 15;
const runtimeCfg = window.__SSL_CHECKER_CONFIG__ || {};
const API_BASE = (runtimeCfg.API_BASE_URL || '').replace(/\/+$/, '');

const $ = (id) => document.getElementById(id);

function show(el, visible) {
  el.classList.toggle('hidden', !visible);
}

function setErr(el, msg) {
  if (!msg) {
    el.classList.add('hidden');
    el.textContent = '';
    return;
 }
  el.textContent = msg;
  el.classList.remove('hidden');
}

async function api(path, options = {}) {
  const { data: { session } } = await supabase.auth.getSession();
  if (!session?.access_token) {
    throw new Error('Not signed in');
  }
  const headers = {
    Authorization: `Bearer ${session.access_token}`,
    ...(options.headers || {}),
  };
  if (options.body != null) {
    headers['Content-Type'] = 'application/json';
  }
  const url = `${API_BASE}${path}`;
  const res = await fetch(url, { ...options, headers });
  const text = await res.text();
  let body = null;
  try {
    body = text ? JSON.parse(text) : null;
  } catch {
    body = text;
  }
  if (!res.ok) {
    const msg = (body && body.error) || text || res.statusText;
    throw new Error(msg);
  }
  return body;
}

function badgeClass(status) {
  switch (status) {
    case 'valid':
      return 'bg-green-100 text-green-800';
    case 'expiring':
      return 'bg-yellow-100 text-yellow-800';
    case 'expired':
    case 'error':
      return 'bg-red-100 text-red-800';
    default:
      return 'bg-slate-100 text-slate-800';
  }
}

function daysLeft(expiry) {
  if (!expiry) return '—';
  const now = Date.now();
  const t = new Date(expiry).getTime();
  const d = Math.ceil((t - now) / (1000 * 60 * 60 * 24));
  if (Number.isNaN(d)) return '—';
  return String(d);
}

function fmtTs(ts) {
  if (!ts) return '—';
  try {
    return new Date(ts).toLocaleString();
  } catch {
    return ts;
  }
}

async function loadDomains() {
  setErr($('dash-error'), '');
  const data = await api('/api/domains');
  const rows = data.domains || [];
  const tb = $('tbody-domains');
  tb.innerHTML = '';
  for (const d of rows) {
    const tr = document.createElement('tr');
    tr.className = 'border-b border-slate-800/80';
    tr.innerHTML = `
      <td class="py-2 pr-4 font-mono text-xs text-slate-200">${d.url}</td>
      <td class="py-2 pr-4 text-xs text-slate-400">${d.issuer || '—'}</td>
      <td class="py-2 pr-4 text-xs text-slate-300">${d.expiry_date ? new Date(d.expiry_date).toLocaleString() : '—'}</td>
      <td class="py-2 pr-4 text-xs text-slate-300">${daysLeft(d.expiry_date)}</td>
      <td class="py-2 pr-4">
        <span class="inline-flex rounded-full px-2 py-0.5 text-xs font-semibold ${badgeClass(d.status)}">${d.status}</span>
      </td>
      <td class="py-2 pr-4 text-xs text-slate-400">${fmtTs(d.last_checked)}</td>
      <td class="py-2">
        <button data-scan="${d.id}" class="mr-2 rounded-md border border-slate-600 px-2 py-1 text-xs text-slate-100 hover:bg-slate-800">Rescan</button>
        <button data-del="${d.id}" class="rounded-md border border-red-900/60 px-2 py-1 text-xs text-red-300 hover:bg-red-950/40">Delete</button>
      </td>`;
    tb.appendChild(tr);
  }

  tb.querySelectorAll('button[data-scan]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      try {
        await api(`/api/domains/${btn.dataset.scan}/scan`, { method: 'POST' });
        await loadDomains();
      } catch (e) {
        setErr($('dash-error'), e.message);
      }
    });
  });
  tb.querySelectorAll('button[data-del]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      if (!confirm('Delete this domain?')) return;
      try {
        await api(`/api/domains/${btn.dataset.del}`, { method: 'DELETE' });
        await loadDomains();
      } catch (e) {
        setErr($('dash-error'), e.message);
      }
    });
  });
}

function showDashboard() {
  show($('view-auth'), false);
  show($('view-dash'), true);
  show($('auth-actions'), true);
  $('threshold-hint').textContent = `Yellow badge: certificate expires within ${expiryThreshold} days.`;
}

function showAuth() {
  show($('view-dash'), false);
  show($('view-auth'), true);
  show($('auth-actions'), false);
}

async function bootstrap() {
  const cfg = await resolvePublicConfig();
  expiryThreshold = cfg.expiryThreshold ?? 15;
  supabase = createClient(cfg.supabaseUrl, cfg.publishableKey, {
    auth: { persistSession: true, autoRefreshToken: true },
  });

  supabase.auth.onAuthStateChange((_event, session) => {
    if (session) {
      showDashboard();
      loadDomains().catch((e) => setErr($('dash-error'), e.message));
    } else {
      showAuth();
    }
  });

  const { data: { session } } = await supabase.auth.getSession();
  if (session) {
    showDashboard();
    await loadDomains();
  } else {
    showAuth();
  }

  $('form-auth').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    setErr($('auth-error'), '');
    const email = $('input-email').value.trim();
    const password = $('input-password').value;
    const { error } = await supabase.auth.signInWithPassword({ email, password });
    if (error) setErr($('auth-error'), error.message);
  });

  $('btn-signup').addEventListener('click', async () => {
    setErr($('auth-error'), '');
    const email = $('input-email').value.trim();
    const password = $('input-password').value;
    const { error } = await supabase.auth.signUp({ email, password });
    if (error) setErr($('auth-error'), error.message);
    else setErr($('auth-error'), 'Check your email to confirm signup (if confirmations are enabled).');
    $('auth-error').classList.remove('hidden');
  });

  $('btn-logout').addEventListener('click', async () => {
    await supabase.auth.signOut();
    showAuth();
  });

  $('form-add').addEventListener('submit', async (ev) => {
    ev.preventDefault();
    setErr($('add-error'), '');
    const url = $('input-url').value.trim();
    try {
      await api('/api/domains', { method: 'POST', body: JSON.stringify({ url }) });
      $('input-url').value = '';
      await loadDomains();
    } catch (e) {
      setErr($('add-error'), e.message);
    }
  });

  $('btn-scan-all').addEventListener('click', async () => {
    try {
      setErr($('dash-error'), '');
      await api('/api/scan-all', { method: 'POST' });
      await loadDomains();
    } catch (e) {
      setErr($('dash-error'), e.message);
    }
  });
}

bootstrap().catch((e) => {
  console.error(e);
  alert(`Startup failed: ${e.message}`);
});

async function resolvePublicConfig() {
  if (runtimeCfg.SUPABASE_URL && runtimeCfg.SUPABASE_PUBLISHABLE_KEY) {
    return {
      supabaseUrl: runtimeCfg.SUPABASE_URL,
      publishableKey: runtimeCfg.SUPABASE_PUBLISHABLE_KEY,
      expiryThreshold: 15,
    };
  }
  const cfgUrl = `${API_BASE}/api/config`;
  const res = await fetch(cfgUrl);
  if (!res.ok) {
    throw new Error('Unable to load public config. Set web/config.js for static hosting.');
  }
  return res.json();
}

// app.js — desktop GUI renderer. All logic lives in Go (tuiModel);
// this only binds DOM events to bound methods and renders strings.
// Wails injects runtime.js + ipc.js before this file runs, exposing
// window.go.main.DesktopApp.<Method>.

const backend = window.go.main.DesktopApp;
const $ = (id) => document.getElementById(id);

function renderRows(listEl, rows) {
  listEl.innerHTML = '';
  if (!rows || rows.length === 0) {
    const li = document.createElement('li');
    li.textContent = '(empty)';
    li.className = 'empty';
    listEl.appendChild(li);
    return;
  }
  for (const row of rows) {
    const li = document.createElement('li');
    li.textContent = row;
    listEl.appendChild(li);
  }
}

function showError(el, e) {
  el.textContent = 'error: ' + e;
}

// ---- tabs ----
document.querySelectorAll('.nav-btn[data-tab]').forEach((btn) => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.nav-btn').forEach((b) => b.classList.remove('active'));
    btn.classList.add('active');
    document.querySelectorAll('.tab').forEach((t) => t.classList.remove('active'));
    const tab = document.getElementById('tab-' + btn.dataset.tab);
    if (tab) tab.classList.add('active');
    if (btn.dataset.tab === 'queue') refreshQueue();
    if (btn.dataset.tab === 'bounces') refreshBounces();
    if (btn.dataset.tab === 'dkim') refreshDKIMs();
    if (btn.dataset.tab === 'donate') renderDonate();
  });
});

async function refreshQueue() {
  try {
    const v = await backend.Queue();
    $('queue-summary').textContent = v.summary;
    renderRows($('queue-list'), v.rows);
  } catch (e) {
    showError($('queue-summary'), e);
  }
}

async function refreshBounces() {
  try {
    const v = await backend.Bounces();
    $('bounce-summary').textContent = v.summary;
    renderRows($('bounce-list'), v.rows);
  } catch (e) {
    showError($('bounce-summary'), e);
  }
}

async function refreshDKIMs() {
  try {
    const v = await backend.DKIMs();
    $('dkim-summary').textContent = v.summary;
    renderRows($('dkim-list'), v.rows);
  } catch (e) {
    showError($('dkim-summary'), e);
  }
}

// ---- forms ----
$('dkim-form').addEventListener('submit', async (ev) => {
  ev.preventDefault();
  const domain = $('dkim-form').domain.value.trim();
  const selector = $('dkim-form').selector.value.trim();
  const algorithm = $('dkim-form').algorithm.value;
  if (!domain || !selector) {
    showError($('dkim-result'), 'domain and selector are required');
    return;
  }
  try {
    const rec = await backend.DKIMGenerate(domain, selector, algorithm);
    $('dkim-result').textContent =
      'Publish this TXT record:\n' + rec +
      '\n\nPrivate key saved next to the db file (never shown here).';
    refreshDKIMs();
  } catch (e) {
    showError($('dkim-result'), e);
  }
});

$('migrate-form').addEventListener('submit', async (ev) => {
  ev.preventDefault();
  const source = $('migrate-form').source.value;
  const input = $('migrate-input').value.trim();
  if (!input) {
    showError($('migrate-result'), 'export path is required — use Browse or paste the full path');
    return;
  }
  try {
    const out = await backend.Migrate(source, input);
    $('migrate-result').textContent = out;
  } catch (e) {
    showError($('migrate-result'), e);
  }
});

$('simulate-form').addEventListener('submit', async (ev) => {
  ev.preventDefault();
  const plan = $('simulate-input').value.trim();
  if (!plan) {
    showError($('simulate-result'), 'plan path is required — use Browse or paste the full path');
    return;
  }
  try {
    const out = await backend.Simulate(plan);
    $('simulate-result').textContent = out;
  } catch (e) {
    showError($('simulate-result'), e);
  }
});

// ---- file pickers (Go-side dialogs via bound methods) ----
$('migrate-browse').addEventListener('click', async () => {
  try {
    const p = await backend.PickExportPath();
    if (p) $('migrate-input').value = p;
  } catch (e) {
    showError($('migrate-result'), e);
  }
});

$('simulate-browse').addEventListener('click', async () => {
  try {
    const p = await backend.PickPlanPath();
    if (p) $('simulate-input').value = p;
  } catch (e) {
    showError($('simulate-result'), e);
  }
});

// ---- donate ----
async function renderDonate() {
  try {
    $('donate-text').textContent = await backend.Donate();
  } catch (e) {
    showError($('donate-text'), e);
  }
}

// Initial load: the queue tab is the home screen — populate it now,
// not on first click.
refreshQueue();



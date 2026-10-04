// wizard.js — the guided setup wizard modal. Step data and actions
// live in Go (wizard.go / wizard_interactive.go); this renders the
// fields per step, collects values on Run, and shows the result.

const wBackend = window.go.main.DesktopApp;
const w$ = (id) => document.getElementById(id);

let steps = [];
let current = 0;

function renderStep() {
  const s = steps[current];
  w$('wizard-title').textContent = s.title;
  w$('wizard-body').textContent = s.body;
  w$('wizard-progress').textContent = (current + 1) + ' / ' + steps.length;

  // fields
  const fieldsEl = w$('wizard-fields');
  fieldsEl.innerHTML = '';
  for (const f of s.fields || []) {
    const label = document.createElement('label');
    const span = document.createElement('span');
    span.textContent = f.label;
    label.appendChild(span);
    let input;
    if (f.kind === 'select') {
      input = document.createElement('select');
      for (const opt of f.options || []) {
        const o = document.createElement('option');
        o.value = opt;
        o.textContent = opt;
        input.appendChild(o);
      }
      input.value = f.value;
    } else {
      input = document.createElement('input');
      input.type = f.kind === 'path' ? 'text' : 'text';
      input.value = f.value;
      input.placeholder = f.hint || '';
    }
    input.dataset.fieldId = f.id;
    label.appendChild(input);
    fieldsEl.appendChild(label);
    if (f.id === 'input' && s.id === 'migrate') {
      const sampleBtn = document.createElement('button');
      sampleBtn.type = 'button';
      sampleBtn.textContent = 'Use sample export';
      sampleBtn.addEventListener('click', async () => {
        try {
          const p = await wBackend.SampleExportPath();
          input.value = p;
        } catch (e) {
          w$('wizard-output').textContent = 'error: ' + e;
          w$('wizard-output').classList.remove('hidden');
        }
      });
      fieldsEl.appendChild(sampleBtn);
    }
  }

  // output area
  const out = w$('wizard-output');
  out.classList.add('hidden');
  out.textContent = '';

  // Copy buttons on the donate step (the wizard's donate body carries
  // the same addresses; render Copy buttons below the body).
  const prevCopy = w$('wizard-fields').querySelector('.copy-row');
  if (prevCopy) prevCopy.remove();
  if (s.id === 'donate') {
    const row = document.createElement('div');
    row.className = 'copy-row';
    const addrs = (s.body.match(/(0x[a-fA-F0-9]{40}|bc1[a-z0-9]{20,})/g)) || [];
    for (const addr of addrs) {
      const isEth = addr.startsWith('0x');
      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = 'copy-btn';
      btn.textContent = isEth ? 'Copy ETH / USDC address' : 'Copy Bitcoin address';
      btn.addEventListener('click', async () => {
        try {
          await wBackend.CopyAddress(addr);
          out.textContent = 'Copied: ' + addr;
        } catch (e) {
          try {
            await navigator.clipboard.writeText(addr);
            out.textContent = 'Copied: ' + addr;
          } catch (e2) {
            out.textContent = 'Copy failed — copy manually:\n' + addr;
          }
        }
        out.classList.remove('hidden');
      });
      row.appendChild(btn);
    }
    w$('wizard-fields').appendChild(row);
  }

  // run button
  const run = w$('wizard-run');
  if (s.fields && s.fields.length > 0) {
    run.textContent = s.nextLabel;
    run.classList.remove('hidden');
  } else {
    run.classList.add('hidden');
  }
  w$('wizard-next').textContent = current === steps.length - 1 ? 'Finish' : 'Next';
}

function collectValues() {
  const values = {};
  for (const input of w$('wizard-fields').querySelectorAll('input, select')) {
    values[input.dataset.fieldId] = input.value;
  }
  return values;
}

async function startWizard() {
  steps = await wBackend.WizardSteps();
  current = 0;
  renderStep();
  w$('wizard-modal').classList.remove('hidden');
}

function closeWizard(markDone) {
  // ANY dismissal — Skip, Finish, Escape — marks the wizard seen, so
  // it never auto-opens again (a wizard that cannot be declined is
  // mandatory). It stays reopenable via the sidebar button.
  wBackend.WizardDone().catch(() => {});
  w$('wizard-modal').classList.add('hidden');
}

w$('wizard-btn').addEventListener('click', startWizard);

w$('wizard-run').addEventListener('click', async () => {
  const s = steps[current];
  const out = w$('wizard-output');
  try {
    const result = await wBackend.WizardRunStep(s.id, collectValues());
    out.textContent = result;
  } catch (e) {
    out.textContent = 'error: ' + e;
  }
  out.classList.remove('hidden');
});

w$('wizard-next').addEventListener('click', () => {
  if (current < steps.length - 1) {
    current += 1;
    renderStep();
  } else {
    closeWizard(true);
  }
});

w$('wizard-skip').addEventListener('click', () => closeWizard(false));

// Escape closes the wizard like Skip (never mandatory).
document.addEventListener('keydown', (ev) => {
  if (ev.key === 'Escape' && !w$('wizard-modal').classList.contains('hidden')) {
    closeWizard(false);
  }
});

// First run: open the wizard automatically once per database.
wBackend.WizardSeen().then((seen) => {
  if (!seen) startWizard();
});

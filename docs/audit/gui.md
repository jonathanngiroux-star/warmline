# GUI + TUI audit (Oct 2026) — Fyne→Wails conversion, interactive wizard

Scope: convert the desktop GUI from Fyne to Wails v2 (maintainer decision),
add the interactive setup wizard to BOTH the GUI and the TUI (every step
carries real input fields and runs the real operation on the user's
values), then audit both surfaces live — not by reading code, by driving
them.

## How the surfaces were driven (live, not simulated)

- **GUI**: built with `-tags "wails dev webkit2_41"` and
  `devserver=localhost:34115`; the Wails dev server serves the real
  frontend with websocket-IPC bindings, driven from a real browser
  (real DOM events, real binding calls, real SQLite db).
- **TUI**: the real binary under a PTY (terminal(pty=true) +
  process_manage write/poll); keystrokes drive the actual tview UI on a
  real TTY; the drawn screen is read from the PTY log.

## Conversion summary

| Surface | Before | After |
|---|---|---|
| Desktop GUI | Fyne v2 (cgo/OpenGL, 100+ Go deps) | Wails v2.16 (webkit2gtk-4.1 on Linux via `webkit2_41` tag; WebView2 syscalls on Windows — cgo-free) |
| Frontend | Fyne widgets | Vanilla JS embedded via `go:embed` — no npm, no bundler, no lockfile |
| Windows build | mingw cross-compile (cgo) | `CGO_ENABLED=0` cross-compile; PE subsystem 2 verified |
| Logic | tuiModel | tuiModel — unchanged; renderers stay thin |

Build tags (documented in README + desktop stub): Linux
`-tags "wails production webkit2_41"`; Windows
`-tags "wails production" -ldflags "-s -w -H windowsgui"`.

## The interactive wizard (both surfaces)

Six steps: welcome → migrate → simulate → dkim → serve → donate. Every
fielded step runs the real operation on the user's values:

1. **migrate** — source + export path → real `migrate --dry-run` report
   ("Use sample export" materializes the embedded fixture for users
   without an export yet)
2. **simulate** — all 7 plan fields (volume_start/target, ramp_days,
   days, bounce_rate, complaint_rate, ip_age) → real trajectory table
   with the not-a-guarantee disclaimer
3. **dkim** — domain/selector/algorithm → real keypair + TXT record;
   private key never shown, selector recorded in the store
4. **serve** — smtp/http/db addresses → live probe: boots the real
   mta.Server on the user's SMTP address, submits a probe message via
   real `net/smtp` dialogue, verifies it landed in the queue, cleans
   up. Throwaway probe db; the user's db is never touched; nothing is
   sent anywhere (no relay).
5. **donate** — both addresses byte-for-byte.

First-run: auto-opens once per database (`wizard_seen` setting);
reopenable any time (GUI sidebar button; TUI `w` hotkey + nav entry).

## Bugs found and fixed (live-driving, RED-first where pinnable)

1. **Wails needs `production` (or `dev`) tag** — a bare `-tags wails`
   build exits with "Wails applications will not build without the
   correct build tags." The desktop stub taught exactly that broken
   command. Fixed the stub to teach
   `-tags "wails production webkit2_41"` and pinned it in
   `launcher_test.go`.
2. **Arch has no webkit2gtk-4.0** (only 4.1); Wails' cgo default wants
   4.0. Fixed via the upstream `webkit2_41` build tag (documented).
3. **Wizard actions used `testdata/` relative paths** — dead outside a
   source checkout, which is exactly where a desktop binary runs. Fixed:
   fixtures embedded via `go:embed` (root `samples` package), byte-equal
   to the CI-gated files (pinned by test), plus `ParseBytes`/
   `LoadPlanBytes` entry points in the migrate/simulate packages.
4. **TUI wizard: nav buttons were unreachable by keyboard** — tview has
   no Tab traversal between sibling primitives in a Flex; the original
   body/form/output/navRow layout trapped focus. Fixed structurally:
   nav (Run/Prev/Next/Skip) became Form buttons (native Tab/Enter), and
   Enter on any input field runs the step directly.
5. **TUI wizard: focus stayed on the old page's form after
   SwitchToPage** — typed text went to hidden forms; Enter hit the wrong
   button. Fixed: `app.SetFocus(w.forms[i±1])` on every transition.
6. **GUI: queue view never populated on startup** (only on nav clicks).
   Fixed: `refreshQueue()` at load. DKIM list also refreshes after
   generate.
7. **TUI: section lists never refilled on entry** (stale data until
   restart) — the `refill` fields were empty stubs. Fixed: wired to the
   real per-list fill fns.
8. **`wails.Run` error was swallowed** (silent exit 1). Fixed: printed
   to stderr — this is how bug 1 was diagnosable at all.
9. **Skip was broken in both UIs (user report)** — the GUI's Skip
   button shipped with `class="hidden"` (invisible), and in BOTH UIs
   skipping did not persist `wizard_seen`, so the wizard auto-opened
   again on every launch — effectively mandatory. Fixed: the Skip
   button is visible on every GUI step; ANY dismissal (Skip, Finish,
   Escape) marks the wizard seen in both UIs (GUI `closeWizard` →
   `WizardDone`, TUI `finish()` → `wizardDone()`); reopening still
   works (sidebar button / `w` hotkey). Pinned by
   `wizard_skip_test.go`; live-verified over the dev-server websocket
   IPC (fresh → skip → seen; steps still 6) and under a PTY (skip →
   relaunch on the same db opens straight to the main TUI; `w`
   reopens the tour).

## Verified live (GUI, browser-driven over websocket IPC)

- All 15 bindings resolve and return real data (Queue/Bounces/DKIMs/
  Migrate/Simulate/DKIMGenerate/Donate/WizardSteps/WizardRunStep/
  WizardSeen/WizardDone/SampleExportPath/PickExportPath/PickPlanPath/
  Version).
- Wizard auto-opened on fresh db; tour completed: migrate on sample
  (real report), simulate (21-day trajectory + disclaimer), dkim on
  custom domain (real RSA key), serve probe on custom ports ("probe
  ok", queue verified, probe db removed), donate (addresses exact),
  Finish → `wizard_seen` persisted; reload does not re-open.
- Wizard-generated DKIM selector visible in the DKIM tab (state flows
  wizard → main UI).
- Error path: missing export file → error names the offending path.
- Empty states: bounces `(no bounces recorded)`; queue summary live.
- No guarantee copy anywhere in the simulate path (disclaimer present).
- No secrets in any UI string (DKIM private key never crosses a
  binding).

## Verified live (TUI, PTY-driven)

- Wizard auto-opens on fresh db under a real TTY.
- Keyboard path: Enter advances Welcome → migrate; typed path lands in
  the field; Enter runs the real dry-run — report rendered in the
  result pane (added/unmapped sections read off the drawn screen).
- Nav buttons (Run dry-run / Prev / Next / Skip tour) render and are
  keyboard-reachable as form buttons; dropdowns open on Enter (tview
  behavior).
- `q` quits cleanly; queue summary renders behind the wizard.

## Gates (all green at audit close)

- `gofmt -l .` clean; `go vet ./...` clean
- `go test ./...` — 12/12 packages ok (incl. 11 wizard tests: steps,
  copy rules, first-run persistence, interactive actions incl. the live
  SMTP probe, error-naming, embedded-fixture parity)
- `go test -race` on mta/serve/store ok
- Linux desktop build compiles (`wails production webkit2_41`, cgo)
- Windows cross-compile cgo-free, PE subsystem 2 (GUI), 18.4 MB
- CI: fyne jobs replaced by wails jobs (Linux apt webkit2gtk-4.1;
  Windows pure cross-compile, no mingw); wizard serve-probe gate added

## GUI + TUI full test suites (Oct 4, second audit pass)

Per request: copy buttons re-verified, full backend audit, and a real
GUI test suite driven through the live UI — mock data (the embedded
sample export + typed plan numbers + mock domains), zero hand-waving.

### Backend (coverage-driven audit)

- Coverage before: cmd 45.6%, serve 56.7%, relay 65.3%, store 70.8%
- **4 real bugs fixed — empty list methods returned nil → JSON `null`**
  (`ListMessages`, `ListBounces`, `ListComplaints`, `ListDKIMs`); JS
  clients crash on `.length` of null. All now return `[]`; pinned by
  `empty_lists_test.go` (marshals bytes as `[]`).
- Dead code removed: `serve.dbHandle`, `cmd.clipboardHint` (no callers).
- New coverage: `relay.splitHostPort` (IPv6 last-colon semantics, error
  names the addr), `serve.Submit` (per-recipient enqueue), `serve.Run`
  (live ephemeral-port bind + real SMTP dialogue in-process — the
  probe sockets are released before Run binds; retry-on-bind-race).

### GUI (in-app test bridge — the only way to drive a Wayland webview)

`testbridge.go` + `frontend/dist/testbridge.js` + `gui_driver.py` +
`gui_suite_full.py` (ported from the Proofspan reference; env prefix
`WARMLINE_GUI_*`). The bridge arms ONLY on `WARMLINE_GUI_TESTBRIDGE=1`
— production launches never poll. Interpreter is a fixed switch (no
eval).

**36/36 checks passed** against the real running app: wizard auto-open,
6-step walk with real operations (sample-export dry-run, typed-numbers
simulation, ed25519 DKIM on a mock domain, live SMTP probe), skip
visible + working + persisted, all tabs, error paths naming the
offending path, donate addresses byte-for-byte, empty states — each UI
claim cross-verified against the SQLite db and the REAL system
clipboard (Klipper).

### Dark-mode fix (user report: white dropdowns)

Native `<select>` popups ignored the dark theme. Root cause: the page
never declared `color-scheme`, so WebKitGTK rendered native form
controls light. Fixed: `color-scheme: dark` on `:root` + explicit dark
`option` rows. Pinned in the suite: computed color-scheme must be dark,
option rows must compute `rgb(30, 34, 41)`.

## Known non-bugs (tview/browser behavior, documented)

- Enter on a focused dropdown opens the option list (tview default).
- Wails' ErrorFormatter prefixes returned errors with "Error:", so the
  UI shows "error: Error: …" once — cosmetic; the offending value is
  still named.
- PTY-driven tab counts differ from interactive terminals; the
  Enter-on-field fix makes the keyboard path deterministic regardless.

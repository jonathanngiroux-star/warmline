#!/usr/bin/env python3
"""Warmline GUI full suite (in-app bridge; nothing mocked).

Covers: wizard auto-open + full 6-step walk with REAL operations on
mock data (the embedded sample export + the user's typed numbers), the
skip path, every main tab, DKIM generation with store cross-check,
donate copy buttons with clipboard cross-check, empty states, and
address bytes.

Server-side cross-verification: the suite reads the same SQLite db the
GUI writes (sqlite3 CLI) — a UI claim plus its persistence is the check.

Run (app must be launched with WARMLINE_GUI_TESTBRIDGE=1):
  python3 gui_driver.py run gui_suite_full.py
"""
import json
import os
import sqlite3
import subprocess
import sys
import tempfile
import time

# the GUI's db (the launcher passes WARMLINE_DB)
DB_PATH = os.environ.get("WARMLINE_GUI_TEST_DB", "/tmp/warmline-gui-test.db")
ETH = "0x85ee7E71f762d772599cbF1EC20E651B30657521"
BTC = "bc1qxe2zx5tv3hdreaej6s2x4p7han85uey828rrhg"

checks = []


def expect(label, cond, detail=""):
    ok = bool(cond)
    print(("PASS " if ok else "FAIL ") + label + (f" — {detail}" if detail and not ok else ""))
    checks.append(ok)
    return ok


def db_query(sql):
    """Read the GUI's SQLite db (cross-verification)."""
    try:
        con = sqlite3.connect(f"file:{DB_PATH}?mode=ro", uri=True, timeout=3)
        rows = con.execute(sql).fetchall()
        con.close()
        return rows
    except Exception as e:  # db may not exist yet
        return [("ERR", str(e))]


def clipboard_read():
    """Ground-truth clipboard via Klipper D-Bus (KDE)."""
    try:
        out = subprocess.run(
            ["gdbus", "call", "--session", "--dest", "org.kde.klipper",
             "--object-path", "/klipper", "--method", "org.kde.klipper.klipper.getClipboardContents"],
            capture_output=True, text=True, timeout=5)
        if out.returncode == 0 and out.stdout.startswith("("):
            # gdbus prints ('<contents>',) — extract the single string
            s = out.stdout.strip()
            if s.startswith("('") and s.endswith("',)"):
                return s[2:-3]
    except Exception:
        pass
    return None


def wait_for(gui, sel, want_substring, timeout=30, label=""):
    """Poll read(sel).text until it contains want_substring."""
    deadline = time.time() + timeout
    last = ""
    while time.time() < deadline:
        r = gui.read(sel)
        if r.get("ok"):
            last = (r.get("value") or {}).get("text") or ""
            if want_substring.lower() in last.lower():
                return last
        time.sleep(0.2)
    expect(f"wait: {label or sel} contains {want_substring!r}", False, f"last={last[-200:]!r}")
    return None


def suite(gui):
    # ---------- 0. bridge sanity ----------
    pong = gui.ping()
    expect("bridge ping", pong.get("value") == "pong")

    # ---------- 0b. dark mode everywhere (user report: white dropdowns) ----------
    # color-scheme: dark on :root is what makes NATIVE popups (select
    # dropdowns) render dark; option rows are styled explicitly too.
    any_select = gui.read("select")
    v = any_select.get("value") or {}
    if v.get("colorScheme") is not None:
        expect("page declares dark color-scheme (native popups dark)",
               "dark" in str(v.get("colorScheme")).lower(),
               f"colorScheme={v.get('colorScheme')!r}")
    else:
        print("SKIP color-scheme read (no select present)")
    n = gui.count("select")
    if (n.get("value") or 0) > 0:
        opt = gui.read("select option")
        ov = opt.get("value") or {}
        bg = ov.get("background")
        # rgb(30, 34, 41) = #1e2229 (var(--panel))
        expect("select option rows styled dark",
               bg and "rgb(30, 34, 41)" in str(bg), f"bg={bg!r}")
        sv = v
        if sv.get("background"):
            expect("select element background is dark theme",
                   "rgb(30, 34, 41)" in str(sv.get("background")), f"bg={sv.get('background')!r}")

    # ---------- 1. wizard auto-opens on a fresh db ----------
    r = gui.read("#wizard-modal")
    modal_open = "hidden" not in (r.get("value") or {}).get("cls", "")
    expect("wizard auto-opened on fresh db", modal_open)
    if not modal_open:
        # reopen it for the rest of the suite
        gui.click_text("Setup wizard")

    # welcome step
    t = gui.read("#wizard-title")
    expect("welcome step title", "Welcome" in (t.get("value") or {}).get("text", ""))

    # skip button is VISIBLE (the user-reported bug)
    r = gui.read("#wizard-skip")
    cls = (r.get("value") or {}).get("cls", "")
    expect("skip button visible on wizard", "hidden" not in cls, f"cls={cls!r}")

    # ---------- 2. wizard walk with REAL operations (mock data) ----------
    # step 1: migrate — use the sample export, run the real dry-run
    gui.click("#wizard-next")  # welcome -> migrate
    time.sleep(0.3)
    gui.click_text("Use sample export")
    time.sleep(0.5)
    gui.click("#wizard-run")
    out = wait_for(gui, "#wizard-output", "unmapped", label="migrate dry-run")
    if out:
        expect("migrate dry-run lists added section", "added" in out.lower())
        expect("migrate dry-run lists risks section", "risk" in out.lower())
        expect("dry-run honesty: nothing was changed", "never mutates" in out.lower() or "dry-run" in out.lower())

    # step 2: simulate — type the user's numbers, run the real engine
    gui.click("#wizard-next")  # -> simulate
    time.sleep(0.3)
    fields = {
        "input[data-field-id=volume_start]": "200",
        "input[data-field-id=volume_target]": "20000",
        "input[data-field-id=ramp_days]": "7",
        "input[data-field-id=days]": "10",
        "input[data-field-id=bounce_rate]": "1.5",
        "input[data-field-id=complaint_rate]": "0.05",
    }
    for sel, val in fields.items():
        gui.fill(sel, val)
    gui.select("#wizard-fields select[data-field-id=ip_age]", "aged")
    gui.click("#wizard-run")
    out = wait_for(gui, "#wizard-output", "Reputation trajectory", label="simulate")
    if out:
        expect("simulate output has the plan line", "200" in out and "20000" in out)
        expect("simulate output has the disclaimer", "Not a deliverability guarantee" in out)
        expect("simulate output shows declared ip_age", "aged" in out)

    # step 3: dkim — generate a real key for the mock domain
    gui.click("#wizard-next")  # -> dkim
    time.sleep(0.3)
    gui.fill("input[data-field-id=domain]", "mock-test.example")
    gui.fill("input[data-field-id=selector]", "guitest")
    gui.select("#wizard-fields select[data-field-id=algorithm]", "ed25519")
    gui.click("#wizard-run")
    out = wait_for(gui, "#wizard-output", "v=DKIM1", label="dkim generate")
    if out:
        expect("dkim record is ed25519 (user's choice)", "k=ed25519" in out)
        # PEM material must never appear; the UI *prose* mentioning the
        # words "Private key saved" is fine and expected.
        expect("dkim output carries no PEM private key material",
               "BEGIN PRIVATE KEY" not in out and "BEGIN RSA" not in out)

    # cross-verify: the selector must be in the db
    rows = db_query("SELECT selector, domain FROM dkims WHERE selector='guitest'")
    expect("dkim selector persisted to sqlite", rows and rows[0][1] == "mock-test.example", f"rows={rows}")

    # step 4: serve — the live probe on real ephemeral ports
    gui.click("#wizard-next")  # -> serve
    time.sleep(0.3)
    gui.fill("input[data-field-id=smtp]", "127.0.0.1:35257")
    gui.fill("input[data-field-id=http]", "127.0.0.1:38087")
    gui.click("#wizard-run")
    out = wait_for(gui, "#wizard-output", "probe ok", timeout=30, label="serve probe")
    if out:
        expect("probe says smtp accepted a message", "accepted a message" in out)
        expect("probe teaches the real command", "warmline serve" in out)

    # step 5: donate — addresses byte-for-byte + copy buttons
    gui.click("#wizard-next")  # -> donate
    time.sleep(0.3)
    body = gui.read("#wizard-body")
    btxt = (body.get("value") or {}).get("text", "")
    expect("wizard donate step shows ETH address", ETH in btxt)
    expect("wizard donate step shows BTC address", BTC in btxt)
    n = gui.count("#wizard-fields .copy-row button")
    expect("wizard donate step has 2 copy buttons", (n.get("value") or 0) == 2, f"n={n.get('value')}")

    # click the ETH copy button, then cross-check the REAL clipboard
    gui.click("#wizard-fields .copy-row button")  # first = ETH
    time.sleep(1.0)
    status = gui.read("#wizard-output")
    stxt = (status.get("value") or {}).get("text", "")
    expect("wizard copy reports the ETH address", ETH in stxt, f"status={stxt[:120]!r}")
    cb = clipboard_read()
    if cb is None:
        print("SKIP clipboard ground-truth (Klipper unavailable)")
    else:
        expect("clipboard holds the ETH address (ground truth)", cb == ETH, f"clipboard={cb!r}")

    # finish the tour -> wizard_seen must persist
    gui.click("#wizard-next")  # Finish
    time.sleep(0.8)
    r = gui.read("#wizard-modal")
    expect("wizard closes after Finish", "hidden" in (r.get("value") or {}).get("cls", ""))
    rows = db_query("SELECT value FROM settings WHERE key='wizard_seen'")
    expect("wizard_seen persisted to sqlite", bool(rows and rows[0][0]), f"rows={rows}")

    # ---------- 3. main tabs ----------
    gui.click_text("Queue", sel="#sidebar")
    q = wait_for(gui, "#queue-summary", "total", label="queue summary")
    if q:
        expect("queue summary is live from sqlite", "queued" in q)

    gui.click_text("Bounces", sel="#sidebar")
    time.sleep(0.4)
    b = gui.read("#bounce-list")
    expect("bounces empty state", "no bounces recorded" in ((b.get("value") or {}).get("text") or ""))

    gui.click_text("DKIM", sel="#sidebar")
    time.sleep(0.4)
    d = wait_for(gui, "#dkim-summary", "selector", label="dkim list")
    if d:
        dl = gui.read("#dkim-list")
        expect("dkim list shows the wizard-generated selector", "guitest" in ((dl.get("value") or {}).get("text") or ""))

    # dkim tab form: generate a second key via the tab (mock domain)
    gui.fill("#dkim-form input[name=domain]", "tab-test.example")
    gui.fill("#dkim-form input[name=selector]", "tabkey")
    gui.click_text("Generate DKIM key")
    out = wait_for(gui, "#dkim-result", "v=DKIM1", label="dkim tab generate")
    rows = db_query("SELECT COUNT(*) FROM dkims WHERE selector='tabkey'")
    expect("dkim tab generate persisted", rows and rows[0][0] == 1, f"rows={rows}")

    # migrate tab: error path names the offending path (mock bad file)
    gui.click_text("Migrate", sel="#sidebar")
    time.sleep(0.3)
    gui.fill("#migrate-input", "/nonexistent/mock-export.json")
    gui.click_text("Dry-run", sel="#tab-migrate")
    out = wait_for(gui, "#migrate-result", "/nonexistent/mock-export.json", label="migrate error")
    if out:
        expect("migrate error names the path", "/nonexistent/mock-export.json" in out)

    # simulate tab: real plan run via the tab form (mock plan values typed)
    gui.click_text("Simulate", sel="#sidebar")
    time.sleep(0.3)
    # write a mock plan to a temp file the GUI can read
    plan = {"volume_start": 100, "volume_target": 5000, "ramp_days": 5,
            "days": 7, "bounce_rate": 0.8, "complaint_rate": 0.02, "ip_age": "new"}
    plan_path = os.path.join(tempfile.gettempdir(), "warmline-mock-plan.json")
    with open(plan_path, "w") as f:
        json.dump(plan, f)
    gui.fill("#simulate-input", plan_path)
    gui.click_text("Simulate", sel="#tab-simulate")
    out = wait_for(gui, "#simulate-result", "Reputation trajectory", label="simulate tab")
    if out:
        expect("simulate tab output reflects the mock plan", "100" in out and "5000" in out)

    # donate tab: copy buttons + addresses
    gui.click_text("Donate", sel="#sidebar")
    time.sleep(0.5)
    dt = wait_for(gui, "#donate-text", ETH, label="donate text")
    if dt:
        expect("donate tab shows BTC address", BTC in dt)
    n = gui.count("#donate-buttons button")
    expect("donate tab has 2 copy buttons", (n.get("value") or 0) == 2, f"n={n.get('value')}")
    gui.click("#donate-buttons button")  # ETH
    time.sleep(1.0)
    cb = clipboard_read()
    if cb is not None:
        expect("clipboard holds ETH after donate-tab copy", cb == ETH, f"clipboard={cb!r}")

    # ---------- 4. reopen via sidebar; skip works and persists ----------
    gui.click_text("Setup wizard")
    time.sleep(0.5)
    r = gui.read("#wizard-modal")
    expect("wizard reopens via sidebar", "hidden" not in (r.get("value") or {}).get("cls", ""))
    gui.click("#wizard-skip")
    time.sleep(0.8)
    r = gui.read("#wizard-modal")
    expect("wizard closes on Skip", "hidden" in (r.get("value") or {}).get("cls", ""))

    # ---------- summary ----------
    passed = sum(1 for c in checks if c)
    print(f"\nSUITE: {passed}/{len(checks)} checks passed")
    return 0 if passed == len(checks) else 1

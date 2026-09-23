"""Capture the admin Codex tickets page in Chromium against fixed demo data.

Starts the Vite dev server on the project's fixed port 4174 (the config
refuses any other port; the script aborts if it is taken) for the duration of
the capture, intercepts every /api request
with deterministic fixtures (no backend, no credentials), and writes
screenshots, DOM observations and a hash-bound manifest into EVIDENCE_DIR.

Usage: python e2e_capture.py FRONTEND_DIR EVIDENCE_DIR SOURCE_IDENTITY
"""
import hashlib
import json
import os
from pathlib import Path
import socket
import subprocess
import sys
import time
from datetime import datetime, timezone
from urllib.parse import urlparse

from playwright.sync_api import sync_playwright

TICKET_STATE = "gAAAAA" + "B" * 286
PROXY_SECRET = "demo-proxy-secret"

SPEED = {
    "slow": dict(round_interval_seconds=45, probe_interval_seconds=5, attempt_timeout_seconds=25, cooldown_seconds=120, max_requests_per_round=4, max_proxy_attempts=2, max_requests_per_account_hour=30),
    "standard": dict(round_interval_seconds=20, probe_interval_seconds=2, attempt_timeout_seconds=25, cooldown_seconds=60, max_requests_per_round=8, max_proxy_attempts=3, max_requests_per_account_hour=60),
    "fast": dict(round_interval_seconds=10, probe_interval_seconds=1, attempt_timeout_seconds=20, cooldown_seconds=30, max_requests_per_round=16, max_proxy_attempts=3, max_requests_per_account_hour=90),
    "burst": dict(round_interval_seconds=5, probe_interval_seconds=0, attempt_timeout_seconds=15, cooldown_seconds=10, max_requests_per_round=30, max_proxy_attempts=5, max_requests_per_account_hour=180),
}
BOUNDS = {
    "round_interval_seconds": {"min": 5, "max": 600},
    "probe_interval_seconds": {"min": 0, "max": 60},
    "attempt_timeout_seconds": {"min": 5, "max": 60},
    "cooldown_seconds": {"min": 5, "max": 3600},
    "max_requests_per_round": {"min": 1, "max": 100},
    "max_proxy_attempts": {"min": 1, "max": 10},
    "max_requests_per_account_hour": {"min": 1, "max": 600},
}


def iso(offset_seconds=0):
    return datetime.fromtimestamp(time.time() + offset_seconds, timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def snapshot():
    return {
        "generated_at": iso(),
        "enabled": True,
        "fail_closed": True,
        "models": ["gpt-6-astra", "gpt-5.6-sol"],
        "target_length": 292,
        "controls": {"version": 1, "preset": "standard", "speed": SPEED["standard"], "proxy_ids": [11, 12, 19]},
        "configured": True,
        "presets": SPEED,
        "preset_order": ["slow", "standard", "fast", "burst"],
        "bounds": BOUNDS,
        "pool": [
            {"proxy_id": 11, "name": "住宅 SG-1", "usable": True},
            {"proxy_id": 12, "name": "住宅 JP-2", "usable": True},
            {"proxy_id": 19, "name": "旧节点 HK-9", "usable": False, "reason": "inactive"},
        ],
        "runtime": {"running": False, "requests_used": 3, "request_budget": 8, "last_round_at": iso(-12), "next_round_at": iso(8), "idle_reason": ""},
        "accounts": [
            {"id": 41, "name": "Pro 演示账号", "status": "active", "schedulable": True, "hour_used": 7, "hour_limit": 60,
             "tickets": [
                 {"model": "gpt-6-astra", "ready": True, "blocked": False, "length": 292, "remaining_seconds": 171, "age_seconds": 69, "proxy_name": "住宅 SG-1"},
                 {"model": "gpt-5.6-sol", "ready": False, "blocked": True, "remaining_seconds": 0, "cooldown_until": iso(45)},
             ],
             "manual": {"account_id": 41, "models": ["gpt-6-astra"], "running": False, "started_at": iso(-300), "finished_at": iso(-280), "attempts": 2, "harvested": ["gpt-6-astra"], "result": "harvested"}},
            {"id": 42, "name": "Plus 备用账号", "status": "active", "schedulable": False, "hour_used": 0, "hour_limit": 60,
             "tickets": [
                 {"model": "gpt-6-astra", "ready": False, "blocked": True, "remaining_seconds": 0},
                 {"model": "gpt-5.6-sol", "ready": False, "blocked": True, "remaining_seconds": 0},
             ]},
        ],
        "events": [
            {"id": "e1", "at": iso(-310), "stage": "probe", "kind": "probe_miss", "account_id": 41, "account_name": "Pro 演示账号", "model": "gpt-6-astra", "proxy_id": 12, "proxy_name": "住宅 JP-2", "http_status": 200, "length": 312, "manual": True, "result": "invalid_state", "detail": "拿到降级票据（312 字节），已拒收，换代理重试"},
            {"id": "e2", "at": iso(-290), "stage": "probe", "kind": "probe_hit", "account_id": 41, "account_name": "Pro 演示账号", "model": "gpt-6-astra", "proxy_id": 11, "proxy_name": "住宅 SG-1", "http_status": 200, "length": 292, "accepted": True, "manual": True, "result": "success", "detail": "拿到合格门票（292 字节）"},
            {"id": "e3", "at": iso(-289), "stage": "ticket", "kind": "accept", "account_id": 41, "account_name": "Pro 演示账号", "model": "gpt-6-astra", "proxy_id": 11, "proxy_name": "住宅 SG-1", "length": 292, "accepted": True, "manual": True, "result": "stored", "detail": "合格门票 292 字节，已入库"},
            {"id": "e4", "at": iso(-60), "stage": "probe", "kind": "probe_miss", "account_id": 41, "account_name": "Pro 演示账号", "model": "gpt-5.6-sol", "proxy_id": 12, "proxy_name": "住宅 JP-2", "http_status": 429, "result": "rate_limited", "detail": "上游限流，45 秒后再试"},
        ],
    }


PROXIES = [
    {"id": 11, "name": "住宅 SG-1", "protocol": "http", "host": "mihomo", "port": 20011, "username": None, "status": "active", "account_count": 0,
     "latency_ms": 182, "ip_address": "203.0.113.21", "country": "新加坡", "city": "Singapore", "expires_at": None, "fallback_mode": "none", "expiry_warn_days": 7, "created_at": iso(-86400), "updated_at": iso(-3600)},
    {"id": 12, "name": "住宅 JP-2", "protocol": "socks5h", "host": "res.example.net", "port": 1080, "username": "demo", "password": PROXY_SECRET, "status": "active", "account_count": 0,
     "latency_ms": 240, "ip_address": "198.51.100.44", "country": "日本", "city": "Tokyo", "expires_at": None, "fallback_mode": "none", "expiry_warn_days": 7, "created_at": iso(-86400), "updated_at": iso(-3600)},
    {"id": 13, "name": "机房 US-3", "protocol": "http", "host": "mihomo", "port": 20013, "username": None, "status": "active", "account_count": 2,
     "expires_at": None, "fallback_mode": "none", "expiry_warn_days": 7, "created_at": iso(-86400), "updated_at": iso(-3600)},
]

NODES = {"items": [
    {"id": 1, "proxy_id": 11, "proxy_name": "住宅 SG-1", "account_id": 41, "account_name": "Pro 演示账号", "model": "gpt-6-astra", "successes": 5, "misses": 1, "network_errors": 0, "account_errors": 0,
     "consecutive_failures": 0, "last_success": iso(-289), "cooldown_until": None, "latency_ms": 2310, "last_result": "success", "updated_at": iso(-289)},
    {"id": 2, "proxy_id": 12, "proxy_name": "住宅 JP-2", "account_id": 41, "account_name": "Pro 演示账号", "model": "gpt-6-astra", "successes": 1, "misses": 3, "network_errors": 1, "account_errors": 1,
     "consecutive_failures": 2, "last_success": iso(-7200), "cooldown_until": iso(30), "latency_ms": 3120, "last_result": "invalid_state", "updated_at": iso(-310)},
], "total": 2, "page": 1, "page_size": 20, "pages": 1}

ADMIN_USER = {"id": 1, "username": "admin", "email": "admin@example.com", "role": "admin", "status": "active", "balance": 0, "concurrency": 5,
              "created_at": iso(-86400), "updated_at": iso(-86400)}


def ok(data):
    return json.dumps({"code": 0, "message": "success", "data": data})


VITE_PORT = 4174


def port_in_use(port):
    with socket.socket() as sock:
        return sock.connect_ex(("127.0.0.1", port)) == 0


def main():
    frontend, evidence, source_identity = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3]
    shots = evidence / "screenshots"
    shots.mkdir(parents=True, exist_ok=False)
    port = VITE_PORT
    if port_in_use(port):
        raise RuntimeError("port 4174 is already serving another frontend; stop it or reuse it deliberately")
    env = dict(os.environ, VITE_DEV_PROXY_TARGET="http://127.0.0.1:9", BROWSER="none")
    server = subprocess.Popen(["pnpm", "exec", "vite", "--host", "127.0.0.1"],
                              cwd=frontend, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    base = f"http://127.0.0.1:{port}"
    unmatched = set()
    observations = {"runtime_identity": f"vite-dev-127.0.0.1:{port}", "pages": []}
    manifest = {"schema_version": 1, "source_identity": source_identity, "runtime_identity": observations["runtime_identity"], "screenshots": []}
    try:
        deadline = time.time() + 90
        while time.time() < deadline:
            try:
                with socket.create_connection(("127.0.0.1", port), timeout=1):
                    break
            except OSError:
                time.sleep(0.5)
        else:
            raise RuntimeError("vite did not start")

        def handle(route):
            path = route.request.url.split("?", 1)[0].split(base, 1)[-1]
            if path.endswith("/api/v1/admin/codex-harvest"):
                body = ok(snapshot())
            elif path.endswith("/api/v1/admin/codex-harvest/nodes"):
                body = ok(NODES)
            elif path.endswith("/api/v1/admin/proxies/all"):
                body = ok(PROXIES)
            elif path.endswith("/api/v1/auth/me") or path.endswith("/api/v1/user/profile"):
                body = ok(ADMIN_USER)
            elif path.endswith("/api/v1/settings/public"):
                body = ok({"site_name": "Sub2API", "registration_enabled": False})
            else:
                unmatched.add(path)
                body = ok({})
            route.fulfill(status=200, content_type="application/json", body=body)

        with sync_playwright() as p:
            browser = p.chromium.launch(executable_path=os.environ.get("PLAYWRIGHT_CHROMIUM") or None)
            for label, viewport in (("desktop", {"width": 1440, "height": 900}), ("mobile", {"width": 390, "height": 844})):
                context = browser.new_context(viewport=viewport, color_scheme="light", locale="zh-CN")
                context.add_init_script(
                    "localStorage.setItem('auth_token','e2e-fixture');"
                    f"localStorage.setItem('auth_user', {json.dumps(json.dumps(ADMIN_USER))});"
                    "localStorage.setItem('locale','zh');localStorage.setItem('theme','light');"
                    # The first-login admin tour would otherwise cover the page.
                    "localStorage.setItem('admin_guide_1_admin_v4_interactive','true');")
                # Only backend calls: Vite also serves source modules under /src/api/.
                context.route(lambda url: urlparse(url).path.startswith("/api/"), handle)
                page = context.new_page()
                page.goto(f"{base}/admin/codex-harvest", wait_until="networkidle")
                page.wait_for_selector('[data-testid="harvest-events"]', timeout=30000)
                page.wait_for_timeout(500)
                html = page.content()
                facts = page.evaluate("""() => ({
                    title: document.querySelector('.page-title')?.textContent?.trim(),
                    sections: [...document.querySelectorAll('section.card h2')].map(e => e.textContent.trim()),
                    poolRows: document.querySelectorAll('[data-testid^="pool-proxy-"]').length,
                    unusableRows: document.querySelectorAll('[data-testid^="pool-unusable-"]').length,
                    accountRows: document.querySelectorAll('[data-testid^="harvest-account-"]').length,
                    eventRows: document.querySelectorAll('[data-testid="harvest-event"]').length,
                    selectedPreset: document.querySelector('[role=tab][aria-selected=true]')?.textContent?.trim(),
                    docScrollWidth: document.documentElement.scrollWidth,
                    viewportWidth: window.innerWidth,
                })""")
                facts.update(viewport=viewport, ticket_state_in_dom=TICKET_STATE in html, proxy_secret_in_dom=PROXY_SECRET in html)
                observations["pages"].append({"label": label, **facts})
                path = shots / f"codex-harvest-{label}-{viewport['width']}x{viewport['height']}.png"
                page.screenshot(path=str(path), full_page=True)
                manifest["screenshots"].append({"path": str(path.relative_to(evidence.parent)), "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
                                                "page_id": f"codex-harvest-{label}", "browser": "playwright-chromium", "viewport": viewport, "captured_at": iso()})
                if label == "desktop":
                    page.click('[data-testid="manual-41"]')
                    page.wait_for_selector('[data-testid="manual-dialog"]')
                    page.wait_for_timeout(800)  # let the modal transition finish
                    dialog = shots / "codex-harvest-manual-dialog-1440x900.png"
                    page.screenshot(path=str(dialog))
                    manifest["screenshots"].append({"path": str(dialog.relative_to(evidence.parent)), "sha256": hashlib.sha256(dialog.read_bytes()).hexdigest(),
                                                    "page_id": "codex-harvest-manual-dialog", "browser": "playwright-chromium", "viewport": viewport, "captured_at": iso()})
                context.close()
            browser.close()
    finally:
        server.terminate()
        try:
            server.wait(timeout=10)
        except subprocess.TimeoutExpired:
            server.kill()
    observations["unmatched_api_paths"] = sorted(unmatched)
    (evidence / "browser-observations.json").write_text(json.dumps(observations, ensure_ascii=False, indent=2) + "\n")
    (evidence / "screenshot-manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
    print(json.dumps(observations, ensure_ascii=False, indent=1))


if __name__ == "__main__":
    main()

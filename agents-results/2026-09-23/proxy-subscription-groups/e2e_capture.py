"""Capture the subscription groups dialog and the add-by-group pool in Chromium.

Reuses the fixed-port Vite runner and fixtures of codex-harvest-pool/e2e_capture.py
and answers /api with deterministic subscription fixtures (no backend, no URLs,
no credentials).

Usage: python e2e_capture.py FRONTEND_DIR EVIDENCE_DIR SOURCE_IDENTITY
"""
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import time
from urllib.parse import urlparse

from playwright.sync_api import sync_playwright

_spec = importlib.util.spec_from_file_location("harvest_capture", Path(__file__).resolve().parents[1] / "codex-harvest-pool" / "e2e_capture.py")
base = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(base)

SUBSCRIPTION_SECRET = "demo-subscription-token"


def meta(display, residential, multiplier="1×", route="直连", protocol="vless", region=None, country=None, flag=None):
    return {"region": region, "country": country, "flag": flag, "residential": residential, "multiplier": multiplier,
            "route": route, "protocol": protocol, "display_name": display}


SUBSCRIPTIONS = [
    {
        "id": "sufe", "name": "苏菲", "format": "uri-list", "updated_at": base.iso(-3600), "node_count": 4, "has_url": True,
        "refresh_interval_minutes": 60, "last_refresh_at": base.iso(-600), "last_refresh_status": "ok", "next_refresh_at": base.iso(3000),
        "usage": {"upload": 3221225472, "download": 32212254720, "total": 182536110080, "expire_at": base.iso(86400 * 21)},
        "info": ["剩余流量：138.17 GB", "距离下次重置剩余：5 天"],
        "groups": [
            {"name": "🎫 打票出口", "source": "derived", "kind": "purpose", "proxy_ids": [11, 12]},
            {"name": "💼 业务出口", "source": "derived", "kind": "purpose", "proxy_ids": [13, 14]},
            {"name": "家宽住宅 · 3×", "source": "derived", "kind": "multiplier", "proxy_ids": [11, 12]},
            {"name": "机房 · 1×", "source": "derived", "kind": "multiplier", "proxy_ids": [13, 14]},
            {"name": "🇯🇵 日本 · 住宅", "source": "derived", "kind": "region", "proxy_ids": [12]},
            {"name": "🇸🇬 新加坡 · 住宅", "source": "derived", "kind": "region", "proxy_ids": [11]},
            {"name": "🇸🇬 新加坡 · 机房", "source": "derived", "kind": "region", "proxy_ids": [14]},
            {"name": "🇺🇸 美国 · 机房", "source": "derived", "kind": "region", "proxy_ids": [13]},
        ],
        "nodes": [
            {"proxy_id": 11, "name": "【3x】中转|新加坡家宽🇸🇬", "info": False, "meta": meta("🇸🇬新加坡-中转-新加坡家宽-住宅IP", True, "3×", "中转", region="SG", country="新加坡", flag="🇸🇬")},
            {"proxy_id": 12, "name": "优秀｜【3x】中转|新日本KDDI家宽🇯🇵", "info": False, "meta": meta("🇯🇵日本-中转-新日本KDDI家宽-住宅IP", True, "3×", "中转", region="JP", country="日本", flag="🇯🇵")},
            {"proxy_id": 13, "name": "cf加速|美国圣何塞", "info": False, "meta": meta("🇺🇸美国-cf加速-美国圣何塞-机房-Vless", False, route="CF", region="US", country="美国", flag="🇺🇸")},
            {"proxy_id": 14, "name": "【2】新加坡高速节点🇸🇬hy2", "info": False, "meta": meta("🇸🇬新加坡-【2】新加坡高速节点-机房-Hysteria2", False, protocol="hysteria2", region="SG", country="新加坡", flag="🇸🇬")},
            {"proxy_id": 4, "name": "套餐到期：2026-09-14", "info": True, "meta": meta("🌐未知-套餐到期：2026-09-14-机房-Vless", False)},
        ],
    },
    {
        "id": "isp", "name": "VPS2ISP 住宅", "format": "mihomo-yaml", "updated_at": base.iso(-900), "node_count": 2, "has_url": False,
        "refresh_interval_minutes": 0, "info": [],
        "groups": [
            {"name": "静态 ISP · 1×", "source": "subscription", "kind": "provider", "proxy_ids": [21]},
            {"name": "家宽住宅 · 20×（手动）", "source": "subscription", "kind": "provider", "proxy_ids": [22]},
            {"name": "🎫 打票出口", "source": "derived", "kind": "purpose", "proxy_ids": [21, 22]},
        ],
        "nodes": [
            {"proxy_id": 21, "name": "🇺🇸美国-布罗格登-Charter静态住宅IP", "info": False, "meta": meta("🇺🇸美国-布罗格登-Charter静态住宅IP", True, protocol="hysteria2", region="US", country="美国", flag="🇺🇸")},
            {"proxy_id": 22, "name": "🇺🇸美国-克利夫兰-Charter静态住宅IP", "info": False, "meta": meta("🇺🇸美国-克利夫兰-Charter静态住宅IP", True, "20×", protocol="hysteria2", region="US", country="美国", flag="🇺🇸")},
        ],
    },
]


def proxy_row(pid, name, port):
    return {"id": pid, "name": name, "protocol": "http", "host": "mihomo", "port": port, "username": None, "status": "active", "account_count": 0,
            "expires_at": None, "fallback_mode": "none", "expiry_warn_days": 0, "created_at": base.iso(-86400), "updated_at": base.iso(-3600)}


PROXY_ROWS = [proxy_row(11, "苏菲 / 【3x】中转|新加坡家宽🇸🇬 / 1a2b3c4d", 20010), proxy_row(12, "苏菲 / 优秀｜【3x】中转|新日本KDDI家宽🇯🇵 / 2b3c4d5e", 20011),
              proxy_row(13, "苏菲 / cf加速|美国圣何塞 / 3c4d5e6f", 20035), proxy_row(14, "苏菲 / 【2】新加坡高速节点🇸🇬hy2 / 4d5e6f70", 20063),
              proxy_row(21, "VPS2ISP 住宅 / 🇺🇸美国-布罗格登-Charter静态住宅IP / 5e6f7081", 20100), proxy_row(22, "VPS2ISP 住宅 / 🇺🇸美国-克利夫兰-Charter静态住宅IP / 6f708192", 20101)]


def harvest_snapshot():
    snap = base.snapshot()
    snap["controls"]["proxy_ids"] = [11]
    snap["pool"] = [{"proxy_id": 11, "name": "苏菲 / 【3x】中转|新加坡家宽🇸🇬 / 1a2b3c4d", "usable": True}]
    return snap


def handle_factory(base_url, unmatched):
    def handle(route):
        path = urlparse(route.request.url).path
        if path == "/api/v1/admin/proxies/subscriptions":
            body = base.ok(SUBSCRIPTIONS)
        elif path == "/api/v1/admin/proxies/all":
            body = base.ok(PROXY_ROWS)
        elif path == "/api/v1/admin/proxies":
            body = base.ok({"items": PROXY_ROWS, "total": len(PROXY_ROWS), "page": 1, "page_size": 20, "pages": 1})
        elif path == "/api/v1/admin/codex-harvest":
            body = base.ok(harvest_snapshot())
        elif path == "/api/v1/admin/codex-harvest/nodes":
            body = base.ok({"items": [], "total": 0, "page": 1, "page_size": 20, "pages": 0})
        elif path in ("/api/v1/auth/me", "/api/v1/user/profile"):
            body = base.ok(base.ADMIN_USER)
        elif path == "/api/v1/settings/public":
            body = base.ok({"site_name": "Sub2API", "registration_enabled": False})
        else:
            unmatched.add(path)
            body = base.ok({})
        route.fulfill(status=200, content_type="application/json", body=body)
    return handle


def shot(page, evidence, manifest, name, viewport, full_page=True, element=None):
    path = evidence / "screenshots" / f"{name}-{viewport['width']}x{viewport['height']}.png"
    if element is not None:
        element.screenshot(path=str(path))
    else:
        page.screenshot(path=str(path), full_page=full_page)
    manifest["screenshots"].append({"path": str(path.relative_to(evidence.parent)), "sha256": hashlib.sha256(path.read_bytes()).hexdigest(),
                                    "page_id": name, "browser": "playwright-chromium", "viewport": viewport, "captured_at": base.iso()})


def main():
    frontend, evidence, source_identity = Path(sys.argv[1]), Path(sys.argv[2]), sys.argv[3]
    (evidence / "screenshots").mkdir(parents=True, exist_ok=False)
    if base.port_in_use(base.VITE_PORT):
        raise RuntimeError("port 4174 is already serving another frontend")
    env = dict(os.environ, VITE_DEV_PROXY_TARGET="http://127.0.0.1:9", BROWSER="none")
    server = subprocess.Popen(["pnpm", "exec", "vite", "--host", "127.0.0.1"], cwd=frontend, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    origin = f"http://127.0.0.1:{base.VITE_PORT}"
    unmatched = set()
    observations = {"runtime_identity": f"vite-dev-127.0.0.1:{base.VITE_PORT}", "pages": []}
    manifest = {"schema_version": 1, "source_identity": source_identity, "runtime_identity": observations["runtime_identity"], "screenshots": []}
    try:
        deadline = time.time() + 90
        while not base.port_in_use(base.VITE_PORT):
            if time.time() > deadline:
                raise RuntimeError("vite did not start")
            time.sleep(0.5)
        with sync_playwright() as p:
            browser = p.chromium.launch(executable_path=os.environ.get("PLAYWRIGHT_CHROMIUM") or None)
            for label, viewport in (("desktop", {"width": 1440, "height": 900}), ("mobile", {"width": 390, "height": 844})):
                context = browser.new_context(viewport=viewport, color_scheme="light", locale="zh-CN")
                context.add_init_script(
                    "localStorage.setItem('auth_token','e2e-fixture');"
                    f"localStorage.setItem('auth_user', {json.dumps(json.dumps(base.ADMIN_USER))});"
                    "localStorage.setItem('locale','zh');localStorage.setItem('theme','light');"
                    "localStorage.setItem('admin_guide_1_admin_v4_interactive','true');")
                context.route(lambda url: urlparse(url).path.startswith("/api/"), handle_factory(origin, unmatched))
                page = context.new_page()
                page.goto(f"{origin}/admin/proxies", wait_until="networkidle")
                page.click('[data-testid="open-proxy-subscriptions"]')
                page.wait_for_selector('[data-testid="subscription-sufe"]')
                page.click('[data-testid="group-sufe-🎫 打票出口"]')
                page.wait_for_selector('[data-testid="group-members-sufe-🎫 打票出口"]')
                page.wait_for_timeout(800)
                html = page.content()
                facts = page.evaluate("""() => ({
                    subscriptions: document.querySelectorAll('[data-testid^="subscription-"][data-testid$="sufe"], [data-testid="subscription-isp"]').length,
                    groupButtons: document.querySelectorAll('[data-testid^="group-sufe-"], [data-testid^="group-isp-"]').length,
                    members: document.querySelectorAll('[data-testid="group-members-sufe-🎫 打票出口"] li').length,
                    noUrlHint: !!document.querySelector('[data-testid="subscription-no-url"]'),
                    docScrollWidth: document.documentElement.scrollWidth,
                    viewportWidth: window.innerWidth,
                })""")
                facts.update(page="proxies-subscriptions", viewport=viewport, subscription_secret_in_dom=SUBSCRIPTION_SECRET in html, label=label)
                observations["pages"].append(facts)
                dialog = page.locator('.modal-content').first
                shot(page, evidence, manifest, f"proxy-subscriptions-{label}", viewport, element=dialog)

                page.goto(f"{origin}/admin/codex-harvest", wait_until="networkidle")
                page.wait_for_selector('[data-testid="harvest-pool-groups"]')
                page.wait_for_timeout(500)
                facts = page.evaluate("""() => ({
                    groupChips: document.querySelectorAll('[data-testid^="pool-group-"]').length,
                    normalizedNames: [...document.querySelectorAll('[data-testid^="pool-proxy-"] td:nth-child(2) div:first-child')].map(e => e.textContent.trim()).slice(0, 3),
                    docScrollWidth: document.documentElement.scrollWidth,
                    viewportWidth: window.innerWidth,
                })""")
                facts.update(page="codex-harvest-pool-groups", viewport=viewport, label=label)
                observations["pages"].append(facts)
                shot(page, evidence, manifest, f"harvest-pool-groups-{label}", viewport, element=page.locator('[data-testid="harvest-pool"]'))
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

#!/usr/bin/env python3
"""Read-only static wiring check; not a compiler, authorization test or full-application CI."""
from pathlib import Path
import re


def check(root: Path) -> list[str]:
    backend = (root / 'backend/internal/server/http.go').read_text()
    routes = (root / 'backend/internal/server/routes/cost_center.go').read_text()
    frontend = (root / 'frontend/src/main.ts').read_text()
    page = (root / 'frontend/src/router/cost-center.ts').read_text()
    if not re.search(r'^\s*routes\.RegisterCostCenterRoutes\(', backend, re.M):
        raise ValueError('native backend registration missing')
    if 'SUB2API_COST_LEDGER_DSN' not in backend:
        raise ValueError('explicit cost storage configuration missing')
    if not re.search(r'admin\.Use\(gin.HandlerFunc\(auth\)', routes) or 'ContextKeyUserRole' not in routes:
        raise ValueError('verified admin and middleware checks missing')
    if 'admin.POST("/ledger/commands", serve)' not in routes:
        raise ValueError('ledger command route missing')
    registration = frontend.find('  registerCostCenterRoute(router)')
    install = frontend.find('  app.use(router)')
    if registration < 0 or install < 0 or registration > install:
        raise ValueError('frontend registration must precede initial navigation')
    if "requiresAdmin: true" not in page or "path: '/admin/cost-center'" not in page:
        raise ValueError('admin page route metadata missing')
    return ['backend_registration', 'admin_guard_and_actor', 'explicit_storage_configuration',
            'frontend_before_navigation', 'admin_page_metadata']


if __name__ == '__main__':
    for result in check(Path(__file__).resolve().parents[2]):
        print('PASS static:', result)

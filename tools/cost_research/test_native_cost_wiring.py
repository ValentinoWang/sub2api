import tempfile
from pathlib import Path
import unittest
from check_native_cost_wiring import check


class NativeCostWiringTests(unittest.TestCase):
    def fixture(self, root):
        files = {
            'backend/internal/server/http.go': 'routes.RegisterCostCenterRoutes(\n"SUB2API_COST_LEDGER_DSN"\n',
            'backend/internal/server/routes/cost_center.go': 'admin.Use(gin.HandlerFunc(auth), limiter.Global())\nContextKeyUserRole\nadmin.POST("/ledger/commands", serve)',
            'frontend/src/main.ts': '  registerCostCenterRoute(router)\n  app.use(router)',
            'frontend/src/router/cost-center.ts': "requiresAdmin: true\npath: '/admin/cost-center'"
        }
        for relative, content in files.items():
            p = root / relative
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_text(content)

    def test_valid_static_contract(self):
        with tempfile.TemporaryDirectory() as t:
            root = Path(t); self.fixture(root)
            self.assertEqual(len(check(root)), 5)

    def test_read_only(self):
        with tempfile.TemporaryDirectory() as t:
            root = Path(t); self.fixture(root)
            before = {str(p): p.read_bytes() for p in root.rglob('*') if p.is_file()}
            check(root)
            self.assertEqual(before, {str(p): p.read_bytes() for p in root.rglob('*') if p.is_file()})

    def test_missing_auth_rejected(self):
        with tempfile.TemporaryDirectory() as t:
            root = Path(t); self.fixture(root)
            (root / 'backend/internal/server/routes/cost_center.go').write_text('public route')
            with self.assertRaises(ValueError): check(root)

    def test_late_registration_rejected(self):
        with tempfile.TemporaryDirectory() as t:
            root = Path(t); self.fixture(root)
            (root / 'frontend/src/main.ts').write_text('  app.use(router)\n  registerCostCenterRoute(router)')
            with self.assertRaises(ValueError): check(root)

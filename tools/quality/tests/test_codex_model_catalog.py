import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


SOURCE = Path(__file__).resolve().parents[3] / "frontend/public/downloads/sync-codex-model-catalog.py"
SPEC = importlib.util.spec_from_file_location("catalog_sync", SOURCE)
sync = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(sync)


class CatalogSyncTests(unittest.TestCase):
    def setUp(self):
        self.body = json.dumps({"models": [{"slug": "test-model", "context_window": 1000,
            "max_context_window": 2000, "effective_context_window_percent": 95,
            "model_messages": {"instructions_template": "preserve this"}}]}).encode()

    def test_keeps_base_path_and_adds_codex_version(self):
        self.assertEqual(sync.catalog_url("https://example.test/v1/", "0.154.0"),
                         "https://example.test/v1/models?client_version=0.154.0")
        for base in ["http://example.test", "https://user:secret@example.test", "https://example.test?key=secret"]:
            with self.assertRaises(sync.CatalogError):
                sync.catalog_url(base, "0.154.0")

    def test_rejects_redirect_before_credentials_can_follow(self):
        with self.assertRaises(sync.CatalogError):
            sync.NoRedirect().redirect_request(None, None, 302, "", {}, "https://elsewhere.test")

    def test_rejects_incomplete_or_wrong_catalog(self):
        for body in [b'<html>login</html>', b'{"data":[{"id":"test-model"}]}', b'{"models":[]}',
                     self.body.replace(b'1000', b'-1'), self.body.replace(b'test-model', b'other-model')]:
            with self.assertRaises(sync.CatalogError):
                sync.validate_catalog(body, "test-model")

    def test_client_rejection_keeps_last_good_file(self):
        with tempfile.TemporaryDirectory() as folder:
            output = Path(folder) / "catalog.json"
            output.write_bytes(b'last good file')
            with patch.object(sync.subprocess, "run", return_value=subprocess.CompletedProcess([], 1, "", "rejected")):
                with self.assertRaises(sync.CatalogError):
                    sync.install_catalog(self.body, output, "codex", "test-model")
            self.assertEqual(output.read_bytes(), b'last good file')
            self.assertEqual(list(output.parent.glob('*.backup-*')), [])

    def test_omitted_effective_percent_uses_client_default(self):
        catalog = json.loads(self.body)
        del catalog['models'][0]['effective_context_window_percent']
        _, summary = sync.validate_catalog(json.dumps(catalog), 'test-model')
        self.assertEqual(summary['effective_context_window_percent'], 95)

    def test_success_preserves_complete_metadata_and_backup(self):
        with tempfile.TemporaryDirectory() as folder:
            output = Path(folder) / "catalog.json"
            output.write_bytes(b'last good file')
            with patch.object(sync.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, self.body.decode(), "")):
                result = sync.install_catalog(self.body, output, "codex", "test-model")
            self.assertEqual(json.loads(output.read_bytes()), json.loads(self.body))
            self.assertEqual(Path(result["backup"]).read_bytes(), b'last good file')
            self.assertEqual(output.stat().st_mode & 0o777, 0o600)

    def test_changed_client_values_do_not_replace_catalog(self):
        with tempfile.TemporaryDirectory() as folder:
            output = Path(folder) / "catalog.json"
            returned = self.body.replace(b'1000', b'500').decode()
            with patch.object(sync.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, returned, "")):
                with self.assertRaises(sync.CatalogError):
                    sync.install_catalog(self.body, output, "codex", "test-model")
            self.assertFalse(output.exists())

    def test_repeated_sync_applies_explicit_rules_without_changing_capabilities(self):
        models = [{"slug": slug, "context_window": 1050000, "visibility": "list",
                   "model_messages": {"instructions_template": "preserve"}}
                  for slug in ["future", "future-main", "other", "other-main",
                               "dated", "dated-2030-01-01", "custom"]]
        body = json.dumps({"models": models})
        with tempfile.TemporaryDirectory() as folder:
            output = Path(folder) / "catalog.json"
            output.write_text(json.dumps({"models": [{"slug": "custom", "visibility": "hide"}]}))

            def load_candidate(command, **kwargs):
                candidate = Path(json.loads(command[2].split("=", 1)[1]))
                return subprocess.CompletedProcess(command, 0, candidate.read_text(), "")

            with patch.object(sync.subprocess, "run", side_effect=load_candidate):
                for _ in range(2):
                    rules = {"aliases": {"future": "future-main", "other": "other-main"}, "visibility": {"custom": "hide"}}
                    result = sync.install_catalog(body, output, "codex", "future-main", rules)
                    self.assertEqual(result["hidden_models"], ["custom", "future", "other"])
                    saved = json.loads(output.read_text())
                    self.assertEqual(len(saved["models"]), len(models))
                    for original, actual in zip(models, saved["models"]):
                        expected = dict(original)
                        if original["slug"] in result["hidden_models"]:
                            expected["visibility"] = "hide"
                        self.assertEqual(actual, expected)

    def test_alias_stays_visible_without_visible_canonical_model(self):
        with tempfile.TemporaryDirectory() as folder:
            for canonical in [None, {"slug": "gpt-6-astra", "visibility": "hide"}]:
                catalog = {"models": [{"slug": "gpt-6", "visibility": "list"}]}
                if canonical:
                    catalog["models"].append(canonical)
                sync.apply_picker_visibility(catalog, {"aliases": {"gpt-6": "gpt-6-astra"}})
                self.assertEqual(catalog["models"][0]["visibility"], "list")

    def test_rules_are_scoped_and_reject_cycles_and_unknown_fields(self):
        valid = {"version": 1, "provider": "gateway", "endpoint": "https://example.test/v1/models",
                 "aliases": {"alias": "middle", "middle": "canonical"}, "visibility": {}}
        invalid = [{**valid, "provider": "another"}, {**valid, "endpoint": "https://elsewhere.test/models"},
                   {**valid, "aliases": {"a": "b", "b": "a"}}, {**valid, "aliases": {"a": "a"}},
                   {**valid, "visibility": {"a": "hidden"}}, {**valid, "unknown": True},
                   {**valid, "version": True}, {**valid, "aliases": ["a"]}]
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "rules.json"
            path.write_text(json.dumps(valid))
            self.assertEqual(sync.load_picker_rules(path, "gateway", valid["endpoint"]), valid)
            for value in invalid:
                path.write_text(json.dumps(value))
                with self.assertRaises(sync.CatalogError):
                    sync.load_picker_rules(path, "gateway", valid["endpoint"])

    def test_chains_explicit_show_and_removed_rules(self):
        body = {"models": [{"slug": slug, "visibility": "list"} for slug in ["a", "b", "c"]]}
        catalog = json.loads(json.dumps(body))
        sync.apply_picker_visibility(catalog, {"aliases": {"a": "b", "b": "c"}, "visibility": {"a": "list"}})
        self.assertEqual([m["visibility"] for m in catalog["models"]], ["list", "hide", "list"])
        fresh = json.loads(json.dumps(body))
        sync.apply_picker_visibility(fresh, {})
        self.assertEqual(fresh, body)

    def test_default_does_not_infer_duplicates_or_import_old_hidden_state(self):
        body = {"models": [{"slug": slug, "context_window": 1000, "visibility": "list"}
                           for slug in ["gpt-6", "gpt-6-astra", "same-name-2030-01-01"]]}
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "catalog.json"
            path.write_text(json.dumps({"models": [{"slug": "gpt-6", "visibility": "hide"}]}))
            with patch.object(sync.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, json.dumps(body), "")):
                sync.install_catalog(json.dumps(body), path, "codex", "gpt-6")
            self.assertEqual(json.loads(path.read_text()), body)

    def test_client_visibility_mismatch_keeps_previous_file(self):
        body = {"models": [{"slug": "test-model", "context_window": 1000, "visibility": "list"}]}
        with tempfile.TemporaryDirectory() as folder:
            path = Path(folder) / "catalog.json"
            path.write_text("previous")
            with patch.object(sync.subprocess, "run", return_value=subprocess.CompletedProcess([], 0, json.dumps(body), "")):
                with self.assertRaises(sync.CatalogError):
                    sync.install_catalog(json.dumps(body), path, "codex", "test-model", {"visibility": {"test-model": "hide"}})
            self.assertEqual(path.read_text(), "previous")


if __name__ == "__main__":
    unittest.main()

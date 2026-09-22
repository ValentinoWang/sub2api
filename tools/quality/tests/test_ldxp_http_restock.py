"""Synthetic restock recovery, inventory integrity, and credential isolation tests."""
import copy
import hashlib
import io
import json
from pathlib import Path
import plistlib
import subprocess
import sys
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import Mock, patch
from urllib.error import HTTPError

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "commerce"))
import ldxp_http_restock as restock


GOODS_ID = 42
DEVICE_ID = "synthetic-device"
CODES = ["a" * 32, "b" * 32]
DEVICE_KEY = "ldxpd_" + "d" * 64
ADMIN_KEY = "admin-" + "e" * 64
PRODUCT = {
    "goods_id": GOODS_ID, "cny_amount": 5, "usd_credit": 5,
    "external_url": "https://www.ldxp.cn/goods/42", "enabled": True,
    "target_stock": 2, "batch_size": 2,
}
ROW = {"id": GOODS_ID, "name": "Synthetic test product", "goods_type": "card", "price": "5.00"}


def hashes(codes):
    return sorted(hashlib.sha256(code.encode()).hexdigest() for code in codes)


def batch(**changes):
    value = {
        "batch_id": "synthetic-batch", "goods_id": GOODS_ID, "status": "claimed",
        "codes": CODES.copy(), "code_count": 2,
        "code_hashes": [hashlib.sha256(code.encode()).hexdigest() for code in CODES],
    }
    return value | changes


def inventory(matched=0, **changes):
    return {
        "blocked": False, "identity_verified": True, "batch_resolved": False,
        "matched_stock": matched, "target_stock": 2,
    } | changes


class Merchant:
    def __init__(self, codes=()):
        self.codes = list(codes)
        self.uploads = []
        self.rows_data = [copy.deepcopy(ROW)]
        self.login_error = None
        self.upload_error = None
        self.accept_count = None
        self.saved = False

    def post(self, route, payload):
        if route != "/merchantApi/user/userinfo" or payload:
            raise AssertionError("Unexpected merchant request")
        if self.login_error:
            raise self.login_error
        return {"id": "synthetic-merchant"}

    def rows(self, route, filters):
        if route != "/merchantApi/Goods/list":
            raise AssertionError("Unexpected catalog request")
        return copy.deepcopy(self.rows_data)

    def inventory_hashes(self, goods_id):
        if goods_id != GOODS_ID:
            raise AssertionError("Unexpected goods identity")
        return hashes(self.codes)

    def upload(self, goods_id, codes):
        self.uploads.append((goods_id, list(codes)))
        self.codes.extend(codes if self.accept_count is None else codes[:self.accept_count])
        if self.upload_error:
            raise self.upload_error
        return {}

    def save(self):
        self.saved = True


class Backend:
    def __init__(self, inventories, claimed=None):
        self.calls = []
        self.inventories = list(inventories)
        self.claimed = claimed
        self.start_error = None
        self.heartbeat = {}
        self.config = {
            "enabled": True,
            "device": {"id": DEVICE_ID, "revoked": False, "goods_ids": [GOODS_ID]},
            "products": [copy.deepcopy(PRODUCT)],
        }

    def call(self, route, body=None, method=None):
        self.calls.append((route, copy.deepcopy(body), method))
        if route == "/config":
            return copy.deepcopy(self.config)
        if route == "/inventory":
            if not self.inventories:
                raise AssertionError("Unexpected inventory reconciliation")
            return copy.deepcopy(self.inventories.pop(0))
        if route == "/claim":
            return {"batch": copy.deepcopy(self.claimed)}
        if route == "/batches/synthetic-batch/start":
            if self.start_error:
                raise self.start_error
            return {"batch": batch(status="uncertain")}
        if route == "/heartbeat":
            return copy.deepcopy(self.heartbeat)
        if route == "/runtime":
            return {"reported": True}
        if route == "/resume":
            return {}
        raise AssertionError("Unexpected backend request: " + route)

    def bodies(self, route):
        return [body for called, body, _ in self.calls if called == route]


class RestockTests(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory()
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.private = self.root / "private"
        self.state = {
            "schema": 1, "site": restock.probe.LOCAL_SITE, "device_id": DEVICE_ID,
            "pins": {str(GOODS_ID): {
                "goods_id": GOODS_ID, "cny_amount": 5, "usd_credit": 5,
                "external_url": PRODUCT["external_url"], "merchant_name": ROW["name"],
            }},
            "paused": False, "reason": "", "target_stock": 2,
            "products": {str(GOODS_ID): {"phase": "idle"}},
        }
        restock.save_state(self.private, self.state)
        self.addCleanup(patch.stopall)
        patch("socket.socket.connect", side_effect=AssertionError("Tests must not access a network")).start()

    def cycle(self, merchant, backend, **kwargs):
        return restock.cycle(self.private, merchant, backend, DEVICE_ID, **kwargs)

    def saved(self):
        return restock.read_state(self.private)

    def assert_paused(self, report, reason):
        self.assertEqual(report["status"], "PAUSED")
        self.assertEqual(report["error"], reason)
        self.assertTrue(self.saved()["paused"])
        self.assertEqual(self.saved()["reason"], reason)

    def test_upload_is_verified_by_exact_code_hashes_and_persisted_without_codes(self):
        merchant = Merchant()
        backend = Backend([inventory(), inventory(2, batch_resolved=True)], batch())
        report = self.cycle(merchant, backend)
        self.assertEqual(report["status"], "RUNNING")
        self.assertEqual(report["uploaded"], 2)
        self.assertTrue(report["at_target"])
        self.assertEqual(merchant.uploads, [(GOODS_ID, CODES)])
        self.assertEqual(backend.bodies("/inventory"), [
            {"goods_id": GOODS_ID, "complete": True, "total": 0, "hashes": []},
            {"goods_id": GOODS_ID, "complete": True, "total": 2, "hashes": hashes(CODES), "batch_id": "synthetic-batch"},
        ])
        local = self.saved()["products"][str(GOODS_ID)]
        self.assertEqual(local["phase"], "idle")
        self.assertNotIn("batch_id", local)
        self.assertTrue(merchant.saved)
        self.assertFalse(self.saved()["paused"])
        for code in CODES:
            self.assertNotIn(code, (self.private / "state.json").read_text())
        self.assertEqual((self.private / "state.json").stat().st_mode & 0o777, 0o600)

    def test_target_already_full_does_not_upload_or_start_a_batch(self):
        merchant = Merchant(CODES)
        backend = Backend([inventory(2)])
        report = self.cycle(merchant, backend)
        self.assertEqual(report["status"], "RUNNING")
        self.assertEqual(report["uploaded"], 0)
        self.assertTrue(report["at_target"])
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/batches/synthetic-batch/start"), [])

    def test_read_only_check_never_claims_or_resumes(self):
        merchant = Merchant()
        backend = Backend([inventory()], batch())
        report = self.cycle(merchant, backend, write=False, resume=True)
        self.assertEqual(report["status"], "VERIFIED")
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/claim"), [])
        self.assertEqual(backend.bodies("/resume"), [])

    def test_lost_upload_response_is_reconciled_in_same_cycle_without_reupload(self):
        merchant = Merchant()
        merchant.upload_error = restock.probe.ProbeError("network_error")
        backend = Backend([inventory(), inventory(2, batch_resolved=True)], batch())
        report = self.cycle(merchant, backend)
        self.assertEqual(report["status"], "RUNNING")
        self.assertEqual(merchant.uploads, [(GOODS_ID, CODES)])
        self.assertEqual(len(backend.bodies("/inventory")), 2)
        self.assertTrue(report["products"][0]["batch_verified"])
        self.assertEqual(self.saved()["products"][str(GOODS_ID)]["phase"], "idle")
        self.assertFalse(self.saved()["paused"])

    def test_restart_after_upload_acceptance_only_reconciles_original_batch(self):
        self.state["products"][str(GOODS_ID)] = {"phase": "uncertain", "batch_id": "synthetic-batch"}
        self.state.update(paused=True, reason="network_error")
        restock.save_state(self.private, self.state)
        merchant = Merchant(CODES)
        backend = Backend([inventory(2, batch_resolved=True)])
        report = self.cycle(merchant, backend, resume=True)
        self.assertEqual(report["status"], "RUNNING")
        self.assertTrue(report["at_target"])
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/batches/synthetic-batch/start"), [])
        self.assertEqual(backend.bodies("/inventory")[0]["batch_id"], "synthetic-batch")
        self.assertNotIn("batch_id", self.saved()["products"][str(GOODS_ID)])

    def test_network_read_failure_recovers_in_a_new_process_after_full_verification(self):
        merchant = Merchant()
        backend = Backend([], batch())
        with patch.object(merchant, "inventory_hashes", side_effect=restock.probe.ProbeError("network_error")):
            self.assert_paused(self.cycle(merchant, backend), "network_error")
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/claim"), [])

        script = (
            "import json, sys; sys.path.insert(0, sys.argv[1])\n"
            "import test_ldxp_http_restock as t\n"
            "merchant = t.Merchant()\n"
            "backend = t.Backend([t.inventory(), t.inventory(), t.inventory(2, batch_resolved=True)], t.batch())\n"
            "backend.config['paused_reason'] = 'disconnected'\n"
            "with t.patch('socket.socket.connect', side_effect=AssertionError('No network in tests')):\n"
            "    report = t.restock.cycle(sys.argv[2], merchant, backend, t.DEVICE_ID)\n"
            "print(json.dumps({'report': report, 'uploads': merchant.uploads, 'calls': backend.calls}))\n"
        )
        restarted = subprocess.run([sys.executable, "-B", "-c", script, str(Path(__file__).parent), str(self.private)],
                                   capture_output=True, text=True, check=True, timeout=10)
        result = json.loads(restarted.stdout)
        self.assertEqual(result["report"]["status"], "RUNNING")
        self.assertEqual(result["report"]["uploaded"], 2)
        self.assertEqual(result["uploads"], [[GOODS_ID, CODES]])
        routes = [route for route, _, _ in result["calls"]]
        self.assertEqual(routes[:5], ["/config", "/heartbeat", "/inventory", "/heartbeat", "/inventory"])
        self.assertLess(routes.index("/inventory"), routes.index("/resume"))
        self.assertEqual(routes.count("/batches/synthetic-batch/start"), 1)
        self.assertFalse(self.saved()["paused"])
        self.assertEqual(self.saved()["reason"], "")

    def test_network_failure_after_upload_is_reconciled_before_automatic_recovery(self):
        merchant = Merchant()
        backend = Backend([inventory()], batch())
        with patch.object(merchant, "inventory_hashes", side_effect=[[], restock.probe.ProbeError("network_error")]):
            self.assert_paused(self.cycle(merchant, backend), "network_error")
        self.assertEqual(merchant.uploads, [(GOODS_ID, CODES)])
        self.assertEqual(self.saved()["products"][str(GOODS_ID)]["phase"], "uncertain")

        restarted_merchant = Merchant(merchant.codes)
        restarted_backend = Backend([inventory(2, batch_resolved=True), inventory(2)])
        restarted_backend.heartbeat = {"paused_reason": "disconnected"}
        report = self.cycle(restarted_merchant, restarted_backend)
        self.assertEqual(report["status"], "RUNNING")
        self.assertEqual(report["uploaded"], 0)
        self.assertEqual(restarted_merchant.uploads, [])
        self.assertEqual(restarted_backend.bodies("/batches/synthetic-batch/start"), [])
        reconciliations = restarted_backend.bodies("/inventory")
        self.assertEqual(reconciliations[0]["batch_id"], "synthetic-batch")
        self.assertEqual(reconciliations[0]["hashes"], hashes(CODES))
        self.assertNotIn("batch_id", reconciliations[1])
        self.assertNotIn("batch_id", self.saved()["products"][str(GOODS_ID)])
        routes = [route for route, _, _ in restarted_backend.calls]
        self.assertLess(routes.index("/inventory"), routes.index("/resume"))

    def test_network_still_unavailable_keeps_original_reason_and_never_uploads(self):
        self.state.update(paused=True, reason="network_error")
        restock.save_state(self.private, self.state)
        for failure_stage in ("post", "rows", "inventory_hashes"):
            with self.subTest(failure_stage=failure_stage):
                merchant = Merchant()
                backend = Backend([inventory()], batch())
                with patch.object(merchant, failure_stage, side_effect=restock.probe.ProbeError("network_error")) as failed:
                    self.assert_paused(self.cycle(merchant, backend), "network_error")
                failed.assert_called_once()
                self.assertEqual(backend.bodies("/claim"), [])
                self.assertEqual(backend.bodies("/resume"), [])
                self.assertEqual(merchant.uploads, [])

    def test_network_recovery_requires_every_product_inventory_before_any_write(self):
        other_id = 99
        other_product = PRODUCT | {"goods_id": other_id, "external_url": "https://www.ldxp.cn/goods/99"}
        other_row = ROW | {"id": other_id, "name": "Another synthetic product"}
        self.state.update(paused=True, reason="network_error")
        self.state["pins"][str(other_id)] = restock.identity(other_product, other_row)
        self.state["products"][str(other_id)] = {"phase": "idle"}
        restock.save_state(self.private, self.state)
        merchant = Merchant()
        merchant.rows_data.append(other_row)
        backend = Backend([inventory(), inventory(identity_verified=False, blocked=True)], batch())
        backend.config["device"]["goods_ids"].append(other_id)
        backend.config["products"].append(other_product)
        backend.config["paused_reason"] = "disconnected"
        with patch.object(merchant, "inventory_hashes", return_value=[]):
            self.assert_paused(self.cycle(merchant, backend), "inventory_mismatch")
        self.assertEqual([body["goods_id"] for body in backend.bodies("/inventory")], [GOODS_ID, other_id])
        self.assertEqual(backend.bodies("/claim"), [])
        self.assertEqual(backend.bodies("/resume"), [])
        self.assertEqual(merchant.uploads, [])

    def test_network_recovery_does_not_release_an_unconfirmed_original_batch(self):
        for result in (inventory(blocked=True, pending_batch=batch(status="uncertain")),
                       inventory(pending_batch=batch()), inventory()):
            with self.subTest(result=result):
                self.state.update(paused=True, reason="network_error")
                self.state["products"][str(GOODS_ID)] = {"phase": "idle", "batch_id": "synthetic-batch"}
                restock.save_state(self.private, copy.deepcopy(self.state))
                merchant = Merchant()
                backend = Backend([result], batch())
                self.assert_paused(self.cycle(merchant, backend), "uncertain")
                self.assertEqual(backend.bodies("/inventory")[0]["batch_id"], "synthetic-batch")
                self.assertEqual(self.saved()["products"][str(GOODS_ID)]["batch_id"], "synthetic-batch")
                self.assertEqual(backend.bodies("/claim"), [])
                self.assertEqual(backend.bodies("/resume"), [])
                self.assertEqual(merchant.uploads, [])

    def test_manual_pause_file_takes_priority_over_network_recovery(self):
        self.state.update(paused=True, reason="network_error")
        restock.save_state(self.private, self.state)
        restock.probe.write_private(self.private / "pause", {"requested_at": 1})
        merchant = Merchant()
        backend = Backend([inventory()], batch())
        with patch.object(merchant, "post") as login:
            self.assert_paused(self.cycle(merchant, backend), "manual")
        login.assert_not_called()
        self.assertTrue((self.private / "pause").exists())
        self.assertEqual(backend.calls, [])
        self.assertEqual(merchant.uploads, [])

    def test_manual_pause_arriving_during_recovery_blocks_the_write_pass(self):
        self.state.update(paused=True, reason="network_error")
        restock.save_state(self.private, self.state)
        merchant = Merchant()
        backend = Backend([inventory()], batch())

        def inventory_then_pause(goods_id):
            restock.probe.write_private(self.private / "pause", {"requested_at": 1})
            return []

        with patch.object(merchant, "inventory_hashes", side_effect=inventory_then_pause):
            self.assert_paused(self.cycle(merchant, backend), "manual")
        self.assertTrue((self.private / "pause").exists())
        self.assertEqual(backend.bodies("/claim"), [])
        self.assertEqual(backend.bodies("/resume"), [])
        self.assertEqual(merchant.uploads, [])

    def test_network_recovery_still_requires_login_and_pinned_product_identity(self):
        for failure in ("login_required", "binding_changed", "backend_auth"):
            with self.subTest(failure=failure):
                self.state.update(paused=True, reason="network_error")
                restock.save_state(self.private, copy.deepcopy(self.state))
                merchant = Merchant()
                backend = Backend([inventory()], batch())
                if failure == "login_required":
                    merchant.login_error = restock.probe.ProbeError(failure)
                elif failure == "binding_changed":
                    merchant.rows_data = [ROW | {"price": "6.00"}]
                else:
                    backend.call = Mock(side_effect=restock.RestockError(failure))
                self.assert_paused(self.cycle(merchant, backend), failure)
                self.assertEqual(backend.bodies("/inventory"), [])
                self.assertEqual(backend.bodies("/claim"), [])
                self.assertEqual(backend.bodies("/resume"), [])
                self.assertEqual(merchant.uploads, [])

    def test_failed_read_only_check_cannot_make_a_protected_pause_recoverable(self):
        for reason in ("manual", "login_required", "backend_auth", "inventory_mismatch", "uncertain"):
            with self.subTest(reason=reason):
                self.state.update(paused=True, reason=reason)
                restock.save_state(self.private, copy.deepcopy(self.state))
                merchant = Merchant()
                merchant.login_error = restock.probe.ProbeError("network_error")
                backend = Backend([])
                report = self.cycle(merchant, backend, write=False)
                self.assert_paused(report, reason)
                self.assertEqual(report["verification_error"], "network_error")
                self.assert_paused(self.cycle(Merchant(), backend), reason)
                self.assertEqual(backend.calls, [])
                self.assertEqual(merchant.uploads, [])

    def test_non_network_pauses_keep_their_cause_without_automatic_recovery(self):
        for reason in ("manual", "login_required", "backend_auth", "backend_error", "binding_changed",
                       "inventory_mismatch", "uncertain", "unsafe_session", "invalid_response"):
            with self.subTest(reason=reason):
                self.state.update(paused=True, reason=reason)
                restock.save_state(self.private, copy.deepcopy(self.state))
                merchant = Merchant()
                backend = Backend([inventory()], batch())
                with patch.object(merchant, "post") as login:
                    for _ in range(2):
                        report = self.cycle(merchant, backend)
                        self.assert_paused(report, reason)
                        restock.attention(report, self.private)
                login.assert_not_called()
                self.assertEqual(json.loads((self.private / "attention.json").read_text())["error"], reason)
                self.assertEqual(backend.calls, [])
                self.assertEqual(merchant.uploads, [])

    def test_partial_or_foreign_existing_inventory_blocks_claim(self):
        scenarios = [
            (CODES[:1], inventory(1, blocked=True), "uncertain"),
            (["f" * 32], inventory(0, identity_verified=False, blocked=True), "inventory_mismatch"),
        ]
        for codes, result, reason in scenarios:
            with self.subTest(reason=reason):
                restock.save_state(self.private, copy.deepcopy(self.state))
                merchant = Merchant(codes)
                backend = Backend([result], batch())
                self.assert_paused(self.cycle(merchant, backend), reason)
                self.assertEqual(backend.bodies("/claim"), [])
                self.assertEqual(merchant.uploads, [])

    def test_partial_upload_stays_uncertain_and_restart_cannot_reupload(self):
        merchant = Merchant()
        merchant.accept_count = 1
        pending = batch(status="uncertain")
        backend = Backend([inventory(), inventory(1, blocked=True, pending_batch=pending)], batch())
        self.assert_paused(self.cycle(merchant, backend), "uncertain")
        self.assertEqual(merchant.uploads, [(GOODS_ID, CODES)])
        self.assertEqual(self.saved()["products"][str(GOODS_ID)]["batch_id"], "synthetic-batch")
        restarted = Backend([inventory(1, blocked=True, pending_batch=pending)], batch())
        self.assert_paused(self.cycle(merchant, restarted, resume=True), "uncertain")
        self.assertEqual(merchant.uploads, [(GOODS_ID, CODES)])
        self.assertEqual(restarted.bodies("/claim"), [])
        self.assertEqual(restarted.bodies("/resume"), [])

    def test_post_upload_matching_count_must_equal_actual_inventory(self):
        merchant = Merchant()
        backend = Backend([inventory(), inventory(1, batch_resolved=True)], batch())
        report = self.cycle(merchant, backend)
        self.assertEqual(report["status"], "PAUSED")
        self.assertTrue(self.saved()["paused"])
        self.assertEqual(self.saved()["products"][str(GOODS_ID)]["phase"], "uncertain")
        self.assertEqual(merchant.uploads, [(GOODS_ID, CODES)])
        self.assertNotIn("batch_verified", report["products"][0] if report["products"] else {})

    def test_lost_start_response_latches_before_any_merchant_write(self):
        merchant = Merchant()
        backend = Backend([inventory()], batch())
        backend.start_error = restock.RestockError("backend_error")
        self.assert_paused(self.cycle(merchant, backend), "backend_error")
        self.assertEqual(merchant.uploads, [])
        local = self.saved()["products"][str(GOODS_ID)]
        self.assertEqual(local["batch_id"], "synthetic-batch")
        self.assertEqual(local["phase"], "uncertain")
        restarted = Backend([inventory(blocked=True, pending_batch=batch(status="uncertain"))], batch())
        self.assert_paused(self.cycle(merchant, restarted, resume=True), "uncertain")
        self.assertEqual(restarted.bodies("/claim"), [])
        self.assertEqual(merchant.uploads, [])

    def test_wrong_goods_hash_count_and_duplicate_codes_never_reach_upload(self):
        malformed = [
            batch(goods_id=99), batch(code_hashes=["0" * 64, "1" * 64]),
            batch(code_count=1), batch(codes=[CODES[0], CODES[0]]),
        ]
        for value in malformed:
            with self.subTest(batch=value):
                restock.save_state(self.private, copy.deepcopy(self.state))
                merchant = Merchant()
                backend = Backend([inventory()], value)
                self.assert_paused(self.cycle(merchant, backend), "binding_changed")
                self.assertEqual(merchant.uploads, [])
                self.assertEqual(backend.bodies("/batches/synthetic-batch/start"), [])

    def test_login_expiry_reports_failed_heartbeat_and_survives_restart(self):
        merchant = Merchant()
        merchant.login_error = restock.probe.ProbeError("login_required")
        backend = Backend([])
        report = self.cycle(merchant, backend)
        self.assert_paused(report, "login_required")
        self.assertTrue(report["pause_reported_to_backend"])
        self.assertEqual(backend.bodies("/heartbeat"), [{"authorization": "failed"}])
        self.assertEqual(backend.bodies("/claim"), [])
        fresh_backend = Backend([inventory()], batch())
        self.assert_paused(self.cycle(Merchant(), fresh_backend), "login_required")
        self.assertEqual(fresh_backend.calls, [])

    def test_auth_failure_while_reporting_pause_does_not_clear_local_pause(self):
        merchant = Merchant()
        merchant.login_error = restock.probe.ProbeError("login_required")
        backend = Backend([])
        with patch.object(backend, "call", side_effect=restock.RestockError("backend_auth")):
            report = self.cycle(merchant, backend)
        self.assert_paused(report, "login_required")
        self.assertFalse(report["pause_reported_to_backend"])

    def test_binding_change_blocks_all_claims(self):
        for changed in ({"name": "Different product"}, {"price": "6.00"}):
            with self.subTest(changed=changed):
                restock.save_state(self.private, copy.deepcopy(self.state))
                merchant = Merchant()
                merchant.rows_data = [ROW | changed]
                backend = Backend([inventory()], batch())
                self.assert_paused(self.cycle(merchant, backend), "binding_changed")
                self.assertEqual(merchant.uploads, [])
                self.assertEqual(backend.bodies("/claim"), [])

    def test_pending_batch_for_another_goods_is_rejected_before_claim(self):
        merchant = Merchant()
        backend = Backend([inventory(pending_batch=batch(goods_id=99))], batch())
        self.assert_paused(self.cycle(merchant, backend), "binding_changed")
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/claim"), [])

    def test_cli_missing_login_persists_pause_and_reports_failed_heartbeat(self):
        config = self.root / "config.json"
        config.write_text(json.dumps({"site": restock.probe.LOCAL_SITE, "target_stock": 2,
                                      "batch_size": 2, "interval_seconds": 30, "goods_ids": [GOODS_ID]}))
        backend = Backend([])
        reports = []
        args = [restock.__file__, "once", "--config", str(config), "--private-dir", str(self.private),
                "--session", str(self.root / "missing-session.json")]
        with patch.object(sys, "argv", args), patch.object(restock, "device_backend", return_value=(backend, DEVICE_ID)), \
                patch.object(restock.os, "umask"), \
                patch.object(restock.signal, "signal"), \
                patch.object(restock, "receipt", side_effect=lambda report, evidence: reports.append(report)):
            self.assertEqual(restock.main(), 1)
        self.assertEqual(reports[-1]["error"], "login_required")
        self.assertTrue(self.saved()["paused"])
        self.assertEqual(self.saved()["reason"], "login_required")
        self.assertEqual(backend.bodies("/heartbeat"), [{"authorization": "failed"}])

    def test_disconnected_device_resumes_after_inventory_verification(self):
        merchant = Merchant(CODES)
        backend = Backend([inventory(2)])
        backend.config["paused_reason"] = "disconnected"
        report = self.cycle(merchant, backend)
        self.assertEqual(report["status"], "RUNNING")
        routes = [route for route, _, _ in backend.calls]
        self.assertLess(routes.index("/inventory"), routes.index("/resume"))
        self.assertEqual(backend.bodies("/resume"), [{}])
        self.assertEqual(merchant.uploads, [])

    def test_disconnected_device_with_inventory_mismatch_cannot_resume(self):
        merchant = Merchant(["f" * 32])
        backend = Backend([inventory(0, identity_verified=False, blocked=True)])
        backend.config["paused_reason"] = "disconnected"
        self.assert_paused(self.cycle(merchant, backend), "inventory_mismatch")
        self.assertEqual(backend.bodies("/resume"), [])
        self.assertEqual(backend.bodies("/claim"), [])

    def test_heartbeat_newly_disconnected_device_resumes_only_after_inventory_matches(self):
        merchant = Merchant(CODES)
        backend = Backend([inventory(2)])
        backend.config["paused_reason"] = ""
        backend.heartbeat = {"paused_reason": "disconnected"}
        report = self.cycle(merchant, backend)
        self.assertEqual(report["status"], "RUNNING")
        self.assertFalse(self.saved()["paused"])
        self.assertEqual(backend.config["paused_reason"], "")
        self.assertEqual([route for route, _, _ in backend.calls], [
            "/config", "/heartbeat", "/inventory", "/resume", "/claim",
        ])
        self.assertEqual(backend.bodies("/inventory")[0]["hashes"], hashes(CODES))
        self.assertEqual(backend.bodies("/resume"), [{}])
        self.assertEqual(merchant.uploads, [])

    def test_heartbeat_newly_disconnected_device_with_inventory_mismatch_stays_paused(self):
        for result in (inventory(0, identity_verified=False, blocked=True), inventory(0)):
            with self.subTest(result=result):
                restock.save_state(self.private, copy.deepcopy(self.state))
                merchant = Merchant(["f" * 32])
                backend = Backend([result])
                backend.config["paused_reason"] = ""
                backend.heartbeat = {"paused_reason": "disconnected"}
                self.assert_paused(self.cycle(merchant, backend), "inventory_mismatch")
                self.assertEqual(backend.config["paused_reason"], "")
                self.assertEqual(backend.bodies("/heartbeat"), [{"authorization": "verified"}])
                self.assertEqual(len(backend.bodies("/inventory")), 1)
                self.assertEqual(backend.bodies("/resume"), [])
                self.assertEqual(backend.bodies("/claim"), [])
                self.assertEqual(merchant.uploads, [])

    def test_disconnect_does_not_override_persisted_local_pause(self):
        self.state.update(paused=True, reason="inventory_mismatch")
        restock.save_state(self.private, self.state)
        backend = Backend([inventory(2)])
        backend.config["paused_reason"] = "disconnected"
        self.assert_paused(self.cycle(Merchant(CODES), backend), "inventory_mismatch")
        self.assertEqual(backend.calls, [])

    def test_setup_changes_only_selected_stock_policy_and_saves_private_device(self):
        other = PRODUCT | {"goods_id": 99, "target_stock": 18, "batch_size": 7, "enabled": False}
        status = {"enabled": True, "batches": [], "products": [copy.deepcopy(PRODUCT), copy.deepcopy(other)]}
        admin = Mock()

        def admin_call(route, body=None, method=None):
            if route == "/status":
                return copy.deepcopy(status)
            if route == "/devices":
                return {"device": {"id": DEVICE_ID}, "device_key": DEVICE_KEY}
            if route == "/config" and method == "PUT":
                return {}
            raise AssertionError("Unexpected setup request")

        admin.call.side_effect = admin_call
        key_path = self.root / "admin.key"
        key_path.write_text(ADMIN_KEY)
        key_path.chmod(0o600)
        config = {"target_stock": 4, "batch_size": 2, "goods_ids": [GOODS_ID]}
        with patch.object(restock, "Merchant") as merchant_class, patch.object(restock, "Backend", return_value=admin):
            merchant_class.return_value.load.return_value = Merchant()
            report = restock.setup(config, self.private, self.root / "session.json", key_path)
        self.assertEqual(report, {"status": "CONFIGURED", "products": 1, "target_stock": 4})
        updates = [call.args[1] for call in admin.call.call_args_list if call.args[0] == "/config"]
        self.assertEqual(updates, [{"enabled": True, "products": [PRODUCT | {"target_stock": 4}, other]}])
        creates = [call.args[1] for call in admin.call.call_args_list if call.args[0] == "/devices"]
        self.assertEqual(len(creates), 1)
        self.assertEqual(creates[0]["goods_ids"], [GOODS_ID])
        device_path = self.private / "device.json"
        self.assertEqual(device_path.stat().st_mode & 0o777, 0o600)
        self.assertEqual(json.loads(device_path.read_text())["key"], DEVICE_KEY)
        self.assertNotIn(ADMIN_KEY, device_path.read_text())
        self.assertNotIn(DEVICE_KEY, json.dumps(report) + (self.private / "state.json").read_text())
        self.assertEqual(set(self.saved()["pins"]), {str(GOODS_ID)})

    def test_setup_cannot_replace_unresolved_batches_or_rewrite_policy(self):
        key_path = self.root / "admin.key"
        key_path.write_text(ADMIN_KEY)
        key_path.chmod(0o600)
        original = (self.private / "state.json").read_bytes()
        config = {"target_stock": 4, "batch_size": 2, "goods_ids": [GOODS_ID]}
        for status in ("claimed", "uncertain"):
            with self.subTest(status=status):
                admin = Mock()
                admin.call.return_value = {"enabled": True, "batches": [batch(status=status)], "products": [copy.deepcopy(PRODUCT)]}
                with patch.object(restock, "Merchant") as merchant_class, patch.object(restock, "Backend", return_value=admin):
                    merchant_class.return_value.load.return_value = Merchant()
                    with self.assertRaises(restock.RestockError) as error:
                        restock.setup(config, self.private, self.root / "session.json", key_path)
                self.assertEqual(error.exception.kind, "uncertain")
                self.assertEqual([call.args[0] for call in admin.call.call_args_list], ["/status"])
                self.assertFalse((self.private / "device.json").exists())
                self.assertEqual((self.private / "state.json").read_bytes(), original)

    def test_exclusive_lock_blocks_a_second_process_and_releases_after_exit(self):
        script = (
            "import sys; sys.path.insert(0, sys.argv[1]); import ldxp_http_restock as r\n"
            "try:\n"
            "    with r.exclusive(sys.argv[2]): print('acquired')\n"
            "except r.RestockError as exc: print(exc.kind)\n"
        )
        command = [sys.executable, "-B", "-c", script, str(Path(restock.__file__).parent), str(self.private)]
        with restock.exclusive(self.private):
            blocked = subprocess.run(command, capture_output=True, text=True, check=True, timeout=10)
            self.assertEqual(blocked.stdout.strip(), "busy")
        released = subprocess.run(command, capture_output=True, text=True, check=True, timeout=10)
        self.assertEqual(released.stdout.strip(), "acquired")

    def test_only_fixed_local_site_is_allowed_in_config(self):
        path = self.root / "config.json"
        config = {"site": restock.probe.LOCAL_SITE, "target_stock": 2, "batch_size": 2,
                  "interval_seconds": 30, "goods_ids": [GOODS_ID]}
        path.write_text(json.dumps(config))
        self.assertEqual(restock.load_config(path), config)
        invalid = [{"site": site} for site in (
            "https://ai.rest2build.lol", "http://127.0.0.1:8080@evil.example",
            "http://localhost:8081", "http://127.0.0.1:8080/evil",
        )] + [{"goods_ids": [True]}, {"goods_ids": [42, 42]}, {"target_stock": 0}, {"batch_size": 21}]
        for changed in invalid:
            with self.subTest(changed=changed):
                path.write_text(json.dumps(config | changed))
                with self.assertRaises(restock.RestockError) as error:
                    restock.load_config(path)
                self.assertEqual(error.exception.kind, "invalid_config")

    def test_backend_secret_stays_on_fixed_loopback_origin(self):
        for admin, key, route, expected_header in (
            (False, DEVICE_KEY, "/config", "Authorization"),
            (True, ADMIN_KEY, "/status", "X-api-key"),
        ):
            with self.subTest(admin=admin):
                backend = restock.Backend(key, admin=admin)
                with patch.object(backend.http, "open", return_value=io.BytesIO(b'{"code":0,"data":{}}')) as request:
                    backend.call(route)
                sent = request.call_args.args[0]
                self.assertTrue(sent.full_url.startswith("http://127.0.0.1:8080/api/v1/"))
                self.assertIn(key, sent.get_header(expected_header))
                self.assertIsNone(sent.get_header("Merchant-token"))
                for foreign in ("https://evil.example/config", "//evil.example/config", "/batches/../config/start"):
                    with self.subTest(route=foreign), patch.object(backend.http, "open") as transport:
                        with self.assertRaises(restock.RestockError):
                            backend.call(foreign)
                        transport.assert_not_called()

    def test_merchant_upload_does_not_include_backend_credentials(self):
        merchant = restock.Merchant(self.root / "unused-session.json")
        merchant.token = "synthetic-merchant-token"
        with patch.object(merchant.http, "open", return_value=io.BytesIO(b'{"code":1,"data":{}}')) as transport:
            merchant.upload(GOODS_ID, CODES)
        request = transport.call_args.args[0]
        self.assertEqual(request.full_url, restock.probe.ORIGIN + "/merchantApi/GoodsCardStorage/add")
        self.assertEqual(request.get_header("Merchant-token"), "synthetic-merchant-token")
        self.assertIsNone(request.get_header("Authorization"))
        self.assertIsNone(request.get_header("X-api-key"))
        self.assertEqual(json.loads(request.data)["content"], "\n".join(CODES))

    def test_backend_redirect_and_remote_error_body_never_leak_secrets(self):
        backend = restock.Backend(DEVICE_KEY)
        secret = "synthetic-private-remote-response"
        failures = [HTTPError("http://127.0.0.1:8080", 401, secret, {}, io.BytesIO(secret.encode())),
                    restock.probe.ProbeError("upstream_rejected")]
        for failure in failures:
            with self.subTest(kind=type(failure).__name__), patch.object(backend.http, "open", side_effect=failure) as transport:
                with self.assertRaises(restock.RestockError) as error:
                    backend.call("/config")
                self.assertNotIn(secret, str(error.exception))
                self.assertNotIn(DEVICE_KEY, str(error.exception))
                self.assertEqual(transport.call_count, 1)
        for raw in (b'<html>synthetic-private-remote-response</html>',
                    b'{"code":9,"data":{},"message":"synthetic-private-remote-response"}'):
            with self.subTest(raw=raw), patch.object(backend.http, "open", return_value=io.BytesIO(raw)):
                with self.assertRaises(restock.RestockError) as error:
                    backend.call("/config")
                self.assertNotIn(secret, str(error.exception))
        with self.assertRaises(restock.probe.ProbeError):
            restock.probe.NoRedirect().redirect_request(None, None, 302, "", {}, "https://evil.example")

    def test_unexpected_exception_details_are_absent_from_report_and_persisted_state(self):
        merchant = Merchant()
        merchant.login_error = RuntimeError("synthetic-password " + DEVICE_KEY)
        report = self.cycle(merchant, Backend([]))
        self.assert_paused(report, "state_invalid")
        public = json.dumps(report) + (self.private / "state.json").read_text()
        self.assertNotIn("synthetic-password", public)
        self.assertNotIn(DEVICE_KEY, public)

    def test_attention_notifies_once_per_pause_and_success_clears_latch(self):
        paused = {"status": "PAUSED", "error": "login_required", "message": "重新登录。", "uploaded": 0}
        with patch.object(restock.sys, "platform", "darwin"), \
                patch.object(restock.subprocess, "run", return_value=SimpleNamespace(returncode=0)) as notify:
            self.assertTrue(restock.attention(paused, self.private, notify=True))
            restock.attention(paused | {"uploaded": 2}, self.private, notify=True)
            self.assertEqual(notify.call_count, 1)
            self.assertEqual(notify.call_args.args[0][0], "/usr/bin/osascript")
            restock.attention({"status": "RUNNING"}, self.private, notify=True)
            self.assertFalse((self.private / "attention.json").exists())
            restock.attention(paused, self.private, notify=True)
            self.assertEqual(notify.call_count, 2)

    def test_attention_unavailable_preserves_pause_facts_and_keeps_secrets_out_of_command(self):
        paused = {"status": "PAUSED", "error": "login_required", "message": "请重新登录。", "uploaded": 0,
                  "private_detail": DEVICE_KEY}
        original = (self.private / "state.json").read_bytes()
        failures = [OSError("notification service unavailable"), subprocess.TimeoutExpired("osascript", 10)]
        for failure in failures:
            with self.subTest(failure=type(failure).__name__):
                (self.private / "attention.json").unlink(missing_ok=True)
                with patch.object(restock.sys, "platform", "darwin"), \
                        patch.object(restock.subprocess, "run", side_effect=failure) as notify:
                    self.assertFalse(restock.attention(paused, self.private, notify=True))
                self.assertEqual((self.private / "state.json").read_bytes(), original)
                self.assertEqual(paused["status"], "PAUSED")
                self.assertEqual(paused["uploaded"], 0)
                self.assertNotIn(DEVICE_KEY, json.dumps(notify.call_args.args))
                alert = json.loads((self.private / "attention.json").read_text())
                self.assertEqual(alert, {"status": "PAUSED", "error": "login_required", "message": "请重新登录。"})
        with patch.object(restock.sys, "platform", "linux"), patch.object(restock.subprocess, "run") as notify:
            (self.private / "attention.json").unlink()
            restock.attention(paused, self.private, notify=True)
            notify.assert_not_called()
        self.assertEqual((self.private / "state.json").read_bytes(), original)

    def service_fixture(self):
        config = {"site": restock.probe.LOCAL_SITE, "target_stock": 2, "batch_size": 2,
                  "interval_seconds": 30, "goods_ids": [GOODS_ID]}
        config_path = self.root / "config.json"
        config_path.write_text(json.dumps(config))
        session_path = self.private / "session.json"
        restock.probe.write_private(session_path, {"token": "synthetic-merchant-token"})
        restock.probe.write_private(self.private / "device.json", {"key": DEVICE_KEY})
        args = SimpleNamespace(config=config_path, private_dir=self.private, session=session_path)
        fake_home = self.root / "fake-home"
        destination = fake_home / "Library/LaunchAgents/lol.rest2build.ldxp-http-local.plist"
        return args, config, fake_home, destination

    def test_service_runs_local_once_with_credential_paths_only(self):
        args, config, fake_home, destination = self.service_fixture()
        outcomes = [SimpleNamespace(returncode=1), SimpleNamespace(returncode=0), SimpleNamespace(returncode=0)]
        with patch.object(restock.Path, "home", return_value=fake_home), patch.object(restock.sys, "platform", "darwin"), \
                patch.object(restock.subprocess, "run", side_effect=outcomes) as launchctl:
            report = restock.install_service(args, restock.load_config(args.config))
        self.assertEqual(report["status"], "SCHEDULED")
        self.assertEqual(report["interval_seconds"], 30)
        spec = plistlib.loads(destination.read_bytes())
        command = spec["ProgramArguments"]
        self.assertEqual(command[:5], [sys.executable, "-B", str(Path(restock.__file__).resolve()), "run", "--notify"])
        self.assertEqual(command[command.index("--config") + 1], str(args.config.resolve()))
        self.assertEqual(command[command.index("--session") + 1], str(args.session.resolve()))
        self.assertEqual(json.loads(args.config.read_text())["site"], "http://127.0.0.1:8080")
        self.assertNotIn("--admin-key", command)
        for secret in (DEVICE_KEY, ADMIN_KEY, "synthetic-merchant-token"):
            self.assertNotIn(secret, destination.read_text())
        self.assertTrue(spec["RunAtLoad"])
        self.assertEqual(spec["StartInterval"], config["interval_seconds"])
        self.assertEqual(spec["Umask"], 0o077)
        self.assertEqual(destination.stat().st_mode & 0o777, 0o600)
        self.assertEqual([call.args[0][1] for call in launchctl.call_args_list], ["print", "bootstrap", "print"])

    def test_service_refuses_to_replace_unknown_plist_with_same_label(self):
        args, config, fake_home, destination = self.service_fixture()
        destination.parent.mkdir(parents=True)
        unknown = plistlib.dumps({"Label": "lol.rest2build.ldxp-http-local", "ProgramArguments": ["/usr/bin/true"]})
        destination.write_bytes(unknown)
        with patch.object(restock.Path, "home", return_value=fake_home), patch.object(restock.sys, "platform", "darwin"), \
                patch.object(restock.subprocess, "run") as launchctl:
            with self.assertRaises(restock.RestockError) as error:
                restock.install_service(args, config)
        self.assertEqual(error.exception.kind, "binding_changed")
        self.assertEqual(destination.read_bytes(), unknown)
        launchctl.assert_not_called()

    def test_service_refuses_to_unload_unknown_running_label_without_plist(self):
        args, config, fake_home, destination = self.service_fixture()
        occupied = SimpleNamespace(returncode=0, stdout=b"program = /usr/bin/true\n")
        with patch.object(restock.Path, "home", return_value=fake_home), patch.object(restock.sys, "platform", "darwin"), \
                patch.object(restock.subprocess, "run", return_value=occupied) as launchctl:
            with self.assertRaises(restock.RestockError) as error:
                restock.install_service(args, config)
        self.assertEqual(error.exception.kind, "binding_changed")
        self.assertFalse(destination.exists())
        self.assertEqual([call.args[0][1] for call in launchctl.call_args_list], ["print"])

    def test_service_bootstrap_failure_never_reports_scheduled(self):
        args, _, fake_home, _ = self.service_fixture()
        reports = []
        command = [restock.__file__, "install-service", "--config", str(args.config),
                   "--private-dir", str(args.private_dir), "--session", str(args.session)]
        outcomes = [SimpleNamespace(returncode=1), SimpleNamespace(returncode=1)]
        with patch.object(restock.Path, "home", return_value=fake_home), patch.object(restock.sys, "platform", "darwin"), \
                patch.object(restock.subprocess, "run", side_effect=outcomes), patch.object(sys, "argv", command), \
                patch.object(restock.os, "umask"), \
                patch.object(restock, "receipt", side_effect=lambda report, evidence: reports.append(report)):
            self.assertEqual(restock.main(), 1)
        self.assertEqual(len(reports), 1)
        self.assertEqual(reports[0]["status"], "PAUSED")
        self.assertEqual(reports[0]["error"], "backend_error")

    def test_service_cli_rejects_remote_config_before_changing_launch_agents(self):
        args, config, fake_home, destination = self.service_fixture()
        args.config.write_text(json.dumps(config | {"site": "https://ai.rest2build.lol"}))
        reports = []
        command = [restock.__file__, "install-service", "--config", str(args.config), "--private-dir", str(args.private_dir)]
        with patch.object(restock.Path, "home", return_value=fake_home), patch.object(restock.sys, "platform", "darwin"), \
                patch.object(restock.subprocess, "run") as launchctl, patch.object(sys, "argv", command), \
                patch.object(restock.os, "umask"), \
                patch.object(restock, "receipt", side_effect=lambda report, evidence: reports.append(report)):
            self.assertEqual(restock.main(), 1)
        self.assertEqual(reports[0]["error"], "invalid_config")
        self.assertFalse(destination.exists())
        launchctl.assert_not_called()


if __name__ == "__main__":
    unittest.main()

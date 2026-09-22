"""Synthetic original-batch recovery with sold delivery evidence and durable retry limits."""
import copy
import hashlib
import json
from pathlib import Path
import subprocess
import sys
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parent))
import test_ldxp_http_restock as fixtures

restock = fixtures.restock
GOODS_ID, CODES = fixtures.GOODS_ID, fixtures.CODES
NOW = 2_000_000_000
POLICY = {"enabled": True, "max_attempts": 2, "retry_seconds": 30}


class RecoveryMerchant(fixtures.Merchant):
    def __init__(self, codes=(), sold=(), events=None):
        super().__init__(codes)
        self.stock = {GOODS_ID: self.codes}
        self.sold = {GOODS_ID: list(sold)}
        self.events = events if events is not None else []
        self.snapshot_errors = {}
        self.after_snapshot = None
        self.before_upload = None
        self.card_ids = {}

    def card_id(self, goods_id, code):
        return self.card_ids.setdefault((goods_id, code), len(self.card_ids) + 1)

    def inventory_hashes(self, goods_id):
        self.events.append(("unsold", goods_id))
        return fixtures.hashes(self.stock.get(goods_id, []))

    def rows(self, route, filters):
        if route == "/merchantApi/Goods/list":
            return super().rows(route, filters)
        if route != "/merchantApi/goodsCardStorage/list":
            raise AssertionError("Unexpected merchant route")
        goods_id, status = filters["goods_id"], filters["status"]
        codes = (self.stock if status == "0" else self.sold).get(goods_id, [])
        return [{"id": self.card_id(goods_id, code), "secret": code, "status": status}
                for code in codes]

    def delivery_snapshot(self, goods_id):
        self.events.append(("snapshot", goods_id))
        if goods_id in self.snapshot_errors:
            raise self.snapshot_errors[goods_id]
        result = restock.Merchant.delivery_snapshot(self, goods_id)
        if self.after_snapshot:
            self.after_snapshot(goods_id)
        return result

    def upload(self, goods_id, codes):
        self.events.append(("upload", goods_id))
        if self.before_upload:
            self.before_upload(goods_id, codes)
        self.uploads.append((goods_id, list(codes)))
        accepted = codes if self.accept_count is None else codes[:self.accept_count]
        self.stock.setdefault(goods_id, []).extend(accepted)
        if self.upload_error:
            raise self.upload_error
        return {}


class RecoveryBackend(fixtures.Backend):
    """A fake delivery contract driven by remote card identities, not canned call counts."""
    def __init__(self, merchant, pending=None, events=None, claimed=None):
        super().__init__([], claimed)
        self.merchant = merchant
        self.events = events if events is not None else merchant.events
        self.pending = copy.deepcopy(pending if pending is not None else {
            GOODS_ID: fixtures.batch(status="uncertain"),
        })
        self.resolved = {}
        self.proof_version = 1
        self.retry_eligible = True
        self.known = {key: set(value["code_hashes"]) for key, value in self.pending.items()}
        if claimed:
            self.known.setdefault(claimed["goods_id"], set()).update(claimed["code_hashes"])

    def call(self, route, body=None, method=None):
        self.events.append(("backend", route, copy.deepcopy(body)))
        if route == "/inventory":
            self.calls.append((route, copy.deepcopy(body), method))
            goods_id = body["goods_id"]
            unsold = set(body["hashes"])
            known = self.known.get(goods_id, set())
            sold = {hashlib.sha256(code.encode()).hexdigest(): self.merchant.card_id(goods_id, code)
                    for code in self.merchant.sold.get(goods_id, [])}
            proofs = body.get("sold_proofs", [])
            if any(sold.get(proof["code_hash"]) != proof["card_id"] for proof in proofs):
                raise AssertionError("Sold proof does not identify a sold remote card")
            accepted = unsold | {proof["code_hash"] for proof in proofs}
            batch = self.pending.get(goods_id)
            resolved = bool(body.get("batch_id") and
                            self.resolved.get(goods_id) == body["batch_id"])
            if batch and body.get("batch_id") == batch["batch_id"] and set(batch["code_hashes"]) <= accepted:
                resolved = True
                self.resolved[goods_id] = batch["batch_id"]
                self.pending.pop(goods_id)
                batch = None
            result = fixtures.inventory(len(unsold & known), identity_verified=unsold <= known,
                                        blocked=bool(batch), batch_resolved=resolved)
            if batch:
                result["pending_batch"] = copy.deepcopy(batch)
            if "sold_proofs" in body and self.proof_version is not None:
                result["delivery_proof_version"] = self.proof_version
            if "sold_proofs" in body and self.retry_eligible is not None:
                result["retry_eligible"] = (self.retry_eligible.get(goods_id)
                                            if isinstance(self.retry_eligible, dict) else self.retry_eligible)
            return result
        if route.startswith("/batches/") and route.endswith("/start") and self.claimed:
            self.calls.append((route, copy.deepcopy(body), method))
            value = self.claimed | {"status": "uncertain"}
            self.pending[value["goods_id"]] = copy.deepcopy(value)
            return {"batch": value}
        result = super().call(route, body, method)
        if route == "/claim" and self.claimed:
            self.pending[self.claimed["goods_id"]] = copy.deepcopy(self.claimed)
        return result


class RecoveryTests(unittest.TestCase):
    setUp = fixtures.RestockTests.setUp
    saved = fixtures.RestockTests.saved
    assert_paused = fixtures.RestockTests.assert_paused

    def seed_pending(self, **changes):
        self.state.update(paused=True, reason="uncertain")
        self.state["products"][str(GOODS_ID)] = {
            "phase": "uncertain", "batch_id": "synthetic-batch",
        } | changes
        restock.save_state(self.private, self.state)

    def capability(self, **changes):
        value = {
            "schema": 1, "origin": restock.probe.ORIGIN,
            "scope": "goods_unsold_and_sold", "goods_ids": [GOODS_ID],
            "verified_at": NOW,
            "merchant_subject_sha256": hashlib.sha256(b"synthetic-merchant").hexdigest(),
            "cases": {"existing_unsold": True, "existing_sold": True, "concurrent_new": True},
        } | changes
        restock.probe.write_private(self.private / "dedup-capability.json", value)

    def cycle(self, merchant, backend, *, now=NOW, **kwargs):
        kwargs.setdefault("recovery", POLICY)
        with patch.object(restock.time, "time", return_value=now):
            return restock.cycle(self.private, merchant, backend, fixtures.DEVICE_ID, **kwargs)

    def assert_no_new_batch(self, backend):
        self.assertEqual(backend.bodies("/claim"), [])
        self.assertFalse(any(route.endswith("/start") for route, _, _ in backend.calls))

    def add_product(self, merchant, backend, *, goods_id=99, codes=None):
        codes = codes if codes is not None else ["c" * 32, "d" * 32]
        product = fixtures.PRODUCT | {"goods_id": goods_id, "external_url": f"https://www.ldxp.cn/goods/{goods_id}"}
        row = fixtures.ROW | {"id": goods_id, "name": "Another synthetic product"}
        backend.config["products"].append(product)
        backend.config["device"]["goods_ids"].append(goods_id)
        merchant.rows_data.append(row)
        merchant.stock[goods_id] = codes[:1]
        merchant.sold[goods_id] = []
        self.state["pins"][str(goods_id)] = {
            key: product[key] for key in ("goods_id", "cny_amount", "usd_credit", "external_url")
        } | {"merchant_name": row["name"]}
        other = fixtures.batch(batch_id="other-batch", goods_id=goods_id, status="uncertain",
                               codes=codes, code_hashes=[hashlib.sha256(c.encode()).hexdigest() for c in codes])
        backend.pending[goods_id] = other
        backend.known[goods_id] = set(other["code_hashes"])
        self.state["products"][str(goods_id)] = {"phase": "uncertain", "batch_id": other["batch_id"]}
        restock.save_state(self.private, self.state)
        return goods_id

    def test_original_batch_fully_unsold_is_read_back_without_retransmission(self):
        self.seed_pending()
        merchant = RecoveryMerchant(CODES)
        backend = RecoveryBackend(merchant)
        report = self.cycle(merchant, backend)
        self.assertEqual(report["status"], "RUNNING")
        self.assertEqual(report["uploaded"], 0)
        self.assertTrue(report["at_target"])
        self.assertEqual(merchant.uploads, [])
        self.assert_no_new_batch(backend)
        self.assertEqual(backend.bodies("/inventory")[0]["batch_id"], "synthetic-batch")
        routes = [route for route, _, _ in backend.calls]
        self.assertLess(routes.index("/inventory"), routes.index("/resume"))
        self.assertNotIn("batch_id", self.saved()["products"][str(GOODS_ID)])

    def test_confirmed_delivery_cannot_clear_a_protective_heartbeat_pause(self):
        for sold_count, during_snapshot in ((0, False), (2, False), (2, True)):
            with self.subTest(sold_count=sold_count, during_snapshot=during_snapshot):
                self.seed_pending()
                merchant = RecoveryMerchant(CODES[sold_count:], CODES[:sold_count])
                backend = RecoveryBackend(merchant)
                if during_snapshot:
                    merchant.after_snapshot = lambda _gid: backend.heartbeat.update(paused_reason="authorization_failed")
                else:
                    backend.heartbeat = {"paused_reason": "authorization_failed"}
                report = self.cycle(merchant, backend)
                self.assert_paused(report, "backend_auth")
                self.assertEqual(backend.bodies("/resume"), [])
                self.assertEqual(merchant.uploads, [])
                self.assertTrue(self.saved()["paused"])

    def test_adopted_new_claim_does_not_inherit_a_resolved_batch_retry_budget(self):
        self.state.update(paused=True, reason="network_error")
        self.state["products"][str(GOODS_ID)] = {
            "phase": "idle", "recovery_attempt": {"batch_id": "resolved-old-batch", "count": 2},
        }
        restock.save_state(self.private, self.state)
        self.capability()
        merchant = RecoveryMerchant()
        merchant.accept_count = 0
        backend = RecoveryBackend(merchant, pending={GOODS_ID: fixtures.batch()}, claimed=fixtures.batch())
        first = self.cycle(merchant, backend)
        self.assert_paused(first, "recovery_wait")
        self.assertNotIn("recovery_attempt", self.saved()["products"][str(GOODS_ID)])
        merchant.accept_count = None
        second = self.cycle(merchant, backend)
        self.assertEqual(second["status"], "RUNNING")
        attempt = self.saved()["products"][str(GOODS_ID)]["recovery_attempt"]
        self.assertEqual(attempt["batch_id"], "synthetic-batch")
        self.assertEqual(attempt["count"], 1)
        self.assertEqual(backend.bodies("/claim"), [])
        self.assertEqual(merchant.uploads, [(GOODS_ID, CODES), (GOODS_ID, CODES)])

    def test_sold_cards_are_delivery_proofs_and_never_sellable_stock(self):
        for sold_count in (1, 2):
            with self.subTest(sold_count=sold_count):
                self.seed_pending()
                merchant = RecoveryMerchant(CODES[sold_count:], CODES[:sold_count])
                backend = RecoveryBackend(merchant)
                report = self.cycle(merchant, backend)
                self.assertEqual(report["status"], "RUNNING")
                self.assertEqual(report["uploaded"], 0)
                self.assertFalse(report["at_target"])
                deliveries = [b for b in backend.bodies("/inventory") if "sold_proofs" in b]
                self.assertTrue(deliveries)
                for delivery in deliveries:
                    self.assertEqual(delivery["total"], 2 - sold_count)
                    self.assertEqual(delivery["hashes"], fixtures.hashes(CODES[sold_count:]))
                    self.assertEqual({p["code_hash"] for p in delivery["sold_proofs"]},
                                     set(fixtures.hashes(CODES[:sold_count])))
                self.assertEqual(self.saved()["products"][str(GOODS_ID)]["stock"], 2 - sold_count)
                self.assertEqual(merchant.uploads, [])
                self.assert_no_new_batch(backend)

    def test_only_missing_original_codes_upload_after_durable_attempt_latch(self):
        self.seed_pending()
        self.capability()
        merchant = RecoveryMerchant(sold=CODES[:1])
        backend = RecoveryBackend(merchant)
        def check_latch(goods_id, codes):
            attempt = self.saved()["products"][str(goods_id)]["recovery_attempt"]
            self.assertEqual(attempt["batch_id"], "synthetic-batch")
            self.assertEqual(attempt["count"], 1)
            self.assertEqual(attempt["missing_count"], 1)
            self.assertEqual(attempt["payload_sha256"], hashlib.sha256(CODES[1].encode()).hexdigest())
        merchant.before_upload = check_latch
        report = self.cycle(merchant, backend)
        self.assertEqual(report["status"], "RUNNING")
        self.assertEqual(merchant.uploads, [(GOODS_ID, CODES[1:])])
        self.assertEqual(report["uploaded"], 1)
        self.assert_no_new_batch(backend)
        actions = [(event[0], event[1]) for event in merchant.events]
        upload_index = actions.index(("upload", GOODS_ID))
        self.assertIn(("snapshot", GOODS_ID), actions[:upload_index])
        self.assertIn(("snapshot", GOODS_ID), actions[upload_index + 1:])
        self.assertGreater(actions.index(("backend", "/resume")), upload_index)
        self.assertTrue(all(code not in (self.private / "state.json").read_text() for code in CODES))

    def test_lost_upload_ack_is_resolved_by_readback_without_another_upload(self):
        self.seed_pending()
        self.capability()
        merchant = RecoveryMerchant(CODES[:1])
        merchant.upload_error = restock.probe.ProbeError("network_error")
        backend = RecoveryBackend(merchant)
        report = self.cycle(merchant, backend)
        self.assertEqual(report["status"], "RUNNING")
        self.assertEqual(merchant.uploads, [(GOODS_ID, CODES[1:])])
        self.assertEqual(report["uploaded"], 1)
        self.assert_no_new_batch(backend)
        self.assertNotIn("batch_id", self.saved()["products"][str(GOODS_ID)])

    def test_missing_expired_wrong_merchant_or_wrong_goods_capability_cannot_retransmit(self):
        changes = [None, {"verified_at": NOW - 30 * 86400 - 1},
                   {"merchant_subject_sha256": hashlib.sha256(b"another-merchant").hexdigest()},
                   {"goods_ids": [99]}, {"cases": {"existing_unsold": True, "existing_sold": False, "concurrent_new": True}}]
        for changed in changes:
            with self.subTest(changed=changed):
                self.seed_pending()
                (self.private / "dedup-capability.json").unlink(missing_ok=True)
                if changed is not None:
                    self.capability(**changed)
                merchant = RecoveryMerchant(CODES[:1])
                backend = RecoveryBackend(merchant)
                self.assert_paused(self.cycle(merchant, backend), "dedup_unverified")
                self.assertEqual(merchant.uploads, [])
                self.assertEqual(backend.bodies("/resume"), [])
                self.assert_no_new_batch(backend)

    def test_read_only_recovery_does_not_retransmit_or_resume(self):
        self.seed_pending()
        self.capability()
        merchant = RecoveryMerchant(CODES[:1])
        backend = RecoveryBackend(merchant)
        self.assert_paused(self.cycle(merchant, backend, write=False), "recovery_wait")
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/resume"), [])
        self.assert_no_new_batch(backend)

    def test_sold_proof_requires_backend_version_ack_before_resume(self):
        self.seed_pending()
        merchant = RecoveryMerchant(sold=CODES)
        backend = RecoveryBackend(merchant)
        backend.proof_version = None
        self.assert_paused(self.cycle(merchant, backend), "backend_upgrade_required")
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/resume"), [])

    def test_missing_code_retry_requires_current_backend_eligibility_and_version(self):
        for version, eligible in ((None, True), (1, None), (1, False)):
            with self.subTest(version=version, eligible=eligible):
                self.seed_pending()
                self.capability()
                merchant = RecoveryMerchant(CODES[:1])
                backend = RecoveryBackend(merchant)
                backend.proof_version, backend.retry_eligible = version, eligible
                report = self.cycle(merchant, backend)
                self.assertEqual(report["status"], "PAUSED")
                self.assertEqual(merchant.uploads, [])
                self.assertEqual(backend.bodies("/resume"), [])
                self.assert_no_new_batch(backend)

    def test_local_protected_pauses_are_not_automatically_released(self):
        for reason in ("manual", "login_required", "backend_auth", "binding_changed", "inventory_mismatch", "recovery_exhausted"):
            with self.subTest(reason=reason):
                self.seed_pending()
                self.state.update(paused=True, reason=reason)
                restock.save_state(self.private, self.state)
                merchant = RecoveryMerchant(CODES)
                backend = RecoveryBackend(merchant)
                self.assert_paused(self.cycle(merchant, backend), reason)
                self.assertEqual(backend.calls, [])
                self.assertEqual(merchant.uploads, [])

    def test_manual_pause_arriving_after_delivery_snapshot_blocks_resume(self):
        self.seed_pending()
        merchant = RecoveryMerchant(sold=CODES)
        merchant.after_snapshot = lambda _: restock.probe.write_private(self.private / "pause", {"requested_at": NOW})
        backend = RecoveryBackend(merchant)
        self.assert_paused(self.cycle(merchant, backend), "manual")
        self.assertEqual(backend.bodies("/resume"), [])
        self.assertEqual(merchant.uploads, [])

    def test_manual_pause_file_prevents_all_recovery_requests(self):
        self.seed_pending()
        restock.probe.write_private(self.private / "pause", {"requested_at": NOW})
        merchant = RecoveryMerchant(CODES)
        backend = RecoveryBackend(merchant)
        self.assert_paused(self.cycle(merchant, backend), "manual")
        self.assertEqual(backend.calls, [])
        self.assertEqual(merchant.uploads, [])

    def test_backend_protected_pause_never_auto_resumes_or_retransmits(self):
        for source in ("config", "heartbeat"):
            for reason in ("authorization_failed", "manual", "inventory_mismatch"):
                with self.subTest(source=source, reason=reason):
                    self.seed_pending()
                    self.capability()
                    merchant = RecoveryMerchant(CODES[:1])
                    backend = RecoveryBackend(merchant)
                    (backend.config if source == "config" else backend.heartbeat)["paused_reason"] = reason
                    report = self.cycle(merchant, backend)
                    self.assertEqual(report["status"], "PAUSED")
                    self.assertEqual(backend.bodies("/resume"), [])
                    self.assertEqual(merchant.uploads, [])
                    self.assert_no_new_batch(backend)

    def restart_cycle(self, now):
        script = """
import json, sys
sys.path.insert(0, sys.argv[1])
import test_ldxp_http_recovery as t
merchant = t.RecoveryMerchant(t.CODES[:1])
merchant.accept_count = 0
merchant.upload_error = t.restock.probe.ProbeError('network_error')
backend = t.RecoveryBackend(merchant)
with t.patch('socket.socket.connect', side_effect=AssertionError('No network in tests')), t.patch.object(t.restock.time, 'time', return_value=float(sys.argv[3])):
    report = t.restock.cycle(sys.argv[2], merchant, backend, t.fixtures.DEVICE_ID, recovery=t.POLICY)
print(json.dumps({'report': report, 'uploads': merchant.uploads, 'calls': backend.calls}))
"""
        result = subprocess.run([sys.executable, "-B", "-c", script, str(Path(__file__).parent), str(self.private), str(now)],
                                capture_output=True, text=True, timeout=10, check=True)
        return json.loads(result.stdout)

    def test_cooldown_and_max_attempts_survive_new_processes(self):
        self.seed_pending()
        self.capability()
        first = self.restart_cycle(NOW)
        self.assertEqual(first["report"]["error"], "recovery_wait")
        self.assertEqual(first["uploads"], [[GOODS_ID, CODES[1:]]])
        attempt = self.saved()["products"][str(GOODS_ID)]["recovery_attempt"]
        self.assertEqual((attempt["count"], attempt["next_at"]), (1, NOW + 30))
        waiting = self.restart_cycle(NOW + 1)
        self.assertEqual(waiting["report"]["error"], "recovery_wait")
        self.assertEqual(waiting["uploads"], [])
        second = self.restart_cycle(NOW + 31)
        self.assertEqual(second["uploads"], [[GOODS_ID, CODES[1:]]])
        self.assertEqual(second["report"]["error"], "recovery_wait")
        exhausted = self.restart_cycle(NOW + 100)
        self.assertEqual(exhausted["report"]["error"], "recovery_exhausted")
        self.assertEqual(exhausted["uploads"], [])
        self.assertEqual(self.saved()["products"][str(GOODS_ID)]["recovery_attempt"]["count"], 2)
        for result in (first, waiting, second, exhausted):
            self.assertFalse(any(route in ("/claim", "/resume") for route, _, _ in result["calls"]))

    def test_new_claim_does_not_inherit_old_recovery_attempt(self):
        self.state["products"][str(GOODS_ID)]["recovery_attempt"] = {
            "batch_id": "previous-batch", "count": 2, "next_at": NOW + 900,
        }
        restock.save_state(self.private, self.state)
        merchant = RecoveryMerchant()
        merchant.accept_count = 1
        merchant.upload_error = restock.probe.ProbeError("network_error")
        backend = RecoveryBackend(merchant, pending={}, claimed=fixtures.batch())
        self.assert_paused(self.cycle(merchant, backend), "uncertain")
        local = self.saved()["products"][str(GOODS_ID)]
        self.assertEqual(local["batch_id"], "synthetic-batch")
        self.assertNotIn("recovery_attempt", local)

    def test_every_product_delivery_snapshot_is_checked_before_any_upload(self):
        self.seed_pending()
        self.capability(goods_ids=[GOODS_ID, 99])
        merchant = RecoveryMerchant(CODES[:1])
        backend = RecoveryBackend(merchant)
        other = self.add_product(merchant, backend)
        merchant.snapshot_errors[other] = restock.RestockError("inventory_mismatch")
        self.assert_paused(self.cycle(merchant, backend), "inventory_mismatch")
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/resume"), [])
        self.assert_no_new_batch(backend)

    def test_all_retransmission_goods_need_capability_before_first_upload(self):
        self.seed_pending()
        self.capability()
        merchant = RecoveryMerchant(CODES[:1])
        backend = RecoveryBackend(merchant)
        self.add_product(merchant, backend)
        self.assert_paused(self.cycle(merchant, backend), "dedup_unverified")
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/resume"), [])

    def test_later_product_without_pending_batch_still_blocks_unsafe_write(self):
        self.seed_pending()
        self.capability(goods_ids=[GOODS_ID, 99])
        merchant = RecoveryMerchant(CODES[:1])
        backend = RecoveryBackend(merchant)
        other = self.add_product(merchant, backend)
        backend.pending.pop(other)
        backend.known[other] = set()
        self.state["products"][str(other)] = {"phase": "idle"}
        restock.save_state(self.private, self.state)
        self.assert_paused(self.cycle(merchant, backend), "inventory_mismatch")
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/resume"), [])
        self.assert_no_new_batch(backend)

    def test_every_missing_code_needs_backend_eligibility_before_first_upload(self):
        self.seed_pending()
        self.capability(goods_ids=[GOODS_ID, 99])
        merchant = RecoveryMerchant(CODES[:1])
        backend = RecoveryBackend(merchant)
        other = self.add_product(merchant, backend)
        backend.retry_eligible = {GOODS_ID: True, other: False}
        report = self.cycle(merchant, backend)
        self.assertEqual(report["status"], "PAUSED")
        self.assertEqual(merchant.uploads, [])
        self.assertEqual(backend.bodies("/resume"), [])
        self.assert_no_new_batch(backend)


class DeliverySnapshotTests(unittest.TestCase):
    setUp = fixtures.RestockTests.setUp

    def merchant(self):
        return restock.Merchant(self.root / "synthetic-unused-session.json")

    def row(self, code=CODES[0], card_id=1, status="0", **extra):
        return {"secret": code, "id": card_id, "status": status} | extra

    def test_snapshot_scans_both_statuses_twice_and_keeps_card_id_provenance(self):
        merchant = self.merchant()
        responses = [{"total": 1, "list": [self.row()]},
                     {"total": 1, "list": [self.row(CODES[1], 2, "1")]}] * 2
        with patch.object(merchant, "post", side_effect=responses) as request:
            snapshot = merchant.delivery_snapshot(GOODS_ID)
        self.assertEqual(snapshot, {"unsold": {fixtures.hashes(CODES[:1])[0]: 1},
                                    "sold": {fixtures.hashes(CODES[1:])[0]: 2}})
        self.assertEqual([call.args[1]["status"] for call in request.call_args_list], ["0", "1", "0", "1"])
        self.assertTrue(all(call.args[1]["goods_id"] == GOODS_ID for call in request.call_args_list))

    def test_duplicate_or_cross_status_card_identity_blocks_snapshot(self):
        scenarios = [
            [[self.row(), self.row(card_id=2)], []],
            [[self.row()], [self.row(card_id=2, status="1")]],
            [[self.row()], [self.row(CODES[1], 1, "1")]],
            [[self.row(status="1")], []],
        ]
        for groups in scenarios:
            with self.subTest(groups=groups):
                merchant = self.merchant()
                with patch.object(merchant, "rows", side_effect=groups * 2):
                    with self.assertRaises(restock.RestockError) as error:
                        merchant.delivery_snapshot(GOODS_ID)
                self.assertEqual(error.exception.kind, "inventory_mismatch")

    def test_card_moving_between_two_snapshots_stays_unconfirmed(self):
        merchant = self.merchant()
        with patch.object(merchant, "rows", side_effect=[[self.row()], [], [], [self.row(status="1")]]):
            with self.assertRaises(restock.RestockError) as error:
                merchant.delivery_snapshot(GOODS_ID)
        self.assertEqual(error.exception.kind, "recovery_wait")

    def test_changed_pagination_total_or_duplicate_page_record_blocks_snapshot(self):
        first = [self.row(f"{i:032x}", i + 1) for i in range(100)]
        for second in ({"total": 102, "list": [self.row("f" * 32, 101)]},
                       {"total": 101, "list": [first[0]]}):
            with self.subTest(second=second):
                merchant = self.merchant()
                with patch.object(merchant, "post", side_effect=[{"total": 101, "list": first}, second]), \
                        patch.object(restock.probe.time, "sleep"):
                    with self.assertRaises(restock.probe.ProbeError) as error:
                        merchant.delivery_snapshot(GOODS_ID)
                self.assertEqual(error.exception.kind, "invalid_response")

    def test_explicit_foreign_goods_row_cannot_be_delivery_evidence(self):
        merchant = self.merchant()
        groups = [[self.row(goods_id=99)], []] * 2
        with patch.object(merchant, "rows", side_effect=groups):
            with self.assertRaises(restock.RestockError) as error:
                merchant.delivery_snapshot(GOODS_ID)
        self.assertEqual(error.exception.kind, "inventory_mismatch")

    def test_target_stock_fits_one_page_without_dropping_the_second_scan(self):
        merchant = self.merchant()
        rows = [self.row(f"{i:032x}", i + 1) for i in range(999)]
        pages = [{"total": 999, "list": rows}, {"total": 0, "list": []}] * 2
        with patch.object(merchant, "post", side_effect=pages) as request, \
                patch.object(restock.probe.time, "sleep") as sleep:
            snapshot = merchant.delivery_snapshot(GOODS_ID)
        self.assertEqual(len(snapshot["unsold"]), 999)
        self.assertEqual(snapshot["sold"], {})
        self.assertEqual(request.call_count, 4)
        self.assertTrue(all(call.args[1]["pageSize"] == 1000 for call in request.call_args_list))
        sleep.assert_not_called()


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
"""Synthetic, isolated regressions for exported LDXP CNY settlement comparison."""

from contextlib import redirect_stderr, redirect_stdout
import csv
import io
import json
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[2] / "commerce"))
import reconcile_ldxp_settlement as settlement


def sale(order="order-001", day="2026-09-14", gross="5.15", fee="0.15", refund="0", net="5.00"):
    return dict(zip(settlement.REQUIRED_FIELDS, (order, day, gross, fee, refund, net)))


class SettlementTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.statement = self.root / "statement.csv"
        self.ledger = self.root / "ledger.csv"

    def write(self, path, rows, fields=None):
        with path.open("w", encoding="utf-8-sig", newline="") as handle:
            writer = csv.DictWriter(handle, fields or settlement.REQUIRED_FIELDS)
            writer.writeheader()
            writer.writerows(rows)

    def compare(self, statement, ledger):
        self.write(self.statement, statement)
        self.write(self.ledger, ledger)
        mapping = settlement.load_mapping(None)
        return settlement.reconcile(settlement.load_export(self.statement, "statement", mapping),
                                    settlement.load_export(self.ledger, "ledger", mapping))

    def run_cli(self, *extra):
        out, err = io.StringIO(), io.StringIO()
        with redirect_stdout(out), redirect_stderr(err):
            status = settlement.main(["--statement", str(self.statement), "--ledger", str(self.ledger),
                                      "--output-dir", str(self.root / "report"), *extra])
        return status, out.getvalue(), err.getvalue()

    def test_exact_money_and_gross_includes_buyer_fee(self):
        report = self.compare([sale()], [sale()])
        self.assertEqual("matched", report["status"])
        self.assertEqual({"gross": 515, "fee": 15, "refund": 0, "net": 500}, report["totals"]["statement_reported_fen"])
        self.assertEqual(12345678901234567890123456789001, settlement.parse_fen("123456789012345678901234567890.01"))
        self.assertEqual("-0.15", settlement.cny(-15))

    def test_later_refund_and_signed_fee_rebate_preserve_daily_net(self):
        rows = [sale(), sale(day="2026-09-17", gross="0", fee="-0.15", refund="5.15", net="-5.00")]
        report = self.compare(rows, rows)
        self.assertEqual("matched", report["status"])
        self.assertEqual([500, -500], [day["statement_reported_fen"]["net"] for day in report["daily"]])
        self.assertEqual(0, report["totals"]["statement_reported_fen"]["net"])

    def test_equal_totals_do_not_hide_order_amount_mismatches(self):
        report = self.compare([sale("A"), sale("B", gross="10.30", fee="0.30", net="10")],
                              [sale("B"), sale("A", gross="10.30", fee="0.30", net="10")])
        self.assertEqual("discrepancies", report["status"])
        self.assertFalse(any(report["totals"]["delta_reported_fen"].values()))
        self.assertEqual(2, sum(issue["kind"] == "amount_mismatch" for issue in report["issues"]))

    def test_duplicate_rows_are_not_silently_deduplicated(self):
        for changed in (False, True):
            with self.subTest(conflicting=changed):
                duplicate = sale(gross="6.15", net="6") if changed else sale()
                report = self.compare([sale(), duplicate], [sale()])
                kind = "conflicting_duplicate_rows" if changed else "duplicate_rows"
                self.assertIn(kind, [issue["kind"] for issue in report["issues"]])
                self.assertIsNone(report["orders"][0]["delta_fen"])
                self.assertEqual(2, report["daily"][0]["statement_record_count"])
                self.assertEqual(1130 if changed else 1030, report["totals"]["statement_reported_fen"]["gross"])

    def test_equal_bad_arithmetic_still_fails(self):
        report = self.compare([sale(net="5.10")], [sale(net="5.10")])
        self.assertEqual("discrepancies", report["status"])
        self.assertEqual(2, sum(issue["kind"] == "arithmetic_mismatch" for issue in report["issues"]))
        self.assertEqual("discrepancy", report["orders"][0]["status"])

    def test_missing_rows_and_accounting_date_mismatch_are_explicit(self):
        report = self.compare([sale(day="2026-09-15"), sale("merchant-only")],
                              [sale(day="2026-09-14"), sale("ledger-only")])
        kinds = {issue["kind"] for issue in report["issues"]}
        self.assertTrue({"settlement_date_mismatch", "missing_in_statement", "missing_in_ledger"} <= kinds)
        self.assertEqual("discrepancies", report["status"])

    def test_precision_nonfinite_negative_gross_and_bad_dates_fail(self):
        for field, value in (("gross_cny", "5.151"), ("gross_cny", "NaN"),
                             ("gross_cny", "Infinity"), ("gross_cny", "1e2"),
                             ("gross_cny", "-5"), ("refund_cny", "-1"),
                             ("settlement_date", "2026-02-30"), ("external_order_no", "order-001 ")):
            with self.subTest(field=field, value=value):
                row = sale()
                row[field] = value
                report = self.compare([row], [sale()])
                self.assertEqual("discrepancies", report["status"])
                self.assertFalse(report["totals_complete"])
                self.assertIn("invalid_row", [issue["kind"] for issue in report["issues"]])

    def test_currency_is_not_converted_or_assumed_from_usd_credit(self):
        self.write(self.statement, [{**sale(), "currency": "USD"}], [*settlement.REQUIRED_FIELDS, "currency"])
        result = settlement.load_export(self.statement, "statement", settlement.load_mapping(None))
        self.assertEqual([], result["rows"])
        self.assertEqual("currency", result["issues"][0]["field"])
        self.statement.write_text(",".join([*settlement.REQUIRED_FIELDS, "currency"]) + "\n" + ",".join(sale().values()) + "\n")
        result = settlement.load_export(self.statement, "statement", settlement.load_mapping(None))
        self.assertEqual("currency", result["issues"][0]["field"])
        with self.assertRaises(settlement.InputError):
            settlement.load_export(self.statement, "statement", {**settlement.load_mapping(None), "currency": "missing-currency"})

    def test_actual_header_mapping_and_date_format_generate_reviewable_files(self):
        mapping = dict(zip(settlement.REQUIRED_FIELDS, ("订单号", "结算日期", "买家实付", "手续费", "退款", "净额")))
        row = {mapping[key]: value for key, value in sale(day="2026/09/14").items()}
        self.write(self.statement, [row], list(mapping.values()))
        self.write(self.ledger, [sale()])
        map_path = self.root / "merchant-map.json"
        map_path.write_text(json.dumps(mapping), encoding="utf-8")
        before = (self.statement.read_bytes(), self.ledger.read_bytes())
        status, _, error = self.run_cli("--statement-map", str(map_path), "--statement-date-format", "%Y/%m/%d")
        self.assertEqual((0, ""), (status, error))
        self.assertEqual(before, (self.statement.read_bytes(), self.ledger.read_bytes()))
        self.assertEqual({"reconciliation.json", "orders.csv", "daily.csv", "report.md"},
                         {path.name for path in (self.root / "report").iterdir()})
        report = json.loads((self.root / "report/reconciliation.json").read_text())
        self.assertEqual(64, len(report["sources"]["statement"]["sha256"]))
        self.assertEqual(mapping, report["sources"]["statement"]["mapping"])
        self.assertEqual(0o600, (self.root / "report/reconciliation.json").stat().st_mode & 0o777)

    def test_csv_output_preserves_identifier_text_and_excludes_unmapped_secrets(self):
        rows = [sale("=1+1"), sale("00001234")]
        self.write(self.statement, [{**row, "redeem_code": "private-code-canary"} for row in rows],
                   [*settlement.REQUIRED_FIELDS, "redeem_code"])
        self.write(self.ledger, rows)
        status, stdout, stderr = self.run_cli()
        self.assertEqual(0, status)
        text = "".join(path.read_text() for path in (self.root / "report").iterdir())
        self.assertNotIn("private-code-canary", text + stdout + stderr)
        csv_text = (self.root / "report/orders.csv").read_text()
        self.assertIn("'=1+1", csv_text)
        self.assertIn("'00001234", csv_text)
        self.assertNotIn("=1+1", stdout)

    def test_empty_inputs_fail_and_discrepancy_exit_code_preserves_reports(self):
        self.assertEqual("discrepancies", self.compare([], [])["status"])
        self.write(self.statement, [sale()])
        self.write(self.ledger, [])
        status, _, _ = self.run_cli()
        self.assertEqual(1, status)
        self.assertTrue((self.root / "report/reconciliation.json").exists())

    def test_structural_errors_same_source_and_existing_output_fail_closed(self):
        self.write(self.ledger, [sale()])
        self.statement.write_text("private-code-canary,private-code-canary\n5,5\n")
        status, _, stderr = self.run_cli()
        self.assertEqual(2, status)
        self.assertNotIn("private-code-canary", stderr)
        self.assertFalse((self.root / "report").exists())
        self.write(self.statement, [sale()])
        status, _, _ = self.run_cli("--ledger", str(self.statement))
        self.assertEqual(2, status)
        self.assertEqual(0, self.run_cli()[0])
        original = (self.root / "report/reconciliation.json").read_bytes()
        self.assertEqual(2, self.run_cli()[0])
        self.assertEqual(original, (self.root / "report/reconciliation.json").read_bytes())


if __name__ == "__main__":
    unittest.main()

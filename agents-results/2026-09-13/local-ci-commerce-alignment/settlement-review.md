# LDXP settlement reconciliation delivery

Implemented `tools/commerce/reconcile_ldxp_settlement.py`, a Python standard-library CLI that independently reads a merchant CSV statement and authoritative local CSV sales ledger. It makes no network requests and performs no database, account, payment, or refund writes. Usage is in `tools/commerce/README.md`; the settlement-only section of `docs/LDXP_SALES_CHANNEL_DEVELOPMENT.md` now links to this actual capability and retains the unverified merchant-API/automation boundary.

The accounting contract is CNY integer fen: net = gross - fee - refund. Gross is actual buyer payment, not product face value or Sub2API USD credit. Positive fees are charges, negative fees are rebates; refunds and gross are nonnegative. A ¥5.15 payment less ¥0.15 fee correctly yields ¥5.00. A later refund uses its actual accounting date and may create negative net. The CLI never infers settlement dates from order creation.

Rows match on settlement/accounting date and external order number. Missing rows, mismatched dates, mismatched money, identical duplicates, conflicting duplicates, malformed amounts/dates/currencies, and arithmetic errors block a matched result. Equal aggregate totals cannot hide order-level errors. Totals include all parseable reported rows, including duplicates; invalid rows are excluded with an explicit incomplete-total flag. Ambiguous duplicate rows never receive an arbitrarily selected per-order comparison.

The output directory must be new. It contains `reconciliation.json` with each discrepancy reason and input SHA-256/column-map provenance, `orders.csv`, `daily.csv`, and a concise Chinese `report.md`. Reports use restricted file permissions. CSV order identifiers are text-protected; JSON retains exact accepted identifiers. Unmapped columns such as redemption codes are not copied into outputs, and console output does not print order rows. Supplied column maps and encodings support real export field names without inventing a merchant format.

Machine verification:

```text
python3 -m unittest discover -s tools/quality/tests -p 'test_liandong_settlement.py' -v
12 tests: PASS, exit 0
```

Complete output is retained in `settlement-tests-final.log`. Tests use synthetic data in isolated temporary files; they exercise the CLI report path and exit codes as well as comparison logic. Coverage includes ¥5.15/¥0.15/¥5.00, exact amounts beyond floating-point/Decimal-default precision, cross-day refunds, signed fee rebates, duplicates/conflicts/missing rows, equal totals with wrong orders, Chinese headers, accounting-date formats, bad currency/precision, source/output preservation, private-column exclusion, and CSV identifier protection. The new test file resides under the existing local-CI discovery directory `tools/quality/tests`; the runner was not modified. `git diff --check` passed.

No real merchant statement or authoritative local sales ledger was provided to this lane, so no live daily settlement has been accepted. Existing code inventory/redemption data is not silently treated as a sales ledger. A file comparison cannot prove funds were settled or refunded. Real statement import, source authenticity, money settlement, and human acceptance remain pending their actual inputs and observations.

Owned source changes: the new CLI, its `tools/commerce/README.md`, `tools/quality/tests/test_liandong_settlement.py`, and only the settlement-related paragraphs of `docs/LDXP_SALES_CHANNEL_DEVELOPMENT.md`. No commit, deployment, runtime registration, or inventory-service/UI edit was performed by this lane.

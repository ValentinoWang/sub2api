#!/usr/bin/env python3
"""Compare two exported CNY ledgers without network access or financial writes."""

from __future__ import annotations

import argparse
import csv
from datetime import datetime
import hashlib
import io
import json
from pathlib import Path
import re
import sys


MONEY_FIELDS = ("gross", "fee", "refund", "net")
INPUT_MONEY_FIELDS = tuple(field + "_cny" for field in MONEY_FIELDS)
REQUIRED_FIELDS = ("external_order_no", "settlement_date", *INPUT_MONEY_FIELDS)
ALL_FIELDS = (*REQUIRED_FIELDS, "currency")
MONEY_PATTERN = re.compile(r"^[+-]?[0-9]+(?:\.[0-9]{1,2})?$")


class InputError(ValueError):
    """A structural input error; its message never contains a raw CSV row."""


def parse_fen(value: str) -> int:
    """Parse decimal CNY directly to integer fen, with no rounding or floats."""
    if not MONEY_PATTERN.fullmatch(value):
        raise ValueError("expected decimal CNY with at most two fraction digits")
    sign = -1 if value.startswith("-") else 1
    whole, _, fraction = value.lstrip("+-").partition(".")
    return sign * (int(whole) * 100 + int(fraction.ljust(2, "0")))


def cny(fen: int) -> str:
    return f"{'-' if fen < 0 else ''}{abs(fen) // 100}.{abs(fen) % 100:02d}"


def load_mapping(path: Path | None) -> dict[str, str]:
    if path is None:
        return {name: name for name in REQUIRED_FIELDS}
    try:
        mapping = json.loads(path.read_text(encoding="utf-8"))
    except (OSError, UnicodeError, json.JSONDecodeError) as exc:
        raise InputError("cannot read a UTF-8 JSON column map") from exc
    if not isinstance(mapping, dict) or set(mapping) - set(ALL_FIELDS):
        raise InputError("column map contains unsupported fields")
    if set(REQUIRED_FIELDS) - set(mapping):
        raise InputError("column map must name every required field")
    if any(not isinstance(value, str) or not value for value in mapping.values()):
        raise InputError("column map values must be non-empty header names")
    if len(set(mapping.values())) != len(mapping):
        raise InputError("column map cannot reuse one header for multiple fields")
    return mapping


def load_export(path: Path, source: str, mapping: dict[str, str],
                encoding: str = "utf-8-sig", date_format: str = "%Y-%m-%d") -> dict:
    try:
        content = path.read_bytes()
        reader = csv.DictReader(io.StringIO(content.decode(encoding), newline=""), strict=True)
        headers = reader.fieldnames
        if not headers or len(headers) != len(set(headers)):
            raise InputError(f"{source}: missing or duplicate CSV headers")
        if any(mapping[field] not in headers for field in REQUIRED_FIELDS):
            raise InputError(f"{source}: required mapped CSV headers are missing")
        if "currency" in mapping and mapping["currency"] not in headers:
            raise InputError(f"{source}: explicitly mapped currency header is missing")
        currency_header = mapping.get("currency", "currency")
        rows, issues, record_count = [], [], 0
        for record_number, raw in enumerate(reader, start=2):
            record_count += 1
            if None in raw or any(raw.get(mapping[field]) is None for field in REQUIRED_FIELDS):
                issues.append({"kind": "invalid_row", "source": source,
                               "record_number": record_number, "field": "column_count"})
                continue
            values = {field: raw[mapping[field]] if field == "external_order_no" else raw[mapping[field]].strip()
                      for field in REQUIRED_FIELDS}
            field = "external_order_no"
            try:
                if not values[field] or values[field] != values[field].strip() or any(ord(char) < 32 for char in values[field]):
                    raise ValueError("invalid order identifier")
                field = "settlement_date"
                parsed_date = datetime.strptime(values[field], date_format)
                if parsed_date.strftime(date_format) != values[field]:
                    raise ValueError("date must match configured export format")
                day = parsed_date.date().isoformat()
                field = "currency"
                if currency_header in headers and (raw.get(currency_header) is None or raw[currency_header].strip() != "CNY"):
                    raise ValueError("only CNY accounting is supported")
                amounts = {}
                for field in MONEY_FIELDS:
                    amounts[field] = parse_fen(values[field + "_cny"])
                    if field in ("gross", "refund") and amounts[field] < 0:
                        raise ValueError("gross and refunds must be nonnegative")
            except (ValueError, OverflowError):
                issues.append({"kind": "invalid_row", "source": source,
                               "record_number": record_number, "field": field})
                continue
            row = {"external_order_no": values["external_order_no"],
                   "settlement_date": day, "record_number": record_number,
                   "amounts_fen": amounts}
            rows.append(row)
            expected_net = amounts["gross"] - amounts["fee"] - amounts["refund"]
            if amounts["net"] != expected_net:
                issues.append({"kind": "arithmetic_mismatch", "source": source,
                               "external_order_no": row["external_order_no"],
                               "settlement_date": day, "record_number": record_number,
                               "expected_net_fen": expected_net,
                               "reported_net_fen": amounts["net"]})
        if record_count == 0:
            issues.append({"kind": "empty_input", "source": source})
    except (OSError, UnicodeError, LookupError, csv.Error) as exc:
        raise InputError(f"{source}: cannot read a well-formed CSV with the configured encoding") from exc
    return {"source": source, "sha256": hashlib.sha256(content).hexdigest(),
            "byte_count": len(content), "record_count": record_count,
            "valid_record_count": len(rows), "mapping": mapping,
            "currency_header": currency_header if currency_header in headers else None,
            "encoding": encoding, "date_format": date_format,
            "rows": rows, "issues": issues}


def sums(rows: list[dict]) -> dict[str, int]:
    return {field: sum(row["amounts_fen"][field] for row in rows) for field in MONEY_FIELDS}


def difference(statement: dict[str, int], ledger: dict[str, int]) -> dict[str, int]:
    return {field: statement[field] - ledger[field] for field in MONEY_FIELDS}


def reconcile(statement: dict, ledger: dict) -> dict:
    issues = [*statement["issues"], *ledger["issues"]]
    indexes, dates = {}, {}
    for export in (statement, ledger):
        index, order_dates = {}, {}
        for row in export["rows"]:
            key = (row["settlement_date"], row["external_order_no"])
            index.setdefault(key, []).append(row)
            order_dates.setdefault(key[1], set()).add(key[0])
        indexes[export["source"]], dates[export["source"]] = index, order_dates
    for order in sorted(dates["statement"].keys() & dates["ledger"].keys()):
        if dates["statement"][order] != dates["ledger"][order]:
            issues.append({"kind": "settlement_date_mismatch", "external_order_no": order,
                           "statement_dates": sorted(dates["statement"][order]),
                           "ledger_dates": sorted(dates["ledger"][order])})
    orders = []
    for day, order in sorted(indexes["statement"].keys() | indexes["ledger"].keys()):
        key = (day, order)
        left, right = indexes["statement"].get(key, []), indexes["ledger"].get(key, [])
        order_issues = []
        for source, rows in (("statement", left), ("ledger", right)):
            if not rows:
                order_issues.append({"kind": f"missing_in_{source}"})
            if len(rows) > 1:
                unique = {tuple(row["amounts_fen"][field] for field in MONEY_FIELDS) for row in rows}
                order_issues.append({"kind": "duplicate_rows" if len(unique) == 1 else "conflicting_duplicate_rows",
                                     "source": source, "record_numbers": [row["record_number"] for row in rows]})
        delta = None
        if len(left) == len(right) == 1:
            delta = difference(left[0]["amounts_fen"], right[0]["amounts_fen"])
            if any(delta.values()):
                order_issues.append({"kind": "amount_mismatch", "delta_fen": delta})
        for issue in order_issues:
            issues.append({**issue, "external_order_no": order, "settlement_date": day})
        arithmetic_bad = any(row["amounts_fen"]["net"] !=
                             row["amounts_fen"]["gross"] - row["amounts_fen"]["fee"] - row["amounts_fen"]["refund"]
                             for row in left + right)
        orders.append({"external_order_no": order, "settlement_date": day,
                       "status": "discrepancy" if order_issues or arithmetic_bad else "matched",
                       "statement_rows": left, "ledger_rows": right, "delta_fen": delta})
    daily = []
    for day in sorted({row["settlement_date"] for row in statement["rows"] + ledger["rows"]}):
        left = [row for row in statement["rows"] if row["settlement_date"] == day]
        right = [row for row in ledger["rows"] if row["settlement_date"] == day]
        daily.append({"settlement_date": day, "statement_record_count": len(left),
                      "ledger_record_count": len(right), "statement_reported_fen": sums(left),
                      "ledger_reported_fen": sums(right), "delta_reported_fen": difference(sums(left), sums(right))})
    left_total, right_total = sums(statement["rows"]), sums(ledger["rows"])
    return {"schema_version": 1, "status": "matched" if not issues else "discrepancies",
            "currency": "CNY", "money_unit": "fen", "net_definition": "gross - fee - refund",
            "fee_sign": "positive charge; negative fee rebate",
            "date_basis": "exported settlement/accounting date; no order-creation-date inference or timezone conversion",
            "totals_basis": "all parseable input records, including duplicates; invalid rows excluded and reported",
            "totals_complete": not any(issue["kind"] == "invalid_row" for issue in issues),
            "sources": {export["source"]: {k: v for k, v in export.items() if k not in ("rows", "issues")}
                        for export in (statement, ledger)},
            "totals": {"statement_reported_fen": left_total, "ledger_reported_fen": right_total,
                       "delta_reported_fen": difference(left_total, right_total)},
            "daily": daily, "orders": orders, "issues": issues}


def safe_csv_identifier(value: str) -> str:
    return "'" + value


def write_reports(report: dict, output_dir: Path) -> None:
    output_dir.mkdir(mode=0o700, parents=True, exist_ok=False)
    def open_output(name: str):
        path = output_dir / name
        descriptor = path.open("x", encoding="utf-8", newline="")
        path.chmod(0o600)
        return descriptor
    with open_output("reconciliation.json") as handle:
        json.dump(report, handle, ensure_ascii=False, indent=2)
        handle.write("\n")
    with open_output("orders.csv") as handle:
        writer = csv.writer(handle)
        writer.writerow(["settlement_date", "external_order_no", "status", "source", "record_number", *INPUT_MONEY_FIELDS])
        for order in report["orders"]:
            for source in ("statement", "ledger"):
                for row in order[source + "_rows"]:
                    writer.writerow([order["settlement_date"], safe_csv_identifier(order["external_order_no"]),
                                     order["status"], source, row["record_number"],
                                     *(cny(row["amounts_fen"][field]) for field in MONEY_FIELDS)])
    with open_output("daily.csv") as handle:
        writer = csv.writer(handle)
        writer.writerow(["settlement_date", "source", *INPUT_MONEY_FIELDS])
        for day in report["daily"]:
            for source in ("statement", "ledger", "delta"):
                writer.writerow([day["settlement_date"], source,
                                 *(cny(day[source + "_reported_fen"][field]) for field in MONEY_FIELDS)])
    with open_output("report.md") as handle:
        status_label = "两份输入逐笔一致" if report["status"] == "matched" else "存在差异"
        handle.write(f"# 链动小铺日结账单核对\n\n核对结果：**{status_label}**。问题数：{len(report['issues'])}。\n\n")
        if not report["totals_complete"]:
            handle.write("**合计不完整：存在无法解析的行，请先处理 JSON 中的无效行问题。**\n\n")
        handle.write("以下金额均为人民币元。净额 = 买家实付总额 - 手续费 - 退款；手续费为正表示收取，为负表示退回。日期采用导出的结算／会计日期。\n\n")
        handle.write("合计包含所有可解析的输入行，包括重复行；无法解析的行未计入。存在问题时，不能将下列合计视为已确认结算额。\n\n")
        handle.write("| 来源 | 买家实付（元） | 手续费（元） | 退款（元） | 净额（元） |\n|---|---:|---:|---:|---:|\n")
        labels = {"statement": "商户导出", "ledger": "本地账本", "delta": "差额（商户减本地）"}
        for source in ("statement", "ledger", "delta"):
            amounts = report["totals"][source + "_reported_fen"]
            handle.write("| " + labels[source] + " | " + " | ".join(cny(amounts[field]) for field in MONEY_FIELDS) + " |\n")
        handle.write("\n逐订单原始金额见 [orders.csv](orders.csv)，每日合计见 [daily.csv](daily.csv)。每笔错误原因、双方行号及输入文件哈希见 [reconciliation.json](reconciliation.json) 的 `issues`。\n\n两份文件匹配不证明来源真实、商户已付款或退款已到账，实际资金结算仍需核实。\n")


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--statement", required=True, type=Path)
    parser.add_argument("--ledger", required=True, type=Path)
    parser.add_argument("--output-dir", required=True, type=Path, help="new output directory; existing paths are never overwritten")
    for source in ("statement", "ledger"):
        parser.add_argument(f"--{source}-map", type=Path)
        parser.add_argument(f"--{source}-encoding", default="utf-8-sig")
        parser.add_argument(f"--{source}-date-format", default="%Y-%m-%d")
    args = parser.parse_args(argv)
    try:
        if args.statement.samefile(args.ledger):
            raise InputError("statement and ledger must be distinct source files")
        exports = [load_export(getattr(args, source), source, load_mapping(getattr(args, source + "_map")),
                               getattr(args, source + "_encoding"), getattr(args, source + "_date_format"))
                   for source in ("statement", "ledger")]
        report = reconcile(*exports)
        write_reports(report, args.output_dir)
    except (InputError, OSError) as exc:
        message = str(exc) if isinstance(exc, InputError) else "cannot read inputs or create a new report directory"
        print(message, file=sys.stderr)
        return 2
    print(f"{report['status']}: {len(report['issues'])} issue(s); reports written to {args.output_dir}")
    return 0 if report["status"] == "matched" else 1


if __name__ == "__main__":
    sys.exit(main())

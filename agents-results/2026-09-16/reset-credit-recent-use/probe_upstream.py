"""Read existing OAuth reset-credit metadata without consuming or refreshing tokens."""
import argparse
from collections import Counter
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import subprocess

from curl_cffi import requests


def probe(account_id, history_only=False):
    # Only these local accounts were checked as active OAuth accounts without proxies.
    if account_id not in {6, 10, 11}:
        raise ValueError("Account outside inspected probe scope")
    sql = (
        "SELECT jsonb_build_object('token',credentials->>'access_token',"
        "'account',credentials->>'chatgpt_account_id') FROM accounts "
        f"WHERE id={account_id} AND platform='openai' AND type='oauth' "
        "AND deleted_at IS NULL AND proxy_id IS NULL"
    )
    db = subprocess.run(
        ["docker", "exec", "sub2api-postgres", "sh", "-c",
         'exec psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atc "$1"', "probe", sql],
        capture_output=True, text=True, check=True,
    )
    auth = json.loads(db.stdout)
    headers = {
        "authorization": "Bearer " + auth["token"], "chatgpt-account-id": auth["account"],
        "openai-beta": "codex-1", "originator": "Codex Desktop", "oai-language": "zh-CN",
        "accept": "application/json", "sec-fetch-site": "none", "sec-fetch-mode": "no-cors",
        "sec-fetch-dest": "empty", "priority": "u=4, i",
    }
    results = []
    endpoints = ["rate-limit-reset-credits/history"] if history_only else ["rate-limit-reset-credits", "rate-limit-reset-credits/history", "usage"]
    for suffix in endpoints:
        result = {"local_account_id": account_id, "endpoint": suffix, "method": "GET",
                  "checked_at": datetime.now(timezone.utc).isoformat()}
        try:
            response = requests.get("https://chatgpt.com/backend-api/wham/" + suffix,
                                    headers=headers, impersonate="chrome", timeout=20, allow_redirects=False)
            result.update(status=response.status_code, body_sha256=hashlib.sha256(response.content).hexdigest())
            if response.status_code == 200:
                body = response.json()
                result["top_level_keys"] = sorted(body) if isinstance(body, dict) else ["array"]
                if suffix == "usage":
                    credits = body.get("rate_limit_reset_credits")
                    result["reset_credit_fields"] = sorted(credits) if isinstance(credits, dict) else []
                    if isinstance(credits, dict) and type(credits.get("available_count")) is int:
                        result["available_count"] = credits["available_count"]
                else:
                    for key in ["history_enabled", "available_count", "total_earned_count"]:
                        if isinstance(body, dict) and type(body.get(key)) in {bool, int}:
                            result[key] = body[key]
                    rows = body if isinstance(body, list) else next(
                        (body[key] for key in ["credits", "rate_limit_reset_credits", "items", "data", "events"]
                         if isinstance(body.get(key), list)), [])
                    if isinstance(body, dict):
                        result["has_next_cursor"] = bool(body.get("next_cursor"))
                        for key in ["as_of", "window_start"]:
                            if isinstance(body.get(key), str):
                                try:
                                    result[key] = datetime.fromisoformat(body[key].replace("Z", "+00:00")).isoformat()
                                except ValueError:
                                    result[key] = "unparseable"
                    result["record_count"] = len(rows)
                    result["record_fields"] = sorted({key for row in rows if isinstance(row, dict) for key in row})
                    result["statuses"] = dict(Counter(row.get("status", "missing") for row in rows if isinstance(row, dict)))
                    result["records"] = []
                    for row in rows:
                        safe = {key: row[key] for key in ["status", "reset_type", "event_type", "type", "kind"] if key in row}
                        safe["field_types"] = {key: type(value).__name__ for key, value in row.items()}
                        for key in ["granted_at", "expires_at", "redeem_started_at", "redeemed_at", "used_at", "occurred_at", "created_at", "timestamp"]:
                            if row.get(key) is not None:
                                try:
                                    safe[key] = datetime.fromisoformat(row[key].replace("Z", "+00:00")).isoformat()
                                except (ValueError, TypeError, AttributeError):
                                    safe[key] = "unparseable"
                        result["records"].append(safe)
            else:
                result["result"] = "upstream_rejected_read"
        except Exception as error:
            result["error_type"] = type(error).__name__
        results.append(result)
        print(json.dumps(result, ensure_ascii=False), flush=True)
    return results


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--account", type=int, required=True)
    parser.add_argument("--history-only", action="store_true")
    args = parser.parse_args()
    results = probe(args.account, args.history_only)
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    output = Path(__file__).with_name(f"probe-account-{args.account}-{stamp}.json")
    output.write_text(json.dumps(results, ensure_ascii=False, indent=2) + "\n")

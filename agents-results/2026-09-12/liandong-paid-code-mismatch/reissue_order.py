"""Recover the confirmed paid order without enabling two redeemable codes."""

import argparse
import datetime as dt
import json
import os
from pathlib import Path
import secrets
import subprocess

ORDER = "LD260912AVGNEY"
SOURCE_CODE_ID = "c932e2f7-b9f5-466d-b094-2565fdef18ad"
SOURCE_BATCH_ID = "215279a8-c98f-4117-b654-0a886162da4d"
ROOT = Path.home() / ".local/share/sub2api/order-recovery" / ORDER
JOURNAL = ROOT / "transfer.json"
NOTE = f"LDXP reissue {ORDER}; MediaClaw source {SOURCE_CODE_ID}; approved USD 5"


def query(sql, *, remote=False):
    if remote:
        command = ["ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=8", "106.52.146.37",
                   "sudo -n -u postgres psql -X -qAt -v ON_ERROR_STOP=1 -d openclaw_account"]
    else:
        command = ["docker", "exec", "-i", "sub2api-postgres", "sh", "-c",
                   'exec psql -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"']
    result = subprocess.run(command, input=sql, text=True, capture_output=True, timeout=30)
    if result.returncode:
        # PostgreSQL diagnostics can contain the entire inserted bearer code.
        raise RuntimeError("Source query failed" if remote else "Destination query failed")
    return result.stdout.strip()


def save(state):
    temporary = ROOT / "transfer.json.tmp"
    fd = os.open(temporary, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w") as stream:
        json.dump(state, stream, indent=2)
        stream.flush()
        os.fsync(stream.fileno())
    temporary.replace(JOURNAL)


def source():
    return query(f"""BEGIN READ ONLY;
SELECT c.status,COALESCE(to_char(c.revoked_at AT TIME ZONE 'UTC','YYYY-MM-DD HH24:MI:SS.US'),''),
       b.id,m.external_product_id,p.code,p.credit_amount,
       (SELECT count(*) FROM openclaw_account.fulfillments f WHERE f.code_id=c.id)
FROM openclaw_account.redemption_codes c
JOIN openclaw_account.redemption_batches b ON b.id=c.batch_id AND b.status='active'
JOIN openclaw_account.product_mappings m ON m.id=b.product_mapping_id AND m.status='active'
JOIN openclaw_account.plans p ON p.id=m.plan_id AND p.status='active'
WHERE c.id='{SOURCE_CODE_ID}';
ROLLBACK;""", remote=True).split("|")


def validate_source(row, state=None):
    expected = [SOURCE_BATCH_ID, "642224", "mediaclaw-cny-5", "5.00000000", "0"]
    if len(row) != 7 or row[2:] != expected:
        raise RuntimeError("Source mapping or fulfillment changed")
    if row[0] == "available" and row[1] == "":
        return
    if state and row[0] == "revoked" and row[1] == state["revoked_at"]:
        return
    raise RuntimeError("Source code is no longer available for this transfer")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    state = json.loads(JOURNAL.read_text()) if JOURNAL.exists() else None
    validate_source(source(), state)
    count = query(f"SELECT count(*) FROM redeem_codes WHERE notes='{NOTE}';")
    if count not in ("0", "1"):
        raise RuntimeError("Multiple destination records require reconciliation")
    if not state and count != "0":
        raise RuntimeError("Existing destination record requires reconciliation")
    if not args.apply:
        print("Preflight passed: exact source and transfer state validated; USD 5 authorized; destination checked.")
        return
    os.umask(0o077)
    ROOT.mkdir(parents=True, exist_ok=True, mode=0o700)
    if state is None:
        state = {"order": ORDER, "source_code_id": SOURCE_CODE_ID, "amount_usd": 5,
                 "code": secrets.token_hex(16), "stage": "prepared",
                 "revoked_at": dt.datetime.now(dt.timezone.utc).strftime("%Y-%m-%d %H:%M:%S.%f")}
        save(state)
    code = state["code"]
    if len(code) != 32 or any(c not in "0123456789abcdef" for c in code):
        raise RuntimeError("Invalid protected transfer journal")
    target = query(f"""BEGIN;
SELECT pg_advisory_xact_lock(hashtextextended('{ORDER}',0));
INSERT INTO redeem_codes(code,type,value,status,notes)
SELECT '{code}','balance',5,'disabled','{NOTE}'
WHERE NOT EXISTS (SELECT 1 FROM redeem_codes WHERE notes='{NOTE}');
SELECT id,status,value FROM redeem_codes WHERE code='{code}' AND notes='{NOTE}' AND type='balance';
COMMIT;""").strip().split("|")
    if len(target) != 3 or target[1] not in ("disabled", "unused", "used") or target[2] != "5.00000000":
        raise RuntimeError("Destination record mismatch; replacement remains disabled")
    if target[1] != "disabled" and source()[0] != "revoked":
        raise RuntimeError("Active replacement has no source revocation")
    state["destination_code_id"] = int(target[0])
    state["stage"] = "destination_staged"
    save(state)
    validate_source(source(), state)
    revoked = query(f"""BEGIN;
SELECT id FROM openclaw_account.redemption_codes WHERE id='{SOURCE_CODE_ID}' FOR UPDATE;
UPDATE openclaw_account.redemption_codes c
SET status='revoked',revoked_at='{state['revoked_at']}+00'
WHERE c.id='{SOURCE_CODE_ID}' AND c.status='available'
AND EXISTS (
 SELECT 1 FROM openclaw_account.redemption_batches b
 JOIN openclaw_account.product_mappings m ON m.id=b.product_mapping_id AND m.status='active'
 JOIN openclaw_account.plans p ON p.id=m.plan_id AND p.status='active'
 WHERE b.id=c.batch_id AND b.id='{SOURCE_BATCH_ID}' AND b.status='active'
 AND m.external_product_id='642224' AND p.code='mediaclaw-cny-5' AND p.credit_amount=5
)
AND NOT EXISTS (SELECT 1 FROM openclaw_account.fulfillments f WHERE f.code_id=c.id);
SELECT status FROM openclaw_account.redemption_codes WHERE id='{SOURCE_CODE_ID}';
COMMIT;""", remote=True)
    if revoked.splitlines()[-1] != "revoked":
        raise RuntimeError("Source revocation did not succeed; replacement remains disabled")
    validate_source(source(), state)
    state["stage"] = "source_revoked"
    save(state)
    activated = query(f"""BEGIN;
UPDATE redeem_codes SET status='unused'
WHERE id={state['destination_code_id']} AND code='{code}' AND notes='{NOTE}'
AND type='balance' AND value=5 AND status='disabled' AND used_by IS NULL AND used_at IS NULL;
SELECT status,value FROM redeem_codes WHERE id={state['destination_code_id']} AND code='{code}';
COMMIT;""")
    if activated not in ("unused|5.00000000", "used|5.00000000"):
        raise RuntimeError("Destination activation requires reconciliation")
    state["stage"] = "replacement_ready" if activated.startswith("unused|") else "redeemed"
    save(state)
    delivery = ROOT / "replacement-code.txt"
    fd = os.open(delivery, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(fd, "w") as stream:
        stream.write(code + "\n")
    print(json.dumps({"order": ORDER, "source_status": "revoked", "amount_usd": 5,
                      "destination_code_id": state["destination_code_id"],
                      "stage": state["stage"], "delivery_file": str(delivery)}))


if __name__ == "__main__":
    main()

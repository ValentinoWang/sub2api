from __future__ import annotations

import argparse
import csv
import os
from pathlib import Path

from .config import Plan, Settings, load_plans
from .fulfillment import fulfill_order
from .provision import ProvisioningError, XUIProvisioner
from .redemption import generate_code
from .store import OrderStore
from .sub2api import Sub2APIClient, Sub2APIError


def build_runtime() -> tuple[Settings, OrderStore, XUIProvisioner, Sub2APIClient | None]:
    settings = Settings.from_env()
    store = OrderStore(settings.database_path, settings.redemption_pepper)
    provisioner = XUIProvisioner(
        base_url=settings.xui_base_url,
        api_token=settings.xui_api_token,
        inbound_id=settings.xui_inbound_id,
        flow=settings.xui_flow,
        subscription_base_url=settings.subscription_base_url,
        insecure_local_tls=settings.xui_insecure_local_tls,
    )
    sub2api = None
    if settings.sub2api_enabled:
        sub2api = Sub2APIClient(
            base_url=settings.sub2api_base_url,
            admin_api_key=settings.sub2api_admin_api_key,
            timeout_seconds=settings.sub2api_timeout_seconds,
        )
    return settings, store, provisioner, sub2api


def build_offline_store(args: argparse.Namespace) -> tuple[OrderStore, dict[str, Plan]]:
    pepper = args.pepper_file.read_bytes().strip()
    if len(pepper) < 32:
        raise SystemExit("pepper file must contain at least 32 bytes")
    plans = load_plans(args.plans)
    return OrderStore(args.database, pepper), plans


def write_codes(path: Path, batch_id: str, plan_id: str, codes: list[str]) -> None:
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    try:
        with os.fdopen(descriptor, "w", newline="", encoding="utf-8") as output:
            writer = csv.writer(output)
            writer.writerow(("batch_id", "plan_id", "card_code"))
            writer.writerows((batch_id, plan_id, code) for code in codes)
    except Exception:
        path.unlink(missing_ok=True)
        raise


def generate_codes(args: argparse.Namespace) -> int:
    store, plans = build_offline_store(args)
    plan = plans.get(args.plan_id)
    if plan is None:
        raise SystemExit(f"unknown plan: {args.plan_id}")
    if not 1 <= args.count <= 10_000:
        raise SystemExit("count must be between 1 and 10000")
    codes = [generate_code() for _ in range(args.count)]
    batch_id = store.create_redemption_batch(plan, codes)
    try:
        write_codes(args.output, batch_id, plan.id, codes)
    except Exception as exc:
        store.discard_available_batch(batch_id)
        raise SystemExit(f"code export failed and batch {batch_id} was discarded: {exc}") from exc
    print(f"created batch {batch_id}: {len(codes)} codes -> {args.output}")
    return 0


def recover(args: argparse.Namespace) -> int:
    _, store, provisioner, sub2api = build_runtime()
    failures = 0
    for order_id in store.retryable_order_ids(args.minimum_age, args.limit):
        try:
            fulfill_order(store, provisioner, sub2api, order_id)
        except (ProvisioningError, Sub2APIError, OSError, ValueError) as exc:
            failures += 1
            print(f"{order_id}: {type(exc).__name__}")
    return 1 if failures else 0


def inventory(_: argparse.Namespace) -> int:
    store, _ = build_offline_store(_)
    for row in store.inventory_counts():
        print(f"{row['batch_id']}\t{row['plan_id']}\t{row['status']}\t{row['count']}")
    return 0


def add_offline_paths(parser: argparse.ArgumentParser) -> None:
    parser.add_argument(
        "--database", type=Path, default=Path("/var/lib/xui-sales/orders.sqlite3")
    )
    parser.add_argument("--plans", type=Path, default=Path("/etc/xui-sales/plans.json"))
    parser.add_argument(
        "--pepper-file", type=Path, default=Path("/etc/xui-sales/redemption-pepper")
    )


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Manage xui-sales redemption fulfillment")
    subparsers = parser.add_subparsers(dest="command", required=True)

    generate_parser = subparsers.add_parser("generate-codes")
    generate_parser.add_argument("--plan-id", required=True)
    generate_parser.add_argument("--count", required=True, type=int)
    generate_parser.add_argument("--output", required=True, type=Path)
    add_offline_paths(generate_parser)
    generate_parser.set_defaults(handler=generate_codes)

    recover_parser = subparsers.add_parser("recover")
    recover_parser.add_argument("--minimum-age", type=int, default=60)
    recover_parser.add_argument("--limit", type=int, default=20)
    recover_parser.set_defaults(handler=recover)

    inventory_parser = subparsers.add_parser("inventory")
    add_offline_paths(inventory_parser)
    inventory_parser.set_defaults(handler=inventory)
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    return int(args.handler(args))


if __name__ == "__main__":
    raise SystemExit(main())

from __future__ import annotations

from .provision import XUIProvisioner
from .store import OrderStore
from .sub2api import Sub2APIClient


def provision_order(store: OrderStore, provisioner: XUIProvisioner, order_id: str) -> bool:
    order = store.get(order_id)
    if order is None:
        raise KeyError(order_id)
    if not order["vpn_required"]:
        return False
    claimed = store.claim_provisioning(order_id)
    if claimed is None:
        return False
    if claimed["status"] == "active":
        return True
    try:
        subscription_url = provisioner.provision(claimed)
    except Exception as exc:
        current = store.get(order_id)
        if current is not None and current["status"] == "provisioning":
            store.mark_provision_failed(order_id, str(exc))
        raise
    store.mark_active(order_id, subscription_url)
    return True


def fulfill_order(
    store: OrderStore,
    provisioner: XUIProvisioner,
    sub2api: Sub2APIClient | None,
    order_id: str,
) -> bool:
    order = store.get(order_id)
    if order is None:
        raise KeyError(order_id)

    first_error: Exception | None = None
    if order["api_status"] != "api_not_required":
        if sub2api is None:
            raise RuntimeError("bundle order requires a configured Sub2API client")
        claimed = store.claim_api_fulfillment(order_id)
        if claimed is not None and claimed["api_status"] != "api_active":
            try:
                if claimed["api_fulfillment_type"] == "subscription":
                    redeem_code_id = sub2api.grant_subscription(claimed)
                elif claimed["api_fulfillment_type"] == "balance":
                    if (
                        claimed.get("api_balance_cny_cents") is not None
                        and claimed.get("api_credited_balance_cents") is None
                    ):
                        quote = sub2api.balance_quote(int(claimed["api_balance_cny_cents"]))
                        claimed = store.freeze_balance_quote(order_id, quote)
                    redeem_code_id = sub2api.grant_balance(claimed)
                else:
                    raise ValueError("unsupported API fulfillment type")
                store.mark_api_active(order_id, redeem_code_id)
            except Exception as exc:
                current = store.get(order_id)
                if current is not None and current["api_status"] == "api_pending":
                    store.mark_api_failed(order_id, str(exc))
                first_error = exc

    current = store.get(order_id)
    assert current is not None, "fulfillment order disappeared"
    if current["vpn_required"] and current["vpn_status"] != "vpn_active":
        try:
            provision_order(store, provisioner, order_id)
        except Exception as exc:
            if first_error is None:
                first_error = exc

    current = store.get(order_id)
    assert current is not None, "fulfilled order disappeared"
    if first_error is not None:
        raise first_error
    return current["fulfillment_status"] == "active"

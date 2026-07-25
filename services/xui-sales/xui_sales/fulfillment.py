from __future__ import annotations

from .provision import XUIProvisioner
from .store import OrderStore
from .sub2api import Sub2APIClient


def provision_order(store: OrderStore, provisioner: XUIProvisioner, order_id: str) -> bool:
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
                redeem_code_id = sub2api.grant_subscription(claimed)
                store.mark_api_active(order_id, redeem_code_id)
            except Exception as exc:
                current = store.get(order_id)
                if current is not None and current["api_status"] == "api_pending":
                    store.mark_api_failed(order_id, str(exc))
                first_error = exc

    current = store.get(order_id)
    assert current is not None, "fulfillment order disappeared"
    if current["vpn_status"] != "vpn_active":
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

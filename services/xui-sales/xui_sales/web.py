from __future__ import annotations

import logging
import hashlib
import json
import secrets
import threading
import time
from collections import defaultdict, deque
from decimal import Decimal, InvalidOperation
from pathlib import Path
from typing import Any
from urllib.parse import urlencode

from flask import Flask, Response, abort, jsonify, redirect, render_template, request, url_for
from werkzeug.middleware.proxy_fix import ProxyFix

from .alipay import AlipayClient
from .config import Settings, load_plans
from .fulfillment import fulfill_order
from .provision import ProvisioningError, XUIProvisioner
from .redemption import RedemptionError
from .store import OrderStore
from .sub2api import Sub2APIClient, Sub2APIError
from .xianyu import (
    XianyuCallbackError,
    XianyuOrderEvent,
    load_product_mappings,
    verify_signature,
)


LOG = logging.getLogger("xui_sales")
SUCCESS_STATUSES = {"TRADE_SUCCESS", "TRADE_FINISHED"}


class RedeemRateLimiter:
    def __init__(self, maximum: int = 10, window_seconds: int = 600):
        self.maximum = maximum
        self.window_seconds = window_seconds
        self._attempts: defaultdict[str, deque[float]] = defaultdict(deque)
        self._lock = threading.Lock()

    def allow(self, key: str) -> bool:
        now = time.monotonic()
        cutoff = now - self.window_seconds
        with self._lock:
            attempts = self._attempts[key]
            while attempts and attempts[0] <= cutoff:
                attempts.popleft()
            if len(attempts) >= self.maximum:
                return False
            attempts.append(now)
            if len(self._attempts) > 10_000:
                for candidate in tuple(self._attempts):
                    if not self._attempts[candidate] or self._attempts[candidate][-1] <= cutoff:
                        del self._attempts[candidate]
            return True


def _amount_to_cents(raw: str) -> int:
    try:
        value = Decimal(raw)
    except InvalidOperation as exc:
        raise ValueError("invalid payment amount") from exc
    cents = int(value * 100)
    if value != Decimal(cents) / 100 or cents <= 0:
        raise ValueError("invalid payment amount")
    return cents


def _public_order(order: dict[str, Any]) -> dict[str, Any]:
    public_status = order["fulfillment_status"] or order["status"]
    return {
        "id": order["id"],
        "plan_name": order["plan_name"],
        "amount": f"{order['amount_cents'] / 100:.2f}",
        "status": public_status,
        "api_status": order["api_status"],
        "api_fulfillment_type": order["api_fulfillment_type"],
        "vpn_status": order["vpn_status"],
        "has_api": order["api_status"] != "api_not_required",
        "has_vpn": bool(order["vpn_required"]),
        "is_bundle": order["api_status"] != "api_not_required" and bool(order["vpn_required"]),
        "source": order["source"],
        "subscription_url": (
            order["subscription_url"] if order["vpn_status"] == "vpn_active" else None
        ),
        "created_at": order["created_at"],
        "updated_at": order["updated_at"],
    }


def create_app(
    settings: Settings | None = None,
    store: OrderStore | None = None,
    alipay: AlipayClient | None = None,
    provisioner: XUIProvisioner | None = None,
    sub2api: Sub2APIClient | None = None,
) -> Flask:
    settings = settings or Settings.from_env()
    plans = load_plans(settings.plans_path)
    store = store or OrderStore(settings.database_path, settings.redemption_pepper)
    xianyu_mappings = {}
    if settings.xianyu_callbacks_enabled:
        assert settings.xianyu_products_path is not None
        xianyu_mappings = load_product_mappings(settings.xianyu_products_path, set(plans))

    if alipay is None and settings.payments_enabled:
        assert settings.alipay_private_key_path is not None
        assert settings.alipay_public_key_path is not None
        alipay = AlipayClient(
            app_id=settings.alipay_app_id,
            gateway=settings.alipay_gateway,
            merchant_private_key_file=settings.alipay_private_key_path,
            alipay_public_key_file=settings.alipay_public_key_path,
            notify_url=f"{settings.public_base_url}/callbacks/alipay",
            return_url=f"{settings.public_base_url}/return/alipay",
        )
    if provisioner is None:
        provisioner = XUIProvisioner(
            base_url=settings.xui_base_url,
            api_token=settings.xui_api_token,
            inbound_id=settings.xui_inbound_id,
            flow=settings.xui_flow,
            subscription_base_url=settings.subscription_base_url,
            insecure_local_tls=settings.xui_insecure_local_tls,
        )
    has_api_plans = any(plan.requires_sub2api for plan in plans.values())
    if sub2api is None and settings.sub2api_enabled:
        sub2api = Sub2APIClient(
            base_url=settings.sub2api_base_url,
            admin_api_key=settings.sub2api_admin_api_key,
            timeout_seconds=settings.sub2api_timeout_seconds,
        )
    if has_api_plans and sub2api is None:
        raise RuntimeError("API plans require SUB2API_ENABLED=true")

    app = Flask(__name__, template_folder="../templates", static_folder="../static")
    app.wsgi_app = ProxyFix(app.wsgi_app, x_for=1, x_proto=1)
    app.config.update(
        SECRET_KEY=settings.secret_key,
        MAX_CONTENT_LENGTH=64 * 1024,
        SEND_FILE_MAX_AGE_DEFAULT=3600,
    )
    redeem_limiter = RedeemRateLimiter()

    @app.after_request
    def security_headers(response: Response) -> Response:
        response.headers.setdefault("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; connect-src 'self'; form-action 'self' https://openapi.alipay.com https://openapi-sandbox.dl.alipaydev.com; base-uri 'none'; frame-ancestors 'none'")
        response.headers.setdefault("Referrer-Policy", "no-referrer")
        response.headers.setdefault("X-Content-Type-Options", "nosniff")
        response.headers.setdefault("X-Frame-Options", "DENY")
        response.headers.setdefault("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
        return response

    @app.get("/healthz")
    def healthz() -> Response:
        return jsonify(
            {
                "status": "ok",
                "payments_enabled": settings.payments_enabled,
                "xianyu_callbacks_enabled": settings.xianyu_callbacks_enabled,
                "redemption_enabled": True,
                "bundle_fulfillment_enabled": sub2api is not None,
            }
        )

    @app.get("/readyz")
    def readyz() -> Response:
        try:
            provisioner.healthcheck()
            if sub2api is not None:
                sub2api.healthcheck()
        except (ProvisioningError, Sub2APIError):
            return jsonify({"status": "not_ready"}), 503
        return jsonify(
            {
                "status": "ready",
                "xui": "reachable",
                "sub2api": "reachable" if sub2api is not None else "not_required",
            }
        )

    def render_index(redeem_error: str | None = None, status: int = 200) -> tuple[str, int]:
        visible_plans = [plan for plan in plans.values() if plan.enabled]
        return (
            render_template(
                "index.html",
                plans=visible_plans,
                payments_enabled=settings.payments_enabled,
                redeem_error=redeem_error,
            ),
            status,
        )

    @app.get("/")
    def index() -> tuple[str, int]:
        return render_index()

    @app.post("/redeem")
    def redeem() -> Response | tuple[str, int]:
        client_key = request.remote_addr or "unknown"
        if not redeem_limiter.allow(client_key):
            return render_index("尝试次数过多，请稍后再试", 429)
        code = request.form.get("code", "")
        try:
            requirements = store.redemption_requirements(code)
            sub2api_user_id = None
            if requirements["api_fulfillment_type"] != "none":
                assert sub2api is not None
                sub2api_user_id = sub2api.resolve_user(request.form.get("sub2api_email", ""))
            order, token = store.redeem_code(code, sub2api_user_id=sub2api_user_id)
        except (KeyError, RedemptionError):
            return render_index("兑换码无效", 400)
        except Sub2APIError as exc:
            return render_index(str(exc), 400)
        except ValueError as exc:
            LOG.warning("rejected redemption: %s", exc)
            return render_index(str(exc), 409)

        if order["fulfillment_status"] != "active":
            try:
                fulfill_order(store, provisioner, sub2api, order["id"])
            except (ProvisioningError, Sub2APIError, OSError, ValueError):
                LOG.exception("redeemed order %s is awaiting provisioning recovery", order["id"])
        return redirect(url_for("order_page", order_id=order["id"], token=token), code=303)

    @app.post("/orders")
    def create_order() -> Response:
        if not settings.payments_enabled or alipay is None:
            abort(503, description="支付宝收款尚未启用")
        plan_id = request.form.get("plan_id", "").strip()
        plan = plans.get(plan_id)
        if plan is None or not plan.enabled:
            abort(400, description="套餐不存在或已下架")
        if plan.requires_sub2api:
            abort(400, description="API 商品仅支持一次性卡密兑换")
        order, token = store.create_order(plan)
        return redirect(url_for("order_page", order_id=order["id"], token=token), code=303)

    @app.get("/orders/<order_id>")
    def order_page(order_id: str) -> str:
        token = request.args.get("token", "")
        order = store.get_authorized(order_id, token)
        if order is None:
            abort(404)
        return render_template(
            "order.html",
            order=_public_order(order),
            token=token,
            payment_ready=settings.payments_enabled and alipay is not None,
        )

    @app.get("/api/orders/<order_id>")
    def order_status(order_id: str) -> Response:
        token = request.args.get("token", "")
        order = store.get_authorized(order_id, token)
        if order is None:
            abort(404)
        response = jsonify(_public_order(order))
        response.headers["Cache-Control"] = "no-store"
        return response

    @app.post("/pay/<order_id>")
    def pay_order(order_id: str) -> str:
        if not settings.payments_enabled or alipay is None:
            abort(503, description="支付宝收款尚未启用")
        token = request.form.get("token", "")
        order = store.get_authorized(order_id, token)
        if order is None:
            abort(404)
        if order["status"] == "active":
            return redirect(url_for("order_page", order_id=order_id, token=token), code=303)
        if order["status"] not in {"pending", "paid", "provision_failed"}:
            abort(409, description="订单当前不能发起支付")
        return_query = urlencode({"order_id": order_id, "token": token})
        params = alipay.page_pay_parameters(
            order_id=order_id,
            amount_text=f"{order['amount_cents'] / 100:.2f}",
            subject=order["plan_name"],
            return_url=f"{settings.public_base_url}/return/alipay?{return_query}",
        )
        return render_template("alipay_form.html", gateway=alipay.gateway, params=params)

    @app.post("/callbacks/alipay")
    def alipay_callback() -> Response:
        if not settings.payments_enabled or alipay is None:
            return Response("failure", status=503, content_type="text/plain")
        params = request.form.to_dict(flat=True)
        if not alipay.verify_notification(params):
            LOG.warning("rejected Alipay callback with invalid RSA2 signature")
            return Response("failure", status=400, content_type="text/plain")
        if not secrets.compare_digest(params.get("app_id", ""), settings.alipay_app_id):
            LOG.warning("rejected Alipay callback with mismatched app_id")
            return Response("failure", status=400, content_type="text/plain")
        if settings.alipay_seller_id and not secrets.compare_digest(
            params.get("seller_id", ""), settings.alipay_seller_id
        ):
            LOG.warning("rejected Alipay callback with mismatched seller_id")
            return Response("failure", status=400, content_type="text/plain")
        if params.get("trade_status") not in SUCCESS_STATUSES:
            return Response("success", status=200, content_type="text/plain")

        order_id = params.get("out_trade_no", "")
        trade_no = params.get("trade_no", "")
        order = store.get(order_id)
        if order is None or not trade_no:
            LOG.warning("rejected Alipay callback for an unknown order")
            return Response("failure", status=400, content_type="text/plain")
        try:
            callback_cents = _amount_to_cents(params.get("total_amount", ""))
        except ValueError:
            return Response("failure", status=400, content_type="text/plain")
        if callback_cents != order["amount_cents"]:
            LOG.warning("rejected Alipay callback with mismatched amount for %s", order_id)
            return Response("failure", status=400, content_type="text/plain")

        try:
            order = store.mark_paid(order_id, trade_no)
            if not fulfill_order(store, provisioner, sub2api, order_id):
                return Response("failure", status=409, content_type="text/plain")
        except (ProvisioningError, Sub2APIError, OSError, ValueError) as exc:
            current = store.get(order_id)
            if current is not None and current["status"] == "provisioning":
                store.mark_provision_failed(order_id, str(exc))
            LOG.exception("paid order %s could not be provisioned", order_id)
            return Response("failure", status=500, content_type="text/plain")
        except Exception:
            current = store.get(order_id)
            if current is not None and current["status"] == "provisioning":
                try:
                    store.mark_provision_failed(order_id, "unexpected provisioning error")
                except Exception:
                    LOG.exception("could not record provisioning failure for %s", order_id)
            LOG.exception("unexpected callback failure for order %s", order_id)
            return Response("failure", status=500, content_type="text/plain")
        return Response("success", status=200, content_type="text/plain")

    @app.post("/callbacks/xianyu")
    def xianyu_callback() -> Response:
        if not settings.xianyu_callbacks_enabled:
            return jsonify({"result": "fail", "msg": "回调未启用"}), 503
        raw_body = request.get_data(cache=True, as_text=False)
        try:
            verify_signature(
                raw_body=raw_body,
                app_id=request.args.get("appid", ""),
                timestamp_raw=request.args.get("timestamp", ""),
                signature=request.args.get("sign", ""),
                expected_app_id=settings.xianyu_app_id,
                app_secret=settings.xianyu_app_secret,
                max_skew_seconds=settings.xianyu_timestamp_skew_seconds,
                merchant_id=settings.xianyu_signature_merchant_id,
            )
            event = XianyuOrderEvent.from_payload(json.loads(raw_body))
            mapping = xianyu_mappings.get((event.product_id, event.item_id))
            if mapping is None:
                raise XianyuCallbackError("unknown product mapping")
            store.record_xianyu_event(event, mapping, hashlib.sha256(raw_body).hexdigest())
        except (XianyuCallbackError, json.JSONDecodeError, UnicodeDecodeError) as exc:
            LOG.warning("rejected Xianyu callback: %s", exc)
            return jsonify({"result": "fail", "msg": "请求校验失败"}), 400
        return jsonify({"result": "success", "msg": "接收成功"})

    @app.get("/return/alipay")
    def alipay_return() -> Response:
        order_id = request.args.get("order_id", "")
        token = request.args.get("token", "")
        if store.get_authorized(order_id, token) is None:
            abort(404)
        return redirect(url_for("order_page", order_id=order_id, token=token), code=303)

    return app

# xui-sales

Unified one-time-code fulfillment for the local 3x-ui panel and optional
Sub2API subscription or balance grants.

The production topology keeps this coordinator next to x-ui. It reaches the
Sub2API host only through the dedicated loopback SSH tunnel. The card platform
never receives x-ui credentials, the Sub2API Admin API key, or subscription
URLs.

## Payment invariant

The browser return URL never provisions a client. Only an Alipay asynchronous
notification that passes RSA2 signature, app ID, seller ID, amount, order ID,
and trade status checks can move an order into provisioning.

## Enable live payments

1. Edit `/etc/xui-sales/plans.json`, set final CNY prices, and enable the plans.
2. Install the merchant application private key at
   `/etc/xui-sales/alipay-merchant-private.pem`.
3. Install Alipay's application public key at
   `/etc/xui-sales/alipay-public.pem`.
4. Set `ALIPAY_APP_ID`, `ALIPAY_SELLER_ID`, and `PAYMENTS_ENABLED=true` in
   `/etc/xui-sales/xui-sales.env`.
5. Restart `xui-sales` and verify `/healthz` before exposing a purchase button.

The Alipay notification URL is:

```text
https://64.83.31.224:8443/callbacks/alipay
```

## Liandong card redemption fulfillment

Liandong is responsible only for payment and delivery of a pre-generated
one-time code. It is not a provisioning authority and does not need a callback
into this service. A bundle plan maps one local network-stabilizer plan to one
Sub2API group:

```json
{
  "id": "monthly-400gb",
  "sub2api_group_id": 3,
  "api_validity_days": 30,
  "duration_days": 30,
  "traffic_gb": 400,
  "ip_limit": 3
}
```

A balance-only plan grants the exact USD balance and never calls x-ui:

```json
{
  "id": "api-balance-10",
  "sub2api_balance": "10.00",
  "duration_days": 1,
  "traffic_gb": 1,
  "ip_limit": 0
}
```

The buyer redeems it at:

```text
https://64.83.31.224:8443/
```

Generate an import batch on the server:

```bash
cd /opt/xui-sales
python3 -m xui_sales.manage generate-codes \
  --plan-id monthly-400gb \
  --count 100 \
  --output /root/liandong-monthly-codes.csv
```

The export is mode `0600`. Import it into the matching card-code
inventory, verify the imported count, transfer any required archival copy to a
protected location, and delete the plaintext server export.

Fix the Liandong product identifier to exactly one local `plan_id` before code
generation. Keep the product unpublished until generated, imported, and
available counts reconcile and one explicitly approved test redemption passes.

The optional signed order callback is:

```text
https://64.83.31.224:8443/callbacks/xianyu
```

Keep `XIANYU_CALLBACKS_ENABLED=false` until the AppKey, AppSecret, exact
signature variant, and `/etc/xui-sales/xianyu-products.json` mappings are
confirmed. The callback records order/refund state for reconciliation; its
payload does not identify the actual plaintext code delivered to the buyer.

## Sub2API transport

`SUB2API_BASE_URL` may use plain HTTP only on loopback. In production, run the
dedicated SSH tunnel and configure the application with:

```text
SUB2API_ENABLED=true
SUB2API_BASE_URL=http://127.0.0.1:19080
SUB2API_ADMIN_API_KEY_FILE=/run/credentials/xui-sales.service/sub2api_admin_api_key
SUB2API_TIMEOUT_SECONDS=8
```

Install the Admin API key as a mode-0600 systemd credential. Do not place it in
the environment file. Both the web service and recovery service require the
credential and the tunnel dependency.

## Verification

Install the Python dependencies and run the service gate from the repository
root:

```bash
python3 -m pip install -r services/xui-sales/requirements.txt
make test-xui-sales
```

The deploy directory contains the production-shaped Web service, recovery
service and timer, SSH tunnel, Nginx site, disabled plan template, and a
secret-free environment example. Credential files belong under
`/etc/xui-sales`; never commit their values.

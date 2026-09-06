# Membership Fulfillment Operations

This guide operates the optional membership fulfillment domain added by
`backend/migrations/236_membership_fulfillment.sql`. It does not establish that
any upstream membership is available, that a payment channel is approved, or
that a product may be sold. The initial database rows are deliberately closed:
both products have `for_sale=false` and `paused=true`.

Use this guide only with administrators who can access the existing Sub2API
admin API and the deployment secret store. Do not put customer session data,
CDKs, browser-agent requests, or storage credentials in tickets, chat, shell
history, or ordinary logs.

## Evidence Levels and Product Boundary

Keep these levels separate in change records and customer-facing statements.

| Level | What it proves | What it does not prove | Sale state |
| --- | --- | --- | --- |
| Platform simulation | Fixtures, mocks, or a browser-agent contract return expected states. | A real dedicated upstream account, a real CDK, payment collection, or an entitlement. | Closed. |
| Real validation | One dedicated test account and one authorized CDK complete a validation order, and the entitlement is independently checked. | Payment opening for other customers or validity of another product/channel. | Closed until all opening gates pass. |
| Product verification | The successful validation order is recorded with a `run_id` and durable evidence reference for the exact SKU, channel, credential mode, and period. | Fresh stock or permission to collect payment. | Still closed by default. |
| Payment opening | Runtime readiness, payment integration, fresh availability, local CDK stock, product verification, and an explicit product update all pass. | Production evidence for a different SKU or a later configuration. | May open that exact product. |
| Production evidence | A controlled paid order has payment proof, immutable intent evidence, independently verified entitlement, and a completed reconciliation record. | General availability after stock, upstream, or configuration changes. | Recheck before every material change. |

`chatgpt_plus` and `chatgpt_pro_20x` are distinct products. Plus validation
uses the `gpt` / `account_id` contract; Pro validation uses the `gptpro` /
`session` contract. Run, record, and approve them independently. A Plus result
never verifies Pro, and a Pro result never verifies Plus. Claude Max has no
validated product or fulfillment evidence in this repository and remains
unavailable and unsellable.

The current `services/membership-browser` implementation refuses every
non-loopback `submitRecharge` unless both the production-submit flag is enabled
and a separate confirmation variable exactly matches the configured supplier
host. Its automated tests exercise submission only against a local fixture.
No real dedicated-account/CDK validation, customer payment opening, or
production evidence exists yet for Plus or Pro; both must remain closed. The
real-validation, payment-opening, and production-evidence rows below specify
the gates for a later approved validation run, not a claim that they have been
met now.

## Deployment Topology

The optional overlay files are
[`deploy/membership-fulfillment.env.example`](../deploy/membership-fulfillment.env.example)
and
[`deploy/membership-fulfillment.compose.example.yml`](../deploy/membership-fulfillment.compose.example.yml).
They extend, rather than alter,
[`deploy/docker-compose.local.yml`](../deploy/docker-compose.local.yml).

Create a deployment-only copy with owner-only permissions, then validate the
fully rendered configuration before starting anything:

```bash
cp deploy/membership-fulfillment.env.example deploy/membership-fulfillment.env
chmod 600 deploy/membership-fulfillment.env
docker compose --env-file deploy/.env --env-file deploy/membership-fulfillment.env \
  -f deploy/docker-compose.local.yml \
  -f deploy/membership-fulfillment.compose.example.yml config
```

The overlay has three boundaries:

- `membership-credential-redis` is an isolated credential cache. It has no
  host port, volume, RDB snapshots, or AOF. Startup refuses the membership
  runtime when Redis reports either persistence mechanism enabled. It must not
  reuse the normal application Redis instance or database number.
- `membership-browser` shares the application container's network namespace,
  binds to `127.0.0.1`, and has no published port. The application calls its
  loopback HTTP listener, which is the only plain-HTTP address accepted by the
  current client. Its bearer token is separate from the image and supplier
  credentials. This loopback boundary is required; do not turn it into a
  host-published or cross-network HTTP service.
- The audit bucket must already have S3 Object Lock enabled. The runtime writes
  a submit intent in `COMPLIANCE` mode for seven years and performs a `HeadObject`
  readback that checks lock mode and retention. A bucket without Object Lock,
  a bypassable retention mode, or failed readback makes the runtime unavailable.

The example exposes the actual configuration names: `MEMBERSHIP_ENABLED`,
`MEMBERSHIP_PAYMENTS_ENABLED`, `MEMBERSHIP_CREDENTIAL_REDIS_URL`,
`MEMBERSHIP_KEYRING_JSON`, `MEMBERSHIP_ACTIVE_KEY_ID`,
`MEMBERSHIP_IDENTITY_KEY`, `MEMBERSHIP_BROWSER_URL`,
`MEMBERSHIP_BROWSER_BEARER_TOKEN`, the browser's `MEMBERSHIP_BROWSER_*` allowlist
and base-URL settings, the two-part production-submit confirmation, and the
`MEMBERSHIP_AUDIT_*` group. Empty values are intentionally invalid for an
enabled sidecar/runtime. Do not invent an upstream endpoint: use only values
from an approved supplier contract and the approved object-storage contract
for the deployment. The overlay maps
`MEMBERSHIP_BROWSER_BEARER_TOKEN` to the application's
`MEMBERSHIP_BROWSER_TOKEN`; do not configure those two sides with different
values.

## Start Closed and Validate in Order

Keep both `MEMBERSHIP_ENABLED=false` and `MEMBERSHIP_PAYMENTS_ENABLED=false`
until the topology config renders and its private dependencies are healthy.
When the runtime is enabled, `MEMBERSHIP_PAYMENTS_ENABLED=false` still prevents
customer checkout. This is the normal state for a real validation run.

The following order prevents a local or simulated pass from opening sales:

1. Confirm the migration is present and both seeded products are closed.
2. Import only authorized, deduplicated validation CDKs for one target SKU.
3. Create a `validation` order through the existing admin membership API and
   provide the required credential with explicit consent. This order is
   zero-price; it is not a customer purchase.
4. Confirm that the browser result has the exact SKU, period, and account ID,
   then independently inspect the actual entitlement on the dedicated test
   account. A browser `succeeded` state without `entitlement_verified=true`
   is converted to `review_required` by the runtime.
5. Record a durable, non-secret evidence reference, then verify that exact SKU
   with the admin product-verification operation. Repeat from step 2 for the
   other SKU; Plus and Pro cannot share a verification run.
6. Refresh upstream availability through the admin operation. It must be no
   older than two minutes, positive, and paired with an available local CDK.
7. After a later implementation can submit a real approved recharge, enable
   payments and explicitly update only the verified product to `for_sale=true`
   and `paused=false`. Re-read the catalog and the admin overview before
   enabling customer traffic. Do not perform this step with the current
   fixture-only browser sidecar.

Checkout is guarded at order creation and payment attachment. It requires a
ready runtime, enabled payments, a verified and unpaused product, fresh
positive upstream availability, a local reserved CDK, and a credential that
has not expired. It does not become open merely because a payment method is
configured elsewhere.

## Credentials, TTL, and Key Rotation

Customer credentials are encrypted before entering the dedicated Redis cache.
Their fixed TTL is 30 minutes. The worker deletes them after terminal results,
after submission or review handoff, and during cleanup of expired, canceled,
closed, or refunded orders. A Redis restart is expected to erase them. Ask the
customer for fresh consent and input; never restore a credential from backups.

The keyring accepts a JSON map of key ID to base64-encoded 32-byte AES key and
stores the key ID with each ciphertext. Rotate it by adding a new key ID,
switching `MEMBERSHIP_ACTIVE_KEY_ID`, and retaining old key IDs long enough to
decrypt all existing CDK records. There is no documented bulk re-encryption
command in this repository, so do not remove an old key until a separately
reviewed migration and readback prove that no ciphertext still uses it.

`MEMBERSHIP_IDENTITY_KEY` produces target fingerprints used for duplicate-target
protection. It requires a planned data migration; changing it in place breaks
the ability to match existing fingerprints. Rotate the browser token in the
secret store and both sidecar/app configuration as one maintenance change,
then verify health before allowing new validation or payment work.

## Daily Operations

At least once per operating shift, review the admin overview for runtime
readiness, product pause state, verified time, inventory-check time, available
stock, queued work, `review_required` tasks, and the membership ledger. For a
controlled data check from the local Compose stack, use the actual service and
database names from `deploy/docker-compose.local.yml`:

```bash
docker compose -f deploy/docker-compose.local.yml exec -T postgres \
  psql -U "${POSTGRES_USER:-sub2api}" -d "${POSTGRES_DB:-sub2api}" \
  -c "SELECT sku, for_sale, paused, verified_at, inventory_checked_at, upstream_available, failures FROM membership_products ORDER BY sku;"

docker compose -f deploy/docker-compose.local.yml exec -T postgres \
  psql -U "${POSTGRES_USER:-sub2api}" -d "${POSTGRES_DB:-sub2api}" \
  -c "SELECT state, count(*) FROM membership_tasks GROUP BY state ORDER BY state;"
```

Run a restricted log scan after every validation, failure, token rotation, and
production test. The scan is an exposure detector, not an approval check. Do
not paste matching lines into a ticket; preserve them only in the restricted
incident system, revoke affected material, and investigate before resuming.

```bash
docker compose --env-file deploy/.env --env-file deploy/membership-fulfillment.env \
  -f deploy/docker-compose.local.yml \
  -f deploy/membership-fulfillment.compose.example.yml logs --since 24h sub2api membership-browser \
  | rg -n -i 'authorization:|bearer[[:space:]]+|access[_ -]?token|session|cookie|cdk|credential'
```

An empty result is expected. A match is a security incident until reviewed; it
does not prove the value is real, but credentials and CDKs must be treated as
potentially exposed.

## Maintenance, Pause, Circuit Breaker, and Refund Review

Before an operation that recreates containers, changes sidecar certificates or
tokens, or interrupts payment processing, publish the existing site-wide
maintenance notice and retain its successful publication record. Do not take
down PostgreSQL volumes, the audit bucket, or the application data directory.

For planned maintenance, immediately set affected products to `paused=true`
and keep `for_sale=false`. This prevents new customer work; already submitted
or processing work remains observable and should be queried or manually
reviewed, never silently discarded. For a rollback that preserves customer
recovery, keep `MEMBERSHIP_ENABLED=true` but set
`MEMBERSHIP_PAYMENTS_ENABLED=false` and leave products paused. Disable the
entire runtime only after every paid order is terminal or explicitly refunded,
all review decisions have evidence, and immutable audit objects were read back.

The worker increments a product failure count for `review_required` or failed
results and automatically pauses the product after the third such result
(the update observes the previous count, so the state change occurs when it
was already at least two). Treat the first review-required result as a warning:
pause manually, inspect the target, CDK, browser evidence, and audit object.
Do not clear the condition by toggling sales. A new successful real validation
and product verification resets the failure counter for that SKU.

Customer refund requests are recorded with one of `not_delivered`,
`wrong_plan`, or `cancel_request`. A membership refund requires full-refund and
confirmed non-delivery through the existing payment administration workflow.
The runtime refuses automatic refund preparation when an attempt may have been
submitted, is uncertain, is leased, or already succeeded. In those cases keep
the order in review, query the upstream result, attach durable evidence, and
only then decide between confirmed fulfillment and a refund.

## Production Evidence and Rollback Record

For each controlled paid production order, retain a non-secret record that
links the payment transaction, order ID, exact SKU, validation run, product
verification time, fresh availability check, CDK fingerprint/state, submit
intent object key and Object Lock readback, independently checked entitlement,
and final ledger/refund state. A platform simulation, browser status, payment
webhook, or an S3 upload alone is not production proof.

Rollback records must state the pre-change image digest/config version, paused
products, affected orders, audit-object readback result, selected rollback
image/config, post-rollback health, and final review/refund disposition. Keep
the database and immutable evidence intact so an incomplete fulfillment can be
reconciled rather than erased.

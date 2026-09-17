# Paid order: incompatible redemption inventory

Status: root cause confirmed; local purchase link contained; USD 5 replacement issued, user redemption pending.

## Verified facts

- User-provided order LD260912AVGNEY: Token 额度 5元, quantity 1, paid CNY 5.15, one delivered card. The actual order page confirms payment and delivery; usage instructions are empty.
- Local Sub2API at 127.0.0.1:8080 has three redeem-code records, none with the delivered prefix; no Liandong product mappings or restock batches.
- An exact server-side HMAC lookup, inside a read-only transaction, matched the delivered card in MediaClaw's `openclaw_account.redemption_codes`. No redemption endpoint was called. No plaintext card, digest, or secret is retained in this report.
- Matched code record: `c932e2f7-b9f5-466d-b094-2565fdef18ad`; status `available`; fulfillment count 0.
- Active batch: `215279a8-c98f-4117-b654-0a886162da4d`; 1,000 issued records.
- Active Liandong product mapping: external product `642224`, purchase URL `https://pay.ldxp.cn/item/9pabna`, plan `mediaclaw-cny-5`; price CNY 5.00, MediaClaw credit amount 5.00000000.
- These credits belong to MediaClaw wallet accounts. Sub2API displays dollar balances and has no mapping from this product to a Sub2API entitlement. Its native payment multiplier default does not establish a contract for this external MediaClaw product.
- CNY 5.15 is the paid total; the difference of CNY 0.15 is not a verified settlement fee without a provider statement.

## Root cause

The local Sub2API purchase configuration pointed to an existing shop whose cards were issued into the independent MediaClaw wallet system. The UI link and shared redemption form had been checked, but product identity, issuance authority, and actual redemption compatibility had not been verified before enabling the link.

The `OC-` prefix alone is insufficient evidence: MediaClaw issues both wallet and admission codes with that format. The exact HMAC and batch/product/plan join proves this card is a wallet code.

## Local containment and verification

- Cleared only `purchase_subscription_url` using a compare-and-set update matching `https://wzyp.cn/shop/MGDY0ZE4`. Preserved `payment_enabled=true` and `purchase_subscription_enabled=true`.
- Public settings through Vite read back the empty URL.
- Changed PaymentView to retain the shop redemption panel while the buyer URL is absent; the panel still suppresses the outbound purchase link. Native online payment remains separately controlled.
- Regression test first failed on missing panel, then passed. PaymentView and LiandongRechargePanel suites: 32 tests passed. Typecheck passed. Browser at `/purchase` displayed the unavailable-purchase notice and input form, with no buyer link. Vite served the updated module on port 4174.
- No production restart, inventory revocation, balance credit, replacement issuance, refund, or merchant change was performed. This is containment, not completed order fulfillment.

## Concrete recovery path awaiting entitlement decision

1. Establish the Sub2API dollar credit for the CNY 5 product and target environment. The failed session is the local Sub2API user; production must not be assumed interchangeable.
2. For this exact paid order, prepare a uniquely keyed transfer/reissue record tying the MediaClaw code record above to one replacement Sub2API code. Do not reuse the raw code in both systems.
3. Recheck that the source card is still available and unfulfilled, then revoke it through an audited conditional operation before enabling the replacement. If source revocation fails, issue no replacement. If destination issuance fails after source revocation, retain a recoverable pending transfer rather than blindly restoring a possibly credited card.
4. Let the intended user redeem the replacement on the selected Sub2API environment and verify the balance delta and duplicate rejection. Do not represent a database lookup or UI test as financial completion.
5. For future sales, use dedicated fixed Sub2API products and Sub2API-issued inventory, including a versioned entitlement amount, batch alignment, and correct return instructions. Existing MediaClaw inventory must remain isolated. Enable the buyer URL only after product identity, target-environment inventory, and a real end-to-end purchase/redemption have been verified.

## Confirmed amount and replacement issuance

- The user explicitly confirmed USD 5 for this product and the commercial rule CNY 1 paid to USD 1 credit. This is a sales rule, not a market exchange rate.
- Executed `../reissue_order.py` after read-only preflight. It staged a disabled Sub2API replacement, conditionally revoked the exact still-unfulfilled MediaClaw code, verified the revocation timestamp against its protected journal, then enabled the replacement.
- Source readback: `revoked`, zero fulfillments. Destination readback: record 4, type balance, USD 5.00000000, `unused`, no user or redemption timestamp, 32-character cryptographically random code.
- The replacement and resumable transfer journal are in the private local directory `~/.local/share/sub2api/order-recovery/LD260912AVGNEY/`, outside Git. Delivery file permissions were verified as 0600. No bearer code is included in project artifacts.
- This issued a replacement into the local Sub2API database only. It did not credit an account, issue a refund, transfer inventory batches, or change production Sub2API. The user must redeem the replacement; actual balance delivery remains pending.
- The source production change was limited to this one purchased code. No other MediaClaw inventory or balance was changed.

## Requested CNY denomination change

The user also asked about denominating all Sub2API credits in CNY at a factor of 7. The direction remains pending clarification: USD 5 could display as CNY 35 credits with unchanged purchasing power, or the user could intend a different commercial price. No global currency, pricing, balance, or multiplier change has been applied.

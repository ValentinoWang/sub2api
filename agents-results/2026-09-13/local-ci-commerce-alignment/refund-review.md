# LDXP unused-rights refund implementation review

The implementation reserves unused local rights for a merchant refund. It never invokes a merchant refund API and never labels an operator-supplied reference as verified money settlement. The operator must first verify the external order, its delivered code, and merchant refund eligibility in the merchant system.

## Interfaces and behavior

- `POST /api/v1/admin/liandong/restock/refunds/prepare`: accepts `external_order_no`, `batch_id`, and `code`. SHA-256 verifies membership in the durable LDXP batch. An external-order advisory lock and redeem-code row lock serialize replay and redemption. Only `unused` codes without `used_by` or `used_at` qualify. The same transaction disables the code and inserts its unique refund reservation.
- Identical preparation returns the existing reservation. Changing the order/code identity conflicts; an unrelated batch or consumed code is rejected. No user balance is debited.
- `POST /api/v1/admin/liandong/restock/refunds/confirm`: accepts `external_order_no` and a non-empty `merchant_refund_reference`. Identical confirmation is idempotent; a changed reference conflicts. The resulting state is `merchant_reference_recorded`, meaning an operator supplied a reference, not provider settlement verification.
- Both endpoints inherit the existing administrator authentication, audit, and compliance chain. Responses use `Cache-Control: no-store`. Full request bodies are omitted from audit storage, including malformed JSON. Driver and binding error details are never returned. Refund responses do not include code plaintext, code hints, or code hashes.
- `GET /api/v1/admin/liandong/restock/parity` was registered at the root agent's request; its handler/service are owned by the root agent.

## Durable safeguards

Migration `backend/migrations/239_liandong_unused_code_refunds.sql` creates a permanent ledger with unique external order, code digest, redeem-code ID, and merchant reference. A composite FK binds the batch and digest to the existing LDXP inventory manifest; a restrictive FK retains the redeem-code row. Record identity and completed confirmation are immutable. State checks forbid confirmation without a merchant reference and timestamp.

Database triggers reject updates or deletion of frozen redeem-code rows, and reject deletion or truncation of refund records. Cascading truncation also reaches the statement guard. Normal code management continues for codes without a refund reservation. No repository API bypass or membership/audit fixture was weakened. There is deliberately no cancellation or re-enable operation for a reservation; unresolved merchant refunds require operational reconciliation while the entitlement remains frozen.

## Source ownership

- New `backend/internal/service/liandong_refund.go`, `liandong_refund_test.go`, and `liandong_refund_integration_test.go`.
- Minimal additions to `backend/internal/handler/admin/payment_liandong.go`; new `payment_liandong_refund_test.go`. A separate narrow refund interface leaves existing restock implementations and test stubs compatible.
- Refund/parity registrations in `backend/internal/server/routes/payment.go` and authentication coverage in `payment_liandong_routes_test.go`.
- Two exact body-omission entries in `backend/internal/server/middleware/audit_log.go`; new `audit_log_liandong_refund_test.go`. Existing membership audit protections remain intact.
- New migration 239, coordinated with the root agent before creation. Historical migration bytes were not edited.

## Validation

Focused service, handler, audit, and route tests passed; captured command/output is in `refund-unit.log`.

The isolated integration test starts a disposable PostgreSQL 18.1 container on a dynamic port, applies all real migrations, and exercises actual `RedeemService` and repositories. It verifies:

1. Preparation and confirmation replay, changed identity/reference rejection, used-code rejection, and unchanged balance on refund rejection.
2. Twelve races between preparation and two simultaneous redemptions: either one $5 balance credit or one refund reservation, never both and never a double credit.
3. Six concurrent identical preparations produce one immutable reservation.
4. Generic single/batch updates and deletion cannot reactivate or remove frozen codes; normal codes remain manageable.
5. Injected reservation-write failure rolls back the disable operation, leaving the unused code redeemable exactly once.
6. Database uniqueness and state checks reject duplicate/invalid ledger entries.
7. A queued generic admin update that began before refund commit still sees the durable reservation and cannot reactivate the code.
8. A rollback-only red case removes the statement trigger and reproduces the old truncation/reactivation bypass; the restored guard rejects direct and cascading truncation.

The first two recorded integration passes are retained in `refund-integration.log` and `refund-integration-final.log`. The final migration including truncation protection passed all eight cases: `go test -tags integration -p 1 ./internal/service -run TestLiandongUnusedCodeRefundIntegration -count=1 -v` (backend cwd), exit 0, package duration 4.432 seconds. Complete output is in `refund-integration-truncate.log`; its PostgreSQL container was terminated successfully. `git diff --check` passed after formatting.

This is local machine evidence only. No production request, merchant operation, credential read, production database write, Git commit/push, live code redemption, or Docker service restart was performed. Full local CI, development/production parity, deployment, merchant money settlement, and human acceptance remain the root task's separate gates.

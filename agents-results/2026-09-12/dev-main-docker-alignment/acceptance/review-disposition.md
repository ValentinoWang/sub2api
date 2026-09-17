# Coordinator review disposition

Scope: user-authorized Git dev/main alignment and a single Docker artifact. Production cutover and real commerce acceptance are not included in the completion claim.

- Recharge M1: fixed by commit 64f459e05d2ec50ac997dd6933794a1fcccaac61. The 33 payment/panel tests passed; independent re-review confirms the disabled subscription tab and stale query behavior are fixed.
- Build F-01: rejected by direct Docker evidence. docker-build.log records successful COPY docs/legal at step 28 and the complete frontend build at step 29. Docker ignore re-inclusion behavior is not Git ignore behavior. No source change is warranted.
- Build F-02: retained as a limitation of the pre-existing GoReleaser/tag release route. This task builds with the root Dockerfile; no tag workflow or GoReleaser publication was invoked. Do not use that alternate image route as a substitute for this artifact.
- Build F-03: resolved in commit 1160e8a3d. Removed one obsolete source-string assertion and added six actual rendered-theme scenarios. All 18 related layout tests passed, with existing CSS-token assertions retained.
- Build F-04: GitHub CI remains blocked by billing. Docker proof is being completed independently; no CI success is claimed.
- Build F-05: rejected. fork is the authoritative user-owned GitHub repository ValentinoWang/sub2api. The build script intentionally overrides the inherited upstream label with the actual source repository. Final image readback must verify that override.
- Build F-06: do not promise byte-for-byte reproducibility from mutable base tags. Build one image, retain its content ID and archive SHA-256, and promote those same bytes to each environment.
- The build review read incomplete mutable logs and prematurely classified backend integration as passing. The completed backend-integration.log and process exit 2 supersede that classification: existing middleware reaper conflict, cache cancellation timing, and append-only membership audit fixture failures remain. Backend product source is unchanged from base 20189b734 except VERSION.

No overall production READY or human PASS is declared.

Public review M-01 duplicates Recharge M1 on the original frozen tree. It is resolved by 64f459e05 and the same focused independent re-review; no additional public finding was established. Build F-04 artifact portion is now satisfied by docker-build.log, image-verification.json, image-smoke.json and archive.json; GitHub CI billing remains blocked.

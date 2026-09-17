# Plugin compatibility with four-part fork releases

Scope: plugin host version compatibility only. No VERSION/build-script changes, dependency changes, commit, deployment, credential access, or full CI execution in this follow-up.

## Finding and execution path

This was an actual plugin regression for the candidate host version `0.2.4.1`, not an unused helper concern:

1. `backend/cmd/server/main.go:146` constructs `handler.BuildInfo` using the binary `Version`.
2. `backend/cmd/server/wire.go:78` and generated `wire_gen.go:395` pass that string unchanged into `PluginHostInfo`.
3. `plugin_package.go:122` evaluates compatibility during package installation; a false result records the installation as incompatible.
4. `plugin_manager.go:574` evaluates compatibility before enable; a false result refuses enable and records incompatible state.
5. Previously, `normalizeSemver("0.2.4.1")` returned an empty string because four numeric components are not SemVer. `matchesSemverRange` therefore rejected every manifest range.

## Change and meaning

`plugin_compatibility.go` adds a separate `normalizeSub2APIVersion` for host versions, requirement bounds, and explicitly tested host versions. It maps canonical numeric `X.Y.Z.N` to the comparison value `vX.Y.Z-fork.N`. The actual displayed/current version remains `0.2.4.1`.

This gives numeric monotonic ordering:

`0.2.3 < 0.2.4.1 < 0.2.4.2 < 0.2.4.10 < 0.2.4`

The final value in that ordering is the upcoming official upstream release. A fork revision does not satisfy `>=0.2.4` or `=0.2.4`, and a plugin that only declares testing upstream `0.2.4` does not attest a fork revision. A plugin range compatible with the fork can remain `untested`, preserving the existing administrator-confirmation boundary. Exact fork tested versions and explicit fork range bounds work.

The existing strict `normalizeSemver` remains unchanged, including its use in `PluginManifest.Validate` for the plugin package's own version. Four-part plugin package versions remain invalid. Existing three-part host/range behavior is preserved. Invalid extra components, alphabetic revisions, and leading zero numeric identifiers fail closed.

## Verification

RED: after adding the new tests but before changing production code, the targeted suite failed at the expected compatible-fork assertion and six true range cases. The package-version rejection test already passed.

Focused green command (backend working directory):

```bash
GOMAXPROCS=2 GOFLAGS=-p=1 go test -count=1 ./internal/service -run '^Test(EvaluatePluginCompatibility|EvaluatePluginCompatibilityRejectsProtocolMismatch|MatchesSemverRange|EvaluatePluginCompatibilityFork|MatchesSemverRangeFork|PluginPackageVersionRejectsForkNotation|PluginPackageInstallerKeepsHostVersionMismatchDisabled)$'
```

Result: PASS, `ok github.com/Wei-Shaw/sub2api/internal/service 1.329s` (uncached, `-count=1`). The selection includes original three-part/protocol and package installation mismatch tests plus three new top-level tests, including a 15-case fork ordering/invalid-input table. `gofmt` and `git diff --check` pass.

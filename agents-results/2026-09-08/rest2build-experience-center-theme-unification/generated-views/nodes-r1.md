### Release R1 node contract summary

- Title: 本地经验中心与统一主题候选
- User value: 使用者在本地体验经验中心的主题目录和卡片内容，能够从 P1 接入帮助进入相应经验，并在统一的中文界面中完成浏览。
- Independent failure: 任一页面仍为单列低密度经验列表、P1 内容没有主题入口、中文页保留未经允许的英文标题、深色导航出现割裂或横向溢出，都使本地候选不通过；不会触发远端写入或生产发布。

| Task ID | Semantic key | Work kind | Write authority | Side effect class | Hard dependencies | Soft dependencies | Decision refs | Execution contract ref | Acceptance contract ref |
|---|---|---|---|---|---|---|---|---|---|
| F | source.identity-baseline | contract-foundation | evidence-only | none | none | none | none | nodes/F.json | none |
| D1 | rest2build.experience-center.local-hmr-scope | decision-acceptance | isolated-record | none | F | none | none | nodes/D1.json | none |
| N1 | requirement.n1 | implementation | implementation | reversible | F; D1 | none | rest2build.experience-center.local-hmr-scope@1 | nodes/N1.json | acceptance-fragments/N1/acceptance-contract.md |
| N2 | requirement.n2 | implementation | implementation | reversible | F; D1; N1 | none | rest2build.experience-center.local-hmr-scope@1 | nodes/N2.json | acceptance-fragments/N2/acceptance-contract.md |
| N3 | requirement.n3 | implementation | implementation | reversible | F; D1; N1 | none | rest2build.experience-center.local-hmr-scope@1 | nodes/N3.json | acceptance-fragments/N3/acceptance-contract.md |
| N4 | requirement.n4 | implementation | implementation | reversible | F; D1; N1 | none | rest2build.experience-center.local-hmr-scope@1 | nodes/N4.json | acceptance-fragments/N4/acceptance-contract.md |
| N5 | requirement.n5 | implementation | implementation | reversible | F; D1; N2; N3; N4 | none | rest2build.experience-center.local-hmr-scope@1 | nodes/N5.json | acceptance-fragments/N5/acceptance-contract.md |
| QR1 | acceptance.release.r1 | release-decision | shared-generated | none | N1; N2; N3; N4; N5 | none | rest2build.experience-center.local-hmr-scope@1 | nodes/QR1.json | acceptance-fragments/QR1/acceptance-contract.md |

### Release R1 status ledger

| Task ID | Stage | Versions | Execution state | Attempt | Execution owner | Acceptance mode | Acceptance authorities | Quorum | Minimum trust | Guard ID | Blocking reason | Evidence | Unlocks |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| F | R1 | none | ACCEPTED | none | 编排器 | none | none | none | none | none | none | none | D1; N1; N2; N3; N4; N5 |
| D1 | R1 | none | ACCEPTED | none | 决策负责人 | none | none | none | none | none | none | none | N1; N2; N3; N4; N5 |
| N1 | R1 | rest2build.experience-center.local-hmr-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N1/acceptance-contract.md | N2; N3; N4; QR1 |
| N2 | R1 | rest2build.experience-center.local-hmr-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N2/acceptance-contract.md | N5; QR1 |
| N3 | R1 | rest2build.experience-center.local-hmr-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N3/acceptance-contract.md | N5; QR1 |
| N4 | R1 | rest2build.experience-center.local-hmr-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N4/acceptance-contract.md | N5; QR1 |
| N5 | R1 | rest2build.experience-center.local-hmr-scope@1 | PLANNED | none | implementation-owner | independent | acceptance-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/N5/acceptance-contract.md | QR1 |
| QR1 | R1 | rest2build.experience-center.local-hmr-scope@1 | PLANNED | none | 发布决策负责人 | external | release-owner:product-owner | 1 | repository-bound | none | none | acceptance-fragments/QR1/acceptance-contract.md | none |

### Release R1 deliverables

| Deliverable ID | Owning node |
|---|---|
| none | none |

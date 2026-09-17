# Acceptance Run: 20260916T091331Z-local-0ab7ad

- Run ID: 20260916T091331Z-local-0ab7ad
- Task ID: cost-native-completion
- Lane: machine/integration-contract
- Status: PARTIAL
- Acceptance contract: agents-results/2026-09-16/cost-native-completion/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 545b4a1c1c97ef13bead61e87688828139d5c2ee84fbef9e1eed5f15399db341
- Source identity: working-tree-cost-native-radar-20260916
- Runtime identity: native-go-postgresql-vue
- Executor or reviewer: Codex
- Started at: 2026-09-16T09:13:31.297779Z
- Completed at: 2026-09-16T09:22:03.870318+00:00
- Evidence directory: evidence/

## Scope

本地 Go 成本模块、独立 PostgreSQL、当前 Vue 成本页面、真实来源只读连接，以及 Codex Radar 的按需公开读取。此结果不代表全项目代码提升、人工签署或生产发布。

## Procedure

执行原生包 race 与 integration 测试、完整 Go 服务编译、Linux 本地成本服务编译、Vue 类型和 ESLint 检查、四项前端交互测试、完整 Vite 构建和公开页面预渲染、17 项 Python 回归、3 项空间门禁测试。通过原生 Go 进程读取真实 Radar，回读 PostgreSQL；通过 Chrome 核对参考资料、账本、账号列表和填写说明。编译均采用 4 GiB 提前停止阈值，保留 3 GiB 底线。

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-01 | PASS | evidence/radar-tests.log | 实库事务、并发幂等、FIFO、过期损失、历史和精确金额回归 |
| AC-02 | PASS | evidence/radar-tests.log | 来源只读事务、账号有效区间与完整交付范围对账；真实绑定仍需用户提供期间依据 |
| AC-03 | PASS | evidence/radar-tests.log | 双窗口、流量复位、条件预测；公开参考不提升为实测容量 |
| AC-04 | PASS | evidence/runtime-readback.json | Go/Vue 编译与本地链路读取；局部验证不替代完整项目 CI |
| AC-05 | PASS | evidence/complete-server-build-space-2.json | 完整服务编译最低余量超过 6 GiB；先前提前停止记录单独保留 |

## Findings

Radar 当前公开页采用 Pro 20x 单模型卡片，周期与价格版本未声明；七天均值和两个月频率保持 null。8 个真实缓存观察的窗口时长为 0，原样保留并标记未知。自动观察没有触发供应商重置。默认折叠通过原生 details 无 open 属性实现，整体说明、入门说明和每个词条均折叠。

全项目本地 CI 在既有未提交源码预检失败，详细阶段状态保存在 ../../../../local-ci-01/summary.json；未绕过门禁，未代替其他任务提交源码。因此整轮结论为 PARTIAL。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/acceptance-layout.log | a64c1aec21ee4003a6e87a7c6aa4f0c21cc3d629c5ddd4a2d9a38e60135a9e57 | 本地验证证据 |
| evidence/browser-observations.json | 4f51e0ec80835d4f018b9fc51c8ba98c222c5ea2dae63744761a6240d1807858 | 本地验证证据 |
| evidence/complete-server-build-space-2.json | 3bc7ef9a6ac51cbd88845b09c365253d5595f2463db701f84484cc0a16f6992b | 本地验证证据 |
| evidence/final-lint.log | e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855 | 本地验证证据 |
| evidence/final-typecheck.log | cf883070db9234ad5a34cfe4118f1f18728162fe8a260e666d91d26fc8b969aa | 本地验证证据 |
| evidence/final-vue-tests-all.log | 889e803e7f3cdce449ee08b44d1aa899f4d54cdae49490ca6ef0efc0d243e91c | 本地验证证据 |
| evidence/frontend-build.log | 806151f280d4893535005d8a21467ffa674dc3215ca724bf132fc9070be401e7 | 本地验证证据 |
| evidence/python-regressions-2.log | 4d07f7afd5ffbc9f8747420ba88b724139b7611a7ba217a806910dc806c627a8 | 本地验证证据 |
| evidence/radar-live.json | 8b7e23691fb29c0ca09f39e216c20b0c63f810187da549342622cc18d0b21c56 | 本地验证证据 |
| evidence/radar-runtime-build-space.json | 2bddab4d59aeb2b6425ccf55e959cf42aa5f0f1d6d4841e8fc3e043291cad7de | 本地验证证据 |
| evidence/radar-tests-space.json | 7e9bf10c9cab1ee3c36649e5a2efc985424538fefa8c43a15cdeeceb414e76ae | 本地验证证据 |
| evidence/radar-tests.log | 26991cdefa210c951a6af6cdf7cc165da11f62a529fd7ee5cbafbf4a490e634e | 本地验证证据 |
| evidence/runtime-readback.json | 2571bad14e550b52f9b239a10e9d3935fa196484c5edf508f46b8eae7b4d98fd | 本地验证证据 |
| evidence/source-identity.json | cb92f5c858e0575a3325a344e8ee3fd371185a5a8cb452b8b624f9bf06c96a58 | 本地验证证据 |
| evidence/space-guard-tests.log | 619715fb0704886d0c58f2bb38fa9294245cbf093e0f61114b323c54bdd2f6d5 | 本地验证证据 |

## Unverified items

完整项目 CI、移动端视觉矩阵、Radar 按钮的最终浏览器点击、真实上游重置后的钩子执行、真实采购凭据完整性和人工签署未作为已通过项目。Radar 的实际 Go 网络读取及 PostgreSQL 保存已经独立验证。真实账号档位由管理员明确选择，不自动推断。

## Conclusion

PARTIAL：本地原生功能可用且针对性验证通过，全项目提升与人工终态未完成。三档完整经营底价仍需要实际凭据、有效归属和覆盖完整的用量资料。

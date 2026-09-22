# Acceptance Contract: cost-native-completion

- Task ID: cost-native-completion
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 使用者
- Approval evidence: 2026-09-16 用户要求补齐原生成本后端及本地联调；合同与人工清单待人工审阅
- Request source: 2026-09-16 用户请求
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: cost.native-ledger, cost.fifo, cost.intervals, cost.quota-observations, cost.local-source
- AC budget: 5
- Baseline identity: 本轮工作区源码；编译、运行和空间证据分别记录
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: acceptance/human/2026-W38/2026-09-16-cost-native-completion

## User and scenario

管理员核对公开订阅参考资料，录入实际付款和预充值核销，明确绑定账号在各档位的生效期间，并只读核对网关用量。采样与预测按显式提供的资料计算。

## Expected outcome

本地参考目录与账本可读；预充值按先入先出批次实际现金价格核销；账号升级期间不重叠；网卡复位、未知窗口和不完整曝光不会产生虚构容量或零费用。资料保存在独立成本数据库，来源数据库只读。

## Non-goals

本次不发布生产，不购买、不充值、不重置真实上游额度，不代填实际采购凭据或选择真实账号档位。手工成本、网关用量、参考计费金额和条件预测不能被当作现网完整经营底价。

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/integration-contract | PostgreSQL 追加账本、并发幂等、过期批次和精确 FIFO 成本正确，历史凭据不会被倒填批次重新定价 | Integration | Automatic | Yes |
| AC-02 | behavior | none | machine/integration-contract | 账号档位期间不重叠；来源事务只读；用量和交付仅在相同完整期间对账 | Integration | Automatic | Yes |
| AC-03 | behavior | none | machine/integration-contract | 流量计算保留计数器复位和缺口；双窗口共同约束交付；贝叶斯结果明确为条件情景 | Integration | Automatic | Yes |
| AC-04 | behavior | none | machine/integration-contract | Go 编译、Vue 类型与交互检查通过；真实本地目录、账本及账号列表可读取，未知额度时长保留质量标记 | Runtime | Automatic | Yes |
| AC-05 | behavior | none | machine/integration-contract | Go 运行期间保留至少 3 GiB；4 GiB 阈值阻止启动或停止本轮编译进程组 | Unit | Automatic | Yes |

## Human acceptance

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 分别核对三档资料、账本和业务提示，并理解数据覆盖边界 | acceptance/human/2026-W38/2026-09-16-cost-native-completion/checklist.md#h-01 | 业务管理员 | Yes |

## Permissions and data impact

现有本地管理员会话验证操作人。采购、分析资料与额度观察写入独立成本库，来源账号与用量使用只读事务。正式业务绑定由管理员明确选择，不推断套餐。示例资料均为合成数据，不用于经营结论。

## Risks and release boundary

全项目本地 CI 需经过其既有提交快照预检；工作区含其他任务未提交改动时保留失败，不修改门禁或替其他任务提交。只有当前源码的局部编译和测试通过时不能宣称全项目提升通过。人工签署与生产发布均另行保留状态。

## Problem

原生参考资料和账本尚未在本地运行时接通，研究算法与账号来源之间缺少持久化、边界校验和实库验证。

## Normal path

管理员读取参考资料，录入凭据与核销，明确账号期间；成本库保存追加记录，来源库只读返回同期用量。全部缺失条件保留状态。

## Exception paths

身份无效则拒绝；同键内容冲突则拒绝；额度不足或批次过期则拒绝核销；账号区间重叠则拒绝；数据读取失败不当作零量。资料不完整不推算容量。

## Invariants

采购现金与参考计费分开，预充值批次与订阅期间费用分开，原记录不覆盖；观测与预测分开。不能自动选择或改变真实账号套餐。

## Data impact

仅向专用成本库追加金额记录与脱敏采样。真实来源事务只读，不读取凭据字段，不改变自用业务库。

## Permissions

已有本地管理员会话验证身份。来源连接同时设置默认只读和只读事务；成本运行角色不能修改、删除或截断已有事件。

## Performance and reliability

本地接口有请求大小与超时限制；账本保持低频事件上限，采样单独存表。写入在数据库锁下幂等提交。Go 编译由磁盘余量监测包裹。

## Protected acceptance tests

保留全部既有期间账、幂等、权限与持久化测试，扩展精确金额、过期、区间与未知采样测试，不弱化断言。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 原生账本、FIFO 与 PostgreSQL 并发测试 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-02 | 只读事务、分段用量与匹配范围测试 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-03 | 流量、双窗口与条件预测用例 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-04 | Go 编译、Vue 检查与本地读取 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-05 | 余量门禁红例、运行监测与最低空间读回 | acceptance/machine/integration-contract/ | Automatic | Yes |
| H-01 | 三档资料及账本业务核对 | acceptance/human/2026-W38/2026-09-16-cost-native-completion/checklist.md#h-01 | Human | Yes |

## Exploratory testing

观察默认空值、连接中断、资料类型切换及读回提示是否清楚；不使用合成资料替代真实账号与凭据的业务核对。

## Production monitoring and rollback

本地成本服务可停止，专用成本数据保留；移除本地成本代理配置即可恢复默认页面代理。生产环境不在本轮发布范围。

## Risks and open decisions

真实发票完整性、网关之外的直接使用量、缺失窗口时长以及人工业务确认仍需实际材料。全项目 CI 未通过时不能声称代码提升完成。该合同保留 DRAFT，机器结果和人工签署分别记录。

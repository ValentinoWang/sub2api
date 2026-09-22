# Acceptance Contract: local-ci-commerce-alignment

- Task ID: local-ci-commerce-alignment
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 使用者
- Approval evidence: 用户已授权本任务修复本地 CI、同一镜像对齐及充值商品闭环；本合同与人工清单尚待审阅，不构成人工通过或开售授权
- Request source: 2026-09-13 当前用户任务
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: commerce.environment-isolation, commerce.five-unit-credit, commerce.unused-refund, commerce.small-stock-retry, release.local-ci-image-parity
- AC budget: 9
- Baseline identity: candidate commit 8365f45ea179b99e627cfbbf751b604a45ad2eee；最终本地 CI 已通过，证据为 acceptance/local-ci-final/summary.json；真实业务与人工验收尚未通过
- Product Context refs: agents-results/2026-09-13/local-ci-commerce-alignment/inventory-ui-context.md
- Role Context refs: agents-results/2026-09-13/local-ci-commerce-alignment/inventory-ui-context.md
- Resolved Surface Contract refs: agents-results/2026-09-13/local-ci-commerce-alignment/inventory-ui-context.md
- Screen Contract ref: agents-results/2026-09-13/local-ci-commerce-alignment/inventory-ui-context.md
- Visual Contract refs: agents-results/2026-09-13/local-ci-commerce-alignment/inventory-ui-context.md
- UI Change declaration: agents-results/2026-09-13/local-ci-commerce-alignment/ui-change.json
- Human acceptance workspace: acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment

## User and scenario

测试购买者与正式商品的受控验收购买者分别使用独立商品及各自的 Sub2API 账户。业务管理员负责少量库存、异常核对和未使用权益的退款协作。本任务验收已有用户界面上的实际业务闭环，并补充管理员库存数量与不确定性展示；结算通过独立只读对账报告核对。

## Problem

本地 CI 的失败必须修复，之后才能提升明确的代码候选；相同构建内容不应掩盖测试与生产在商品归属、到账规则和库存处理上的差异。购买、发货、兑换和退款不能以配置存在或健康检查代替实际验证。

## Expected outcome

两套独立商品均为人民币 5 元换取美元计价余额 5 元，各自在指定账户服务中完成购买、发货与一次到账；错误入口和重复操作不产生权益。未使用权益可先冻结再按商家流程退款，已兑换权益不得进入此退款流程。少量补货、重试及库存差异具有清楚可见的结果。真实商户结算账单与独立本地销售台账按订单和结算日期逐项核对人民币总额、手续费、退款和净额。

## Non-goals

不开发链动小铺原生支付，不验证提现、跨币种兑换、已使用权益退款或其他面额、会员商品。没有完成实际签署时不宣称人工验收通过；本文不授权无边界购买、开售、批量补货、删除业务记录或重置本地真实数据。合同、机器结果及人工结果均不创建 SSOT 或 Obsidian 快照。

## Normal path

```gherkin
Given 测试商品与正式商品已独立准备，均明确售价人民币 5 元及到账美元余额 5 元
When 购买者在获准商品完成购买并在对应服务的兑换页面使用收到的兑换内容
Then 对应账户只增加美元余额 5 元，刷新后保留结果，另一服务不产生余额变化
And 管理员可核对发货及少量库存的实际变化
```

## Exception paths

错误服务入口、重复兑换、刷新和提交重试不得重复到账；未发货、无法确认库存或外部服务超时必须显示可理解的状态。仅未使用的权益可预留退款，冻结后不可兑换或重新启用；重复预留和重复确认不得生成第二笔权益或退款结果。商家退款结果不确定时不得写成已退款。补货结果不确定时暂停追加，先核对既有库存再恢复。

## Invariants

测试与生产的数据、商品标识及发码材料相互独立；商品语义和一比一余额规则一致。Sub2API 是余额账本，一份兑换内容最多到账一次。退款预留只冻结未使用权益，不扣除已到账余额；本地记录商家退款参考不等于商家已退钱。库存仅按获准的 1 至 10 件少量计划处理。敏感值不进入合同、人工步骤、普通日志或公开证据。

## Data impact

实现涉及退款记录迁移、权益冻结及永久保护；部署前必须验证同一源码和镜像在独立数据环境上的模式与迁移校验和。实际业务验收会分别产生购买、发货、兑换、退款协作及少量补货记录，必须保留可追溯事实，不以删除记录回退权益。破坏性数据库测试仅使用临时隔离容器。

## Permissions

购买者只使用自己的获准账户和指定商品。商品管理、补货和退款处理仅由业务管理员执行。正式购买及真实商家退款由当前任务明确授权范围和商家规则约束，缺少授权或可用业务入口则阻塞。人工通过不自动授权开售或真实上游提交。

## Performance and reliability

本合同不新增吞吐或延迟承诺。关键要求为并发兑换与退款至多一次权益、请求中断后的幂等恢复，以及外部补货结果不明时停止追加并显示待核对状态。等待超过商品标注时限应记录为阻塞或失败，不能无限重试。

## Acceptance criteria

| ID | Class | Lane | Requirement | Mode | Blocking |
| --- | --- | --- | --- | --- | --- |
| AC-01 | process-provenance | machine/static | 本地 CI 绑定明确提交及树，全部必需阶段实际执行并通过；失败、跳过或未执行不能作为通过，保留阶段日志及摘要 | Automatic | Yes |
| AC-02 | process-provenance | release | 本地与生产运行同一明确构建镜像；技术版本、源码提交、镜像身份、模式哈希及非空迁移校验集合一致，数据环境身份独立 | Automatic | Yes |
| AC-03 | behavior | machine/integration-contract | 两边分别存在独立人民币 5 元商品、美元余额 5 元及相同业务规则；商品与发码材料不共用，商品及补货映射严格校验 | Automatic | Yes |
| AC-04 | behavior | machine/integration-contract | 正确入口兑换仅到账一次，错误归属、重复提交及并发操作不增加额外权益 | Automatic | Yes |
| AC-05 | behavior | machine/integration-contract | 仅未使用权益可退款预留，预留与冻结原子完成，重复处理幂等；已用码被拒绝，冻结与记录不可删除、截断或重新启用 | Automatic | Yes |
| AC-06 | behavior | machine/integration-contract | 少量库存及补货数量限制为 1 至 10，重试不重复发码或增库存，无法对账时停止追加并保留可恢复状态 | Automatic | Yes |
| AC-07 | behavior | machine/non-functional | 未授权及非管理员不能管理商品、补货或退款；敏感退款输入不出现在响应与审计正文，脱敏快照拒绝未知或敏感字段 | Automatic | Yes |
| AC-08 | behavior | external-sandbox | 分别保存测试与生产受控商品的实际购买、发货、兑换、重复、退款和库存核对记录；销售一致性门禁的事实来自这些记录，不以手填布尔值或单套记录替代两套实际执行 | Automatic | Yes |

| AC-09 | behavior | machine/integration-contract | 独立商户账单与本地销售台账使用精确人民币分计算，按订单及结算日核对总额、手续费、退款、净额；缺失、重复、冲突、金额不平与非法输入阻塞对账，不用美元额度冒充人民币实付或捏造供应商账单 | Automatic | Yes |

## Human acceptance

逐项独立签署，两种商品不得互相替代。当前没有真实交易、人工执行或签署结果；合同和清单为草稿，不生成 handoff。

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 测试商品购买者能理解归属并完成购买、发货、正确兑换与重复确认 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-01 | 测试购买者 | Yes |
| H-02 | 正式商品购买者能理解归属并完成购买、发货、正确兑换与重复确认 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-02 | 正式商品验收购买者 | Yes |
| H-03 | 测试商品未使用权益退款可理解且已兑换权益不会误退 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-03 | 业务管理员 | Yes |
| H-04 | 正式商品未使用权益退款可理解且已兑换权益不会误退 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-04 | 业务管理员 | Yes |
| H-05 | 测试商品少量补货、重试和库存差异能被管理员看懂并处理 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-05 | 业务管理员 | Yes |
| H-06 | 正式商品少量补货、重试和库存差异能被管理员看懂并处理 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-06 | 业务管理员 | Yes |

| H-07 | 业务管理员能从真实结算核对结果中逐项解释金额及差异 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-07 | 业务管理员 | Yes |

## Protected acceptance tests

none；本合同为事后补齐的运营验收草稿，不追溯声明既有测试已锁定，不授权改写或弱化测试。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 本地 CI 完整阶段及防空跑门禁 | tools/quality/run_local_ci.sh、tools/quality/test_local_ci.py；acceptance/ 下唯一执行目录 | Automatic | Yes |
| AC-02 | 同一镜像发布读回及模式、迁移对比 | tools/quality/check_commerce_environment_parity.py；acceptance/release/ | Automatic | Yes |
| AC-03 | 商品一致性门禁与运行配置脱敏读取 | tools/quality/tests/test_commerce_environment_parity.py、backend/internal/service/liandong_parity_test.go | Automatic | Yes |
| AC-04 | 原子兑换、错误归属和重复执行 | backend/internal/service/liandong_refund_integration_test.go；acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-05 | 退款永久保护、故障回滚及并发竞争 | backend/internal/service/liandong_refund_integration_test.go、backend/internal/handler/admin/payment_liandong_refund_test.go | Automatic | Yes |
| AC-06 | 补货状态、少量库存约束及一致性门禁 | backend/internal/service/liandong_restock_persistence_test.go、tools/quality/tests/test_commerce_environment_parity.py | Automatic | Yes |
| AC-07 | 管理权限、输入省略及脱敏边界 | backend/internal/server/routes/payment_liandong_routes_test.go、backend/internal/handler/admin/payment_liandong_refund_test.go；acceptance/machine/non-functional/ | Automatic | Yes |
| AC-08 | 两套实际业务事实与销售一致性结果绑定 | acceptance/external-sandbox/、acceptance/production/；人工结果仅引用项目人工工作区 | Automatic | Yes |
| H-01 | 对应完整业务闭环的实际操作与双人签署 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-01 | Human | Yes |
| H-02 | 对应完整业务闭环的实际操作与双人签署 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-02 | Human | Yes |
| H-03 | 对应完整业务闭环的实际操作与双人签署 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-03 | Human | Yes |
| H-04 | 对应完整业务闭环的实际操作与双人签署 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-04 | Human | Yes |
| H-05 | 对应完整业务闭环的实际操作与双人签署 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-05 | Human | Yes |
| H-06 | 对应完整业务闭环的实际操作与双人签署 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-06 | Human | Yes |

| AC-09 | 独立账单逐笔与日合计对账，精确金额及异常输入 | tools/quality/tests/test_liandong_settlement.py；acceptance/settlement/ | Automatic | Yes |
| H-07 | 实际账单和独立台账核对与签署 | acceptance/human/2026-W37/2026-09-13-local-ci-commerce-alignment/checklist.md#h-07 | Human | Yes |

## Exploratory testing

分别观察两个商品的说明、兑换去向和退款措辞是否容易混淆；使用刷新、短暂断网及重新进入流程，判断用户能否理解当前结果。此类探索不替代关键自动断言或正式人工签署。

## Production monitoring and rollback

机器侧记录发布前后健康和关键功能、部署身份、独立环境快照及销售门禁结果；这些证据不进入人工清单。任何重复到账、错误归属、冻结权益被使用或无法解释的库存差异都阻塞继续销售与补货，由业务负责人组织核对。部署异常按已授权发布记录回退镜像；不得通过回退或删除退款记录恢复已经冻结的权益。

## Risks and open decisions

当前完整本地 CI 尚在执行；两套商品的实际购买、发货、兑换、退款、补货与签署均未完成。可用商品、独立账户、真实商家权限及退款处理入口由业务负责人准备；缺失任一项保持阻塞。商家实际退钱只能由商家和购买者的可见结果证明，本地参考记录不能替代。所有必需机器证据和当前合同审批完成后再通过中央工具入队，人工执行属于后续终态阶段。

# Acceptance Contract: ldxp-local-recharge-debug

- Task ID: ldxp-local-recharge-debug
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 使用者
- Approval evidence: 2026-09-12 用户授权启动本地服务并进行链动小铺充值调试
- Request source: 2026-09-12 用户请求
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: ldxp.local-recharge-flow, ldxp.local-runtime
- AC budget: 4
- Baseline identity: 本地运行中的 sub2api-local:0.2.5-local-1b221364260b
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- Human acceptance workspace: acceptance/human/2026-W37/2026-09-12-ldxp-local-recharge-debug

## User and scenario

管理员在本机配置一个已获准用于测试的链动小铺商品；购买者从链动小铺购买该商品，并在本机 Sub2API 账户中兑换收到的兑换码。

## Problem

需要确认本机环境能支持链动小铺卡密销售后的余额充值闭环，而不是把链动小铺当作 Sub2API 的原生支付方式。

## Expected outcome

购买者收到兑换码后可在 Sub2API 成功兑换固定余额；重复兑换不会重复到账；管理员可在链动小铺工具中看到与本次测试相符的库存和处理状态。

## Non-goals

不接入链动小铺原生支付，不测试提现、结算、退款或远端生产环境；不由验收人员提供、记录或展示任何密钥、完整兑换码或支付凭证。

## Normal path

```gherkin
Given 管理员已准备获准测试的链动小铺商品和 Sub2API 测试账户
When 购买者完成一次商品购买并在 Sub2API 输入收到的兑换码
Then 固定余额只增加一次，且管理员能观察到对应的库存和处理结果
```

## Exception paths

未登录或没有管理员权限的访问必须被拒绝。兑换码重复输入、页面刷新或恢复网络后再次提交时，余额不得重复增加；有异常时页面必须给出可理解的提示。

## Invariants

链动小铺仅作为外部卡密销售渠道。Sub2API 是余额权益的唯一账本；一张兑换码只能成功兑换一次；敏感配置不出现在页面或验收记录中。

## Data impact

一次验收可能产生测试兑换码、一次兑换记录和相应余额变动。验收结束后由业务负责人决定是否保留测试数据；本任务不执行清理或补货操作。

## Permissions

仅管理员可进入链动小铺工具并调整测试商品；购买者仅可使用自己的测试账号兑换码。非管理员访问应被拒绝。

## Performance and reliability

本地服务必须可访问。外部链动小铺页面、支付和发码结果由实际验收观察；网络中断后的再次操作不得造成重复余额。

## Acceptance criteria

| ID | Class | Lane | Requirement | Mode | Blocking |
| --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | machine/local-runtime | 本机后端健康可读，链动小铺管理入口存在且未认证访问被拒绝 | Automatic | Yes |
| AC-02 | behavior | machine/unit | 商品映射、批次恢复、重复兑换和敏感字段保护保持自动化测试通过 | Automatic | Yes |
| AC-03 | behavior | machine/integration-contract | 补货批次的映射、pending、uploaded、failed 状态可持久化读取 | Automatic | Yes |
| AC-04 | behavior | machine/unit | 迁移定义商品、批次和兑换码审计表的关键约束 | Automatic | Yes |

## Human acceptance

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 管理员能理解测试商品和库存状态，并完成一次受控测试准备 | acceptance/human/2026-W37/2026-09-12-ldxp-local-recharge-debug/checklist.md#h-01 | 业务管理员 | Yes |
| H-02 | 购买者能完成购买、兑换、余额核对和重复兑换确认 | acceptance/human/2026-W37/2026-09-12-ldxp-local-recharge-debug/checklist.md#h-02 | 测试购买者 | Yes |

## Protected acceptance tests

none；当前合同仍为草稿，既有自动化测试不在本合同下冻结。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 本机健康与未认证访问检查 | acceptance/machine/local-runtime/ | Automatic | Yes |
| AC-02 | Liandong service 和 handler 测试 | backend/internal/service/, backend/internal/handler/admin/ | Automatic | Yes |
| AC-03 | Liandong SQL mock 测试 | backend/internal/service/liandong_restock_persistence_test.go | Automatic | Yes |
| AC-04 | 迁移结构回归测试 | backend/migrations/liandong_sales_channel_migration_test.go | Automatic | Yes |
| H-01 | 受控测试商品准备 | acceptance/human/2026-W37/2026-09-12-ldxp-local-recharge-debug/checklist.md#h-01 | Human | Yes |
| H-02 | 购买和兑换闭环 | acceptance/human/2026-W37/2026-09-12-ldxp-local-recharge-debug/checklist.md#h-02 | Human | Yes |

## Exploratory testing

观察商品说明是否容易理解；在刷新页面或短暂网络中断后恢复兑换流程，记录任何难以理解的提示。

## Production monitoring and rollback

本任务仅覆盖本机运行。测试中出现非预期余额、库存差异或异常发码时，停止继续购买和补货，保留业务可见的订单与页面信息，交由业务管理员处理。

## Risks and open decisions

真实购买、发码和库存变化依赖链动小铺测试商品及其外部服务可用性。尚未取得的原生支付协议不属于本次验收范围。

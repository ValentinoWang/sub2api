# Acceptance Contract: ldxp-restock-recovery

- Task ID: ldxp-restock-recovery
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 使用者
- Approval evidence: 用户本轮要求参考GitHub开源方案并完成本地补货恢复；未签署发布验收
- Request source: 本轮用户指令
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: ldxp.delivery-proof, ldxp.original-batch-retry, ldxp.provider-dedup
- AC budget: 5
- Baseline identity: 当前工作区；执行记录绑定源码摘要
- Product Context refs: none
- Role Context refs: none
- Resolved Surface Contract refs: none
- Screen Contract ref: none
- Visual Contract refs: none
- UI Change declaration: none
- SSOT path: none
- SSOT node: none
- Human acceptance workspace: none

## User and scenario

本机补货守护进程维护5个测试商品每档999张，商户HTTP上传结果未知时需继续完成原批次，不重复生成或售出同一码。

## Problem

当前仅核对未售库存，无法结束已售批次，也没有已验证的缺失码重试路径。一批20张当前未确认，不能以删除状态或虚报库存解除。

## Expected outcome

完整未售与已售快照确认交付；在已验证商户历史去重能力及本站原码权益准入下，只有限重试原批次缺失码，读回后恢复维护。

## Non-goals

不部署生产、不混用正式码库、不创建付费订单、不复制第三方开源项目业务代码，不宣称永久可用。

## Normal path

先核对所有商品及待处理批次，未售码用于库存计数，已售码仅作交付证明；缺失码准入通过后有限重试并逐码读回。

## Exception paths

登录失效、人工暂停、跨商品/设备、未知码、权益失效、分页不完整、未验证去重或重试用尽均不继续投放；重试计数与原码摘要持久保存。

## Invariants

本站是唯一权益账本；未售与已售集合分离；未知结果不得通过生成新批次掩盖；原批次所有权和码集合不可替换。

## Data impact

为测试商品少量投放原批次码；本地新增交付证明表。受控去重实验的重复测试记录精确清理，既有订单和余额不更改。

## Permissions

沿用本机测试商品补货、商户网页登录授权和本地开发权限。生产不在本次授权范围。

## Performance and reliability

每60秒调度，单商品每轮最多20张；恢复最多3次并指数退避。先完成所有相关商品核对后写入；任何未确认上传均保留原批次。

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | none | machine/local-runtime | 实测同商品已有未售、已有已售、并发相同新码去重，无额外可售重复码 | Runtime | Automatic | Yes |
| AC-02 | behavior | none | machine/unit | 完整双扫描、原商品身份、已售proof与可售库存分离，缺失原码重试且不claim新码 | Unit | Automatic | Yes |
| AC-03 | behavior | none | machine/unit | 全商品预检、能力/权益准入、持久退避预算及人工暂停生效 | Unit | Automatic | Yes |
| AC-04 | behavior | none | machine/integration-contract | 同事务校验原批次权益、持久交付证明与幂等完成；跨设备批次及旧证明不能完成新批次 | Integration | Automatic | Yes |
| AC-05 | behavior | none | machine/local-runtime | 当前20张原批次真实恢复，后续定时补货及重复核对正确 | Runtime | Automatic | Yes |

## Human acceptance

none。本轮不要求再次付款，不生成正式人工签署；有限本地实测不等于生产或全天候验收。

## Protected acceptance tests

保留原默认未开启恢复策略时的59项安全断言。新恢复契约使用独立新增测试，不弱化旧暂停保护。

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 真实去重边界实验 | acceptance/machine/local-runtime/ | Automatic | Yes |
| AC-02 | Python恢复回归 | acceptance/machine/unit/ | Automatic | Yes |
| AC-03 | 多商品、暂停、预算回归 | acceptance/machine/unit/ | Automatic | Yes |
| AC-04 | 临时Postgres交付证明回归 | acceptance/machine/integration-contract/ | Automatic | Yes |
| AC-05 | 原批次恢复和定时后续 | acceptance/machine/local-runtime/ | Automatic | Yes |

## Exploratory testing

商户去重能力仅为本轮实测证据，不冒充远端服务端唯一约束合同；发现行为变化或重复库存立即暂停。

## Production monitoring and rollback

仅更新本地测试后端；替换前保存旧镜像与配置身份、数据库备份，核对迁移。遇问题保留凭据与批次，不自动回退已发放权益。

## Risks and open decisions

商户登录态有效期、未来去重行为及未知持续故障无法无限保证。持续补满999和多SKU真实购买不能由本次单批恢复替代。正式环境部署需另行说明并授权。

# Acceptance Contract: proxy-subscription-groups

- Task ID: proxy-subscription-groups
- Contract version: 1
- Contract status: DRAFT
- Test baseline: PLANNED
- Acceptance owner: 产品负责人
- Approval evidence: TBD
- Request source: 2026-09-23 用户询问订阅导入方式并要求按订阅地址给出分组，选择“分组 + 排除信息条目 + 自动刷新”，并参考 Codex_degrade 的格式
- SSOT node: none
- SSOT path: none
- Readiness mode: FORMAL
- Decision refs: none
- Assumption IDs: none
- Invalidation keys: proxy-subscription-groups.format, proxy-subscription-groups.refresh
- AC budget: 4
- Baseline identity: 4ed420cac
- Product Context refs: agents-results/2026-09-23/proxy-subscription-groups/ui-context.md
- Role Context refs: agents-results/2026-09-23/proxy-subscription-groups/ui-context.md
- Resolved Surface Contract refs: agents-results/2026-09-23/proxy-subscription-groups/ui-context.md
- Screen Contract ref: agents-results/2026-09-23/proxy-subscription-groups/ui-context.md
- Visual Contract refs: agents-results/2026-09-23/proxy-subscription-groups/ui-context.md
- UI Change declaration: agents-results/2026-09-23/proxy-subscription-groups/ui-change.json
- Human acceptance workspace: acceptance/human/2026-W39/2026-09-23-proxy-subscription-groups

## User and scenario

管理员导入一个或多个代理订阅（节点列表或 Codex_degrade 这类 Mihomo YAML 订阅），在代理管理中按订阅查看分组、流量与到期，定时或手动刷新；在打票管理中按“🎫 打票出口”等分组把住宅节点一次选入打票代理池。

## Problem

订阅只支持节点分享链接，Mihomo YAML 订阅（包括 Codex_degrade 的住宅订阅）无法导入；剩余流量、到期、官网等说明条目被当成代理导入；订阅地址不保存，只能重新粘贴才能刷新；没有分组，打票代理池只能逐个勾选。

## Expected outcome

- 导入支持 Mihomo/Clash YAML 内联节点并保留订阅自带分组（嵌套分组展开，DIRECT/REJECT 去除，禁止链式拨号等越界字段），也支持原有节点列表。
- 名称含剩余流量、到期、重置、官网、发布页等的说明条目不导入为代理，作为订阅说明展示；旧订阅里已导入的说明条目在下次刷新时停用。
- 按 Codex_degrade 格式为节点生成规范名称和分组：🎫 打票出口（住宅/家宽）、💼 业务出口（机房）、“类型 · 倍率”、“国旗 国家 · 住宅/机房”；订阅自带同名分组优先。
- 订阅地址用服务端密钥加密保存，可立即刷新或按 15–1440 分钟定时刷新（默认 60 分钟，0 关闭）；刷新保持未变节点的代理编号，下线节点停用；读取 subscription-userinfo 的流量与到期；刷新失败记录原因且不包含订阅地址。
- 打票管理可按分组把当前可用代理选入或移出代理池。

## Non-goals

不支持 proxy-providers 远程节点集合；不重命名已有代理；不改变代理出口、账号绑定和打票调度逻辑；不在生产导入订阅或修改代理。

## Normal path

导入或刷新时拉取订阅，识别格式，分离说明条目，按指纹复用或创建代理并重载 Mihomo，保存加密地址、流量、说明和订阅分组；列表接口按节点名实时分类并生成分组。

## Exception paths

YAML 只有 proxy-providers、节点类型不受支持、端口非法或全部是说明条目时导入失败；未保存地址的订阅不能刷新并提示重新导入；拉取、解析或 Mihomo 重载失败时保留原节点并记录失败原因。

## Invariants

订阅地址、分享链接和节点凭据不出现在接口、页面和错误信息中；未变节点的代理编号不变；说明条目不进入任何分组。

## Data impact

不新增数据库迁移；订阅状态文件（权限 0600）增加格式、加密地址、刷新设置与结果、流量、说明和订阅分组字段；YAML 节点以 Mihomo 配置保存。

## Permissions

新增接口仅管理员可用；刷新和修改间隔需要管理员显式操作或已开启的定时刷新。

## Performance and reliability

定时刷新每分钟检查一次到期订阅，单次刷新 2 分钟超时；刷新与导入共用状态锁，串行执行。

## Acceptance criteria

| ID | Class | Source requirement refs | Lane | Requirement | Verification layer | Mode | Blocking |
| --- | --- | --- | --- | --- | --- | --- | --- |
| AC-01 | behavior | 用户分组与格式要求 | machine/unit | YAML 与节点列表解析、说明条目、分类与规范名称、分组、流量头、地址加密与脱敏 | Unit | Automatic | Yes |
| AC-02 | behavior | 用户自动刷新要求 | machine/integration-contract | 导入、刷新复用编号与停用下线节点、失败记录、间隔校验、Mihomo 接受生成配置、接口与路由 | Integration | Automatic | Yes |
| AC-03 | behavior | 用户页面要求 | machine/unit | 订阅与分组面板、导入间隔、打票管理按分组选入 | Unit | Automatic | Yes |
| AC-04 | behavior | 项目 CI 规范 | machine/integration-contract | 完整本地 CI 结果如实记录 | Integration | Automatic | Yes |

## Human acceptance

| ID | Summary | Checklist path | Required role | Blocking |
| --- | --- | --- | --- | --- |
| H-01 | 管理员能导入订阅、看懂分组并按分组选入打票代理池 | acceptance/human/2026-W39/2026-09-23-proxy-subscription-groups/checklist.md#h-01 | 产品负责人 | Yes |

## Protected acceptance tests

| Path | SHA-256 | Covers |
| --- | --- | --- |
| none | none | 未锁定人工验收基线；保留有效测试断言 |

## Requirements-test traceability

| Requirement | Verification | Evidence target | Mode | Blocking |
| --- | --- | --- | --- | --- |
| AC-01 | 后端定向测试 | acceptance/local-ci-01/summary.json | Automatic | Yes |
| AC-02 | 订阅导入与刷新测试、Mihomo 配置校验 | acceptance/local-ci-01/summary.json | Automatic | Yes |
| AC-03 | 前端组件与页面测试 | acceptance/local-ci-01/summary.json | Automatic | Yes |
| AC-04 | 完整本地 CI | acceptance/local-ci-01/summary.json | Automatic | Yes |
| H-01 | 管理员在演示环境导入订阅并选入分组 | acceptance/human/2026-W39/2026-09-23-proxy-subscription-groups/checklist.md#h-01 | Human | Yes |

## Exploratory testing

真实订阅服务商返回的节点命名是否都能被正确分类、真实 VPS2ISP 订阅导入后各节点能否连通，本次源码测试不提供证据。

## Production monitoring and rollback

本任务未决定生产发布；发布需另行确认并记录。

## Risks and open decisions

旧订阅没有保存地址，需要管理员重新导入一次才能刷新；节点分类依赖服务商命名，未知命名归为机房。人工验收保持待验收，不构造批准或签字。

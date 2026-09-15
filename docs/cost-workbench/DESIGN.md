# 管理员成本工作台设计

研究日期：2026-09-16。代码基线：`dev@507a820c6792782ac07f9962a06c6224ce6cba70`。

状态：**研究脚本、公开初步信息与可交互设计稿；不是已上线管理员功能。**

先运行 `python docs/cost-workbench/render.py`，再打开生成的 `docs/cost-workbench/dashboard.html`。生成器同时导出两张 SVG。

[工作台生成器](render.py) · [只读采集工具](../../tools/cost_research/README.md) · [公开数据快照](../../tools/cost_research/public_snapshot.json)

## 1. 结论与数据边界

建议新增 `/admin/cost-center`，复用账户、用量、上游倍率、OpenAI 额度、现有运维与管理员鉴权，增加独立的现金采购、服务期间成本、配额事件与基础设施账本。成本页不直接修改网关售价。

当前没有生产管理员凭据、账户采购凭证、已运行服务器的账单或配额历史，所以不能给出你现网的总成本或账户真实后验。公开可读 Radar 标注“8月9日19:49更新”，提供 20x Pro=1649.72、5x Pro=412.43、Plus=82.49 美元 API 等值周额度；后两项由站点推算。它没有在本轮可读取内容中给出 GPT-6 独立周额度、完整七日均值或 2026-07-16 至 2026-09-16 的事件明细。32 是其全部历史记录数，不能除以两个月当频率。

七天额度是一个窗口的容量，不是过去七天估计值的移动平均。不同模型的 API 等值估价也不是多个互相独立的预算池。站点页中的 Sol 参考价与新模型目录价不同，旧美元等值不能直接按新价格换成 token。

官方参考月费为 Pro 200/100 美元和 Plus 20 美元，不等于用户实际采购价。账户归属、支付币种、税费、汇率、有效服务期必须以凭据录入。订阅允许用途与 API 采购不是同一个合同；Plus 中转成本作为沙盘计算，不能据此认定订阅转售获得授权。

## 2. 已核对的代码与复用点

| 现有文件/接口 | 已有能力 | 成本页需补的内容 |
|---|---|---|
| `backend/internal/service/usage_log.go` | AccountID/GroupID、token 分类、TotalCost/ActualCost、AccountRateMultiplier/AccountStatsCost、耗时、FirstTokenMs、可选上游请求ID | 采购现金来源、价格版本、所有有成本的尝试与服务期间分摊；不能仅凭字段名认定 ActualCost 是现金采购支出 |
| `backend/internal/service/upstream_billing_probe.go` | 上游倍率、用户分组、模型费率探测 | 供应商→账户→采购批次；站内额度换现金、优惠到期与核销；不复写探测器 |
| `backend/internal/server/routes/admin.go` | 管理员鉴权/审计；accounts upstream-billing/rates、openai/:id/quota、dashboard、ops | 管理员专用成本查询/录入接口，与销售账、用量修复分离 |
| `backend/internal/service/openai_quota_service.go` | primary/secondary window、limit_window_seconds、used_percent、reset_at、additional limits、可用卡及到期信息 | 历史快照、预算池ID、容量版本；null 表示无数据；窗口不硬编码；不保存邮箱或原始凭据 |
| `backend/internal/service/openai_quota_auto_reset.go` | 重置前查询、队列/扫描补偿、幂等兑换、重置后刷新、审计、不可逆卡指纹 | 在既有流中发出净增量事件；研究工具不发兑换请求；幂等短期状态不是两个月成本历史 |
| `docs/COMMERCE_RELEASE.md`、`docs/MEMBERSHIP_FULFILLMENT.md` | 会员商品、订单、权益履约 | 已出售套餐与已采购套餐分开，不能拿会员支付总额当上游成本 |
| `frontend/src/router/index.ts` | Vue 管理员页面路由 | 拟增加 CostCenterView、成本 API 模块、导航及中英文文案；本次未改线上页面 |

现有配额服务明确出现父账户与 shadow 显示缓存的关系，因此建账维度必须有 `quota_pool_id`，不能每个账户行都算一份完整容量。请求模型、实际上游模型和共享额度池分别建模。

## 3. 数据模型与会计口径

建议新增逻辑表（最终 Ent 名称随仓库规范落地），不复用订单表冒充成本表：

| 表 | 核心内容 |
|---|---|
| cost_suppliers / cost_assets | 站点、账号、主机、线路和共享预算池的关联；不复制 API Key |
| cost_purchase_lots | 采购币种/实付/手续费/税/汇率版本，到账额度，购买时间，有效区间，凭据、退款和贷项 |
| cost_accrual_entries | 服务期间费用、成本中心、账户分摊规则、来源与反向更正记录 |
| server_tariff_versions | 套餐月费/开服费、流量单位、出站或双向、包含量、阶梯、账期/时区、共享流量池、95计费与带宽策略 |
| traffic_observations | host/interface/source、字节计数、起止时间、boot/db epoch、采样缺口；不存载荷 |
| quota_snapshots | pool/account/plan、窗口类型/长度、使用比例/重置时点、观察时点与来源可靠性 |
| quota_events | 公告、直接生效、赠卡、用卡、生效、到期分别记录；前后快照、事件指纹、是否确认及净增区间 |
| external_observations | 原始来源哈希、采集时间与来源更新时间、适配器版本、候选/确认/过期状态 |
| cost_estimates | 模型版本、输入证据、参数、P10/P50/P90、有效期、假设与回测误差 |

钱：现金账记录支付，权责账按有效服务期间确认，消耗账记录实际交付。预充值未消费余额不是立即全部成本；固定订阅费用在服务期间确认；赠送额度不会凭空产生现金收入。汇总必须标注“已付款 / 本期费用 / 预付余额 / 估计”而不是一个含糊成本数。

上游 API 采购批次换算：`现金单价 = 该批次实际现金 / 可消费额度`。按 FIFO 或统一选定加权平均核销，并单独处理到期、退款和赠送；维持一致会计政策。不能把新充值优惠倒灌改写全部历史使用成本。

请求采购成本为所有收费尝试之和。销售扣款、参考目录估值和现金采购成本独立存储；失败重试、缓存创建/命中、reasoning 分类避免重复计量。失败日志是否具备足够 usage 必须核对，缺少则列入待对账，而不是估成零。可用 UpstreamRequestID 辅助对账，但其为可选字段，不能宣称每次都可精确关联。

## 4. 流量方案：vnStat 优先

| 项目 | 适用角色 | 本方案取舍 |
|---|---|---|
| vergoh/vnstat | 宿主机内核网卡累计量，持久化日/月数据，JSON 输出字节 | 首选；无需抓包。配置账期并保留足够的细粒度历史 |
| prometheus/node_exporter | 主机性能和 RX/TX 计数，已有 Prometheus 时可复用 | 负责曲线/预警；increase 会外推，不能等同精确发票 |
| netdata/netdata | 统一多维运维看板 | 已装可复用；不是为一个成本页必须引入的新平台 |
| opencost/opencost | Kubernetes 工作负载成本分摊 | 多租户 K8s 时有价值，单 VPS 不优先 |
| ntop/ntopng | 更细的网络流可视化/归属分析 | 流归属确有需求时再评估，不默认抓包收集业务内容 |

`C_network = tariff(billable_bytes, billing_cycle, region, direction, tariff_version)`。

简单套餐：`C_network = max(0, charged_GB - included_GB) × overage_rate`。套餐包含流量只能在同一账期扣一次；月费已经包流量时不再按每 GB 重复加价；GB 与 GiB 分别为 10^9、2^30 bytes。阶梯须逐级积分而非总量乘最高档价。带宽包或95计费另建模型，不能直接套体积公式。

只采被云厂商计费的物理出口，不叠加 eth0/docker0/veth。香港与新加坡各算各自账单：同一业务跨两台机器产生两份可收费出口是两笔真实资源消耗，不应误删；在同一台机器的虚拟网卡重复计数则应剔除。

流量仪表显示：本账期使用、已包含剩余、月底P50/P90预测、预计超额费、实际账单、对账差额。采样缺口/计数器复位/接口迁移时保留状态。将主机日志、系统更新、探测和其他应用流量留在“平台开销/未分配”，不要把全网卡流量强行归到 AI 请求。

固定开服费区分现金流与摊销；月租按真实账期/有效服务小时分摊。人工费用使用投入人天×全成本日薪；开发投入、值班固定费和随业务量变化的客服费用分开。各账户分摊权重之和等于1，闲置成本也必须有归属。

## 5. 配额：观察不是后验，净增不是发卡次数

事件顺序：`看见公告 → 留存自身余量快照 → 账户实际收到卡/直接重置 → 卡被实际使用 → 观察实际增量`。发卡、用卡和直接重置是不同随机过程；一张卡到期未用不产生可交付量。

同预算池和同容量定义下：

`delta_e = max(0, remaining_after - remaining_before + consumption_between)`。

若刚好在重置前余量80%，重置到100%，且期间无消费，净增是20%，不是100%。观察间发生消费时必须补上消耗，否则净增被低估。若前后容量B改变，使用绝对量而不是直接减比例。迟收到公告且缺失前快照时，只给区间/未知，不伪造事前余量。公告时间、站点发现时间、账户实际生效时间分别记。

只有在“本周期开始时预算满额、各额外重置同池、容量B稳定”的情景下，可写 `Q_week = B × (1 + Σdelta_e)`。实际有效交付还受需求、账号停用、其他窗口和失败消耗限制。自然周重置与额外重置不能同时重复加；跨账期须用事件重放，不能每月一律当4周。

### 5.1 基础周容量校准

选择没有重置、价格版本变化、计划变更或未知其他用途消耗的观察窗。`B_i = V_i / delta_used_fraction_i`，V是该窗互斥 token 分类按**冻结参考价**估出的用量价值。百分比刻度粗或delta接近0时给更大测量误差，不用极小分母硬除。

对 log(B) 可用正态先验与观测似然做收缩，自己的高质量样本权重大于公共数据。若不同模型的额度消耗率不一致，采用 `delta_used = Σ(theta_model,category × token_count) + noise`，用非负回归/分层贝叶斯校准，各模型共用预算约束。不能只因为 GPT-6 目录更贵就认为订阅内容量美元等值按相同比例增加。没有专门实测，GPT-6 每百万 token 成本暂不输出。

Pro 自用尤其要注意，网关看不到直接在 Codex/ChatGPT 的全部消耗；此时不能用“网关内token/全部额度下降”当完整观测。导入本人授权的本地 usage 或明确标注分母范围。

### 5.2 额外重置频率与增益

两个月定义为 `[2026-07-16, 2026-09-16)`，共62天，不是固定8周。公开全站事件只算一次，不因观察到10个账号而变成10次独立官方事件。区分数据覆盖与事件数；缺测不能填零。

对单类、近似平稳事件，Gamma-Poisson：`lambda ~ Gamma(a0,b0)`（b为周数），自身完整观察W周、N次后，`lambda | data ~ Gamma(a0+N,b0+W)`。弱外部先验仅在历史覆盖明确且同套餐同政策时构造；原始32次历史不满足条件。政策改变、批次集中派卡或明显过度离散时，分段/负二项/按周区块自助法更合理。

适用资格或公告是否兑现这样的二元事件可用 Beta-Binomial。连续的剩余额度比例不是二项试验，不把“还剩80%”伪装成80次成功。本原型对逐事件净增采用 Bayesian bootstrap；更完整实现可用零/一膨胀Beta和层级计划差异。

按后验抽样容量、事件数、事件净增和利用率，再逐样本计算 `unit_cost = total_period_cost / useful_delivered_volume`，输出成本P50/P90。不能用平均成本除平均容量冒充平均单位成本。0有效交付时成本不可定义/无法回收，不输出0元。

原型只对事件数/净增抽样，B和利用率为条件输入；因此其区间不涵盖容量、质量和需求全部风险。不要标成完整真实后验。

## 6. 自用与中转成本公式

`C_pro_self = actual_pro_subscription_cost_allocated_to_period + allocated_self_use_resources`。

`C_plus_relay = actual_plus_subscription_period_cost + allocated_server + allocated_network + operations + other_metered_upstream_cost_not_already_included`。

两者的实际单位成本均为 `C / 同期实际有用交付量`，分别报告现金采购底价和含运营全成本。共享账户服务多个模型时，按校准后的额度消耗份额分摊，不按请求个数均摊，也不每个模型各分摊100%月费。

仅作历史快照敏感性演算：令30天、无额外重置、无其他费用、全部同口径。Pro200容量1649.72/周，Plus容量82.49/周。公式：

`k = monthly_price / ((30/7) × B × utilization × (1 + expected_extra_resets_per_week × mean_net_gain))`。

| 情景（非你的真实成本） | Pro200现金 / $1冻结参考量 | Plus现金 / $1冻结参考量 |
|---|---:|---:|
| 无额外重置、利用率100% | 0.0283 | 0.0566 |
| 无额外重置、利用率50% | 0.0566 | 0.1131 |
| 无额外重置、利用率20% | 0.1414 | 0.2829 |

这里的“$1参考量”不是一美元可退款余额，更不是每百万token。Plus基础容量是站方按档位推算的旧值，因此这个表只用于检查公式和看利用率影响，不可据此发布当前商业售价。实际支付若以人民币记账，将分子替换为实付人民币及相关费用即可，不应强用一个当日汇率重算历史。

只有模型、互斥计费分类和冻结价已对齐，才能在明确分摊策略后展示输入/输出/缓存每百万token成本；优先直接显示一类标准工作负载的实测成本，避免从一个混合总成本臆造三套单价。

## 7. 竞品监控

官方目录参考 GPT-6=10/1/50、Sol=4/0.4/20 美元/百万输入/缓存/输出token。OpenRouter 索引返回 Sol 促销价2/0.2/10及Flex1/0.1/5，公开延迟P50分别4.98s、9.47s，吞吐42/56tps；但是该模型页面的直接可读快照却显示5/30。因此本次存为**冲突待验证的候选记录**，不是可执行采购报价。两条Provider行也不是两个独立最终供应商。

每条记录保存模型版本、服务tier、reasoning、上下文段、缓存口径、普通/促销价、到期时间、金额单位、分组/用户条件、来源时间与获取时间。OpenRouter 的公开模型 Endpoints API 返回费率、近期延迟/吞吐/可用性字段，是比解析视觉价格更稳定的采集接口；保留原始单位，不把文档示例数据当真实测量。

自己的探测应单独存地区、网络路径、样本数、TTFT、E2E、TPS、成功率和测试负载。公开延迟不能与自己的不同任务/地区样本直接排名；HTTP价目页响应速度不是推理TTFT。本次没有发起付费推理测试。

## 8. 管理者界面与输入流程

概览六项：本期应计成本、现金已付与预付余额、流量预算/超额预测、有效容量P50/P10、实际/预测单位成本、待对账与数据缺口。每个数都显示“凭据 / 观测 / 预测 / 假设”和最近时间。

二级页签：上游与采购、服务器与套餐、额度事件、竞品、对账。新增套餐表单支持供应商/计划/现金与币种/服务期/包含流量或额度/计费方向/限速策略/账期/是否自动续费等；保存套餐不执行购买或续费。编辑只新增费率版本，不改变过去账。

账户表显示所属预算池、实付与分摊月费、5h/7d等实际存在窗口、卡数/到期、来源日期、有效周容量区间、使用率和单位成本。缺失就显示待接入，不画零成本或100%完整率。

拟定路由：`GET /api/v1/admin/cost-center/summary`、`/upstreams`、`/traffic`、`/quota-estimates`、`/market`；采购与费率写入用独立管理员权限、校验和审计，乐观锁阻止覆盖；后验重新估计不更改账本。本次这些接口**尚未实现**。

## 9. 验收与推进

第一步接入现金采购与服务期间账、vnStat及静态成本汇总；第二步在既有重置前后发出追加事件，补只读历史；第三步积累并对账后再上线后验预测；第四步接入经过验证的动态价格。

必须测试：父子账号共享池去重、赠卡未用不增容量、重置前80%只净增20%、读数之间消费修正、自然重置重复识别、未知窗口不默认无限、价格版本切换、月流量仅扣一次、网卡复位、过期余额、重试收费、没有usage的失败、字段缺失与采集403/429。

研究脚本12项离线单测通过；联网采集、真实账号/主机验证、现有Go/Vue全量构建未运行。不创建或修改生产定价，不发布公开 Pages，不声称通过 Study_Skills v2 全套几何/发布门禁。

## 10. 图形与来源

图形借鉴 Study_Skills learning-figure 2.2 的问题/读图顺序/结论/边界合同，使用 `figure-models.json` 作为本设计的单一语义来源，生成静态SVG和HTML。不是教程，不改Study_Skills。图在关闭JS时仍可见；交互模拟只作附加功能。

主要一手来源：

- https://github.com/ValentinoWang/sub2api/tree/dev
- https://github.com/ValentinoWang/Study_Skills/tree/main/skills/learning-figure
- https://codexradar.com/en/
- https://deng.codexradar.com/en/
- https://github.com/codex-radar/dradar （客户端，不含服务端完整历史）
- https://help.openai.com/en/articles/9793128-what-is-chatgpt-pro
- https://help.openai.com/en/articles/6950777-what-is-chatgpt-plus
- https://developers.openai.com/api/docs/models/compare
- https://openrouter.ai/openai/gpt-5.6-sol-20260709
- https://openrouter.ai/docs/api/api-reference/endpoints/list-all-endpoints-for-a-model
- https://github.com/vergoh/vnstat
- https://humdi.net/vnstat/man/2.11/vnstat.html
- https://github.com/prometheus/node_exporter
- https://prometheus.io/docs/prometheus/3.12/querying/functions/
- https://github.com/netdata/netdata
- https://github.com/opencost/opencost
- https://opencost.io/docs/configuration/on-prem/
- https://github.com/ntop/ntopng

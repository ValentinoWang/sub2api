# 本地原生成本工作台

本地页面入口为 `http://127.0.0.1:4174/admin/cost-center/accounting`。参考目录和三档比较位于 `comparison` 子页。页面采用 Go 原生计算和独立 PostgreSQL 成本库；缺失容量不以默认值替代。

## 成本口径

- 期间订阅、服务器和人工费用按原凭据服务区间确认。
- 预充值以 `credit_lot` 记录实际现金和额度，不作为订阅期间费用；`credit_use` 按付款时间及事件顺序 FIFO 核销未过期批次。赠送批次允许现金为 0。过期未用现金价值记为未分配损失。精确有理数汇总后才舍入显示。
- 倒填充值批次或消耗会影响已有 FIFO 结果时拒绝；更正先处理依赖消耗，再处理来源批次。所有更正追加记录，原记录保留。
- `account_interval` 记录一个真实 OpenAI 来源账号在明确起止期间的档位；同账号或资产期间不能重叠。管理员明确选择账号和凭据，不根据名称猜测档位，不修改上游账号套餐。
- 用量对账读取网关 `usage_logs`。它不覆盖直接在上游使用的消耗。金额是网关保存的参考计费值，不是采购发票。交付单位为 `request` 且完整覆盖同一账号、档位、模型与查询期间时才报告请求数匹配或差额；无记录、跨期或覆盖不完整时不伪造差额。
- 窗口回放必须提供同一池、模型和价格版本下的完整初态、窗口和事件。每次交付同时受全部窗口限制；自然窗口到期恢复与额外补充不会重复计算。缺少双窗口或覆盖不全时总量为 null。
- 贝叶斯预测为 Gamma-Poisson 事件后验加净增的 Bayesian bootstrap；容量、利用率和费用仍是条件输入。无完整自身曝光期或逐次净增不运行预测。它不是采购发票、实测容量或完整成本底价。

## 本地运行

日常 8080 自用后端继续提供会话和只读来源。独立开发成本服务复用本地管理员会话，只有成本路径通过 Vite 的 `VITE_COST_LOCAL_PROXY` 指向该服务；默认其他代理仍为 8080。该独立进程是本地开发入口，不替代完整服务的生产认证、审计、限流与合规中间件。

完整 Go 服务已接入相同计算与存储代码，通过 `SUB2API_COST_LEDGER_DSN` 显式配置成本库，通过 `SUB2API_COST_SOURCE_DSN` 显式配置只读来源。留空不会退回自用数据库自动迁移。

Mac 本地开发构建：

```bash
cd backend
GOMAXPROCS=2 GOFLAGS=-p=1 GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
  python3 ../tools/quality/run_with_space_guard.py -- \
  go build -trimpath -o "$HOME/.local/share/sub2api-cost-local/cost-local" ./cmd/cost-local
cd ..
python3 tools/cost_research/start_local_native.py
```

`start_local_native.py` 使用已有 Docker 网络与 `sub2api` 的来源配置，创建独立的 `sub2api-cost-local-postgres` 和本地开发服务；凭据与数据仅保存在仓库外的私有运行目录。来源 DSN 默认只读，查询另使用只读事务，不读取账号凭据列。成本运行角色只拥有必要的读取、追加及锁操作权限。构建二进制以只读挂载运行，不从未提交源码构建镜像。

初始化等待正式 TCP 服务就绪后执行成本迁移。初始化时的临时 Unix socket 不代表 PostgreSQL 已就绪。运行目录已有数据库时原数据保留。

Go 命令的空间门禁默认在余量不足 4 GiB 时拒绝启动或停止本轮进程组，为 3 GiB 底线留出余量。可传 `--evidence` 保存实际最低余量。不要把 DSN 或凭据放在该门禁的命令参数中。

## 额度观察

完整服务在正常额度查询和成功缓存重置后快照处调用独立观察存储。写观察失败不会改变上游重置结果，日志明确记录缺口；不能把写失败当作已经收到增益。

本地开发入口每分钟只读已有账号缓存并去重存储，保留原观测时点，不重新探测供应商、不刷新上游令牌、不触发用卡。窗口时长为 0 时保留原值并标记 `WINDOW_DURATION_UNKNOWN`，不能作为已校准窗口送入回放。`source/quota` 固定声明观察覆盖不完整，不推断净增。

```text
GET  /api/v1/admin/cost-center/source/accounts
GET  /api/v1/admin/cost-center/source/reconciliation?start=...&end=...&model=...
GET  /api/v1/admin/cost-center/source/quota?account_id=...&start=...&end=...
POST /api/v1/admin/cost-center/analysis/traffic
POST /api/v1/admin/cost-center/analysis/replay
POST /api/v1/admin/cost-center/analysis/forecast
```

## 采样文件

在成本核算页选择“导入采样与推演资料”，选择类型和 JSON 文件后计算；核对结果后可另存资料到成本账本。以下文件仅为合成格式示例，不能用作实际经营输入：

- [流量格式](../../tools/cost_research/examples/traffic-synthetic.json)
- [窗口回放格式](../../tools/cost_research/examples/replay-synthetic.json)
- [条件预测格式](../../tools/cost_research/examples/forecast-synthetic.json)

流量使用两次 vnStat JSON v2 累计快照；明确网卡、快照更新时间、账期已包含量、出站或双向和 GB 定义。日、月、总量不叠加。计数器复位或覆盖不全时费用为 null。该计算是显式费率估算，不自动登记采购发票。资料计算入口限 256 KiB，保存的单条账本命令保持 16 KiB。

## 验证与边界

PostgreSQL 测试已纳入 `integration` 标签，可创建并清理专用临时容器；显式提供测试 DSN 时仍必须声明隔离 opt-in。Linux 与 macOS 都执行文件账本回归。

```bash
cd backend
go test -race -p 1 -tags=integration ./internal/costing
```

本轮机器证据、编译空间记录和界面读取观察位于 `agents-results/2026-09-16/cost-native-completion/acceptance/`。完整项目 CI 保持原入口和提交快照预检；当前工作区含其他任务未提交改动，预检失败不被当作通过。人工清单仍为待验收，生产发布未执行。

## Codex Radar 与填写说明

三档情景比较页提供“Codex Radar 公开参考”，由管理员点击后读取固定的公开地址。Go 解析当前 quota 卡片和站方速度表，使用十分钟请求缓存，并将来源哈希、读取时间和规范化字段追加到成本库。失败时显示不可达，不回填旧快照。它是按需公开读取，不是后台定时爬虫。

2026-09-16 读取的页面列出 Pro 20x 分别仅用 Astra、Sol、Luna 的美元等值参考量；这些情景不能相加。当前卡片没有明确周期和换算价格版本，因此二者保持 null，不命名为自身周容量。Plus、Pro 5x 同口径量、七天逐日均值、完整两个月重置序列没有足够字段时保持未知。速度表保留站方模型和思考强度，不能当作本机链路实测。

本次 `robots.txt` 返回网页 HTML，未将该返回认定为有效爬虫策略；未启用自动站点爬取或后台 Radar 定时采集。

各输入下方提供简要定义与例子；成本工作台底部提供可搜索的“填写说明与术语解释”，说明数据应从何处获取、没有资料时如何处理。例子均为说明用数，不写入实际账本或自动代替真实数据。

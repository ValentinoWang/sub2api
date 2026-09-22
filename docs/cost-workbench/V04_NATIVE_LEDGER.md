# 原生成本账本 v0.4

成本研究模块版本，不修改 Sub2API 产品发布版本。源分支基线为 `0e83332ad7aaff2bc6866d68168652a5f8f14832`。

## 已实现的纵向切片

Vue 成本账本组件 → 已注册管理员 API → Go 校验/核算 → PostgreSQL 适配器。相同 Go 服务另有 Linux 文件存储适配器和独立 HTTP 验收进程。

原生入口 `/admin/cost-center` 在 `frontend/src/main.ts` 中先于 `app.use(router)` 注册，使用现有全局管理员路由守卫。后端在 `server.ProvideRouter` 完成原有 `SetupRouter` 后注册独立成本路由组，复用管理员认证、面板限流、审计和合规中间件。每次请求再读取已验证的 AuthSubject 与管理员角色，记账 actor 不接受前端传值。未增加侧栏入口，当前从明确地址进入。

这条纵向切片已完成源码接线；当前环境尚未运行完整 Sub2API 或编译 Vue，因此不能把代码注册等同于整站运行验收。

## 存储与安全启用

`SUB2API_COST_LEDGER_DSN` 显式指定成本 PostgreSQL 连接。默认留空时，采购账本 API 返回 503；原生三档计算器仍可使用。不会隐式回退到现有自用库，也不会自动建表、迁移、充值或重置账号。

先在隔离的 PostgreSQL 环境中评审并执行：

```bash
# PGHOST/PGPORT/PGDATABASE/PGUSER/PGPASSWORD 由受控环境注入，不把密码写入命令行。
psql -v ON_ERROR_STOP=1 -f backend/internal/costing/migrations/001_cost_center.sql
```

迁移只创建成本命名的两张表和追加式保护触发器，不修改账户、使用记录或销售计费表。运行角色需要成本事件表 SELECT/INSERT 及锁表 SELECT/UPDATE；不能拥有这些表或持有 ALTER/TRUNCATE/DELETE 权限。表所有者可以绕过触发器，这不是防数据库管理员篡改的承诺。

SQL 适配器在每次低频管理员操作中打开一个最大连接数为 1 的连接池，并在结束时关闭；没有新增无法回收的全局池。每次操作最长 5 秒。高频部署应通过现有依赖注入改用生命周期受管的池，`SQLLedgerStore.Open` 可直接返回主程序管理的池和空释放函数；当前没有自动借用它。

所有写入先对 `cost_center_lock` 行执行 `SELECT ... FOR UPDATE`，再在同一 READ COMMITTED 事务内读事件、检查业务约束、追加新事件并提交。响应丢失后的同键重试返回原结果，不能再次计费。SQL 的事务与锁语义参考 PostgreSQL 官方说明和 Go database/sql 文档；本轮 SQL 驱动替身测试不证明实际 PostgreSQL 隔离、语法或权限配置已经验收。

## 当前事件合同

| kind | 用途 | 约束 |
|---|---|---|
| purchase | 已付款订阅、服务器、流量、人工、其他期间费用 | 实付金额为十进制字符串；币种、付款时点、服务起止、引用与证据必须显式提供 |
| delivery | 已完成的指定模型/单位/账号的区间聚合量 | 不允许未来交付；同账号、工作负载、单位的区间不能重叠；跨查询边界不自动分摊 |
| allocate | 将未分配费用归属三档 | 权重正数且精确合计为1；一笔来源只能有一个有效分配；不是新增费用 |
| void | 更正误录 | 目标、预期内容哈希与原因必填；追加记录，不删除或退款；先作废依赖分配，再作废来源采购 |

三档分别为 `plus`、`pro5x`、`pro20x`；公共费用归入 `unallocated`。每笔订阅要求明确档位。当前原生资产标识为管理员显式填写的非敏感业务引用，不会自动认定它对应现有真实账号；生产账号归属验证仍待接入。

全局幂等键与提交者绑定。相同键、相同动作、相同管理员重试返回原事件；同键不同内容或管理员返回409。另有供应商＋凭据业务编号去重。原费用只读，历史知识截止使用 `as_of` 筛选当时已经记录的事件。

金额使用 `big.Rat` 精确有理数；API金额字符串输出保留6位。分摊在精确值上进行；展示舍入可能产生百万分之一币种的差异，展示值不能反向作为源账。

## 报表口径

- `cash_paid`：查询期间实际付款，且付款时间不晚于知识截止。
- `period_expense`：现有采购记录按整个查询服务区间计算的期间费用；在月中查询整月时包含未来服务，不应叫已发生费用。
- `recognized_expense`：仅摊到查询终点与知识截止两者较早的时点。
- `remaining_service_value`：已付款但在上述时点之后尚未经过的服务价值，不是可退款余额。
- `recorded_scope_unit_cost`：已记录费用/同范围已完成交付；缺乏费用或交付、存在未分配费用、跨币种或工作负载分母不完整等情况返回 null，而不是0元。

报告固定标记 RECORDED_SCOPE_ONLY 或 IN_PROGRESS_PERIOD。每个成本数字仍不代表已录全全部供应商、资源与人工费用，也不代表已完成模型成本归因。

月中示例：订阅30，9月1日至10月1日服务；9月16日0时查询。整期费用30，已发生15，剩余服务价值15。这是合成日期算例，不是你的账单。

## 接口

```text
GET  /api/v1/admin/cost-center/catalog
POST /api/v1/admin/cost-center/compare
GET  /api/v1/admin/cost-center/ledger/health
GET  /api/v1/admin/cost-center/ledger/events?after=0&limit=50
GET  /api/v1/admin/cost-center/ledger/summary
POST /api/v1/admin/cost-center/ledger/commands
```

记账 POST 使用 `Idempotency-Key` 请求头。最大正文16KiB，拒绝多JSON、未知字段、客户端 actor 字段和未支持币种。分页最多200条。DB错误不向前端泄露DSN或原始数据库错误。前端同一提交内容在失败重试时复用幂等键。

第一版原生事件流上限10,000条、文件验收适配器上限32MiB。它用于低频采购和区间交付聚合，不用于逐请求token日志或高频网卡/配额采样。达到上限时明确拒绝新写，不能静默丢历史。

## 本地原生 API 验收

Linux/WSL、Python 3.11+、Go 1.23+ 可运行独立标准库切片：

```bash
python tools/cost_research/run_native_workbench.py \
  --ledger "$HOME/.local/share/sub2api-native-cost/ledger.json"
```

启动输出动态 loopback URL 和本次临时口令；口令只用于本地验收，不存数据库，不是生产管理员认证替代品。运行目录须在源码之外，权限0700；文件0600，拒绝符号链接目标，跨进程 flock，原子替换并 fsync。不要对公网开放。

该入口提供原生 API，不包含 Vue 构建产物。真实 Vue 页面需要在完整仓库使用项目既定前端开发环境访问 `/admin/cost-center`。

```bash
python tools/cost_research/check_native_ledger_e2e.py
python tools/cost_research/check_native_cost_wiring.py
python -m unittest discover -s tools/cost_research -p 'test_*.py' -v
```

独立验收不会修改项目的 `backend/go.mod`。完整仓库声明 Go 1.27.0；本环境只有Go1.23.2，独立包通过不等于全应用通过。

PostgreSQL集成测试明确启用，并必须指向隔离数据库：

```bash
# COST_LEDGER_TEST_DSN、COST_LEDGER_TEST_ALLOW_ISOLATED_SCHEMA=yes 安全注入。
cd backend
go test -tags costpg ./internal/costing -run TestPostgresLedgerIntegration -count=1
```

测试创建独立随机 schema，结束只删除本次创建的schema。缺配置时测试失败，不用skip伪造通过。

## 本轮证据

- 对话完整源码包中的 Python 既有与迁移后回归：147项通过。这包括此前未推送的完整 Python v0.3 工作台；GitHub 原生分支不包含该 Python 全量工作台，不应在原生分支期待相同测试数量。
- Go原生包：race测试通过，含SQL驱动替身事务合同；详细计数见本轮summary。
- 真实Go进程＋HTTP＋文件存储：24项通过，包括32次并发同键重试、分页、费用分摊、重启、作废依赖及历史回看。
- 静态注册检查：通过；它不是编译或鉴权攻击测试。
- PostgreSQL真实执行、完整Vue编译与浏览器、项目完整本地CI：NOT_RUN。未宣称人工PASS、生产部署或真实供应商可用。

机器证据：`agents-results/2026-09-16/cost-native-ledger/acceptance/`。GitHub 保留摘要及真实HTTP验收结果；逐条Go/Python测试日志同时保存在对话源码包。

## 尚未迁移的范围

Python v0.3 中的 FIFO 预充值核销、账号升级生效区间、vnStat采样、双限额窗口回放和贝叶斯预测没有被伪装成已接入原生数据库。额度事件自动挂钩、现有使用记录对账、退款/税务处理、自动关闭账期、供应商自动采集、Vue侧栏入口和自动定价执行也未完成。

下一阶段应先在隔离PostgreSQL运行本轮集成测试和完整项目本地CI，再接入真实账号/用量外键及只读成本观察。不要把研究目录的旧价格快照作为可执行采购报价。

一手实现参考：
- https://go.dev/doc/database/execute-transactions
- https://www.postgresql.org/docs/17/explicit-locking.html
- https://www.postgresql.org/docs/17/transaction-iso.html

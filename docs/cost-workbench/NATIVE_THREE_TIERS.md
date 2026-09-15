# 三档成本比较：Go / Vue 原生接入候选

研究日期：2026-09-16。基线为研究提交 `a893f6186d322761da032e7360323d065321aafd`；本目录的 v0.3 是成本研究原型版本，不改变 Sub2API 产品发布版本。

## 这条分支实际包含什么

原生纯计算包 `backend/internal/costing/`、HTTP 处理层、管理员 Gin 路由注册函数、Vue 页面、API 类型及客户端、受精确源码校验的两处接线脚本。本分支不是完整成本账本上线：PostgreSQL 采购、流量、额度事件表和生产采集尚未接入。

现有两处中央路由保持未修改；需要在完整检出中执行下面的脚本，才会注册新接口和页面。侧栏入口未添加，接线后直接访问 `/admin/cost-center`。完整 Python/SQLite v0.3 工作台包含在本次对话源码包，不在这条原生子集分支中；此前 v0.2 没有成功推送，不通过上传依赖不全的 Python 文件伪装为完整交付。

## 同一合同，三个独立 SKU

`plus`、`pro5x`、`pro20x` 必须恰好各出现一次。拒绝含糊的 `pro`。实际采购金额、期内容量、有用比例与工作负载可用性由各档独立输入。官方20/100/200美元月费仅作参考；宣传倍率不自动转为某个模型的真实容量。

`demand` 表示合格交付目标，`period_capacity` 是指定账期的原始容量，不是周额度。计算：

```
有效容量 = 原始容量 × 有用比例
交付量 = min(合格需求, 有效容量)
缺口 = 合格需求 - 交付量
单位成本 = 本期费用 / 交付量
```

只有能满足全部合格需求、明确确认该工作负载可用、数据齐全且采购条件允许的档位，才进入最低总费用比较。并列返回多个档位。交付为零时单位成本为 null；未知容量不变成0。金额使用十进制字符串和 Go big.Rat，不经过浮点现金计算。

`existing` 是现有账号成本情景；`new_purchase` 是新购情景，不等于购买授权。目录记录2026-09-10起Pro20x暂停新购/升级，但现有续订不受影响。目录附核查时间和有效截止，新购资料过期后拒绝给出可买结论。该有效期是本工具主动采用的一天复核策略，不是供应商承诺。目录不是可执行报价，修改目录必须重新核验来源。

## 受校验接线

在**完整仓库检出**、没有其他编辑者同时写相关文件时，从根目录执行：

```bash
python tools/cost_research/apply_native_wiring.py --check
python tools/cost_research/apply_native_wiring.py --apply
git diff --check
git diff -- backend/internal/server/routes/admin.go frontend/src/router/index.ts
```

默认只检查；仅显式 `--apply` 写本地源码。脚本校验：

- `admin.go` 原始 blob `a18d069b4f5b7b3742ba294d121a35f6abe0e826`；
- `frontend/src/router/index.ts` 原始 blob `53ea004b5ca6de1368c0d792034e851c6edda869`。

同一精确改动重复应用是无操作；源码漂移或文件缺失立即拒绝，不提供强制覆盖。先预检两文件后才写入；此脚本不是跨文件原子事务，运行时不要并发编辑，写入失败须检查Git diff。不会改变数据库、上游账号、套餐、支付或生产价格。

脚本将在现有管理员认证、审计及合规中间件后挂载：

```
GET  /api/v1/admin/cost-center/catalog
POST /api/v1/admin/cost-center/compare
```

返回现有 `{code,message,data}` 风格。HTTP主体上限64KiB；拒绝未知字段、多JSON对象、错误内容类型。独立HTTPHandler没有授权函数时默认拒绝；Gin封装依赖传入的是现有已认证管理员组，不可移到公开组。

## 测试和未运行项

纯Go包在Go1.23.2环境以 `GOTOOLCHAIN=local GO111MODULE=off go test -race -v` 通过；包括三档、缺测、精确小数、同价、采购时效、HTTP拒绝条件及4个共享golden样例。完整检出可在backend目录执行 `go test ./internal/costing`。

```bash
python -m unittest discover -s tools/cost_research -p 'test_native_wiring.py' -v
cd backend
go test ./internal/costing
```

Go1.23.2独立包通过不代表整个仓库的工具链或依赖已构建。Vue已编写但未运行完整类型检查、Vite或生产构建。两处中央路由的完整文件未在当前部分检出中应用；接线脚本行为测试使用明确的合成源文件，不能当作全仓集成通过。没有执行根目录 `tools/quality/run_local_ci.sh`，也没有人工验收、生产发布或真实供应商验证。

4组共享golden数据全部是合成假设：费用20/100/200、期内容量100/500/2000，并非实测比例。需求50/300/900时，已有账号分别选Plus/Pro5x/Pro20x；900新购情景无可行档位。不得把这一结果发布为实际最低采购成本。

## 来源

- OpenAI Plus：https://help.openai.com/en/articles/6950777-what-is
- OpenAI Pro：https://help.openai.com/en/articles/9793128-what-is-chatgpt-pro/
- 仓库现有管理员路由、API client、响应封装和AppLayout均按dev读取核对。

下一步应在隔离数据库和完整检出中完成中央路由接线、本地CI和实际浏览器验收，再讨论采购账本的PostgreSQL迁移。计算器不能代替真实权限、数据覆盖或对账。

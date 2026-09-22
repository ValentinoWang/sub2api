# LDXP 浏览器登录授权补货适配

本地实现复核记录；不是生产发布或真实商户验收。

- HTTP 401/403 明确进入持久暂停：复用 `Enabled=false`，新增状态与只读状态响应字段 `session_verification_required`。状态重载和保存均保持暂停，重启不会恢复自动补货。
- 自动补货、商品列表、库存预览、库存对账、连接验证均识别此状态。只读请求失败后重新加载最新状态再写入暂停，避免覆盖并发批次；库存入口停止请求余下商品。
- 上传 401/403 同时保留结果不确定与原有对账锁。上传成功后的库存认证失败也暂停并保留待对账批次。HTTP 502、未知库存、未证实的业务 `code` 和错误文案不被猜成认证失效。
- 保存非空新授权先持久设置验证锁，再保存加密配置，不开启自动补货。存在待处理批次时，仅允许产品、策略和制码密钥不变的纯授权替换。
- 连接验证先读商品接口，再读取全部配置商品的未售库存总数；全部成功才清除授权验证锁。验证后仍关闭自动补货，原有对账锁与批次不会清除，需另行对账和手动启用。
- 开启、手动运行和恢复任务受验证锁限制。使用现有应用错误协议返回 HTTP 409 与 `LDXP_SESSION_VERIFICATION_REQUIRED`，并提供重新登录、保存授权、验证库存、手动启用的安全提示。

修改范围：`backend/internal/service/liandong_restock_service.go`、`liandong_restock_core.go`、`liandong_tool_domain.go`、`liandong_inventory.go`、新增 `liandong_session_test.go`，以及两个现有管理员 handler 测试文件。没有修改 handler 业务实现、数据库结构、生产服务或真实商户状态；没有读取真实授权。

验证记录：

- 首轮 focused：`go test ./internal/service -run 'TestLiandong(Session|RestockHTTP502|Toolkit)' -count=1`，PASS，服务测试时间 1.916s。
- 首次全部 LDXP 服务回归：`go test ./internal/service -run 'TestLiandong' -count=1`，PASS，1.899s。
- HTTP 合约：`go test ./internal/handler/admin -run 'TestLiandong(ToolkitHandlerMapsDomainErrors|AdminEnableReturnsSessionVerificationConflict)' -count=1`，PASS，1.080s。覆盖标准 409 原因与不泄漏包装错误细节。
- 增加库存入口、并发快照与恢复任务门禁后，回归发现恢复任务检查顺序影响原有“运行中立即拒绝”行为。保留原测试，调整为先执行原有 busy 判定再检查验证锁。中止失败进程的栈保留于本任务工具记录。
- 最终服务回归：`go test ./internal/service -run 'TestLiandong' -count=1 -timeout=60s`，PASS，3.940s。覆盖 401/403 库存/上传/上传后库存、重启暂停、未知业务码不误判、库存不明保持验证锁、验证后保持关闭及对账锁、并发批次保留、运行中恢复任务即时拒绝。
- `git diff --check`：PASS。

独立复核补充：库存和商品列表的请求不持有状态锁，旧授权请求可能在新授权保存、验证、手动启用后才返回 401/403。为请求附带进程内凭据版本，初始化载入或明确保存非空授权才递增版本（包括重新保存同一授权）；仅商品配置更新不递增。暂停写入在状态锁内确认当前版本，旧授权请求只返回原请求错误，不再覆盖新授权状态。版本不包含凭据、哈希或持久敏感值；重启后没有可继续回调的旧进程请求。

- Barrier 红例：`go test ./internal/service -run 'TestLiandongSessionLateAuthenticationFailure' -count=1 -timeout=30s`，4 个子例均按预期失败，复现库存/商品列表与新 token/重新保存同 token 的迟到认证拒绝覆盖问题。
- 修复后：`go test ./internal/service -run 'TestLiandong' -count=1 -timeout=60s`，PASS，2.146s，含上述 4 个 barrier 子例以及当前授权失败仍暂停、并发批次保留等回归。
- 二次边界复核增加空 `MerchantToken` 配置保存：两个 barrier 子例先复现“当前授权迟到失败被误忽略”；修正版本递增条件后，全部 `TestLiandong` 再次 PASS，1.441s。最终 barrier 矩阵为库存/商品列表 × 新授权/重存同授权/仅保存商品配置，共 6 个子例；前四者不能覆盖新状态，后两者必须暂停当前授权。

测试仅使用本地 `httptest` 与内存/原有 SQL mock，不以模拟认证失败或机器 PASS 宣称真实登录、库存对账或人工验收通过。未运行完整本地 CI，未构建或部署镜像。未证实任何 LDXP 业务认证状态码，因此只按明确 HTTP 401/403 判定。

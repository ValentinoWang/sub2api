# ranxi2001/sub2api v2.8.0 打票实现核对

核对对象为 [v2.8.0](https://github.com/ranxi2001/sub2api/releases/tag/v2.8.0)，发布于 2026-09-22，对应提交 `3719ff895d3d8565b750a40e01a76a57c864a758`。仅静态检查发布说明及票据、出站绑定、响应反馈、Cookie 测试代码，没有运行该分支或真实上游验证。原文及源码快照哈希保存在 acceptance/reference/manifest.json。

| 能力 | v2.8.0 的实现 | 对本次任务的意义 |
| --- | --- | --- |
| turn-state 寿命 | 默认 TTL 3600 秒；有签发时间时再用签发后 3570 秒封顶 | 没有实现 150／210／240 秒策略，不移植其一小时规则 |
| Cookie 新鲜期 | 独立 HarvestCookiesAt，240 秒后不再注入；合格响应才能更新 Cookie | 240 秒仅是 Cookie 窗口，不是 turn-state 的上游有效期证据 |
| 出口与会话绑定 | 保存采集代理、节点、会话；业务 HTTP/WS 复用；定向节点依赖外部 Mihomo Selector | 能避免仅替换票据却改变来源上下文，但会改变本项目现有业务出口行为，需独立验证 |
| 身份字段 | 改写 session_id，移除多种客户端身份头以及 client_metadata、prompt_cache_key、device_id | 涉及当前项目会话隔离和缓存行为，不能独立复制其中一个函数就声明连续性成立 |
| 响应续票与失效 | HTTP 200 返回合格新票后更新链；不合格响应撤销当前票或启用备用票；不直接重放已计费请求 | 有参考价值，需要覆盖并发响应、模型关联、持久化失败和 HTTP/WS 所有实际路径 |
| 票据形态 | 解析信封块数及时间，个人/团队目标通常为 292/332 | 长度不能单独证明实际模型或回答质量；本次保留当前 292 策略 |
| 采集管理 | 发布说明列出预算、冷却、节点学习、手动采集；新增迁移 239/240 | 是另一组数据与运维改造，不应为了调整寿命引入这些迁移 |

源码依据：

- [票据与 Cookie 时间](https://github.com/ranxi2001/sub2api/blob/3719ff895d3d8565b750a40e01a76a57c864a758/backend/internal/service/openai_codex_ticket.go#L37)，turn-state 默认寿命见同文件 L203，有效上限见 L352。
- [出站身份绑定](https://github.com/ranxi2001/sub2api/blob/3719ff895d3d8565b750a40e01a76a57c864a758/backend/internal/service/openai_codex_ticket_egress.go#L62)。
- [业务响应反馈](https://github.com/ranxi2001/sub2api/blob/3719ff895d3d8565b750a40e01a76a57c864a758/backend/internal/service/openai_codex_ticket_feedback.go#L83)。
- [Cookie 边界测试](https://github.com/ranxi2001/sub2api/blob/3719ff895d3d8565b750a40e01a76a57c864a758/backend/internal/service/openai_codex_ticket_cookie_test.go#L13)。

本次集成结论：实现用户明确指定的默认本地票龄策略，不引入 experiment/transparent 模式。维持当前采集来源及出口策略；新规则按首次本地 CapturedAt 而非已认证的上游签发时间计龄。已知同票重复捕获保持原年龄，旧记录的 ExpiresAt 被本地 240 秒上限收紧。该变化不宣称已经移植 Codex_degrade 的业务回收池或 v2.8.0 的绑定、Cookie、预算功能。

建议后续以“票据 + 账号/模型 + 出口 + 会话 + Cookie + 业务反馈”作为完整边界评估绑定方案，先解决与当前会话隔离和代理设置的冲突，再决定移植范围。本次不整分支合并，不运行参考仓库的安装脚本。

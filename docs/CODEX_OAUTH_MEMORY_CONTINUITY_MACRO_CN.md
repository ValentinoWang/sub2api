# Codex OAuth 记忆连续性宏观实施脚本

## 目标

在同一套 Sub2API 中，将上游 OpenAI OAuth 账号视为可替换的执行通道，而不是
记忆所有者。对每一个 Sub2API 用户，使用稳定的逻辑会话身份保存可恢复的会话链；
当调度从 OAuth 账号 A 切换到账号 B 时，继续该用户自己的上下文。

这里的“统一”是：

```text
同一 Sub2API 用户 + 同一 Codex task
                 -> 一条 provider-independent continuity chain
                 -> 可先后使用 OAuth A、OAuth B、API Key C
```

绝不能做成所有用户共享一份记忆。不同用户即使提交相同的 `session_id`、
`conversation_id` 或 `prompt_cache_key`，也必须得到不同的连续性身份。

## 不可伪造的边界

1. OAuth 登录本身不提供一个可批量导出的“Codex 记忆库”。Sub2API 无法从各个
   OAuth 账号反向下载不存在或无权访问的完整历史。
2. `disable_response_storage = true` 时，上游更不能被当作永久历史库。
3. 已经存在于用户 Mac 上的 `~/.codex/memories/`、`sessions/` 和
   `archived_sessions/`，只能通过用户明确授权的客户端导入流程进入新的连续性账本。
4. 正在流式输出的 WebSocket、尚未完成的工具调用和进程内连接状态不可迁移。
   必须先完成或明确中止，再从最后一个已提交 turn 恢复。
5. Redis 只保存临时路由亲和与租约；PostgreSQL 才能保存服务端持久连续性账本。

## 当前已实现的灰度纵切（2026-07-22）

当前代码已覆盖 Codex 实际使用的 HTTP `/responses` + SSE 主路径，以及客户端
Responses WebSocket v2 入站路径：

```text
认证后的 Sub2API user_id + api_key_id + 显式 task/session
  -> HMAC continuity_id（不包含上游 OAuth account_id）
  -> PostgreSQL 加密 replay snapshot
  -> 调度切换账号时删除旧 previous_response_id
  -> 按原顺序重放已完成 input/output
  -> 仅 response.completed / response.done 后原子提交
```

启用配置：

```yaml
gateway:
  openai_continuity:
    enabled: true
    secret: "至少 32 字符的独立随机密钥"
    api_key_ids: [管理员 API Key 的数据库 ID]
    retention_days: 30
    max_replay_bytes: 33554432
```

`api_key_ids` 为空表示对所有 API Key 开启，因此灰度阶段必须填写管理员 Key ID。
客户端必须持续发送同一个明确的 `session_id`、`conversation_id` 或
`prompt_cache_key`；缺失这些信号时不会用内容哈希猜测两个请求属于同一 task。

当前纵切尚不包括 Mac 历史任务导入、用户自助导出/删除接口和大型图片对象化。
`max_replay_bytes` 只限制连续性快照写入，不限制原请求、工具输出或附件上传；超限时
原请求仍正常完成，但服务端记录 `openai.http_continuity_commit_failed`，该 turn 不会
被声称已持久保存。大型图片的完整跨账号恢复需要先实现对象存储引用，不能反复重放
Base64。

## 宏观执行脚本

以下内容是实现任务的强制顺序。每一步均需先补失败测试，再修改生产代码。

### Stage 0：冻结语义并盘点现状

```text
读取并记录：
  gateway.openai_ws.sticky_session_ttl_seconds
  gateway.openai_ws.sticky_response_id_ttl_seconds
  gateway.openai_ws.ingress_previous_response_recovery_enabled

确认现有链路：
  session_hash -> account_id                  Redis，临时亲和
  previous_response_id -> account_id          Redis，临时亲和
  response_id -> conn_id / turn_state         进程内，不能持久迁移

输出基线指标：
  previous_response_not_found 总数
  account_switch_total
  sticky_session_hit_total
  sticky_previous_hit_total
  failover 后去锚重试次数
```

不得通过单纯延长 TTL 宣称“记忆统一”。TTL 只能延长账号亲和，不能把某个 OAuth
账号私有的 `previous_response_id` 变成跨账号可用。

### Stage 1：建立稳定且隔离的连续性身份

在认证完成后、账号调度之前生成：

```text
principal_id  = Sub2API user_id
credential_id = api_key_id
thread_id     = 优先使用可信客户端 task/thread header
                其次使用 conversation_id/session_id
                缺失时生成新的服务器 thread_id 并返回给客户端

continuity_id = HMAC-SHA256(
  CONTINUITY_SECRET,
  "v1" || principal_id || credential_id || thread_id
)
```

要求：

- 不使用 OAuth account ID 参与 `continuity_id`，否则账号切换后身份会改变。
- 必须包含 Sub2API 用户身份；API Key 是否纳入由产品策略决定，默认纳入以保持现有
  API Key 隔离行为。
- 不信任客户端直接提交的 `principal_id`。
- 日志只记录截断后的 continuity hash，不记录原始 task ID 或消息正文。

### Stage 2：新增 PostgreSQL 连续性账本

新增迁移，至少包含：

```text
codex_continuity_threads
  id, continuity_id(unique), user_id, api_key_id
  client_thread_fingerprint, status, version
  created_at, updated_at, expires_at

codex_continuity_turns
  id, thread_id, sequence(unique per thread)
  request_id(unique), state(pending/completed/failed/aborted)
  canonical_input_encrypted
  canonical_output_encrypted
  upstream_account_id, upstream_response_id
  input_sha256, output_sha256
  created_at, committed_at

codex_continuity_artifacts
  id, thread_id, turn_id, sha256, media_type, byte_size
  storage_key, created_at, expires_at
```

要求：

- 正文和工具结果必须应用加密、保留期和删除策略；默认关闭，按用户或分组显式启用。
- `request_id` 保证重试幂等，同一个请求不能重复执行有副作用的工具结果。
- 大图片和附件只保存一次对象及哈希，turn 中保存引用；不得把 Base64 在每次重放时
  重复写入 PostgreSQL。
- `pending` turn 不能成为恢复锚点；只有完整收到终态后才能原子提交为 `completed`。

### Stage 3：接入请求与正常亲和路径

```text
on request:
  authenticate Sub2API user and API key
  derive continuity_id
  acquire per-continuity advisory lock or bounded lease
  reject concurrent sequence conflicts, not attachment size
  append pending turn idempotently by request_id

  if previous_response_id is bound to a healthy compatible account:
      route to that account
      keep previous_response_id
  else if current session_hash binding is healthy and compatible:
      route to that account
  else:
      select a new compatible account

  stream upstream events unchanged
  persist only terminal, canonical response items
  commit turn and refresh Redis bindings
```

不要把日志、推理中间增量或未完成的 `image_generation_call.result` 当作已提交记忆。

### Stage 4：OAuth 账号切换与无锚恢复

当原账号不可用、配额暂停或明确返回 `previous_response_not_found`：

```text
finish or abort the current upstream attempt
load last completed turn sequence from PostgreSQL
materialize a canonical Responses input from committed turns
preserve user/assistant/tool/function_call/function_call_output ordering
replace large inline artifacts with durable artifact references
apply bounded compaction only when context exceeds the configured budget
send to the new OAuth account WITHOUT the old previous_response_id
bind the new response_id to the new account after terminal success
commit exactly one recovered turn using the original request_id
```

恢复不得静默丢弃 `function_call_output`。如果当前请求依赖未完成工具调用，返回明确的
可恢复错误，等待客户端重试，不能在另一个 OAuth 账号上重复执行工具。

### Stage 5：导入现有本地 Codex 记忆

现有历史的导入必须是独立、显式授权的客户端工具：

```text
client export:
  read ~/.codex/memories, sessions, archived_sessions
  preserve original files locally
  select exact tasks owned by the current user
  remove auth tokens and unrelated environment data
  create a signed manifest with per-file SHA-256
  upload through an authenticated, size-streaming import endpoint

server import:
  verify user identity, signature, hashes, quotas, and schema
  stage into an isolated import batch
  parse session_meta and completed turns
  reject cross-user ownership ambiguity
  deduplicate by task ID + turn ID + content hash
  require user confirmation before publishing into continuity threads
```

不得从服务器日志“还原记忆”，也不得把所有用户上传的数据汇总成公共语料。

### Stage 6：删除、导出和租户边界

必须同时提供：

- 用户按 task 导出自己的连续性账本。
- 用户按 task 或全量删除自己的服务端连续性数据。
- 管理员只能看到计数、大小、状态和错误码，默认不能查看正文。
- 账户删除时级联删除或进入可审计的延迟删除队列。
- Redis 丢失后，持久账本仍存在；PostgreSQL 不可用时禁止假装已保存。

## 验收门禁

实现不得上线，直到以下测试全部通过：

1. 同一用户、同一 task：OAuth A 完成 turn 1，强制切换 OAuth B 后 turn 2 能正确
   引用 turn 1，客户端不收到 `previous_response_not_found`。
2. 两个用户提交相同原始 `session_id`，生成不同 `continuity_id`，任何响应、附件、
   tool output 均不可互见。
3. 同一 `request_id` 重试五次，只产生一个已提交 turn，副作用工具不重复执行。
4. 上游在流式中途断开，只留下 `aborted/failed`，不会污染下一次恢复上下文。
5. Redis 清空并重启服务后，可从 PostgreSQL 已完成 turn 在新 OAuth 账号上恢复。
6. PostgreSQL 写入失败时请求明确失败，不返回“记忆已保存”的假成功。
7. 三张大图连续使用时对象只落一次，恢复请求不重复内联历史 Base64。
8. `disable_response_storage=true` 时仍可依靠本系统的显式 opt-in 账本恢复；关闭
   continuity 功能时不保存正文，保持现有无存储语义。
9. 导入包含其他用户 task、路径穿越、坏哈希或凭据文件时 fail closed。
10. 原有不启用 continuity 的 API Key 行为和吞吐基线不发生回归。

## 灰度顺序

```text
observe-only
  -> 仅计算 continuity_id 和指标，不保存正文
shadow-write
  -> 写账本但仍使用现有调度，不参与恢复
single-user recovery
  -> 只给管理员 API Key 开启跨 OAuth 恢复
small-group opt-in
  -> 小范围用户显式开启，可随时关闭
general availability
  -> 完成删除/导出/保留期与容量告警后再开放
```

每个阶段都保留一键关闭开关。关闭后回到现有 Redis sticky 行为，不删除用户已经明确
保留的账本，也不自动修改客户端 `~/.codex` 数据。

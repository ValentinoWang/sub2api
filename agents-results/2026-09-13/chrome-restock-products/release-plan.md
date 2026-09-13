# Chrome 辅助补货发布准备

状态：辅助脚本离线验证通过，实际发布未执行。本文是运维执行说明，不是 SSOT 开发决策或人工验收清单。所有机器结果留在本 bundle 的 `acceptance/`；数据库、环境配置、应用数据和原始 inspect 只保存到操作人指定的受限备份目录。

执行边界：根据本轮最新用户要求，生产保持 `0.2.4.2`，先完成本地验证与可审阅候选，再提前告知并等待用户明确确认。以下维护公告、备份、切换、恢复命令仅为后续手册，不授权现在运行。商户商品与库存写操作同样暂停。本准备结果不能把本轮状态提升为“可发布”或“已发布”。

## 候选冻结与同镜像提升

本轮目标版本 `0.2.4.3`，基线 `e77fce24d8e94f826e604e1c0895e90de93cd4ba`。先汇总业务 lane、完成当前候选完整本地 CI，再由主任务提交。未提交文件不能进入镜像。按 `deploy/build_image.sh FULL_COMMIT` 从该提交构建一次 Linux/amd64 镜像；保留完整构建日志、镜像 ID 和 OCI revision/version。仅在有足够磁盘空间的构建环境执行，不在本次仅余约 5 GB 的准备步骤构建。

从仓库根运行（变量由执行人从当前证据填写，不从旧报告复制运行状态）：

```bash
TASK_DIR=agents-results/2026-09-13/chrome-restock-products
python3 "$TASK_DIR/make_candidate.py" \
  --commit "$RELEASE_COMMIT" \
  --base-commit e77fce24d8e94f826e604e1c0895e90de93cd4ba \
  --version 0.2.4.3 --image-id "$RELEASE_IMAGE_ID" \
  --output "$TASK_DIR/acceptance/candidate.json"
```

生成器读取 Git 提交中的迁移文件，并按迁移执行器的 `sha256(trimmed SQL)` 规则计算。旧迁移被改写或删除则阻塞。镜像 ID 参数必须来自实际构建后的读取；候选文件本身不是镜像构建证明。dev 和 prod 必须使用同一份 candidate.json、同一镜像导出包、同一镜像 ID；分别校验传输包 SHA-256 和加载后 ID，不在生产重新构建。生成器不读取工作树 SQL，不接受用占位 SHA 代替已提交候选。

## 新迁移与回滚证明

新浏览器设备、配置、库存表和批次关联必须由应用正常启动迁移，不能手工跳过或登记 schema_migrations。最终新增文件名取 candidate 的 `added_migrations`，不在 helper 固定编号。旧应用读取新表结构的兼容性必须先在隔离数据库上验证：应用候选迁移后，运行待回滚旧镜像，确认启动健康、登录/充值入口/兑换与模型请求等原有功能可用；保存实际测试日志、源码与镜像身份。禁止在自用 8080 或生产数据库做此破坏性演练。

验证通过后，由执行人从真实记录制作受限或机器证据 JSON，至少包含：

```json
{
  "candidate_sha256": "candidate.json 的实际 SHA-256",
  "rollback_image_id": "本端当前旧镜像的完整 sha256: ID",
  "application_rollback_on_migrated_database_passed": true
}
```

该字段不是脚本自证。没有实际隔离验证不得填 true，也不得绕过切换前检查。每端旧镜像可能不同，必须分别绑定。切换失败的自动回滚，以及功能核验失败后的 `rollback_app.py`，都只恢复应用与原 `.env`，保留迁移后的数据库。不能回灌旧 dump 覆盖维护期间的新订单、余额或兑换记录；若需要数据恢复，另行制定精确恢复计划并取得授权。

## 每端串行切换

1. 先 dev、后 prod。保存当前容器、镜像、端口、挂载及业务配置的受限清单。保留旧镜像。两种补货 worker 必须维持关闭，设备配对、保存新商品和启用补货属于后续业务操作。
2. 每端在中断前发布全站 active/popup 维护公告并读回成功 ID。`release_api.py` 在操作前独占创建输出文件；若网络结果不明或留下空文件，先查公告，不能直接重试 POST 产生重复通知。
3. 在本端部署目录创建全新的 0700 备份目录。脚本保存数据库 `pg_dump -Fc` 并校验 TOC、配置/应用数据归档、容器/Compose 状态和迁移清单；生成 SHA-256 清单。备份的开始时间、候选和环境受切换脚本校验，超过 30 分钟重新备份。备份是在线快照，不承诺应用数据目录与数据库的跨介质事务一致性。
4. `replace_app.py` 校验 fresh backup、维护 receipt、回滚证明、旧容器身份、候选 OCI、镜像 ID、环境/挂载/端口与 Compose 不变量，随后仅 `up --no-deps ... sub2api`。它等待健康并要求数据库增量精确等于候选新增迁移；异常恢复原 `.env` 和旧应用。数据库和 Redis 不重建。
5. 切换后修复内置工具资产至候选版本；读回 API、两 worker 禁用状态、公开商品/充值页、多商品业务映射及隔离环境身份，执行真实授权模型流式请求和原业务功能核验。`/health`、API 初检或脚本 `RECREATED` 都不能代替功能验收。
6. dev 通过后在 prod 重复备份与维护流程，使用同一个已验证镜像。运行 `tools/quality/commerce_release.py --config PRIVATE_CONFIG --stage artifact --output MACHINE_RESULT` 收集现有 commerce 的两端迁移/结构与旧业务一致性。不要传 `--enable-sales`。旧 commerce parity 尚不覆盖新 Chrome 设备/商品表的完整业务契约，需额外保存新 API 及页面机器证据。
7. 只有健康与实际功能核验通过，才生成绑定 candidate SHA、环境、当前时间、`status: FUNCTIONAL_ACCEPTANCE`、`health_passed: true`、`functional_checks_passed: true` 的机器记录，发布恢复公告并读回，再归档维护公告。失败则调用 delayed，保留维护提示并继续核验；这不是人工 PASS。

本地调度端 API 示例（不把密钥值放进命令或输出）：

```bash
python3 "$TASK_DIR/release_api.py" maintenance --environment "$RELEASE_ENV" \
  --candidate "$CANDIDATE_FILE" --admin-key-file "$ADMIN_KEY_FILE" --output "$NOTICE_FILE"
python3 "$TASK_DIR/release_api.py" repair --environment "$RELEASE_ENV" \
  --candidate "$CANDIDATE_FILE" --admin-key-file "$ADMIN_KEY_FILE" --output "$REPAIR_FILE"
python3 "$TASK_DIR/release_api.py" verify --environment "$RELEASE_ENV" \
  --candidate "$CANDIDATE_FILE" --admin-key-file "$ADMIN_KEY_FILE" --output "$VERIFY_FILE"
python3 "$TASK_DIR/release_api.py" recovery --environment "$RELEASE_ENV" \
  --candidate "$CANDIDATE_FILE" --admin-key-file "$ADMIN_KEY_FILE" \
  --maintenance "$NOTICE_FILE" --acceptance "$FUNCTIONAL_FILE" --output "$RECOVERY_FILE"
```

目标部署主机示例：将 `backup_app.py`、`replace_app.py`、`rollback_app.py`、`release_common.py`、candidate、维护 receipt、回滚证明复制到同一受限目录，核对传输哈希后运行，不能使用旧的 stdin 单脚本方式。所有参数使用本端绝对路径。

```bash
python3 "$HELPER_DIR/backup_app.py" "$DEPLOY_DIRECTORY" "$NEW_BACKUP_DIRECTORY" "$CANDIDATE_FILE" "$RELEASE_ENV"
python3 "$HELPER_DIR/replace_app.py" "$DEPLOY_DIRECTORY" "$CANDIDATE_FILE" "$OLD_IMAGE_ID" \
  "$NEW_BACKUP_DIRECTORY" "$RELEASE_ENV" "$NOTICE_FILE" "$ROLLBACK_PROOF"
```

若切换已成功，但后续业务核验失败：

```bash
python3 "$HELPER_DIR/rollback_app.py" "$DEPLOY_DIRECTORY" "$CANDIDATE_FILE" \
  "$NEW_BACKUP_DIRECTORY" "$RELEASE_ENV" "$NOTICE_FILE" "$ROLLBACK_PROOF"
python3 "$TASK_DIR/release_api.py" delayed --environment "$RELEASE_ENV" \
  --candidate "$CANDIDATE_FILE" --admin-key-file "$ADMIN_KEY_FILE" \
  --maintenance "$NOTICE_FILE" --output "$DELAYED_FILE"
```

回滚不会把机器验收失败改成通过。自动回滚的 `rollback_runtime_verified` 只说明旧容器身份/运行不变量，仍需检查健康与功能。若已有公告 receipt 超过 30 分钟，先重新读回原公告并形成当前维护证据，不能更改旧文件时间冒充新读回。

## 本次离线验证

`python3 -B -m unittest discover -s "$TASK_DIR/tests" -v` 覆盖候选身份、过期/错环境 receipt、迁移丢失/漂移、备份篡改、迁移失败自动回滚、候选成功、缺功能验收拒绝恢复、恢复读回失败不归档、成功发布后才归档。语法通过 AST 验证，未调用 Docker、构建、网络 API 或读取任何真实密钥。结果见 `acceptance/release-prep/summary.json`。

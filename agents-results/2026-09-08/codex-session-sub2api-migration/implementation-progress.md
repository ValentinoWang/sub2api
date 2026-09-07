# 文档交付与后续开发状态

记录日期：2026-09-08。范围：开发 SSOT、离线迁移工具、站内经验入口和本地嵌入分发验证；不执行用户历史迁移或生产发布切换。

## 当前状态

主文档、声明输入、八份待批准验收合同、十个节点及其依赖已建立。N1-N6 的首版实现、离线包、站内详情页、P1 入口、预渲染与嵌入下载保护均已完成，并已由 Python 夹具、Vue 测试、生产构建和 Go 嵌入测试验证。F 是现状来源登记，D1 是用户对文档范围的确认；QR1/QR2 仍需要真实客户端接续记录和人工签字，不能由自动化替代。

本文件是技术交接记录，不是人类验收清单。最终正式结论以 `.ssot/validation-report.json` 为准；不把独立诊断脚本通过当成统一门禁通过。

## 已发现的校验阻塞

| 分类 | 事实与影响 | 后续负责入口 |
| --- | --- | --- |
| execution-blocker | 项目缺少 `.harness/manifest.yaml`，统一入口的 `runtime-skill-provenance` 无法验证项目运行时技能。 | Harness 项目接入流程；不能为通过检查伪造清单。当前文档任务不安装跨项目治理系统。 |
| incident | 所有已提交来源只作为现状参考，编译器不生成规范性来源登记；统一检查的 `source-requirement-conservation` 则要求严格登记或旧版守恒记录，声明 schema 又不接受旧版字段。 | SSOT 编译器/检查器的同一来源模式合同应由 Harness 所属项目修复并回归；不手改生成记录，不把现状代码伪装成用户新需求规范。 |
| execution-blocker | 全库审计读取既有 `2026-08-16/Athlete-OS_repo/cb-r58-acceptance-closure-ssot/_ssot-snapshot-manifest.json` 超过两分钟无返回；已对该审计子进程发送 SIGTERM，退出原因非通过。 | iCloud 原文件可读后重跑 `--audit-archive`；不删除、替换或重建其他任务快照。最终重试结果保留在统一报告。 |

上述阻塞不表示设计选择尚未决定，因此不创建 `openproblem.md`。它们阻止正式门禁通过声明，不授权产品实现或用户数据修改。

## 复现与最终验证

以下命令从 Sub2api 仓库根目录执行。已完成的实现验证包括：`python3 -m unittest discover -s tools -p 'test_codex_*.py'`（18 项）、迁移包解压与 SHA-256 对照、迁移经验页相关 Vitest（29 项）、`pnpm --dir frontend run typecheck`、`pnpm --dir frontend run build` 和 `cd backend && go test -tags=embed ./internal/web/...`。HMR 页面在桌面与 390px 宽度的本地浏览器中已检查，复制提示词成功且无横向溢出或控制台错误。

Python 依赖仅服务于文档编译工具，不是迁移工具的安装流程。

```bash
uv run --with pyyaml --with jsonschema python \
  /Users/vsiyo/.codex/skills/report-to-ssot-development-paths/scripts/compile_ssot.py \
  --input agents-results/2026-09-08/codex-session-sub2api-migration/ssot-input.json \
  --bundle agents-results/2026-09-08/codex-session-sub2api-migration --project-root . --check

export SSOT_ARCHIVE_ROOT='/Users/vsiyo/Library/Mobile Documents/iCloud~md~obsidian/Documents/日记/开发/每日开发/SSOT合集'
python3 /Users/vsiyo/.codex/skills/ssot-obsidian-snapshot/scripts/snapshot_ssot.py \
  --source agents-results/2026-09-08/codex-session-sub2api-migration --project-root . --check
python3 /Users/vsiyo/.codex/skills/ssot-obsidian-snapshot/scripts/snapshot_ssot.py \
  --audit-archive --project-root .
uv run --with pyyaml --with jsonschema python \
  /Users/vsiyo/.codex/skills/report-to-ssot-development-paths/scripts/validate_ssot_bundle.py \
  agents-results/2026-09-08/codex-session-sub2api-migration \
  --report agents-results/2026-09-08/codex-session-sub2api-migration/.ssot/validation-report.json
```

统一检查最后运行；报告中的 `push_gate_eligible` 和 `release_complete` 分开读取。归档只包含主文档和生成清单，源目录保留实现进度、合同、机器记录和检查输出。全库审计失败或中止时总状态保持 partial。

## 当前验收边界

自动验证证明支持的本地 JSONL/SQLite 夹具、离线包与站内分发行为符合实现范围。它不证明任意历史客户端格式、正在运行的连接或 ChatGPT 网页历史可迁移。真实 macOS、Linux、Windows 客户端必须各自重新打开同一个迁移前旧任务、引用保留事实并完成一轮新交流；在每个平台填写可见结果并由验收人与复核人签字前，`docs/human-acceptance/CODEX_SESSION_MIGRATION.md` 保持“待验收”。

## 源码同步边界

GitHub Fork 主线已推送并读回 `60f5ff5770b071919d9b19fb94f16340b1912f32`。2026-09-08 对 `ubuntu@43.156.50.78:/home/ubuntu/sub2api` 的只读 SSH 检查被服务器以 `Permission denied (publickey)` 拒绝，未读取工作树、未同步源码、未重启服务，也未执行生产发布。服务器管理员授权现有工作站密钥后，必须先重新检查远端工作树、分支和远端地址，再进行常规拉取。

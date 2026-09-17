# 本机模型目录配置生效验证

日期：2026-09-15，23:05 CST。范围：本机 Codex 配置及短请求；不包含生产部署或长输入压测。

## 配置与备份

- `/Users/vsiyo/.codex/config.toml` 顶层第 3 行增加 `model_catalog_json = "/Users/vsiyo/.codex/model-catalogs/sub2api.json"`。
- 配置原件备份：`/Users/vsiyo/.codex/catalog-config-backup.uMd5jY/config.toml`。写入前逐字节比对备份；写入后验证去掉新增行即与备份完全相同，权限仍为 0600。
- 保留全部原有 provider、features、memories、profiles、MCP、trust 和其他配置。未添加手写 `model_context_window` 或压缩阈值。
- 使用公开教程脚本重新获取当前配置入口的目录，并用桌面内置引擎验证后保存。19 个模型，SHA-256 `83ee91dbd9f3f227cbd6712321117cfadfb72f82db6905be88f9e16ced68874b`；与前次目录相同。
- 目录备份：`/Users/vsiyo/.codex/model-catalogs/sub2api.json.backup-1789484403812449000`。完整目录与配置备份只保留本机，不存入网站。

## 默认加载与真实请求

实际程序：`/Applications/ChatGPT.app/Contents/Resources/codex`，版本 `0.154.0-alpha.6.2`。

1. 修改前无覆盖执行 `debug models`：Astra 默认窗口 272,000，最大窗口 872,000，比例 95%。
2. 修改后无覆盖执行同一命令：默认和最大窗口均为 1,050,000，比例 95%，可用预算 997,500。此检查不传入临时 `-c model_catalog_json`。
3. 启动同一桌面引擎的独立 app-server，读取已保存的默认配置。使用只读、ephemeral 的测试会话，不覆盖模型、provider、目录或窗口。为避免无关外部工具访问，仅在测试进程中禁用 MCP、apps 和 memories，未持久修改这些设置。
4. 真实模型回复 `CATALOG_OK`；收到 `thread/tokenUsage/updated` 中的 `modelContextWindow: 997500`，并收到 `turn/completed`，状态 `completed`。
5. 请求输入 20,447 tokens、输出 34 tokens；仅证明短请求可用，未证明最大长度输入可用。

通过结果：[config-runtime-result-20260915T150545Z.json](config-runtime-result-20260915T150545Z.json)。验证脚本：[check-config-runtime.py](check-config-runtime.py)。

首次验证已收到回复与 997,500 窗口，但文本缓冲使脚本未及时消费完成事件并超时；保留 [首次失败结果](config-runtime-result.json)。修正验证脚本为无缓冲管道后复验通过，没有修改网关或放宽通过条件。

## 当前任务与回滚

这条已加载任务在配置修改后的 23:04 CST 仍报告 258,400。默认配置生效与正在运行任务热更新是不同状态；没有中断当前生成、重启应用或操作其他任务。结束活跃任务后重新启动 Codex，再检查任务运行时窗口。

回滚本次配置启用只需移除新增的 `model_catalog_json` 行；若后续配置已变化，不用整份旧备份覆盖新改动。需要恢复目录时使用上述本机目录备份。Codex memories 和任务记录保持原样。

# Acceptance 目录与文档说明

`acceptance/` 是 Sub2api 项目级人工验收入口，不是测试、截图、日志或发布证据的汇总目录。

任务级机器、E2E、视觉、sandbox、生产和发布证据放在 `agents-results/YYYY-MM-DD/<task>/acceptance/`。跨任务人工验收的清单、绑定、入队事件和签署结果放在 `acceptance/human/`。

## 固定入口

```text
acceptance/
├── README.md
├── index.md
├── human-acceptance-log.md
├── human-acceptance-log.json
└── human/
    ├── .gitkeep
    └── YYYY-Www/
        └── {未-}YYYY-MM-DD-<task-id>/
            ├── binding.md
            ├── checklist.md
            ├── handoff.json
            └── runs/<run-id>/
                ├── result.md
                └── evidence/
```

`README.md` 由项目维护，用于说明边界和流程。`index.md` 与 `human-acceptance-log.{md,json}` 由工具生成，只负责定位和投影，不是验收事实。签署结果、binding、checklist、handoff 和任务合同才提供各自范围内的事实。

## 任务证据

任务根使用以下按需目录，不创建没有实际用途的空 lane：

```text
agents-results/YYYY-MM-DD/<task>/
├── acceptance-contract.md
└── acceptance/
    ├── index.md
    ├── machine/{static,unit,integration-contract,e2e,local-runtime,non-functional}/
    ├── visual-fidelity/
    ├── persistent-runtime/
    ├── exploratory/
    ├── external-sandbox/
    ├── production/
    └── release/
```

每次执行创建唯一 run。`result.md` 给出 `PASS`、`FAIL`、`PARTIAL` 或 `BLOCKED` 的有界结论，`evidence/` 保存原始支撑材料。终态 run 不覆盖；源码、环境、角色或合同版本变化时创建新 run。发布只引用明确 run ID、路径和哈希，不使用可变 `latest` 或符号链接。

## 人工验收

稳定逻辑路径为 `acceptance/human/YYYY-Www/YYYY-MM-DD-<task-id>/`。没有当前有效人工 `PASS` 时，物理目录增加 `未-` 前缀；合同和元数据始终使用不带前缀的逻辑路径。

`binding.md` 绑定任务、合同、清单和所需角色。`checklist.md` 只收录机器无法唯一判断的中文人工步骤。`handoff.json` 证明同一源码身份的必需机器证据已通过并进入人工队列，不代表人工通过。人工 `runs/<run-id>/result.md` 才记录指定角色的签署结论。

## 历史目录

[`docs/human-acceptance/`](../docs/human-acceptance/README.md) 是只读历史来源，保留已有路径、清单和引用，不继续追加新任务或新验收记录。新任务必须按本目录的 split-root 规则建立合同、工作区及绑定；历史来源不参与当前生成投影，也不能据此推导当前人工 `PASS`。

仅当 `acceptance/` 或 `acceptance/human/` 内确实存在历史直接子目录时，才分别在 `acceptance/legacy-manifest.json` 或 `acceptance/human/legacy-manifest.json` 中登记。声明格式为：

```json
{
  "schema_version": 1,
  "legacy_paths": ["existing-direct-child"]
}
```

声明项必须是相应根下已存在的直接子目录，不允许 `../docs/human-acceptance`、绝对路径或其他跨根声明。当前不存在需要登记的此类目录，因此不创建 legacy manifest。历史目录只为保留已有路径和字节，不参与当前状态计算，也不得继续追加新证据。不得新建根级 `REL1`、`REL2` 或其他 release 分类目录。

## 标准命令

先将 `HARNESS_ENGINEERING_HOME` 设置为持久的 Harness_Engineering 检出目录。项目不会覆盖已有 `.agents` 内容，也不依赖临时目录链接。

```bash
python3 "$HARNESS_ENGINEERING_HOME/Core/skills/design-acceptance-contract/scripts/manage_acceptance_artifacts.py" log --project-root .
python3 "$HARNESS_ENGINEERING_HOME/Core/skills/design-acceptance-contract/scripts/manage_acceptance_artifacts.py" sync-human-status --project-root .
python3 "$HARNESS_ENGINEERING_HOME/Core/skills/design-acceptance-contract/scripts/manage_acceptance_artifacts.py" check agents-results/YYYY-MM-DD/<task> --project-root .
make test-acceptance-layout
```

`make test-acceptance-layout` 调用 `tools/quality/run_acceptance_artifact_layout_guard.sh`，依次使用显式 `HARNESS_ENGINEERING_HOME`、`.harness/upstream`、兄弟目录 `../Harness_Engineering` 的中央 manager 执行 `check-project`；找不到 manager 时检查失败。`make test` 也包含该门禁。

禁止手工修改生成投影来改变状态，禁止仅移除 `未-` 宣告通过，禁止在没有有效合同、binding 和机器证据时制造 handoff，也禁止把截图存在当作 API、持久化、权限、生产或人工验收通过。

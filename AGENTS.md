# Sub2API Project Instructions

## 人类验收与自动验收边界

### 统一地址

- 新任务统一遵循 `acceptance/README.md` 的 split-root 规则：机器、E2E、视觉、sandbox、生产和发布证据位于 `agents-results/YYYY-MM-DD/<task>/acceptance/`；人工清单、绑定、入队和签署结果位于 `acceptance/human/YYYY-Www/{未-}YYYY-MM-DD-<task-id>/`。
- `acceptance/index.md` 与 `acceptance/human-acceptance-log.{md,json}` 是中央 manager 生成的索引和投影，不能手工修改来改变验收状态。合同、binding、checklist、handoff 和签署结果提供各自范围内的事实。
- 没有当前有效人工 `PASS` 时，物理工作区必须保留 `未-`；合同和元数据使用不带前缀的稳定逻辑路径。新任务必须绑定当前合同和清单，机器通过后进入人工队列，实际人工执行属于终态阶段。
- `docs/human-acceptance/` 下的历史清单保留既有路径、字节和引用，不再新增任务或验收记录，也不参与当前人工状态投影。其 README 只维护历史提示与到新入口的导航，不得把它重新定义为当前签署入口。不得用 acceptance legacy manifest 声明这个跨根历史路径。
- 自动验收代码保留在测试目录，运行输出进入任务证据根，不得把自动验收输出复制到项目级人类验收目录。禁止新建根级 `acceptance/REL*` 发布目录。

### 人类验收文档允许的内容

- 验收范围和不在本次验收范围内的业务。
- 验收人开始前需要拿到的商品、测试账号、测试资料和预期权益。
- 人类可以执行的具体操作，例如打开页面、提交资料、购买、兑换、刷新、重复操作、申请退款和查看通知。
- 人类可以直接观察和核对的结果，例如页面状态、金额、库存、到账权益、错误提示、通知和下载内容。
- 明确的通过、不通过、阻塞条件。
- 非敏感截图或人工记录的引用，以及验收人、复核人和签字时间。

### 人类验收文档禁止的内容

- Git 提交、分支、远端同步状态或源代码差异。
- 发布版本、镜像名称、镜像 ID、摘要或构建信息。
- 测试命令、自动化测试输出、覆盖率、HTTP/API 探针或运行日志。
- 数据库迁移、环境变量、内部开关、进程、容器或服务器状态。
- 维护公告、部署步骤、发布状态、切换过程或回滚过程。
- 内部任务 ID、`run_id`、Object Lock 读回、内部审计对象或其他实现证据。
- Session、Token、CDK、密码、密钥、完整账号资料或供应商原始响应等敏感值。

以上技术事实需要保留时，应写入对应任务机器证据、开发、运维或受限发布记录，不写入人工步骤正文。`binding.md`、合同和工具管理的元数据按中央规范保留必要任务标识与哈希；历史清单保持只读。

### 状态与结论

- 自动测试、构建、部署、健康检查或生产探针通过，不等于人类验收通过。
- 没有完成实际操作、结果核对、验收记录以及验收人和复核人签字时，状态必须保持“待验收”或“阻塞”。
- Plus、Pro20x 或其他不同 SKU、通道、凭证模式和周期必须分别验收；一个对象的通过记录不能替代另一个对象。
- 人类验收通过不自动授权开售、启用支付或启用真实供应商提交；这些是单独的运营与发布决定。

### 修改后的检查

修改人类验收文档后至少确认：

1. 新清单位于 `acceptance/human/YYYY-Www/{未-}YYYY-MM-DD-<task-id>/`，日期与 ISO 周一致；合同、binding 和引用使用稳定逻辑路径。历史 `docs/human-acceptance/` 内容与引用保持原样。
2. 文档只描述人类操作、可观察结果、判定和签字，不包含上面的开发或发布证据。
3. 新文档没有被 `.gitignore` 隐藏，并已出现在 `git status --short --untracked-files=all` 或 `git ls-files` 中。
4. Markdown 相对链接可以解析；历史路径仅作为历史引用保留，`git diff --check` 通过。
5. 文档和验收记录不包含任何真实凭证或敏感数据。
6. 使用中央 `manage_acceptance_artifacts.py log --project-root .` 刷新三投影，并运行 `make test-acceptance-layout`。门禁使用 `HARNESS_ENGINEERING_HOME`、`.harness/upstream` 或持久兄弟目录 `../Harness_Engineering` 解析中央来源；缺失或校验失败不能视为通过。

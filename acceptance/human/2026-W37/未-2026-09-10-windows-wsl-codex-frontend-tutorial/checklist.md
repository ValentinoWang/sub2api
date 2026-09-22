# 人工验收清单：windows-wsl-codex-frontend-tutorial

- 任务编号：windows-wsl-codex-frontend-tutorial
- 人工验收绑定：acceptance/human/2026-W37/2026-09-10-windows-wsl-codex-frontend-tutorial/binding.md
- 验收合同：agents-results/2026-09-10/windows-wsl-codex-frontend-tutorial/acceptance-contract.md
- 合同版本：1
- 清单状态：草稿
- 所需人工角色：产品负责人
- 清单负责人：使用者
- 批准证据：待使用者审阅
- 执行结果：acceptance/human/2026-W37/2026-09-10-windows-wsl-codex-frontend-tutorial/runs/<run-id>/result.md

## H-01

- 验收问题：一个第一次使用 WSL 和 Codex 的前端开发者，是否能分清应在 Shell 与 Codex 中输入的内容，并理解人和 Agent 各自负责什么？
- 必须人工判断的原因：教程是否容易理解、是否让初学者产生正确工作流认知，需要真实阅读和操作判断。
- 闭环名称：在真实仓库中完成一次 Agent 前端开发。
- 验收角色：产品负责人，无需登录。
- 进入方式：[本地教程](http://127.0.0.1:4174/experiences/windows-11-wsl-codex-frontend)。
- 前置条件：使用桌面浏览器和手机宽度窗口；准备一个可用于练习的非生产前端仓库。
- 验收步骤：
  1. 从经验分享页找到并打开 Windows 11 WSL Codex 教程。
  2. 阅读首屏，判断学习目标、完整流程和 Human/AI 职责是否一眼可分辨。
  3. 阅读第 1 至第 3 步，确认能够判断哪些内容输入 WSL Shell。
  4. 阅读第 4 至第 9 步，确认能够判断哪些内容输入 Codex，并点击复制一条 Shell 命令和一条 Prompt。
  5. 缩窄窗口到手机宽度，查看流程、长命令、长 Prompt 和最终练习是否容易阅读。
  6. 阅读最终练习，判断自己是否知道怎样让 Codex 调查、计划、开发、测试、Review 并交回人工验收。
- 预期观察：Terminal 与 Prompt 视觉明确不同，步骤和责任边界清楚，复制反馈可见，桌面与移动端均可阅读，最终练习可直接用于真实仓库。
- 副作用确认：本清单只阅读和复制页面内容；是否执行安装脚本或在个人仓库实施练习由验收人自行决定。
- 判断标准：无需重新阅读全量代码或手工寻找项目命令，也能理解并开始一次完整流程为通过；输入位置或职责含糊、页面难以阅读为不通过；页面无法访问为阻塞。
- 预计时长：8 分钟。
- 是否阻塞：否。
- 结果记录：执行后记录实际观察，并由验收人及复核人签署；当前为待验收。

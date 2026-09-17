# Acceptance Run: 20260910T161231Z-visual-7dbbdb

- Run ID: 20260910T161231Z-visual-7dbbdb
- Task ID: windows-wsl-codex-frontend-tutorial
- Lane: visual-fidelity
- Status: PASS
- Acceptance contract: agents-results/2026-09-10/windows-wsl-codex-frontend-tutorial/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 9221df47eac56d5f5b1e9839bbb4ab53e82dcbe3a20088186c6d05ed1d331db3
- Source identity: 20189b7348fa74020e066241890c240e0c28cad4+worktree-7dbbdb86976a
- Runtime identity: vite-hmr-127.0.0.1:4174
- Executor or reviewer: Codex browser inspection
- Started at: 2026-09-10T16:04:09Z
- Completed at: 2026-09-10T16:12:31Z
- Evidence directory: evidence/

## Scope

固定 HMR 地址中的教程桌面与移动布局、复制反馈、横向溢出、控制台和可编辑 Figma 视觉意图。

## Procedure

1. 直接打开教程路由，检查桌面首屏信息层级。
2. 将视口设为 390x844，检查 10 步正文和页面滚动宽度。
3. 点击第一条 Linux 命令的复制按钮并读取可见状态。
4. 检查浏览器控制台并区分教程错误和本地后端登录态错误。
5. 在现有 Figma 文件中建立桌面和移动 Frame，递归检查节点类型、图片填充、字体与文本溢出。

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-02 | PASS | evidence/browser-observations.json | 完整内容和两种输入卡在真实浏览器中可见 |
| AC-04 | PASS | evidence/browser-observations.json | 桌面首屏层级清楚；390x844 无页面级横向溢出；Figma 为原生可编辑节点 |

## Findings

- 浏览器控制台没有教程组件自身异常。
- 本地 8080 的失效登录态导致全局应用初始化记录 401 和相关预加载错误，属于既有运行环境状态。
- 深色 CSS 与浅色 CSS 同步实现；公共页没有可见的主题切换入口，本轮没有单独生成深色浏览器截图。

## Unverified items

真实 Windows 11、WSL2、不同浏览器的剪贴板权限，以及初学者实际跟随完成练习仍未自动验证。

## Conclusion

AC-04 通过；人工 H-01 仍为待验收。

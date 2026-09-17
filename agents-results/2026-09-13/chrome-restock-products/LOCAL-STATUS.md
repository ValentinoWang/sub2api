# 本地 Chrome 补货验证进度

当前状态：本地代码、镜像和基础界面检查通过；真实 Chrome 补货验证阻塞，未完成人工验收，不可据此发布生产。

- 候选版本：0.2.4.3；提交 `0c8afe26a4c136a9fea2eb1ec7134c9fbe64f889`。
- 完整本地 CI：19 阶段全部 PASS，前端 298 个测试文件、2256 项测试通过。证据：`acceptance/local-ci-4/summary.json`。
- 本地8080已替换为候选镜像并健康；迁移241已应用，原数据、挂载与端口保留。当前候选的隔离迁移及旧镜像回滚测试通过。证据：`acceptance/local-image-proof-4.json`、`acceptance/local-replacement-1.json`。
- 本地维护公告1已归档、恢复公告2已发布。证据：`acceptance/local-recovery-1.json`。
- 管理员浏览器页面实际显示0.2.4.3、五档测试映射5/10/20/50/100、设备等待浏览器验证；购买页暂不展示未核验商品，保留兑换入口。证据：`acceptance/local-render-readback-1.json`。
- Chrome设备仅授权独立测试商品874783、875537、875562、875570、875577；规则¥1面额对应$1额度，手续费不入额度。
- 两种自动补货仍关闭；没有本轮真实库存上传、购买或兑换验证。Chrome工具连接返回Debugger unattached；通过普通原生窗口完成本地界面读回。扩展菜单未观察到小铺补货助手，仍需用户手动加载。
- HMR4174检查时已停止，现重新启动，4174健康代理及Vite客户端均200；后端固定本地8080。
- 清理本任务独立Go/lint缓存约6.3GiB，Docker构建缓存3.459GB；保留HMR依赖、数据库、受限备份和回滚镜像。证据：`acceptance/task-cache-cleanup-2.json`、`acceptance/docker-build-cache-cleanup-2.log`。
- 本轮未推送Git、未合并main、未操作生产发布或正式商品。

剩余步骤：用户在Chrome手动加载 `/Users/vsiyo/.local/share/sub2api/ldxp-browser-extension-dev`，选择本地8080并绑定受限设备，然后执行独立测试商品双扫描、少量补货、重复执行不增码、中断暂停与恢复核验。真实购买由用户完成。未经上述验证不得称自动补货已跑通；生产发布须先说明并获得用户确认。

完整机器证据位于隔离候选工作树：/Users/vsiyo/.local/share/sub2api/worktrees/chrome-restock-products-20260913/agents-results/2026-09-13/chrome-restock-products

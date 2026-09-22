# 本地 CI 工具链核验

独立目录：`/Users/vsiyo/.local/share/sub2api/ldxp-recovery-toolchain/bin`。

- Python：3.12.14；精确包装 Codex bundled runtime；plistlib 往返、pyexpat 和 `str | None` 验证通过。
- Node：24.19.0；精确包装已有 Codex bundled runtime。
- pnpm：9.15.9；直接用该 Node 执行已缓存的 pnpm.cjs，避免系统 pnpm fallback。
- Go：1.27.0；精确包装现有 toolchain。
- golangci-lint：2.13.0；固定官方 release 下载，归档 SHA-256 与官方 checksums 一致。
- govulncheck：1.8.0；通过 Go module 安装于独立工具目录，模块校验与编译信息见 versions.log。

使用时仅对当前命令设置 PATH：

```bash
PATH=/Users/vsiyo/.local/share/sub2api/ldxp-recovery-toolchain/bin:$PATH bash tools/quality/run_local_ci.sh <新的证据目录>
```

不要用 CI_NODE_BIN_DIR 将其他 pnpm 置于上述目录前。未修改仓库依赖或用户全局配置；本记录只证明工具准备与版本核验，不证明完整 CI、数据库或补货恢复通过。

完整来源、SHA-256 和版本输出见 toolchain.json、versions.log；官方校验清单保存在同目录。

2026-09-16 Python 补充核验：候选 checkout `e87acb34928b915721ed6268da529052186aa0e6` 中 `tools/quality/tests/test_pnpm_audit_exceptions.py` 的 11 个定向测试全部通过；未改源码或测试。完整 CI 仍由主任务运行。

# 0.2.6.1 发布准备记录

状态：阻塞，未发布到生产。

- 本地分支：`dev`；当前提交：`5d839653d2ed02965a43964f8402401c5ea69a8c`。
- `backend/cmd/server/VERSION` 为 `0.2.6.1`；跟踪文件工作区干净。
- 已修复前端设置字段类型、页面隐藏按钮行为和后端 Wire 依赖注入遗漏。
- 设置页和自定义页面测试：54 项通过，见 `settings-custom-page-tests.log`。
- `local-ci-01` 因源码修正而中止，保留退出码 143。
- `local-ci-02` 发现 Grok 免费额度缓存测试过早断言；其余后端包通过。已将该测试改为等待实际缓存写入，保留原有拦截断言。
- 修正后重复测试和 `local-ci-03` 均遭遇 `no space left on device`，不能视为验证通过。完整 CI 后续阶段未执行，见 `local-ci-03/summary.json`。
- 从提交 `89be52f2257bf46ea82b8656a443c5fdf4738356` 的独立 Git 快照完成 Linux/amd64 镜像构建，日志为 `image-build-89be52f22.log`。构建日志记录镜像 `sub2api-local:0.2.6.1-89be52f2257b`、ID `sha256:d1dd8439bdff4ee6cfec9260078d5fdc0df832fcbf02432823f738ef8d777b65`。此提交与当前 HEAD 仅相差上述测试修正。
- 磁盘空间不足后，本地 Docker API 不再及时响应；镜像读回、二进制版本运行校验和镜像导出尚未完成。不能将该镜像标记为可发布候选。
- 生产入口 `ubuntu@43.156.50.78` 拒绝当前 SSH 身份，返回 `Permission denied (publickey,password)`。等待当前有效登录方式。
- 生产公开 `/health`、`/`、`/Api_subscribe`、`/admin/ops` 最近检查均返回 HTTP 200；这不证明认证登录或模型上游可用。
- 未上传镜像、未发布维护通知、未切换容器、未删除生产旧镜像、未修改生产数据库或 Redis。

后续必须先恢复本地磁盘和 Docker 可用性并完成验证，取得有效生产访问方式，然后按生产部署流程保留现有数据库、切换应用，并在验收成功后删除旧应用版本。

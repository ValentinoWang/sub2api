# 本地 8080 更新记录

本次已将本地应用从 0.1.183 更新为技术版本 0.2.5，包含当前工作区已完成的前端修改。

- 入口：http://127.0.0.1:8080/home
- 本地镜像：`sub2api-local:0.2.5-local-1b221364260b`
- 镜像摘要：`sha256:be86989dffeb042d5bbf60f6ed6567d07cb449c72402794673776adf6568f7f8`
- 当前构建：`20189b7348fa-local-1b221364260b`，`linux/amd64`，在本机通过 Docker 架构兼容运行。
- 部署入口：`deploy/docker-compose.local.yml`；镜像已固定在忽略提交的 `deploy/.env`。
- 已完成完整构建、工具包校验、15 个迁移、核心数据摘要核对、5 次健康检查、6 个公开页面及静态资源检查，以及 5 个页面的实际 Chrome 浏览检查。
- 用户 1、模型账号 10、API Key 3、分组 4、代理 66、订单 0；数量及选定身份字段与更新前一致。PostgreSQL、Redis、Mihomo 容器未替换。
- 已删除本次构建的独有缓存 3.703 GB；共用镜像层保留。旧应用进程的内存缓存随容器替换消失；Chrome 首页已强制刷新。未清空 Redis、浏览器 Cookie、站点存储或其他项目缓存。
- 磁盘可用空间在缓存清理后约 8.8 GiB。

## 恢复材料

私有备份：`/Users/vsiyo/.local/share/sub2api-backups/20260910-local-8080/`。该目录不在仓库内，包含旧环境、Compose、原容器配置及已验证的数据库备份。旧镜像 `sub2api-local:0.1.183-proxy-subscription-ss-20260903` 保留。

恢复前先确认更新后是否有新数据写入，再决定仅恢复旧镜像或恢复数据库；不能直接覆盖新的业务记录。不要将私有备份上传到 Git。

## 验证边界

这里只完成本地部署，不代表模型付费请求、真实订单或人工验收通过。未更新远端服务器、推送 Git 或创建远端 release。模型实测展示仍是文档提案，未在本次实现。

机器证据：[本地运行验证](acceptance/machine/local-runtime/runs/20260910T094005Z-local-8080-667c7b/result.md)。人工清单位于项目 `acceptance/human/2026-W37/未-2026-09-10-local-8080-version-sync/`，当前待审阅。

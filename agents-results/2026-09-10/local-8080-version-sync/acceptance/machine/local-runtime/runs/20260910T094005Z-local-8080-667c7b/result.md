# Acceptance Run: 20260910T094005Z-local-8080-667c7b

- Run ID: 20260910T094005Z-local-8080-667c7b
- Task ID: local-8080-version-sync
- Lane: machine/local-runtime
- Status: PASS
- Acceptance contract: agents-results/2026-09-10/local-8080-version-sync/acceptance-contract.md
- Contract version: 1
- Contract SHA-256: 427ef1e326a89b501ef5e7a031f37442853bd2c3a631a02ad78a4e351419572d
- Source identity: 20189b7348fa74020e066241890c240e0c28cad4+local-1b221364260b
- Runtime identity: sub2api-local:0.2.5-local-1b221364260b
- Executor or reviewer: Codex
- Started at: 2026-09-10T09:40:05.786679Z
- Completed at: 2026-09-10T09:47:58.302687Z
- Evidence directory: evidence/

## Scope

本机 127.0.0.1:8080 的当前工作区安装、数据保留与缓存处理；不涉及生产发布、付费请求或人工签署。

## Procedure

1. 私有备份原 Compose、环境、容器配置和 PostgreSQL；校验备份目录与迁移历史。
2. 从当前工作区执行 Dockerfile 完整 linux/amd64 构建，包括 Vue 类型检查、Vite、预渲染、Go embed 和 LDXP 工具包。
3. 校验镜像程序版本和工具校验值，在本机 deploy/.env 固定镜像，并使用 docker compose up -d --no-deps --no-build --force-recreate sub2api 替换应用。
4. 对照数据库备份核验 15 个迁移执行及 7 个兼容历史标记；对照更新前统计及身份字段摘要，检查依赖容器未更换。
5. 连续五次健康请求，读取六个公开页面及其静态资源，在实际 Chrome 中查看五个关键页面并强制刷新首页。
6. 按本次构建的 58 个精确缓存 ID 清理构建缓存，回收 3.703 GB；保留镜像共用层、旧镜像和业务数据。

## Requirement disposition

| Requirement | Result | Evidence | Notes |
| --- | --- | --- | --- |
| AC-01 | PASS | evidence/runtime-verification.json; evidence/browser-observations.json; evidence/cache-summary.json | 新页面和资源正常，旧内存缓存随进程替换，实际浏览器强制刷新；没有宣称清空整个浏览器缓存 |
| AC-02 | PASS | evidence/before.json; evidence/runtime-verification.json; evidence/migration-after.json | 业务数量及选定身份字段一致，迁移校验无冲突 |

## Findings

- 构建必须使用 amd64，本机 Docker 兼容运行；本地健康通过，不等同于模型请求性能验证。
- 迁移记录从 270 增至 292，其中 15 个为当前迁移，另外 7 个为 238 主动登记的旧业务迁移兼容标记，已核对源 SQL 和摘要。
- 旧容器的 PROXY_SUBSCRIPTION_DEFAULT_REFRESH_INTERVAL_MINUTES 变量在当前源码已无读取点；数据库中的代理及账号代理关联保持一致。其旧值在私有容器备份中保留。
- 工具初次使用 --version 不符合其 CLI 语法，后用 version 子命令确认 0.2.5；此检查未调用供应商服务。
- 没有修改业务源代码；安装包含本任务开始前已存在的工作区修改。

## Evidence manifest

| Artifact | SHA-256 | Meaning |
| --- | --- | --- |
| evidence/artifact-verification.json | 827544c41c19c6c2b2b00ce62a97518418882f2ade7d495cbd034ed3e83cb3a5 | Local build, runtime, data or cache evidence |
| evidence/before.json | 21fa0070eb7c5d9968459f5fa8797385a39c276c3790f3bdcf1459e889d15a5e | Local build, runtime, data or cache evidence |
| evidence/browser-observations.json | d71ad6da0b2fae67d3f2585709dc8f566e3cbd92c208351d1f4e0c8b3563228b | Local build, runtime, data or cache evidence |
| evidence/build-cache-after.txt | 232adbd8bc5e56b14b25aaeb64dcaa6a16d2b5ba97aa3498c65cbc542080f022 | Local build, runtime, data or cache evidence |
| evidence/build-cache-before.txt | 613cc395c7d066da20c9cfa32829518f93bd0e80070debaaea0350e79b357933 | Local build, runtime, data or cache evidence |
| evidence/build-cache-cleanup.log | dad948e0ff8f6411391b120e1d84e5fec76d3ea42a12c4b580f7a69a71900480 | Local build, runtime, data or cache evidence |
| evidence/cache-removal-plan.json | 957f9aec837ea8a93f4abbe7451265527bee2077eb4b7bf904c929ad8d4eefc4 | Local build, runtime, data or cache evidence |
| evidence/cache-summary.json | bf7d53e5d34bc649f69a3f54ec51aa74f902d09b966fc1544afd3b5a39b0e740 | Local build, runtime, data or cache evidence |
| evidence/docker-build.log | 08b99e75acd42ed362f2baf80cf11046372211cdbf7a47ab491fb98da25f03d0 | Local build, runtime, data or cache evidence |
| evidence/migration-after.json | 4ba7530c0314e539d89ab830839a1a7fdaa8262de77be90b739a40b378a7dd23 | Local build, runtime, data or cache evidence |
| evidence/migration-preflight.json | 8b378518fa98216b509e96ce57df29387b5dae52614fb549fc6c41b477a9ca0a | Local build, runtime, data or cache evidence |
| evidence/runtime-verification.json | d1908626ca68688f0cee241e34c4d3a0713ce84b86a212c4b34f735d423a4853 | Local build, runtime, data or cache evidence |
| evidence/startup-summary.json | 5c0c1c217257ec9b8feea49a7e69c9f4c5e4b86842c29d75f393997e9c94888e | Local build, runtime, data or cache evidence |

## Unverified items

未执行模型付费请求、登录后写操作、生产发布或人工验收。浏览器执行了定向页面强制刷新，没有删除跨站缓存、Cookie 或登录资料。HMR 服务的开发依赖缓存不属于本次 Docker 构建，保持原样。

## Conclusion

本地安装及可控验证完成。AC-01、AC-02 通过；人工清单仍为待审阅，未产生人工 PASS 或生产发布结论。

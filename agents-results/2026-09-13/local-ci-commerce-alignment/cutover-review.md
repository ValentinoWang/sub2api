# APP 镜像替换独立审查

审查日期：2026-09-13。审查对象：`8365f45ea179b99e627cfbbf751b604a45ad2eee`，技术版本 `0.2.4.1`。指定四文件与当前候选提交无差异。范围为只读源码审查与本记录；未编译、运行测试、读取凭据或连接生产，未确认实际运行状态。

## 结论

未发现必须阻断本次 APP 镜像替换的新增源码问题。完成当前提交本地 CI、一次构建、双端镜像身份核对、实际迁移读回及配置保持后，才可记录镜像替换成功。正式交易和商户登录仍未验证，销售及人工验收保持未完成。239/240 隔离副本验证是主任务提供的前提，本审查未重复执行。

## 必要核对与具体风险

1. **一次构建，同一镜像。** `deploy/build_image.sh` 从指定提交 `git archive` 构建，固定 `linux/amd64`，完整提交和版本写入 OCI 标签；未提交文件不会进入镜像。候选标签为 `sub2api-local:0.2.4.1-8365f45ea179b`。记录首次构建 image ID、平台、标签；将该镜像 save/load 或按不可变 digest 分发后，核对两端运行容器 `.Image` 与初次记录完全一致。仅相同版本、tag 或 Git commit 不足。Dockerfile 的基础镜像标签可漂移，远端重新构建即使相同提交也可能产生不同产物。
2. **APP 保留配置，数据库各自独立。** 复用各环境现有 APP mounts、网络、端口、运行参数及配置来源，只替换镜像；不要套用新 compose 模板默认值覆盖现有服务。保留 `/app/data`、数据库/Redis 服务及其卷，切换前后比较既有配置来源的私有摘要或布尔一致性，避免打印整份 `docker inspect` 的 Env。数据库身份两端必须不同且每端前后稳定。保留加密/JWT/TOTP/兑换码配置，不复制一个环境的密钥或数据库覆盖另一个环境。入口脚本会递归尝试调整 `/app/data` 所有权，需留意挂载只读文件及目录可写性。
3. **迁移前置验证不等于线上已经执行。** 239 新增退款保留表及保护触发器；240 新增兼容列并安装 DeepMath 策略触发器，安装本身不回填账号，后续账户或分组关联更新会按策略改变账号字段。实际启动后核对完整当前迁移文件与 checksum、无未知额外历史、两端逻辑结构一致。不要删除迁移历史或回退 SQL；APP 回滚也不会自动撤销这些数据库变化。若真实读回与隔离副本结论不同，应停止完成声明并调查。
4. **artifact PASS 不证明 ¥5 到账 $5 商品。** `artifact` 比较镜像/版本/提交/结构/迁移/业务规则及数据库隔离，但不要求商品列表非空，不强制每个商品 CNY=USD，也不要求包含 ¥5->$5；`native_balance_multiplier` 只要求正数且两端一致。必须额外读回实际商品面额与到账值，要求 ¥5 商品对应 `usd_credit == "5"`，其余充值面额按任务要求逐项核对。仅 `goods_to_credit_ratio == "1"` 不足以证明实际商品配置。
5. **保留总开关可能恢复后台业务。** `ProvideLiandongRestockService` 会读取持久化配置并启动 worker，原 `restock_enabled=true` 在重启后仍有业务副作用。切换前后记录 `purchase_enabled`、`restock_enabled`、商品启用状态与待对账标志，并按主任务已有授权和运行策略处理；不因镜像替换自动启用新开关。未完成真实业务记录时不可调用 `--enable-sales`；该选项会启用两端补货及生产购买。
6. **toolkit 资产和已安装副本需独立验证。** 镜像包含新版本及 hash manifest，现有 `/app/data` 里的已安装 toolkit 可能仍是旧版本；显式环境配置的版本/hash 也可能与新 manifest 不符。`/health` 与 commerce artifact 门禁均不检查 toolkit Ready。通过现有管理员脱敏状态界面/API读取资产、安装副本、manifest 校验及可写性状态，发现不匹配时按获授权的独立操作处理，不能用健康通过推断商户登录或上传可用。
7. **最后清理应保护运行及回退材料。** 构建脚本只自动删除临时源码上下文，不清理 BuildKit 缓存。清缓存应在双端读回完成后限定到已授权缓存；不删数据库/Redis 卷、`/app/data`、现用镜像或未完成恢复判断所需镜像/证据。维护公告及恢复记录由主任务按既定发布流程执行。

## 可执行只读检查

以下变量应由操作者从实际服务配置确认；命令不输出 Env 或凭据。在本地和生产分别执行镜像/容器检查，生产命令由已授权主任务通过现有连接执行。

```bash
git ls-remote fork refs/heads/dev refs/heads/main

docker inspect --type container --format '{{.Image}} {{.State.Status}} {{if .State.Health}}{{.State.Health.Status}}{{end}}' "$APP_CONTAINER"
docker image inspect "$IMAGE_ID" --format '{{.Id}} {{.Os}}/{{.Architecture}} {{index .Config.Labels "org.opencontainers.image.version"}} {{index .Config.Labels "org.opencontainers.image.revision"}}'
docker inspect --type container --format '{{json .Mounts}} {{json .HostConfig.PortBindings}} {{.HostConfig.NetworkMode}}' "$APP_CONTAINER"
docker exec "$APP_CONTAINER" /app/sub2api --version
curl --fail --silent --show-error http://127.0.0.1:8080/health
```

确认 fork/dev 和 fork/main 均为候选完整 SHA；记录运行容器 image ID 相同、平台 `linux/amd64`、版本 `0.2.4.1`、完整 revision 对应候选。健康只证明 APP 存活。配置中的 API URL 还须确实路由到被检查 APP，工具的数据库身份绑定不能区分共享同一库的多个 APP。

在两端替换之后，由持有既有受限配置的主任务运行下列只读门禁；本审查不读取该配置或密钥。首次配置省略旧 `acceptance_file`，以免过期业务记录绑定阻断纯产物核对。

```bash
python3 tools/quality/commerce_release.py \
  --config /absolute/private/commerce-environments.json \
  --stage artifact \
  --output agents-results/2026-09-13/local-ci-commerce-alignment/acceptance/commerce-artifact-after.json
```

检查退出码为 0、`passed=true`、两端 `image_id/source_commit/version/schema_hash/migrations` 一致且数据库身份不同；查看 `collection_audit` 中退役历史无未知项。另读脱敏快照逐项确认 ¥5->$5、其他面额、各环境原开关/商品状态、配置存在标志和预期倍率，记录 toolkit 只读状态。实际配置或镜像变化会使旧销售绑定失效，不能沿用先前成功记录宣告正式交易完成。

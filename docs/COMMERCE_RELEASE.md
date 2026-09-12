# 本地与生产充值发布检查

`tools/quality/commerce_release.py` 在每次调用时重新采集两个环境的运行镜像、迁移记录、数据库结构和有效业务配置。它管理本发布流程；不会拦截管理员从其他入口直接调用 API，也不产生人工验收 PASS。

## 环境配置

配置文件顶层必须恰好包含 `dev`、`prod`。每个环境要求 `url`、`app_container`、`db_container`、`admin_key_file`，可选 `ssh_host`、`ssh_key`、`acceptance_file`，不接受其他字段。

```json
{
  "dev": {
    "url": "http://127.0.0.1:8080",
    "app_container": "local-app",
    "db_container": "local-postgres",
    "admin_key_file": "private/dev-admin.key"
  },
  "prod": {
    "url": "https://production.example.invalid",
    "app_container": "production-app",
    "db_container": "production-postgres",
    "ssh_host": "production-host",
    "ssh_key": "private/production-ssh.key",
    "admin_key_file": "private/prod-admin.key"
  }
}
```

以上是不可直接用于真实环境的示例。将配置放在受限的本地运维目录；相对文件路径以配置所在目录为基准。管理员密钥必须保存在当前用户拥有、权限恰为 `0600` 的普通文件中，不能使用符号链接。工具只在 HTTP 请求头 `x-api-key` 中使用密钥，不将其写入命令行、结果或日志。SSH 使用已有主机信任记录，禁止自动接受未知主机密钥。远端 API 必须使用 HTTPS；HTTP 只接受回环地址；不跟随重定向。

`url` 必须连接被检查应用容器，`db_container` 必须是该应用实际使用的 PostgreSQL。API 返回的数据库身份摘要必须等于容器内 `pg_control_system()` 与 `current_database()` 的组合摘要，否则检查阻塞。该校验绑定数据库，不能证明共享同一数据库的另一个应用容器身份；配置维护者仍须保证 API URL 与应用容器对应。

## 只读检查

在仓库根目录执行，输出放到本任务机器证据目录：

```bash
python3 tools/quality/commerce_release.py \
  --config /absolute/private/commerce-environments.json \
  --stage artifact \
  --output agents-results/YYYY-MM-DD/task/acceptance/commerce-artifact.json
```

`artifact` 要求同版本、完整来源提交、不可变镜像 ID、数据库结构、非空迁移历史和充值规则，并要求数据库隔离。它不要求商户/兑换码凭据已配置或业务验收完成，但调用管理员只读接口仍需要可用管理员密钥。

数据库结构使用 `pg_dump --schema-only --no-owner --no-privileges`，允许两个环境使用不同数据库角色和权限。规范化只删除导出注释、外部空白行和随机 `restrict/unrestrict` 标记，保留函数体和实际结构。原始数据库身份、数据库导出和传输响应不写入证据文件。

结果包含脱敏快照、快照 SHA-256、`acceptance_binding_sha256.dev/prod` 和阻塞原因。结果文件权限为 `0600`。金额与倍率在有效 API 中分别使用整数面额和十进制字符串，拒绝浮点数字形式，避免精度丢失。缺少来源标签、配置无效、读权限不足或接口错误均返回非零退出码。

## 绑定真实业务记录

首先在环境配置中省略 `acceptance_file`，执行只读 `artifact` 检查取得当前绑定摘要。在隔离的测试商品、小库存和正确环境中完成真实检查，保存原始记录，再创建每个环境自己的验收证据 JSON。不能用示例记录或测试夹具替代真实操作。

证据文件必须包含：

- `environment`：`dev` 或 `prod`。
- `binding_sha256`：结果中该环境的当前绑定摘要。它绑定镜像、提交、结构、迁移、数据库、商品 ID、业务规则、兑换码密钥摘要和商品启用状态；不绑定发布过程中的购买/补货总开关。
- `checks`：必须恰好包含 `upload`、`delivery`、`redeem`、`duplicate_redeem`、`refill`、`duplicate_refill`、`inventory_reconciled` 七项。每项必须有 `passed` 布尔值、`evidence_file` 和 `evidence_sha256`。文件路径相对该证据 JSON 所在目录，不能向外跳转；摘要必须对应实际记录字节。

仅当实际结果成功时填写 `passed: true`。每项都验证文件存在且 SHA-256 相符；镜像或配置变化、记录字节改变、缺项或未通过会阻塞。该工具验证记录绑定和提供的布尔结论，不执行人工操作或鉴定签字。发布文档应引用实际记录，人工验收继续遵守项目既有流程。

完成后在配置中设置每个环境的 `acceptance_file`，执行：

```bash
python3 tools/quality/commerce_release.py \
  --config /absolute/private/commerce-environments.json \
  --stage sales \
  --output agents-results/YYYY-MM-DD/task/acceptance/commerce-sales.json
```

`sales` 进一步要求商户/兑换码凭据及有效购买地址已配置，商品 ID 和兑换码密钥隔离，逻辑商品和补货策略一致，所有余额充值按人民币面额与美元到账数值 1:1，包含 ¥5 到账 $5 商品，目标库存和单次补货量均为 1–10，阈值不超过目标库存，无待对账状态，七项记录全部通过。购买地址配置标志只验证 URL 基本有效，实际地址是否导向正确商品仍由真实购买记录证明。两个环境匹配的禁用商品可以完成只读门禁；门禁不会自动修改单个商品状态。

## 执行开关变更

仅在本次任务已明确授权真实开售时，在同一命令追加 `--enable-sales`。工具仍重新采集快照和验证记录，不能通过旧结果文件跳过检查。两个环境的 ¥5 到账 $5 商品必须已经启用，否则不会执行任何写操作。应在补货总开关关闭时配置商品策略，完成小库存检查并绑定该配置，再启用总开关；修改商品启用状态会使旧绑定失效。

通过后，工具依次调用两个环境的 `POST /api/v1/admin/liandong/restock/enable`，再对生产调用 `PUT /api/v1/admin/settings`，请求体仅包含 `purchase_subscription_enabled: true`。每步读取脱敏接口确认实际状态，检查业务配置未漂移；不提交整份设置或其他字段。

在发送写请求前记录原开关，因此请求成功但响应丢失也触发恢复。任何步骤失败时逆序恢复本次尝试变更的开关并读回。恢复失败会明确报告需要人工恢复，不能视为已关闭或已完成。原本已启用的开关保持原值。恢复开关不能撤销其间已经发生的补货、订单或其他业务副作用；仍需按记录核对库存和订单。

本检查不宣称完成生产发布、人类验收、平台官方退款或结算。`unused_only` 只描述系统内未使用兑换码退款规则，不代表外部平台已确认退款或资金到账。

针对性测试：

```bash
python3 -m unittest discover -s tools/quality/tests -p 'test_commerce*.py' -v
```

完整代码提升仍使用项目的本地 CI。

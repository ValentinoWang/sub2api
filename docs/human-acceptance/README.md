# 人类验收清单

> **只读历史入口：本目录已停止接收新任务和新验收记录。** 当前规则与入口见 [Acceptance 目录与文档说明](../../acceptance/README.md)。新任务的机器、视觉、sandbox、生产和发布证据放在 `agents-results/YYYY-MM-DD/<task>/acceptance/`；人工清单、绑定、入队和签署结果放在 `acceptance/human/YYYY-Www/{未-}YYYY-MM-DD-<task-id>/`。没有当前有效人工 `PASS` 时保留 `未-` 前缀。
>
> 以下为原历史索引正文，保留既有清单路径、内容和引用；其中关于本目录用途的描述仅适用于历史记录。历史清单不参与当前人工状态投影，也不能证明新任务已通过验收。不得移动清单或使用 legacy manifest 将本目录声明为 `acceptance/` 内的历史子目录。

这个目录统一存放需要验收负责人实际操作、观察、记录和签字的人类验收文档。

- [会员充值履约](MEMBERSHIP_FULFILLMENT.md)：Codex/GPT Plus、Codex/GPT Pro20x 的客户页面、后台操作、实际权益到账和售后验收。
- [链动小铺销售渠道](LDXP_SALES_CHANNEL.md)：链动小铺购买、收码、Sub2API 兑换、余额到账和防重复兑换验收。
- [Codex 本地旧对话接续](CODEX_SESSION_MIGRATION.md)：从经验页取得工具后，诊断、迁移并重开原旧任务的用户可见结果。
- [上游整合后的人类验收](UPSTREAM_INTEGRATION.md)：统一检查公开页面与主题、登录注册、通道状态、支付说明、代理管理和模型账号探测，并引用既有专项业务清单。

这里不保存构建版本、Git 提交、镜像、迁移、环境变量、自动化测试、部署日志或发布过程。此类技术证据应留在对应开发、运维或受限发布记录中。没有完整的验收记录、验收人和复核人签字时，相关业务只能标记为“待验收”。

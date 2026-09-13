# ERR-005：中转站已有新模型，为什么 Claude Code 看不到？

- 栏目：经验分享 / Claude Code 使用经验
- 适用：通过自定义 Anthropic 兼容网关使用 Claude Code 的用户
- 更新：2026-09-13

## 1. 问题说明

中转站已经提供最新模型，但 Claude Code 的 `/model` 仍只显示内置模型。下面以 Fable 5.1（模型 ID：`claude-fable-5-1`）为例说明。

这不一定是中转站缺少模型。下面四个结果需要分开判断：客户端认识模型 ID；网关目录向当前凭证返回模型；`/model` 启用网关发现；实际模型请求成功。

## 2. 解决方案

先确认实际网关的 `/v1/models` 向自己的凭证返回 `claude-fable-5-1`。如果目录已有模型，但 `/model` 仍不显示，在 Claude Code 用户级 `settings.json` 的 `env` 中加入：

```json
{
  "env": {
    "CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY": "1"
  }
}
```

这不是一份可以覆盖原配置的完整文件。必须保留原有 API 地址、认证和其他设置。关闭旧会话、重新启动 Claude Code，再打开 `/model` 检查 Fable 5.1 是否出现并可选。

### 让 Codex 帮你处理

```text
我通过一个兼容 Anthropic 接口的自定义中转站使用 Claude Code。中转站已经提供最新模型，但 Claude Code 的 /model 列表里看不到。下面以 Fable 5.1（模型 ID：claude-fable-5-1）为例，请在我的电脑上定位并修复客户端模型发现问题。

先确认 Claude Code 实际版本、启动方式和用户级配置文件位置。只检查必要配置；API Key、认证令牌、完整 Base URL 查询参数和认证文件不得输出。保留现有自定义 API 地址、认证、模型、权限、Hooks、MCP 与其他环境变量，不要用示例配置覆盖整个文件。

请把以下四层分开验证：
1. 当前 Claude Code 是否认识 claude-fable-5-1 这个模型 ID；
2. 当前实际连接的中转站 /v1/models 是否向我的凭证返回该模型；
3. Claude Code 是否启用了网关模型发现；
4. 新会话是否在 /model 中显示并能选中 Fable 5.1。

如果前两层通过但模型列表仍缺失，请在用户级 settings.json 的 env 中加入 CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY=1。修改前先建立可恢复备份，只增补该字段，不删除或打印现有敏感值。随后结束旧会话、启动新会话并重新打开 /model 验证；旧进程不一定会热加载环境配置。

默认只验证目录、列表和选中状态，不发送真实模型生成请求。请明确说明“模型可见”不等于“实际请求成功”。若我另行同意做最小请求，只执行一次短请求并报告脱敏状态，不循环重试。

最后分别报告：客户端版本、模型目录是否含 claude-fable-5-1、发现开关是否生效、/model 是否显示、是否实际选中、是否执行过生成请求。若仍未显示，保留客户端错误和脱敏目录响应，判断应继续检查本机版本/配置还是联系中转站管理员。
```

## 3. 原因、验证与注意事项

Claude Code 默认可能只显示自己的内置模型目录。启用 `CLAUDE_CODE_ENABLE_GATEWAY_MODEL_DISCOVERY` 后，客户端才会主动读取自定义接入地址提供的模型列表。旧会话不一定热加载用户级环境设置，因此修改后要用新会话验证。

验收应逐层记录：

- 网关目录包含目标模型，只证明该 ID 已公布。
- `/model` 显示目标模型，只证明客户端发现成功。
- 当前会话明确选中目标模型，证明选择状态正确。
- 实际发送并完成一次请求，才证明生成链路可用。

目录、列表和选择验证不需要发送生成请求。若没有明确授权产生费用，就停在“可见并可选”，不要把它写成“请求成功”。

---

## rest2build

**歇一会儿，让 AI 接着干。**

rest 是你的，build 交给 AI。

rest2build 提供面向 Codex、Claude Code 等工具的 AI 模型接入服务。同时围绕公益 Skills、AI 使用经验分享与 Harness 工程，持续开展内容与实践。

[ai.rest2build.lol](https://ai.rest2build.lol/)

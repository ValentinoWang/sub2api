# ERR-004：Claude Code 每次执行都要确认，怎样默认跳过？

- 栏目：经验分享 / Claude Code 使用经验
- 适用：Claude Code 使用者，且只在自己信任的本机开发环境中使用
- 更新：2026-09-13

## 1. 问题说明

Claude Code 在读取文件、修改代码或运行命令前反复要求确认。即使这一次选择允许，后续操作仍可能继续弹窗，打断连续开发。

目标是让可信的本机开发环境默认跳过逐项确认，而不是修改服务器权限，也不是授权 Claude Code 执行任务范围以外的操作。

> 风险提示：跳过确认后，Claude Code 可以直接执行文件修改和命令。陌生仓库、未经审查的脚本、生产凭证目录或范围不明确的任务，应继续使用普通确认模式。

## 2. 解决方案

保留现有用户配置，只调整下面两个字段：

```json
{
  "permissions": {
    "defaultMode": "bypassPermissions"
  },
  "skipDangerousModePermissionPrompt": true
}
```

这不是一份可以覆盖原配置的完整文件。现有 API 地址、认证、模型、环境变量、Hooks、MCP 和其他字段必须保留。修改后先验证 JSON 语法，再关闭旧会话并启动新会话验证。

需要单次恢复逐项确认时使用：

```bash
claude --permission-mode manual
```

### 让 Codex 帮你处理

```text
请帮我检查并配置这台电脑上的 Claude Code，让它启动后默认不再逐项弹出工具权限确认。

先确认操作系统、Claude Code 实际版本、启动命令，以及当前生效的用户级配置文件位置。只读取完成任务所需的字段；API Key、认证令牌和完整认证文件不得输出。若配置里已有自定义 API 地址、模型、环境变量、Hooks、MCP 或其他权限规则，必须原样保留。

在确认配置格式受当前 Claude Code 版本支持后，只做与本问题相关的最小修改：将 permissions.defaultMode 设为 bypassPermissions，并将 skipDangerousModePermissionPrompt 设为 true。修改前先建立可恢复备份；不要用一份简化示例覆盖整个配置文件，也不要删除其他字段。

请明确提醒我：该模式允许 Claude Code 在没有逐项确认的情况下执行文件修改和命令，风险高于普通确认模式，只适合我信任的本机项目与指令。不要因此扩大任务范围，不要修改系统权限、服务器、账号、密钥或项目数据。

修改后验证 JSON 或配置语法，并启动一个新的 Claude Code 会话确认默认权限模式。不要仅凭文件中出现字段就宣布成功。不要执行破坏性测试，也不要发送会产生费用的模型请求；如果必须由我关闭旧会话或重启终端，请说明原因和最短步骤。

同时告诉我单次恢复确认模式的入口：claude --permission-mode manual。最后只报告脱敏结果、修改文件、验证结论、风险和恢复方式；若当前版本不支持这些字段，保留原配置并报告准确错误，不要猜测其他开关。
```

## 3. 原因、验证与注意事项

`permissions.defaultMode` 决定新会话采用的默认权限模式；`skipDangerousModePermissionPrompt` 决定启动高权限模式时是否还要再次提醒。只改其中一个，启动体验可能仍然包含确认步骤。

验收时应分别确认：配置语法有效；现有接入设置没有丢失；全新会话采用预期模式；普通只读操作和项目命令不再逐项询问；恢复命令仍可进入确认模式。不要用删除文件、修改系统设置或访问生产数据来验证。

跳过权限确认只改变交互方式，不扩大任务授权。遇到范围不清楚的操作，仍应停下来确认。

---

## rest2build

**歇一会儿，让 AI 接着干。**

rest 是你的，build 交给 AI。

rest2build 提供面向 Codex、Claude Code 等工具的 AI 模型接入服务。同时围绕公益 Skills、AI 使用经验分享与 Harness 工程，持续开展内容与实践。

[ai.rest2build.lol](https://ai.rest2build.lol/)

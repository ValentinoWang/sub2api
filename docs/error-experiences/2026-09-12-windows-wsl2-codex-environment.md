# ERR-003 · Windows 11 + WSL2 里 Codex 装好了却不能正常使用，怎么排查？

> 故障排查 · Codex 使用错误说明 · 解决你使用 Codex 或 Claude Code 的最后一公里

更新：2026-09-12（Asia/Shanghai） · 适用：Codex CLI 使用者 · Windows 11 + WSL2

## 情况说明

你已经在 Windows 11 上打开 Ubuntu，也安装过 Codex，但终端仍提示找不到命令，或者同一条命令在 PowerShell 和 WSL 中得到不同结果。还有一种常见表现是：Codex 可以启动，项目却一直很慢、权限异常，或者 Windows 能联网而 WSL 中登录和请求失败。

这些现象往往不是同一个故障。Windows、WSL2 发行版、WSL 内的 Codex、工作目录和网络连接是五个连续环节。先确认故障停在哪一层，比反复重装更有效。

**本文适合这些情况：**

- 在 PowerShell 能看到某个命令，进入 Ubuntu 后却提示 `command not found`。
- 不确定当前窗口到底是 PowerShell、CMD 还是 WSL Shell。
- `codex --version` 有输出，但不知道运行的是 Windows 侧还是 WSL 侧的程序。
- 项目位于 `/mnt/c/Users/...`，出现速度慢、权限、大小写或软链接异常。
- Windows 浏览器能访问本站，WSL 内却出现 DNS、证书、代理或连接超时。

本文只处理本机 Windows、WSL2 与 Codex CLI 环境。它不涉及让 Codex 编写前端页面，也不要求普通使用者登录、重启或修改中转站服务器。

## Codex 帮你处理

把下面的完整提示词发给能操作你这台电脑的 Codex。它会先标明命令应该在哪个终端执行，再从只读检查开始。

```text
我在 Windows 11 + WSL2 中使用 Codex CLI，现在遇到以下一种或多种情况：终端里找不到 codex；运行到的版本或安装位置不对；同一条命令在 PowerShell 和 Ubuntu 中结果不同；项目位于 /mnt/c 后速度慢、权限或软链接异常；Windows 能访问网络，但 WSL 中 Codex 登录或请求失败。我的目标是先判断问题属于 Windows、WSL2、Codex 安装、工作目录还是网络配置，再做最小修复，让 Codex 能从 WSL 中稳定启动并完成一次最小交互。

请把我当作普通使用者。我可以操作自己的 Windows、WSL 发行版和 Codex 客户端，但没有中转站服务器、Docker 或管理员后台权限。不要要求我发送完整 API Key、认证文件或私人对话；命令输出中如果出现用户名、目录、令牌或代理凭证，请先提醒我脱敏。

请先只读诊断，并明确告诉我每条命令应该在“Windows PowerShell”还是“WSL 的 Ubuntu Shell”中执行，不要把两种终端的命令混在同一个代码块里。

第一步，在 Windows PowerShell 中检查 WSL2 本身：
- 运行 wsl --status 和 wsl -l -v，确认目标发行版存在、状态正常且 VERSION 为 2；
- 如果 WSL 未安装、发行版无法启动或版本不是 2，先说明现状和影响，再给出微软当前官方文档对应的最小处理步骤；
- 不要删除、注销或重装发行版，不要执行 wsl --unregister，除非我另行明确授权并已有可验证备份。

第二步，在 WSL 的 Ubuntu Shell 中确认当前环境：
- 检查 echo $WSL_DISTRO_NAME、pwd、uname -a 和 printf '%s\n' "$HOME"；
- 判断我是否误在 PowerShell/CMD 中执行 Linux 命令，或者虽然打开了 WSL，却仍在 /mnt/c/... 的 Windows 挂载目录工作；
- 如果工作目录在 /mnt/c，只说明它可能带来的文件 I/O、权限、大小写和软链接差异。先检查仓库是否有未提交内容和大文件，再给出迁移到 ~/code/... 的安全计划，不直接移动、覆盖或删除原目录。

第三步，确认 WSL 实际运行哪一个 Codex：
- 运行 command -v codex、type -a codex 和 codex --version，并检查解析出的路径属于 WSL 还是 Windows 挂载路径；
- 只在确认 WSL 中缺少 Codex 或当前安装损坏后，才参考 Codex 当前官方安装说明进行安装或修复；不要同时混用多个安装方式，也不要把 Windows 侧可执行文件手工复制进 WSL；
- 保留现有 Codex 配置、登录状态、memories、任务历史、skills、MCP 和项目 trust。不要通过删除整个 ~/.codex 来解决 command not found 或网络问题。

第四步，核对配置和网络边界：
- 确认当前进程读取的是 WSL 用户目录下的配置，不把 Windows 与 WSL 的 HOME、PATH、代理变量或认证状态当作自动共享；
- 只显示 Base URL 的协议、主机和必要路径，Key 只报告“已配置/未配置”，不要打印值；
- 分别检查 DNS、TLS、代理变量和目标入口的可达性。Windows 浏览器可访问不等于 WSL 一定继承相同代理；收到 401/403 说明已经到达某个 HTTP 服务，但仍需核对地址、认证和权限；超时、DNS 或证书错误应保留脱敏错误正文；
- 不要登录、重启或修改中转站服务器，不要更改账号、密钥、上游路由或计费配置。任何可能产生费用的模型请求先征得我同意，只做一次简短验证，不循环重试。

定位后，只执行与根因直接相关、可恢复的本机修复。修复完成后请分别报告：
1. WSL 发行版和版本是否正确；
2. 当前 Shell、HOME 和工作目录属于哪一侧；
3. command -v codex 的实际路径与版本；
4. 配置和网络检查通过到哪一层；
5. Codex 是否能从 WSL 启动，以及最小交互是否实际完成；
6. 哪些项目没有验证，是否需要我操作或联系站点管理员。

不要把“安装命令执行完”“路径里出现 codex”或“HTTP 有响应”单独当作恢复成功。遇到缺少权限、工具或信息时，说明具体阻塞和下一步，不虚构已修复。
```

## 给人看的：原因、证据与经验

### 先把问题放回正确的一层

| 检查层 | 它决定什么 | 失败时常见表现 |
| --- | --- | --- |
| Windows 与 WSL2 | 发行版能否启动，是否实际运行 WSL2 | Ubuntu 打不开、发行版不存在、版本仍为 1 |
| 当前 Shell | 命令由 PowerShell 还是 Linux Shell 解释 | `$HOME`、路径和安装命令行为不一致 |
| Codex 可执行文件 | 当前命令实际运行哪一份 Codex | 找不到命令、版本不对、调用 Windows 挂载路径 |
| HOME 与工作目录 | 配置从哪里读取，项目使用哪套文件语义 | 登录状态不同、配置不生效、权限或软链接异常 |
| WSL 网络 | DNS、TLS、代理和 API 入口能否连通 | Windows 可访问，WSL 中超时或认证失败 |

### PowerShell 和 WSL Shell 各自检查什么？

在 Windows PowerShell 中运行：

```powershell
wsl --status
wsl -l -v
```

目标发行版应存在，并在 `VERSION` 列显示 `2`。如果不是 2，先查当前微软文档和机器条件，不要删除或注销发行版。

在 WSL 的 Ubuntu Shell 中运行：

```bash
echo "$WSL_DISTRO_NAME"
pwd
uname -a
printf '%s\n' "$HOME"
```

应看到发行版名称、Linux 内核和 WSL 用户的 HOME。若命令语法在当前窗口无法解释，先确认是否开错终端。

### “安装过 Codex”不等于 WSL 正在运行它

Windows 和 WSL 有各自的用户目录与 `PATH`。即使 Windows 侧已经安装，WSL 也不一定拥有同一份命令；反过来，WSL 可能通过互操作解析到 Windows 挂载目录里的可执行文件。

在 WSL 中检查：

```bash
command -v codex
type -a codex
codex --version
```

`command -v` 给出本次会运行的路径，`type -a` 列出当前 Shell 能找到的候选。先确认路径和版本，再决定是否安装或修复。为了解决一个 `command not found` 就删除整个 `~/.codex`，会把配置、任务历史和其他本地状态一起置于风险中。

### 为什么工作目录在 `/mnt/c` 时更容易出问题？

`/mnt/c` 是 WSL 对 Windows 文件系统的挂载入口。它可以使用，但文件 I/O、权限位、大小写、文件监听和软链接的行为可能与 WSL 自己的 Linux 文件系统不同。遇到依赖安装慢、权限反复变化或软链接失败时，工作目录是需要核对的证据。

新的仓库通常放在 `~/code/项目名` 更省事。已有仓库不能直接搬走：先确认未提交修改、未跟踪文件和大文件，再制定复制、校验和回退步骤。原目录验证完成前不要删除。

### Windows 能联网，为什么 WSL 仍可能失败？

WSL 有自己的进程环境。Windows 浏览器使用的代理、证书或登录状态不一定自动传给 Ubuntu Shell。排查时要把 DNS、TLS、代理和认证分开看。

| 结果 | 说明了什么 | 下一步 |
| --- | --- | --- |
| DNS 解析失败 | 还没有到达 HTTP 服务 | 检查 WSL DNS、VPN 和网络策略 |
| TLS 或证书错误 | 已开始建立安全连接，但信任链或代理可能有问题 | 保留脱敏错误，核对系统时间、代理和证书来源 |
| 连接超时 | 可能是路由、代理、防火墙或服务不可达 | 记录时间、目标主机和是否持续复现 |
| 401 / 403 | 到达了某个 HTTP 服务，但认证或权限未通过 | 核对实际 Base URL 与 Key 状态，不公开 Key |
| 请求成功 | 这一次最小链路可用 | 仍需确认 Codex 实际使用同一配置完成交互 |

网络探针成功不等于 Codex 已经选中正确配置；Codex 能启动也不等于付费模型请求已完成。可能产生费用的验证应先说明并只做一次。

### 建议的处理顺序

1. 在 PowerShell 查看 WSL 发行版、状态和版本。
2. 在 Ubuntu 中查看发行版名称、内核、HOME 和工作目录。
3. 读取 Codex 的实际命令路径和版本，判断它属于 Windows 还是 WSL。
4. 确认项目位置和 WSL 用户目录中的配置，不用另一侧的结果代替。
5. 分开确认可达性、认证和一次真实 Codex 交互。

### 怎样才算恢复？

- **环境明确：**目标发行版运行在 WSL2，当前窗口能识别为该发行版的 Linux Shell。
- **命令明确：**`command -v codex` 指向预期的 WSL 安装位置，版本可读取。
- **目录明确：**知道当前 HOME 和工作目录位于哪一侧，不再混用 Windows 与 Linux 路径。
- **连接明确：**DNS、TLS、代理、认证分别检查，错误停在哪一层有可复述的证据。
- **行为恢复：**Codex 能从 WSL 启动，并在获得费用授权后完成一次简短交互。

联系站点管理员时，提供 Windows 版本、发行版名称与 WSL 版本、Codex 路径与版本、发生时间和时区、脱敏错误正文及请求标识。不要发送完整 API Key、认证文件、整份配置目录或私人对话。

本文从 [Windows 11 + WSL2 使用 Codex：第一次真实前端开发](../../frontend/src/views/public/WslCodexFrontendTutorialView.vue) 中提炼环境排障部分，并补齐诊断边界。页面结构、分类、提示词复制和响应式布局可以在本地验证；真实 Windows 11 / WSL2 命令执行与初学者理解仍需人工实操，本文不把它们描述为已验证。

参考：[Codex WSL 官方文档](https://learn.chatgpt.com/docs/windows/wsl)。

---

rest2build

歇一会儿，让 AI 接着干。

rest 是你的，build 交给 AI。

rest2build 提供面向 Codex、Claude Code 等工具的 AI 模型接入与账号充值服务。公益 Skills 帮你完成配置与重复操作，AI 使用经验帮助定位和恢复问题，Harness 工程将任务拆解、复核与验收融入团队开发流程。

[ai.rest2build.lol](https://ai.rest2build.lol/)

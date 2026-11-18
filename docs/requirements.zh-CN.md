# Owlsort CLI 需求文档（中文）

- 2026-11-18 起草。参考 [Mole](https://github.com/tw93/Mole)「开源 CLI + 付费桌面端」的模式，CLI 独立实现、不要求与桌面端对齐。
- 状态：规划阶段，代码为骨架。英文的工程文档见本目录其他文件：[architecture.md](architecture.md)、[safety-model.md](safety-model.md)、[config.md](config.md)、[commands.md](commands.md)、[roadmap.md](roadmap.md)、[decisions.md](decisions.md)。

## 1. 定位

`owlsort`：一个开源、免费的命令行文件整理工具，在 macOS、Linux、Windows 上把下载、桌面这类文件夹里堆积的零散文件按规则归类。

- **它是一个独立的产品**，不是桌面版的命令行外壳。代码与桌面版完全分开，不共用实现，**不要求和桌面端功能对齐**，桌面端的改动也不必同步到 CLI。
- **保留 Owlsort 的招牌**：先预览再动手、每次整理都能撤销、只动零散文件不动用户自己的文件夹。这是它和同类工具（包括 Mole 的永久删除式清理）最大的区别。
- **和付费桌面版的关系**：CLI 面向习惯用终端、想写脚本自动化的人；桌面版提供图形界面、预览动画、桌面机器人等。CLI 的 README 介绍桌面版，作为升级方向，但 CLI 本身不推销、不弹提示。

## 2. 目标与不做

### 目标

1. 一个单文件可执行程序，不需要装运行时，三个平台都能用。
2. 开箱即用：`owlsort init` 之后，内置规则就能整理下载和桌面。
3. 规则写在一个人能直接读写的配置文件里，适合放进 dotfiles。
4. 安全：默认只预览；动手前确认；每一批都能撤销；绝不永久删除用户文件（见 §6）。
5. 适合脚本：所有命令可以输出 JSON，退出码有明确含义，非交互环境下不会悄悄动文件。
6. 不联网：没有遥测、没有许可证、没有自动更新检查。

### 不做

- 图形界面、托盘、通知、动画、桌面机器人。
- 与桌面版的功能对齐、数据互通（读取桌面版的规则或历史）。能导入桌面版导出的规则文件列为「以后再看」（§12）。
- 脚本动作、AppleScript、快捷指令等会执行任意代码的动作。
- 永久删除。最危险的动作只到「放入回收站」。
- 内容搜索（按文件里的文字匹配）、重复文件检测：首版不做。
- AI 相关的一切。

## 3. 用户与典型用法

| 用户 | 用法 |
|---|---|
| 开发者、终端用户 | 偶尔 `owlsort preview` 看一眼，`owlsort tidy` 整理，后悔了 `owlsort undo` |
| 想自动化的人 | `owlsort service install`，让它在后台照看下载文件夹；或在 cron、launchd 里定时 `owlsort tidy --yes` |
| 服务器、NAS 用户 | 在 Linux 上整理一个上传目录、下载目录，规则文件进版本库 |

## 4. 架构与技术栈

### 4.1 独立仓库、独立实现

- 新建公开仓库（名字待定，§14），不依赖桌面版的任何代码。
- **为什么不共用桌面版的引擎**：不要求对齐，共用就只剩坏处。要改造 Windows 桌面版正在用的核心库（路径规则写死为 Windows）；核心库公开后要考虑许可证和贡献者协议；桌面端每次改动都可能牵动 CLI。独立实现后，两边互不影响，社区的提交也能直接合并。

### 4.2 技术栈：Go

| 部分 | 选择 | 理由 |
|---|---|---|
| 语言 | Go | 一台机器即可交叉编译出三个平台的静态单文件；启动快、体积小；Mole 也用 Go |
| 命令行框架 | spf13/cobra | Go 生态最常用；子命令、帮助、shell 补全自带 |
| 配置文件 | TOML（BurntSushi/toml） | 比 YAML 少缩进陷阱，适合手写规则；注释友好 |
| 终端输出 | 自己写简单的表格与颜色（必要时用 charmbracelet/lipgloss） | 依赖尽量少 |
| 文件监听 | fsnotify | macOS 用 kqueue、Linux 用 inotify、Windows 用 ReadDirectoryChangesW；只监听最外层文件夹，够用 |
| 操作记录 | 追加写入的 JSON Lines 文件 | 不需要数据库，也就不需要 cgo，交叉编译不受影响；人可以直接查看 |
| 系统调用 | golang.org/x/sys | Windows 回收站（Shell API）、扩展属性等，纯 Go 调用，不用 cgo |
| 构建发布 | GoReleaser | 一次产出所有平台的包、校验和、Homebrew 与 Scoop 清单；支持 macOS 签名公证 |
| CI | GitHub Actions（公开仓库免费） | 三个平台跑测试 |

**原则：不使用 cgo。** 这样才能在一台机器上编出所有平台的包。需要系统能力的地方用纯 Go 调用或系统自带命令实现。

## 5. 核心概念

- **监控文件夹**：要整理的文件夹，默认下载和桌面。
- **规则**：条件 + 动作。按顺序匹配，默认命中后停止。
- **只整理最外层**：只看监控文件夹最外层的零散文件。子文件夹被视为「用户已经整理好的」，不碰。单条规则可以打开「也整理子文件夹」。
- **保护**：代码项目（有 `.git`、`package.json` 等）、笔记库（Obsidian 的 `.obsidian/`、Logseq）里的文件不动；用户列出的「不整理的文件和文件夹」不动。
- **批次**：一次 `tidy` 是一批，撤销按批次、从最近一批往回。

## 6. 安全模型（必须满足）

1. **默认只预览**：`preview` 不动任何文件；`tidy` 先显示预览，确认后才执行。
2. **非交互环境不动文件**：没有终端时，`tidy` 不带 `--yes` 就只预览，以退出码 4 结束。
3. **不覆盖**：目标位置已有同名文件时，按规则的冲突策略「自动改名」或「跳过」，绝不覆盖。
4. **不永久删除**：动作里没有永久删除；「放入回收站」是最重的动作。
5. **能撤销**：每一步都写入操作记录；`undo` 按记录把文件放回原处。撤销前逐个确认「文件还在目标位置、原位置没被占用」，不满足就跳过并说明。
6. **执行时重新确认**：预览和执行之间文件变了（被删、被改名、还在下载中），执行时跳过并说明。
7. **下载中的文件不动**：`.crdownload`、`.part`、`.download` 等，以及最近几秒还在变化的文件。
8. **规则文件读不了就停**：配置损坏时不退回内置规则去整理，直接报错退出（参考 Mole：白名单读不了就停）。
9. **后台模式更保守**：`watch` 下默认只执行移动、复制、重命名；放入回收站需要在规则里显式允许后台执行。

## 7. 功能范围

### 7.1 首版（v0.1）

| 能力 | 说明 |
|---|---|
| 内置规则 | 截图 → 图片/截图；PDF → 文档/PDF；安装包（dmg、pkg、exe、msi、apk…）→ 安装包；压缩包 → 压缩包；办公文档、其他图片、音视频默认关闭 |
| 条件 | 扩展名、名称包含 / 开头 / 结尾 / 等于 / 正则、文件类别、大小大于 / 小于、修改 / 创建天数、所在路径包含；条件可「全部满足 / 任一满足」，可取反 |
| 动作 | 移动到文件夹、复制到文件夹、按日期或模板重命名、放入回收站 |
| 目标路径变量 | `{date}` `{year}` `{month}` `{day}` `{category}` `{ext}` |
| 只整理最外层、也整理子文件夹 | 见 §5 |
| 保护 | 代码项目、Obsidian / Logseq 笔记库、不整理列表 |
| 冲突策略 | 自动改名（默认）/ 跳过 |
| 预览、整理、撤销、历史 | 见 §8 |
| 解释 | `explain <文件>`：命中哪条规则、哪个条件，或被哪条保护挡住 |
| 没有规则的文件 | `uncovered`：最外层还有哪些文件没有规则覆盖，按扩展名分组 |
| 后台照看 | `watch` 前台运行；`service install` 交给系统常驻 |
| 输出 | 人读的表格；`--json` 给脚本 |
| 语言 | 英文、简体中文（跟随 `LANG`，可用 `--lang` 指定）；命令名和参数名始终英文 |

### 7.2 以后再看

- 导入桌面版导出的规则文件（`owlsort rules import desktop.json`），按能对应的条件和动作转换，对应不上的跳过并列出。
- 重复文件、内容搜索。
- 更多语言。
- macOS 上的 Finder 标签条件与动作。

## 8. 命令

```
owlsort init [--folders ~/Downloads,~/Desktop] [--lang zh-Hans]   生成配置和内置规则
owlsort preview [path...]        会整理什么、去哪里（不动文件）
owlsort tidy [path...] [--yes]   先预览，确认后整理
owlsort undo [--batch <id>]      撤销最近一批（或指定批次）
owlsort history [--limit 20]     每批一行：时间、文件数、去向
owlsort explain <file>           为什么会（不会）被整理
owlsort uncovered [path...]      没有规则覆盖的文件

owlsort rules list | show <name> | enable <name> | disable <name>
owlsort rules edit               用 $EDITOR 打开配置文件，保存后校验
owlsort rules check              检查规则：缺值、正则写错、被前面的规则遮挡、目标逃出监控文件夹

owlsort folders list | add <path> | remove <path>
owlsort keep add <path> | remove <path> | list     不整理的文件和文件夹

owlsort watch [path...]          前台监控，新文件稳定后按规则整理（后台安全策略）
owlsort service install | uninstall | status       launchd / systemd 用户服务 / Windows 计划任务

owlsort doctor                   环境检查：配置能否读取、文件夹是否存在、回收站是否可用
owlsort completion bash|zsh|fish|powershell
```

全局参数：`--json`、`--config <file>`、`--lang`、`--quiet`、`--no-color`（同时遵守 `NO_COLOR`）。

退出码：`0` 成功；`1` 一般错误；`2` 参数或配置错误；`3` 部分完成（有文件被跳过）；`4` 未确认，没有动文件；`5` 没有可撤销的批次。

### 预览输出示例

```
$ owlsort preview
Downloads · 6 files can be tidied
  report-final.pdf          → 文档/PDF/
  Screenshot 2026-11-18…png → 图片/截图/
  Owlsort-0.2.11.dmg        → 安装包/
  photos.zip                → 压缩包/
  …
Skipped: 2 (in a code project: 1, still downloading: 1)
Run `owlsort tidy` to move them. Every run can be undone with `owlsort undo`.
```

## 9. 配置与数据

### 9.1 位置

| | macOS / Linux | Windows |
|---|---|---|
| 配置 | `$XDG_CONFIG_HOME/owlsort/config.toml`（默认 `~/.config/owlsort/`） | `%APPDATA%\owlsort\config.toml` |
| 操作记录 | `$XDG_STATE_HOME/owlsort/journal.jsonl`（默认 `~/.local/state/owlsort/`） | `%LOCALAPPDATA%\owlsort\journal.jsonl` |

与桌面版的数据目录完全分开，互不读取。

### 9.2 配置文件示例

```toml
folders = ["~/Downloads", "~/Desktop"]
keep = ["~/Downloads/Taxes"]
conflict = "rename"   # rename | skip

[[rule]]
name = "Screenshots"
match = "all"
when = [
  { category = "image" },
  { name_contains = ["Screenshot", "截屏", "CleanShot", "Snipaste"] },
]
move_to = "图片/截图"

[[rule]]
name = "Desktop by date"
enabled = false
when = [{ in_folder = "~/Desktop" }]
move_to = "Archive/{date}/{category}"
```

### 9.3 操作记录

每一步一行 JSON：批次号、时间、动作、原路径、新路径、规则名、结果。只追加，不改写；撤销也写成新的一行。文件里有用户的文件路径，README 要写明。

## 10. 平台差异

| 能力 | macOS | Linux | Windows |
|---|---|---|---|
| 放入回收站 | 移到 `~/.Trash`（重名时自动改名）。系统的「放回原处」不可用，撤销靠操作记录 | freedesktop 回收站规范（`~/.local/share/Trash` 的 `files` + `info`），桌面环境的「还原」可用 | Shell 回收站接口（`SHFileOperationW`），可在资源管理器里还原 |
| 跳过的文件 | `.DS_Store`、iCloud 未下载的占位（`.icloud`）、`.app` 等包当作一个文件 | 隐藏文件 | 隐藏和系统文件；OneDrive 仅在线的文件不读 |
| 常驻 | launchd 用户代理 | systemd 用户服务 | 计划任务（登录时运行） |
| 路径大小写 | 按卷判断（APFS 默认不分大小写） | 分大小写 | 不分大小写 |

## 11. 开源与品牌

- **许可证：GPL-3.0**。别人可以用、改、分发，但改过的版本也必须开源，不能拿去做闭源产品。
- **商标：单独一份 `TRADEMARK.md`**。代码开源，「Owlsort」的名字和图标保留：分支必须改名换图标，不能暗示是官方版本，不能用 Owlsort 的名字卖付费或竞争产品（参考 Mole 的写法）。
- **贡献**：直接接受社区提交；`CONTRIBUTING.md` 写明「整理只在临时目录里测试」等约定。
- **安全问题**：`SECURITY.md` 写明私下报告的方式。

## 12. 与桌面版的关系

- **代码**：完全独立，互不依赖。
- **功能**：不要求对齐。桌面版的新功能不必进 CLI；CLI 也可以有桌面版没有的东西（比如规则文件、JSON 输出）。
- **记录**：CLI 不跟随桌面版。若某个安全问题两边都有（比如一种该保护却没保护的目录），各自修复，在各自的仓库里记录。
- **同时使用**：用户可能同时装了桌面版和 CLI。`watch` 启动时，若检测到桌面版正在后台整理同一个文件夹，提示并默认不启动（`--force` 可继续）。两边各自撤销自己的操作，靠 §6 第 5 条的逐个确认避免冲突。
- **引导**：README 有一节介绍桌面版（图形界面、预览动画、桌面机器人）和下载地址；命令行输出里不放广告。

## 13. 测试、发布与里程碑

### 测试

- 规则匹配、路径变量、冲突改名、保护判断：纯函数单元测试。
- 整理、撤销、回收站：在临时目录里真实执行，三平台 CI 都跑。
- 命令输出（含 `--json`）与退出码：快照测试。
- **整理只在临时目录里测**，CI 和本地都一样，绝不碰真实的下载、桌面。

### 发布

- 版本号独立于桌面版，从 `v0.1.0` 开始，遵循语义化版本。
- GitHub Releases：macOS（arm64、x64）、Linux（x64、arm64）、Windows（x64、arm64）单文件与校验和。
- macOS 二进制需要 Developer ID 签名并公证，否则从浏览器下载会被系统拦截。
- Homebrew：先用自己的 tap；仓库有一定热度后申请进 Homebrew 官方仓库。
- Windows：Scoop、winget。
- 一行安装脚本（`curl … | sh`、PowerShell 的 `iwr … | iex`），支持指定版本和安装目录。

### 里程碑

| | 内容 | 验收 |
|---|---|---|
| M0 | 仓库、GoReleaser、CI；配置文件读写与校验；扫描（只看最外层、跳过规则）；`init`、`preview`、`explain` | 三平台临时目录里 `preview` 结果正确 |
| M1 | `tidy`、`undo`、`history`；操作记录；三平台回收站；冲突策略；执行时重新确认 | 整理、撤销、重开终端后历史仍在，三平台都跑通 |
| M2 | `rules`、`folders`、`keep`、`uncovered`、`rules check`、`doctor`、`completion` | 规则检查能指出缺值、正则错误、遮挡 |
| M3 | `watch`、`service`、与桌面版共存检测 | 放进新文件后自动整理；回收站动作需显式允许 |
| M4 | 签名公证、Homebrew tap、Scoop、winget、安装脚本、README（中英）、TRADEMARK、CONTRIBUTING、SECURITY | 新机器上一条命令装好，`owlsort init && owlsort preview` 可用 |

## 14. 待决定（也见 [decisions.md](decisions.md)）

1. **仓库名与命令名**：`owlsort` 作为命令名是否和 Homebrew、Scoop 里已有的软件冲突，上线前查；仓库名用 `owlsort-cli` 还是 `owlsort`。
2. **配置格式**：本文选 TOML，你若更习惯 YAML 也可以。
3. **首版语言**：英文 + 简体中文，还是只做英文。
4. **是否放在你的个人 GitHub 账号下**（与 Mole 放在作者账号下一样），还是建一个组织。

## 附：曾经考虑过的方案

最初的方案是「共用桌面版的核心库、功能逐项对齐」。放弃的原因：不要求 CLI 与桌面端对齐；共用代码要改造桌面版正在使用的引擎、要处理开源许可证与闭源桌面版的关系（贡献者协议），而对齐的要求一取消，共用的好处就不存在了。

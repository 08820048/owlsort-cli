# Owlsort CLI

**整理下载和桌面里散落的文件：先预览，随时能撤销。**

[English](README.md)

> **状态：早期阶段。** 整理在 macOS 上已经跑通全流程：`init`、`preview`、`explain`、
> `tidy`、`undo`、`history`。Linux 和 Windows 能编译、CI 里跑同一套测试，但还没有人工试过；
> `rules check`、`uncovered`、`doctor` 能读配置、告诉你哪里有问题；改配置还是直接编辑文件（M2）。
> 先在你有备份的文件夹上试。想参与的话，从 [docs/roadmap.md](docs/roadmap.md) 开始看。

`owlsort` 按你能读懂、能编辑的规则，把堆在文件夹里的截图、PDF、安装包、压缩包移到整齐的地方。
它围绕一个想法：**动手之前先看到结果，任何一次整理都能撤销。**

```
$ owlsort preview
Downloads · 6 files can be tidied
  report-final.pdf           → 文档/PDF/
  Screenshot 2026-11-18….png → 图片/截图/
  Owlsort-0.2.11.dmg         → 安装包/
  photos.zip                 → 压缩包/
Skipped: 2 (in a code project: 1, still downloading: 1)

$ owlsort tidy          # 显示同样的预览，询问，再移动
$ owlsort undo          # 把上一次放回去
```

*（计划中的输出，见 [docs/commands.md](docs/commands.md)。）*

## 安装

```sh
curl -fsSL https://raw.githubusercontent.com/08820048/owlsort-cli/main/install.sh | sh
owlsort init && owlsort preview
```

安装脚本会用发行版的 `checksums.txt` 校验下载，对不上就什么都不装。或者自己编译，
只需要 Go：

```sh
go install github.com/08820048/owlsort-cli/cmd/owlsort@latest
```

macOS 的二进制用 Developer ID 签名并经过公证，下载后可以直接运行。
详见 [docs/install.md](docs/install.md)。

## 原则

- **先预览**：`preview` 不动文件；`tidy` 动手前先问。没有终端时只预览，所以脚本不会意外移动文件。
- **随时撤销**：每一步都有记录，`undo` 会放回去，并且先检查，绝不覆盖任何东西。
- **从不永久删除**：没有删除动作，最重的是放入系统回收站。
- **从不覆盖**：重名时两个都保留，或者跳过。
- **只整理零散文件**：只处理监控文件夹的最外层。你自己的文件夹、代码项目、笔记库都不碰。
- **不联网**：没有遥测，没有更新检查，没有账号。
- **配置是纯文本**：一个 TOML 文件，能读、能改、能放进 dotfiles。

完整清单和对应的测试见 [docs/safety-model.md](docs/safety-model.md)。

## 文档

完整的中文需求文档：[docs/requirements.zh-CN.md](docs/requirements.zh-CN.md)。
英文的工程文档（架构、安全模型、配置、命令、路线图）见 [README.md](README.md) 里的列表。

## 参与

请先读 [CONTRIBUTING.md](CONTRIBUTING.md) 和 [AGENTS.md](AGENTS.md)。简单说：测试只动临时目录，不用 cgo，
不联网，安全模型优先于任何功能。

## Owlsort 桌面版

Owlsort 另有 Mac 和 Windows 的桌面版，有图形界面、预览动画和桌面机器人，是 [owlsort.com](https://owlsort.com)
上的付费产品。这个命令行工具和它们互相独立：免费、开源，有自己的代码、自己的数据和自己的版本号。

## 许可证

代码使用 [GNU GPL v3.0](LICENSE)。「Owlsort」的名字和图标不在这份许可证范围内，见 [TRADEMARK.md](TRADEMARK.md)。

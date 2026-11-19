# Meow-AI-Arean社区AI比赛

一个面向社区的 1v1 AI Bot 算法对抗赛项目。Bot 在程序生成的沙漠战场中争夺 VP、守护母舰；仓库提供本地运行的 Go 裁判与引擎、参赛模板、参考 Bot、赛事配置和赛季榜单。

> 这是源码和赛事资料仓库，不是在线报名平台或托管式对战服务。项目目前处于 β 测试阶段。

本项目采用 [MIT License](LICENSE) 开源。

## 项目现状

- 正式赛执行链支持编译 Bot、封存源码哈希、冻结赛程、运行对局和生成评分。
- 仓库包含 Go 原生规则引擎及历史 JavaScript oracle；当前赛季（第 5 季）配置使用 Go 原生引擎。
- 当前规则版本为 **v1.4**：跳过式生产队列、母舰 RU 基础收入、四子系统分配、CU/RU 双资源运输线、资源成对同类、脱战自愈、母舰削弱（HP 1800 / 攻击 13 / 移速 0.30 / 坡限 4），1200 tick、七领域确定性计分。
- 第 4、5 季基准赛均为 6 名选手、45 场对阵、90 局，零故障、90/90 局通过确定性复核，见[赛季榜单](war_of_desert/LEADERBOARD.md)。
- 正式赛逐局 JSON、赛程和回放属于本地生成的赛事产物，并未全部包含在当前 Git 推送内容中。
- `Dev/` 当前推送的是规则规格、计划、调研与归档文档；可运行的正式赛代码位于 `war_of_desert/`。

## 功能

- 1v1 Bot 对战，程序生成地图，固定 seed 便于复现。
- Bot 以独立进程运行，通过标准输入/输出的 JSON 行协议与裁判通信。
- 裁判执行决策时限、故障判罚、换边对局和确定性复核。
- 赛后可生成 Bradley–Terry 实力榜与独立的加权总分榜。
- 提供 Go 原生引擎、冻结的 JavaScript oracle、golden trace 回归测试及参考 Bot。

## 目录

```text
社区比赛__WoD/
├── 裁判台.bat            ← 双击打开 GUI 裁判台
├── referee_desk.py       ← 裁判台本体（tkinter，标准库）
├── 赛务.py               ← 发卷 / 开赛 / 暂停 / 收卷（也可单独命令行用）
├── 算法赛/               ← 比赛工作区，每个子目录是一名选手
│   └── RTS算法赛@<模型名>/
└── war_of_desert/        引擎与执行链（Go 模块）
```

## 裁判台

双击 `裁判台.bat`。两个页签：

- **赛季**：选 `war_of_desert/tournament/*.json`，按 自检 → 编译封存 → 冻结赛程 → 开赛 → 评分 走完；
  右上角封存灯实时反映 `arena seal` 的结果。
- **工作区**：指定比赛根目录（默认 `算法赛/`），扫描 `RTS算法赛@<模型名>` 这类目录。
  - **赛务**：选中一个目录点「发卷并开赛」——即使目录是空的，也会把选手包铺进
    `<工作区>/workspace/`（RULES.md、botkit、模板 decide.go、clock.py），并开始计时
    （90 分钟目标 / 120 分钟强制收卷）。随时可「暂停 / 恢复 / 收卷」，收卷把
    `workspace/submission/decide.go` 冻结到 `交卷/` 并记 SHA-256 与用时。
  - **评测**：勾选任意几个（不必全选，双击则只跑那一个）编排一场比赛——选中的选手
    互相对局，并与 5 个参考 Bot 对局，跑完出 BT 榜。有收卷副本时以收卷副本为准。
    工作区会先复制到 `war_of_desert/workspace/_contestants/<名字>/` 再编译，
    评测目录留 `工作区清单.json` 记录来源与源码 SHA-256。

需要带 tkinter 的 Python。若 `python` 是精简发行版，用 `py -3 referee_desk.py` 或直接双击 bat。

## 快速开始（命令行）

需要 Go 1.22 或更高版本。以下命令在仓库的 `war_of_desert/` 目录执行。

```bash
cd war_of_desert

# 运行正式赛源码的 Go 测试
go test ./cmd/... ./internal/... ./workspace/...

# 编译 v1.2 基准赛配置中的 Bot，并封存源码哈希
go run ./cmd/arena build -config tournament/formal_v12.json

# 运行模板 Bot 与参考 Bot 的换边自检
go run ./cmd/arena check -config tournament/formal_v12.json -seed 20261109
```

使用 `engine.kind = "oracle"` 的配置时还需要 Node.js。Python 仅用于可选的菜单脚本 `start.py` 和部分辅助工具。

## 编写参赛 Bot

从 [`war_of_desert/workspace/submission/decide.go`](war_of_desert/workspace/submission/decide.go) 开始。参赛者通常只修改 `submission/`，规则和通信协议见[选手规则](war_of_desert/workspace/RULES.md)，参赛目录说明见[选手工作区](war_of_desert/workspace/README.md)。

编译并自检：

```bash
cd war_of_desert
go run ./cmd/arena build -config tournament/formal_v12.json
go run ./cmd/arena check -config tournament/formal_v12.json -seed 20261109
```

`check` 默认使用提交模板和配置中的第一个参考 Bot，换边运行两局并进行确定性复核。它验证的是运行流程，不代表 Bot 的策略强度。

## 运行整季基准赛

组织方可按顺序执行以下命令。配置使用当前第 3 季 v1.2 基准赛，生成物写入 `war_of_desert/results/formal-v12/`。

```bash
cd war_of_desert
go run ./cmd/arena build -config tournament/formal_v12.json
go run ./cmd/arena plan -config tournament/formal_v12.json
go run ./cmd/arena run -config tournament/formal_v12.json
go run ./cmd/arena rate -config tournament/formal_v12.json
```

## 文档入口

| 内容 | 文档 |
|---|---|
| 规则与参赛约束 | [选手规则](war_of_desert/workspace/RULES.md) |
| 选手模板、参考 Bot 与本地命令 | [选手工作区](war_of_desert/workspace/README.md) |
| 赛季配置 | [v1.2 配置（当前）](war_of_desert/tournament/formal_v12.json) · [v1.1 配置（历史）](war_of_desert/tournament/formal_v11.json) · [v1.0 配置（历史）](war_of_desert/tournament/formal.json) |
| 历季成绩摘要 | [赛季榜单](war_of_desert/LEADERBOARD.md) |
| 赛事产物格式与复核 | [结果目录说明](war_of_desert/results/README.md) |
| 引擎和 oracle 结构 | [引擎说明](war_of_desert/engine/README.md) |
| 规则规格 | [Beta 规则计划](Dev/Plan_Beta.md) · [Oracle 纪律](Dev/ORACLE.md) |
| 调研与来源 | [Dev 文档索引](Dev/Docs/README.md) |

## 许可

本项目采用 [MIT License](LICENSE)。如使用仓库中的第三方参考资料，请同时遵守其各自的授权与署名要求。

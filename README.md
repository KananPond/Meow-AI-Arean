# Meow-AI-Arean社区AI比赛

一个面向社区的 1v1 AI Bot 算法对抗赛项目。Bot 在程序生成的沙漠战场中争夺 VP、守护母舰；仓库提供本地运行的 Go 裁判与引擎、参赛模板、参考 Bot、赛事配置和赛季榜单。

> 这是源码和赛事资料仓库，不是在线报名平台或托管式对战服务。执行链已可开赛（当前正式赛 **v1.9.2 · 第 6 季**，ruleset `v15`，1500 tick），组织流程见下方「裁判台」。
> v1.9.2 在 v1.4 基线上叠加遗产／插旗／胜利分、研究舰与遗产科技、CEP／射界／炮塔、火控与五级军衔（v1.9.1 基础上修复运货死锁，指纹 `077336ff`）；规则见 [RULES_V15.md](war_of_desert/workspace/RULES_V15.md)，714 局标定数据（v1.9.1 口径）见 [exp-v191-700](war_of_desert/experiment-records/rule-experiments/exp-v191-700/README.md)。默认选手包已按 v1.9.2 发卷。

本项目采用 [MIT License](LICENSE) 开源。

## 项目现状

- 正式赛执行链支持编译 Bot、封存源码哈希、冻结赛程、运行对局和生成评分。
- 仓库包含 Go 原生规则引擎及历史 JavaScript oracle；当前赛季（第 6 季 v1.9.2）配置使用 Go 原生引擎。
- 当前正式赛规则版本为 **v1.9.2**（ruleset `v15`，1500 tick；v1.9.1 基础上修复运货死锁与采集/遗产状态机冲突，指纹变更；714 局标定为 v1.9.1 口径，正式开赛前建议重跑）：在 v1.4 基线（跳过式生产队列、母舰 RU 基础收入、四子系统分配、CU/RU 双资源运输线、资源成对同类、脱战自愈、母舰削弱）之上叠加胜利分三本账（击杀／遗产／插旗）、遗产物运输入库、研究舰与 R1–R4 遗产科技、CEP 与射界／炮塔、火控 C1–C3、5 级单位等级、人口上限 36。
- v1.4 保留为历史基线（`tournament/formal_v14.json`、`algorithm-contest/tests/history-seasons/formal-v14/`）；v1.5–v1.8 为规则实验阶段，产物见 `war_of_desert/experiment-records/rule-experiments/exp-v*`。
- 第 4、5 季基准赛均为 6 名选手、45 场对阵、90 局，零故障、90/90 局通过确定性复核，见[赛季榜单](war_of_desert/LEADERBOARD.md)；v1.9.1 以 7 Bot × 17 seed × 换边 2 局 = 714 局完成标定（平局 4.2%、决胜座位 49.9:50.1、母舰斩杀 7.1%），见 [exp-v191-700](war_of_desert/experiment-records/rule-experiments/exp-v191-700/README.md)。
- 正式赛逐局 JSON、赛程和回放属于本地生成的赛事产物，并未全部包含在当前 Git 推送内容中。
- `Dev/` 当前推送的是规则规格、计划、实验、调研与归档文档；可运行的正式赛代码位于 `war_of_desert/`。

## 功能

- 1v1 Bot 对战，程序生成地图，固定 seed 便于复现。
- Bot 以独立进程运行，通过标准输入/输出的 JSON 行协议与裁判通信。
- 裁判执行决策时限、故障判罚、换边对局和确定性复核。
- 赛后可生成 Bradley–Terry 实力榜与独立的加权总分榜。
- 提供 Go 原生引擎、冻结的 JavaScript oracle、golden trace 回归测试及两个官方基线（中等 / 极强）。

## 目录

```text
社区比赛__WoD/
├── 裁判台.bat            ← 双击打开 GUI 裁判台
├── referee_desk.py       ← 裁判台本体（tkinter，标准库）
├── 赛务.py               ← 发卷 / 开赛 / 暂停 / 收卷（也可单独命令行用）
├── algorithm-contest/               ← 比赛工作区，每个子目录是一名选手
│   ├── baseline@1.exe    基线一（预编译，中等战术）
│   ├── baseline@2.exe    基线二（预编译，极强·多轮调试）
│   ├── match-records/          裁判台「比赛」页签的成绩归档（本地生成，不入库）
│   ├── tests/             历史比赛记录与「测试」页签的测试成绩（本地生成）
│   └── rts-contest@<model-name>/
│       └── workspace/
│           ├── arena/     正式赛引擎+裁判+比赛程序（源码，发给选手的那份）
│           ├── bots/      骨架 + 两个预编译基线
│           ├── submission/<选手名>_bot.go  ← 唯一要改的文件
│           ├── tournament/local.json
│           └── 本地对局.bat / 本地赛季.bat
├── war_of_desert/        引擎与执行链（Go 模块）
│   ├── experiment-records/          v1.5 → v1.9.1 规则实验归档（实验日志见其 README）
│   └── results/          当前与今后的正式赛事产物
└── Dev/                  规则规格、计划、调研与实验（[Dev 索引](Dev/README.md)）
    └── experiments/             [Plan_V15.md](Dev/experiments/Plan_V15.md)：v1.5 → v1.9.1 规则演进与标定
```

## 比赛怎么初始化

```bash
py -3 -X utf8 referee_desk.py --init "algorithm-contest" 1
```

也可以打开裁判台，在「编码」页签逐个工作区点「发卷」。

做两件事：

1. **初始化工作区**：以 `algorithm-contest/rts-contest@model-name/workspace/` 为**模板**，
   复制到每个选手目录（`RULES.md`、`prompt.md`、`arena/`、`bots/`、
   `tournament/`、`clock.py`、`本地对局.bat`），**不开始计时**——计时由裁判台
   「编码」页签的「开始编码」逐个开。

   选手包里的参考 Bot 只有三个：`bots/skeleton/`（骨架，只演示 stdin/stdout 协议，不含任何策略）、
   `bots/baseline@1.exe`、`bots/baseline@2.exe`（两个预编译基线）。

   **`arena/` 是正式赛引擎、裁判与比赛程序的源码副本**，不是模拟器：规则
   `internal/sim`、裁判 `internal/judge`（协议校验、超时杀进程、Job Object 沙箱、
   确定性复核）、入口 `cmd/arena`（`local` / `check` / `plan` / `run` / `rate`）。
   选手双击 `本地对局.bat` 跑的就是赛场上那条执行链，`go.mod` 的
   `replace desertarena => ./arena` 让 `botkit` 也解析到这份副本。

   交付物按选手名命名：`submission/<选手名>_bot.go`。例如 `rts-contest@HY-4/`
   要交的是 `workspace/submission/HY-4_bot.go`——**不是 `decide.go`**。
2. **冻结赛程**：按「所有报名者两两相遇 × 每个 seed 换边两局」生成
   `algorithm-contest/赛程.json`，记录报名者（基线 + 选手）与全部对阵。

选手名单由目录内容自动识别：

| 类型 | 位置 | 说明 |
|---|---|---|
| 基线 | `algorithm-contest/baseline@*.exe` | **预编译**可执行文件，不参与编译，直接上场 |
| 选手 | `algorithm-contest/rts-contest@<model-name>/` | 目录名即选手名，以收卷后的 `submissions/<选手名>_bot.go` 为准 |

两个基线的身份：`baseline@1` = **中等战术水平**（攻城消耗剖面，refbot 库）；
`baseline@2` = **多轮调试的极强上限对手**（acebot 库：集火/伤兵微操/护旗/劫旗/遗产竞速），
调试记录见 [baseline-tuning](war_of_desert/experiment-records/bot-experiments/baseline-tuning/README.md)。
旧 7 个参考 Bot 已归档至 [reference-bots-archive](war_of_desert/experiment-records/reference-bots-archive/README.md)。
评测时基线排在名册前面，BT 以 `baseline@1` 为锚（=0），因此不同场次的选手分数可以直接横向比较。

## 裁判台

双击 `裁判台.bat`。上方三个页签：**编码 / 比赛 / 测试**，默认停在「编码」。

### 编码：给参赛 AI 开一段写代码的时间窗

这一段只管「铺包 + 计时 + 到点收卷」，**全程不跑对局**。

1. **选工作区**：指定比赛根目录（默认 `algorithm-contest/`），在左边列表里选中一个
   `rts-contest@<model-name>` 目录；规则版本可选 v1.9 / v1.4。
2. **自检**：查包是否齐备、`go build ./submission` 与 `arena` 副本能否编译；
   缺件会顺手补发。**自检不通过，「开始编码」是灰的。**
3. **发卷**：把模板选手包铺进 `<工作区>/workspace/`，只补缺件、不覆盖已有文件。
4. **提示词**：点「复制」把 `prompt.md` 放进剪贴板（或「查看」只读预览），
   自己去智能体那边粘贴发送——**裁判台不代发**。
5. **开始编码**：只开始计时（90 分钟目标 / 120 分钟强制收卷），不跑对局；
   随时「暂停 / 继续」。
6. **收卷封存**：到 120 分钟自动把 `workspace/submission/<选手名>_bot.go`
   冻结到 `submissions/`，记 SHA-256、用时与是否超时；也可以「立即收卷」。裁判台关着时
   错过的，下次打开会按 `.赛务.json` 里的开始时间补封。交付物和模板逐字节相同
   （AI 没写出东西）时会单独告警。

### 比赛：正式对局

选择参赛工作区的 Bot、比赛规则版本，点「开始对战」。裁判台调用
`war_of_desert` 的官方 arena（`build → plan → run → rate`），对手为
`algorithm-contest/baseline@1.exe`、`baseline@2.exe`；选中的选手之间也会互相对局。
有收卷副本时以收卷副本为准。成绩写入 `algorithm-contest/match-records/<时间戳>/`
（`成绩.md`、`成绩_加权总分.md`、`成绩.json`、`结果.jsonl`、`replays/`）。

### 测试：编码 + 比赛

「测试」页签包含上面两段的全部功能，但成绩只写入
`algorithm-contest/tests/test-results/<时间戳>/`，不会混进正式比赛记录。

需要带 tkinter 的 Python。若 `python` 是精简发行版，用 `py -3 referee_desk.py` 或直接双击 bat。

## 快速开始（命令行）

需要 Go 1.22 或更高版本。以下命令在仓库的 `war_of_desert/` 目录执行。

```bash
cd war_of_desert

# 运行正式赛源码的 Go 测试
go test ./cmd/... ./internal/... ./workspace/...

# 编译当前赛季（第 6 季 v1.9.2）配置中的 Bot，并封存源码哈希
go run ./cmd/arena build -config tournament/formal_v19.json

# 运行模板 Bot 与参考 Bot 的换边自检
go run ./cmd/arena check -config tournament/formal_v19.json -seed 20261109
```

使用 `engine.kind = "oracle"` 的配置时还需要 Node.js。Python 仅用于可选的菜单脚本 `start.py` 和部分辅助工具。

## 编写参赛 Bot

在 `algorithm-contest/rts-contest@<model-name>/workspace/` 里写。只改一个文件：
`submission/<选手名>_bot.go`。规则见该目录下的
[`RULES.md`](algorithm-contest/rts-contest@model-name/workspace/RULES.md) 与
[`RULES_V15.md`](algorithm-contest/rts-contest@model-name/workspace/RULES_V15.md)，包内容见
[`README.md`](algorithm-contest/rts-contest@model-name/workspace/README.md)。

包内自带**正式赛同一套引擎、裁判与比赛程序**（`arena/` 源码）：双击
`本地对局.bat` 就编译并对打两个基线，`本地赛季.bat` 跑完整赛季并出 BT 榜。

编译并自检：

```bash
cd war_of_desert
go run ./cmd/arena build -config tournament/formal_v19.json
go run ./cmd/arena check -config tournament/formal_v19.json -seed 20261109
```

`check` 默认使用提交模板和配置中的第一个参考 Bot，换边运行两局并进行确定性复核。它验证的是运行流程，不代表 Bot 的策略强度。

## 运行整季基准赛

组织方可按顺序执行以下命令。配置使用当前第 6 季 v1.9.2 基准赛，生成物写入 `war_of_desert/results/formal-v19/`。

```bash
cd war_of_desert
go run ./cmd/arena build -config tournament/formal_v19.json
go run ./cmd/arena plan -config tournament/formal_v19.json
go run ./cmd/arena run -config tournament/formal_v19.json
go run ./cmd/arena rate -config tournament/formal_v19.json
```

## 文档入口

| 内容 | 文档 |
|---|---|
| 规则与参赛约束 | [选手规则](war_of_desert/workspace/RULES.md) · [v1.9.2 增量](war_of_desert/workspace/RULES_V15.md) |
| 选手模板、参考 Bot 与本地命令 | [选手工作区](war_of_desert/workspace/README.md) |
| 赛季配置 | [v1.9.2 正式配置（当前）](war_of_desert/tournament/formal_v19.json) · [v1.9.1 标定配置](war_of_desert/tournament/experiment_v19_700.json) · [v1.4 配置（历史）](war_of_desert/tournament/formal_v14.json) · [v1.3 配置（历史）](war_of_desert/tournament/formal_v13.json) · [v1.2 配置（历史）](war_of_desert/tournament/formal_v12.json) · [v1.1 配置（历史）](war_of_desert/tournament/formal_v11.json) · [v1.0 配置（历史）](war_of_desert/tournament/formal.json) |
| 历季成绩摘要 | [赛季榜单](war_of_desert/LEADERBOARD.md) |
| 赛事产物格式与复核 | [结果目录说明](war_of_desert/results/README.md) |
| v1.9.1 规则与 714 局标定 | [Plan_V15.md](Dev/experiments/Plan_V15.md) · [exp-v191-700](war_of_desert/experiment-records/rule-experiments/exp-v191-700/README.md) |
| 引擎和 oracle 结构 | [引擎说明](war_of_desert/engine/README.md) |
| 规则规格 | [Beta 规则计划](Dev/Plan_Beta.md) · [Oracle 纪律](Dev/ORACLE.md) |
| 调研与来源 | [Dev 文档索引](Dev/Docs/README.md) |

## 贡献者

- **KananPond**：赛事设计、规则决策、验收与发布维护。
- **TRAE（DeepSeek-V4.1-Flash，AI 编码智能体）**：v1.5 → v1.9.1 规则实验的引擎实现、参考 Bot、裁判台适配、标定与文档整理。
- **ZCode（GLM-5.3-Flash）**：引擎早期移植、测试与文档，见 [`war_of_desert/README.md`](war_of_desert/README.md) 的 AI 参与声明。
- **Qoder CN （Qwen-4-Max）**：Bot调优。

## 许可

本项目采用 [MIT License](LICENSE)。如使用仓库中的第三方参考资料，请同时遵守其各自的授权与署名要求。

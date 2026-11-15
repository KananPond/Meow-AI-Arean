# Meow-AI-Arean社区AI比赛

面向社区的 AI Bot 算法对抗赛：在程序生成的沙漠战场争夺 VP、守护母舰，以可复现赛制与回放验证策略。Bot 只需实现 `init/decide`。

本项目采用 [MIT License](LICENSE) 开源。

> **当前阶段：β 测试**（正式版 v1.2-beta 语义：生产队列跳过式出队 + 母舰 RU 基础收入）。
>
> **AI 参与声明**：本仓库的引擎移植、参考 Bot、测试与文档由 AI（ZCode 智能体）在人类指导下参与编写；
> 规则设计、赛制与验收由人类主导，全部对局结果均经引擎内置的确定性复核门禁（golden trace + orderLog 重放）。

> **正式版**：[`war_of_desert/`](war_of_desert/README.md) —— 已办完第 1 季基准赛的正式赛事（Go 执行链 + 冻结快照 oracle）。
> [`Dev/MVP/`](Dev/MVP/README.md) 是规则**实验项目**（JS 规则引擎 + 测试环境）：规则在这里演化与标定，不举办比赛；
> 它的冻结快照 `MVP@288db07`（tag `baseline/288db07`，分支 `rules-v1.1` 同指向）是 war_of_desert 的规则语义来源（oracle）。

## 当前状态

| 部分 | 形态 | 参数 | 状态 |
|---|---|---|---|
| **正式版 v1.1** | `war_of_desert/`：Go 原生引擎（`internal/sim`，golden trace 回归）+ Go 执行链；配置 `tournament/formal_v11.json` | 1200 tick ｜ 120ms 决策 ｜ 七领域计分 | **第 2 季基准赛完赛**：5 选手 20 场 40 局零故障，40/40 确定性复核通过；榜单见 [`LEADERBOARD.md`](war_of_desert/LEADERBOARD.md) |
| **正式版 v1.0** | `war_of_desert/`：Go 执行链（`cmd/arena` build → plan → run → rate）+ 快照 oracle 引擎宿主 `engine/oracle_host.js` | 600 tick ｜ 120ms 决策 ｜ 64KB 契约 | 第 1 季基准赛完赛：5 选手 20 场 40 局零故障，40/40 确定性复核通过 |
| **规则实验** | `Dev/MVP/`：JS 规则引擎 + 测试环境；最新分支 `Dev`/`main` @ `c8c7ddc`（v0.11.0，改动未提交） | 256² ｜ 1200 tick（60s）｜ 120ms 决策 ｜ 128KB 行上限 | 10/10 套件通过；v0.11 修复时间基准/协议/冻结链 |
| **语义锚点** | `MVP` 冻结快照 `MVP@288db07`：tag `baseline/288db07`、分支 `rules-v1.1`；另有归档分支 `archive/v1.1-experiments`（v1.1 经济调优实验） | — | oracle 校验：`python -X utf8 scripts/verify_oracle.py` |
| **攻守实验** | 一方守擂、一方进攻；工事、矿场、母舰 HP 标定（`cmd/siege`、`赛事手册_攻守.md`） | HP 1250 ｜ 工事 16 ｜ 1260 tick | 实验分支，不随正式版投稿 |

## 两个目录

| 目录 | 内容 |
|---|---|
| [`war_of_desert/`](war_of_desert/README.md) | **正式版**：赛事手册、Go 执行链（build/plan/run/rate/selftest）、Go 原生引擎与 oracle 双轨、选手包、`bots/封存记录.json`、两季成绩、`LEADERBOARD.md`、攻守实验 |
| [`Dev/`](Dev/README.md) | **规则实验**：`MVP/` 规则引擎与测试、`Plan_Beta.md` 规格、`ORACLE.md` 快照纪律、`Docs/` 调研、`实验/` 攻守、`审查/`、`归档/`、`reference/` |

## 入口

```bash
# 正式版（war_of_desert/）
cd war_of_desert
python -X utf8 start.py          # 交互菜单：build / plan / run / rate / check / selftest / season / status
python -X utf8 start.py season   # 整季流水线：build → plan → run → rate
go run ./cmd/arena check -seed 20261109
python -X utf8 scripts/verify_oracle.py   # oracle 快照校验
go run ./cmd/siege -smoke -seed 20261201  # 攻守实验

# 规则实验（Dev/MVP/）：规则测试 / 覆盖率 / 自包含测试环境
cd Dev/MVP && npm test
cd Dev/MVP && npm run coverage
cd Dev/MVP && npm run sync:formal
cd Dev/MVP/正式比赛测试/workspace && node arena.js full
```

## 文档地图

| 想知道什么 | 看哪里 |
|---|---|
| **正式赛怎么组织**（赛季流程、收卷、仲裁、评分口径） | [`war_of_desert/赛事手册.md`](war_of_desert/赛事手册.md) |
| 第 1 / 2 季正式赛成绩（BT 榜 + 计分卡加权总分榜） | [`war_of_desert/LEADERBOARD.md`](war_of_desert/LEADERBOARD.md) |
| **观赛**（回放怎么播放） | [`war_of_desert/回放查看器.html`](war_of_desert/回放查看器.html) |
| 攻守实验手册 | [`war_of_desert/赛事手册_攻守.md`](war_of_desert/赛事手册_攻守.md) |
| 规则实验当前实现、数值、v0.11 修复记录 | [`Dev/MVP/README.md`](Dev/MVP/README.md) |
| 规则规格与快照纪律（规则实验线的上游依据） | [`Dev/Plan_Beta.md`](Dev/Plan_Beta.md) |
| **改造方案（v0.11 → v1.0-candidate）：12 条 DoD、六阶段、文件级清单** | [`Dev/plan.md`](Dev/plan.md) |
| v0.11 修复后测试报告（10 套件/覆盖率/100 局） | [`Dev/审查/MVP_测试报告_20261113.md`](Dev/审查/MVP_测试报告_20261113.md) |
| v0.10 设计审查（评估协议、计分卡、机制覆盖） | [`Dev/审查/WoD_设计审查_20261113.md`](Dev/审查/WoD_设计审查_20261113.md) |
| 攻守实验方案 | [`Dev/实验/Plan_Siege.md`](Dev/实验/Plan_Siege.md) |
| 被取代的历史计划与旧审查 | [`Dev/Docs/archive/`](Dev/Docs/archive/README.md) |
| 原作调研与事实卡片 | [`Dev/Docs/`](Dev/Docs/README.md) |

## 现状与边界

- **正式版（war_of_desert）**：赛务链完整（编译封存 → 赛程冻结 → 对局 → BT + 八领域评分 → 确定性复核），但规则语义不自研——由只读快照 `MVP@288db07` 经 `engine/oracle_host.js` 承载，快照不再回灌；Go 引擎本体未实现（单位口径、选手包自包含、冻结链、`hello` 契约等问题见审查附录 C）。
- **规则实验（Dev/MVP）**：是规则演化的开发现场，不是已举办的比赛：没有模型条件、没有 BT/CI/并列、没有许可、没有沙箱、没有归档 HTML 回放。v1.1 起规则语义以 `Dev/MVP` HEAD 为准（见 `Plan_Beta.md`）。
- `hello` 在 256² 下约 110 KB；实验线 `MAX_LINE=128KB`，512² 前必须先做压缩/分块（正式线契约仍为 64KB / 600 tick）。
- 八领域里母舰完整度、算力、KD、保存兵力多数局接近常数；排名仍是积分榜。详见审查报告。
- 攻守实验**尚未产品化**：缺选手包、许可、回放查看器与沙箱；参考 Bot 有已知组合缺陷。

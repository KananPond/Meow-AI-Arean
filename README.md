# 社区比赛__WoD · Desert Arena

> **当前阶段：正式版 v1.0 已落地，引擎仍为冻结 oracle**
> **当前有效规格**：[Plan_Beta.md](Plan_Beta.md) v1.0
> **当前规则预言机**：`MVP@288db07f164bc6c680a2d878c47153804d3fdcab`，只读冻结
> **正式赛**：[war_desert/](war_desert/README.md) 已跑完第 1 季基准赛：5 名选手、20 场、40 局、零故障、40/40 确定性复核。赛事手册见 [war_desert/赛事手册.md](war_desert/赛事手册.md)。

---

## 这个项目是什么

Desert Arena 是面向本地 AI 的 1v1 算法对抗赛：红蓝双方在程序生成的沙漠地形中争夺 VP 点、保护母舰，并在严格决策预算下提交 Bot 代码。项目同时保留叙事榜方向，但叙事功能不属于当前引擎迁移阻断项。

正式版执行链已用 Go 标准库落地：独立 Bot 进程、真实行分隔 JSON 协议、120ms 决策死线、连续 3 次红牌、确定性复核、BT 实力值与八领域计分卡。引擎仍由冻结 oracle 的独立进程宿主承载，Go 引擎本体尚未实现。

## 当前事实源

| 文件/目录 | 作用 |
|---|---|
| [Plan_Beta.md](Plan_Beta.md) | **当前唯一正式规格与迁移计划**：规则基线、协议契约、架构边界、验收门禁、偏差登记 |
| [ORACLE.md](ORACLE.md) | 冻结 MVP 的运行、复现、覆盖率记录和 golden trace 规范 |
| `MVP/` | 冻结 JS 规则预言机；只读，不得修改 |
| `MVP/正式比赛测试/` | 自包含正式比赛测试环境：workspace、裁判快照、模板 Bot、runner、计时与收卷 |
| `war_desert/` | **Go 正式赛执行链**：`cmd/arena`、`internal/*`、选手包、赛程、成绩与回放；引擎仍为冻结 oracle 宿主 |
| `Docs/` | 原作调研、事实卡片、版权与术语资料 |
| `Plan_Alpha.md` | 历史构想，已归档 |
| `MVP_plan.md` | 历史 MVP 计划，已废止 |
| `MVP_*审查/评估报告` | 历史审查证据，不作当前规则事实源 |

## 快速核对冻结预言机

```bash
git -C MVP rev-parse HEAD
npm --prefix MVP run check
npm --prefix MVP test
```

期望：HEAD 为 `288db07f164bc6c680a2d878c47153804d3fdcab`；语法检查和 10 个测试套件全部通过。详细口径见 [ORACLE.md](ORACLE.md)。

## 正式比赛测试环境

`MVP/正式比赛测试/` 是在冻结 MVP 之上新增的自包含测试环境，不修改冻结提交中的文件。进入 `MVP/正式比赛测试/workspace/` 后运行：

```bash
node arena.js check
node arena.js play
node arena.js full
```

默认使用内置 `Tactical Flow` 对模板 `Submission Starter`，600 tick、换边两局；详细说明见 `MVP/正式比赛测试/README.md`。

## 赛事计时与强制收卷

- 选手自查时间：`python -X utf8 war_desert/workspace/clock.py`
- 组织方开赛/暂停/恢复/收卷：`python -X utf8 war_desert/开赛.py`
- 默认 90 分钟目标、120 分钟强制收卷；
- 权威记录保存在 `war_desert/赛务记录.json`；
- 收卷对象是 `war_desert/workspace/submission/decide.go`；
- 到点冻结会记录 SHA-256、用时、accepted、缺件状态，超时或未提交均不计正式成绩。

详细契约见 [Plan_Beta.md](Plan_Beta.md) §3.5。

## 当前规则快照

以 [Plan_Beta.md](Plan_Beta.md) §2 为准：

- 256×256 地图，默认 `minecraft` 生成器；
- 20 tick/s，单局 600 tick；
- 120ms 决策预算，指令固定晚 1 tick 生效；
- 连续 3 次故障红牌；
- 单 CU、四项母舰分配、四条科技树、RK4 外弹道；
- 五级判定链与八领域加权计分；
- 视野可配置，但只改变信息集，不改变模拟。

## 当前基线与待办

已完成：

- 外层迁移基线文档纳入版本控制，基线提交 `ee4e6ef`，tag `oracle-baseline-2026-11-12`。
- 正式赛赛制层 v1.0：Go 裁判、行协议引擎宿主、进程隔离、赛程冻结、BT + bootstrap、八领域双榜、选手包与 5 个参考 Bot。
- 第 1 季基准赛跑通：`selftest` 三条故障路径通过，40 局零故障、40/40 复核。

待办：

1. 生成并保存 golden trace；冻结 trace 生成器版本。
2. 补独立 JSON Schema 与一致性测试（Go DTO 已在 `internal/protocol` 落地）。
3. 实现 Go 引擎本体并通过 `Plan_Beta.md` §6.2 门禁，替换 `engine/oracle_host.js`。
4. 把坡度、通行、视线与弹道公式收敛为单一共享实现。
5. 将 256² `hello` 从 113 KB 收敛到 64 KiB。
6. 建立不修改冻结 MVP 的覆盖率/分支覆盖报告链。

## 重要纪律

- 不修改冻结 MVP。
- 不把历史计划中的 10 TPS、512²、双资源、人类可玩交互当成当前需求。
- 正式赛引擎仍是冻结 oracle；Go 引擎通过 §6.2 门禁前，不得宣称已替代 oracle。
- 不在 Go 正式版中直接照搬 MVP 的 UMD、同进程 driver、JS 哈希或 `Game` God Class。
- 任何规则变化必须先更新 `Plan_Beta.md`，再更新实现与测试。

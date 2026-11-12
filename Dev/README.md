# Dev · 开发与历史资料区

> **正式比赛在上一层仓库根目录**：[`../README.md`](../README.md)、[`../赛事手册.md`](../赛事手册.md)。
> 本目录只放开发期资料：冻结预言机、原作调研、规格与计划、历次审查证据、第三方参照。
> 除 `MVP/` 作为规则语义参考外，这些内容**不参与正式赛执行**。

---

## 目录

| 路径 | 作用 |
|---|---|
| [`MVP/`](MVP/) | 冻结 JS 规则预言机（`MVP@288db07f`），只读，不得修改 |
| [`MVP/正式比赛测试/`](MVP/正式比赛测试/) | 冻结 MVP 之上的自包含测试环境：workspace、裁判快照、模板 Bot、runner、计时与收卷 |
| [`Docs/`](Docs/) | 原作调研、事实卡片、操作映射、术语表、版权与来源 |
| [`Plan_Beta.md`](Plan_Beta.md) | **当前唯一正式规格与迁移计划**：规则基线、协议契约、架构边界、验收门禁、偏差登记 |
| [`ORACLE.md`](ORACLE.md) | 冻结 MVP 的运行、复现、覆盖率记录与 golden trace 规范 |
| [`Plan_Alpha.md`](Plan_Alpha.md) | 历史构想，已归档 |
| [`MVP_plan.md`](MVP_plan.md) | 历史 MVP 计划，已废止 |
| `MVP_*修复/审查/评估报告*.md` | 历次审查证据，不作当前规则事实源 |
| [`模板/`](模板/) | 第三方只读参照（本地克隆，不入库） |

正式赛的代码、选手包与赛季产物在根目录：

```text
../cmd/ ../internal/ ../engine/ ../tournament/ ../workspace/
../bots/ ../results/ ../replays/ ../notes/ ../start.py ../开赛.py
```

---

## 核对冻结预言机

在**仓库根目录**执行：

```bash
git -C Dev/MVP rev-parse HEAD
npm --prefix Dev/MVP run check
npm --prefix Dev/MVP test
```

期望：HEAD 为 `288db07f164bc6c680a2d878c47153804d3fdcab`；语法检查和 10 个测试套件全部通过。详细口径见 [`ORACLE.md`](ORACLE.md)。

## 正式比赛测试环境

`MVP/正式比赛测试/` 是在冻结 MVP 之上新增的自包含测试环境，不修改冻结提交中的文件。进入 `Dev/MVP/正式比赛测试/workspace/` 后运行：

```bash
node arena.js check
node arena.js play
node arena.js full
```

默认使用内置 `Tactical Flow` 对模板 `Submission Starter`，600 tick、换边两局；详细说明见 `Dev/MVP/正式比赛测试/README.md`。

它验证的是 JS 测试链路；正式赛跑的是根目录的 Go 执行链，两者不要混用。

## 规则快照

以 [`Plan_Beta.md`](Plan_Beta.md) §2 为准：

- 256×256 格，默认 `minecraft` 生成器；
- 20 tick/s，单局 600 tick；
- 120 ms 决策预算，指令固定晚 1 tick 生效；
- 连续 3 次故障红牌；
- 单 CU、四项母舰分配、四条科技树、RK4 外弹道；
- 五级判定链与八领域加权计分；
- 视野可配置，但只改变信息集，不改变模拟。

## 迁移待办

1. 生成并保存 golden trace；冻结 trace 生成器版本。
2. 补独立 JSON Schema 与一致性测试（Go DTO 已在 `../internal/protocol` 落地）。
3. 实现 Go 引擎本体并通过 `Plan_Beta.md` §6.2 门禁，替换 `../engine/oracle_host.js`。
4. 把坡度、通行、视线与弹道公式收敛为单一共享实现。
5. 将 256² `hello` 从 113 KB 收敛到 64 KiB。
6. 建立不修改冻结 MVP 的覆盖率/分支覆盖报告链。

## 重要纪律

- 不修改冻结 MVP。
- 历史计划中的 10 TPS、512²、双资源、人类可玩交互都不是当前需求。
- 正式赛引擎仍是冻结 oracle；Go 引擎通过 §6.2 门禁前，不得宣称已替代 oracle。
- 不把 MVP 的 UMD、同进程 driver、JS 哈希或 `Game` God Class 照搬进 Go 正式版。
- 任何规则变化必须先更新 `Plan_Beta.md`，再更新实现与测试。

## 赛事计时与收卷（指向根目录）

- 选手自查时间：`python -X utf8 workspace/clock.py`
- 组织方开赛/暂停/恢复/收卷：`python -X utf8 开赛.py`
- 默认 90 分钟目标、120 分钟强制收卷；
- 权威记录：根目录 `赛务记录.json`；
- 收卷对象：根目录 `workspace/submission/decide.go`；
- 详细契约见 [`Plan_Beta.md`](Plan_Beta.md) §3.5。

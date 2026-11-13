# Dev · 开发与历史资料区

> **正式比赛在隔壁目录**：[`../war_of_desert/README.md`](../war_of_desert/README.md)。
> 本目录放开发期资料：规则预言机源码、原作调研、规格与计划、审查证据、第三方参照。
> `MVP/` 是可继续开发的规则源码（2026-11-13 解冻）；正式赛的语义锚点是
> [`../war_of_desert/engine/oracle/`](../war_of_desert/engine/oracle/MANIFEST.json) 的字节快照。
> 这些内容**不参与正式赛执行**。

---

## 分支

| 分支 | 用途 |
|---|---|
| `main` | 投稿/发布快照，只在确认投稿时从 `Dev` 合并 |
| `Dev` | 唯一开发分支：v1.0 正式赛、攻守实验、审查与方案文档都在这里演进 |

> 本仓库顶层有 `Dev/` 目录，与分支名 `Dev` 同名，`git log Dev` 会报
> `ambiguous argument`。引用分支时写完整引用 `refs/heads/Dev`（如 `git log refs/heads/Dev --`）；
> `git switch Dev` / `git status` 不受影响。

---

## 现行文档

| 文档 | 作用 | 状态 |
|---|---|---|
| [`Plan_Beta.md`](Plan_Beta.md) | **v1.0 规则基线**：规则、协议契约、架构边界、验收门禁、偏差登记 | 生效中 |
| [`Plan_Siege.md`](Plan_Siege.md) | **攻守改造全记录**：规则改动、模式契约、母舰 HP 与工事预算标定、BT 评分 | 生效中 |
| [`设计审查_正式版v1.0_20261113.md`](设计审查_正式版v1.0_20261113.md) | 最近一次设计审查：单位口径、计分卡、名次统计力、规范一致性 | 现行结论 |
| [`ORACLE.md`](ORACLE.md) | 快照校验、复现、覆盖率记录与 golden trace 规范 | 生效中 |
| [`Docs/`](Docs/) | 原作调研、事实卡片、操作映射、术语表、版权与来源 | 参考层 |
| [`Docs/archive/`](Docs/archive/) | **已归档的历史文档**：被取代的计划、历次审查与修复报告 | 只读，不作事实源 |
| [`MVP/`](MVP/) | JS 规则预言机源码（**2026-11-13 已解冻**）；v1.0 语义锚点为 tag `baseline/288db07` | 开发中 |
| [`MVP/正式比赛测试/`](MVP/正式比赛测试/) | `MVP@288db07` 之上的自包含测试环境：workspace、裁判快照、模板 Bot、runner、计时与收卷 | 快照 |
| [`模板/`](模板/) | 第三方只读参照（本地克隆，不入库） | 参考 |

判断标准很简单：**能据此写规则、跑对局、做标定的，是现行文档；只能用来回溯历史的，进 `Docs/archive/`。**

---

## 目录

```text
Dev/
├── README.md                 本文件
├── Plan_Beta.md              v1.0 规则基线与迁移规格
├── Plan_Siege.md             攻守改造与标定记录
├── 设计审查_正式版v1.0_20261113.md
├── ORACLE.md                 快照校验与预言机运行规范
├── Docs/
│   ├── README.md             调研资料索引
│   ├── 01~05_*.md            调研报告、事实卡片、操作映射、术语表、方法
│   ├── ATTRIBUTION.md        来源登记
│   ├── LICENSE.md            CC BY 4.0 声明
│   ├── sources.json          机器可读来源清单
│   └── archive/              已归档历史文档（含索引 README.md）
├── MVP/                      JS 规则预言机源码（独立 git 仓库，外层忽略；已解冻）
└── 模板/                     第三方只读参照（不入库）
```

正式赛的代码、选手包与赛季产物在 `../war_of_desert/`：

```text
../war_of_desert/cmd/ ../war_of_desert/internal/ ../war_of_desert/engine/ ../war_of_desert/tournament/
../war_of_desert/workspace/ ../war_of_desert/bots/ ../war_of_desert/results/
../war_of_desert/start.py ../war_of_desert/开赛.py
```

---

## 规则快照

### v1.0 正式赛（`engine/oracle/rules.js`，只读快照）

以 [`Plan_Beta.md`](Plan_Beta.md) §2 为准：

- 256×256 格，默认 `minecraft` 生成器；20 tick/s，单局 600 tick；
- 120 ms 决策预算，指令固定晚 1 tick 生效；连续 3 次故障红牌；
- 单 CU、四项母舰分配、四条科技树、RK4 外弹道；
- 五级判定链与八领域加权计分；视野可配置，只改信息集不改模拟。

### 攻守实验（`engine/oracle/rules_siege.js`，独立变体）

只读快照 `rules.js` **不改**；攻守是并行变体，`rulesHash` 独立。当前采纳参数：

| 参数 | 值 |
|---|---|
| 母舰 HP | 1250（双方） |
| 守方工事预算 | 16（5 炮塔 + 1 墙） |
| tickLimit | 1260（63 s） |
| 攻方胜率 | 45.0%（80 局）/ 46.0%（100 局），0 故障 |
| 评分 | 攻/守双参数 Bradley–Terry（`internal/rating/siege.go`） |

规则、协议与全部标定过程见 [`../war_of_desert/赛事手册_攻守.md`](../war_of_desert/赛事手册_攻守.md)
与 [`Plan_Siege.md`](Plan_Siege.md)。

---

## 核对快照与预言机

在**仓库根目录**执行：

```bash
python -X utf8 war_of_desert/scripts/verify_oracle.py   # 快照字节 vs MANIFEST
git -C Dev/MVP rev-parse baseline/288db07^{commit}      # 基线锚点
npm --prefix Dev/MVP run check
npm --prefix Dev/MVP test
```

期望：快照校验输出 `ORACLE_SNAPSHOT_OK`；tag 指向 `288db07f164bc6c680a2d878c47153804d3fdcab`；
语法检查与 10 个测试套件全部通过。

`Dev/MVP` 已解冻，HEAD 不再要求等于 `288db07`——但**语义锚点是快照，不是 HEAD**。
用 MVP 直接生成 golden trace 时，必须记录当时 HEAD。详细口径见 [`ORACLE.md`](ORACLE.md)。

## 正式比赛测试环境

`MVP/正式比赛测试/` 是在 `MVP@288db07` 之上新增的自包含测试环境，不修改其裁判快照。进入
`Dev/MVP/正式比赛测试/workspace/` 后运行：

```bash
node arena.js check
node arena.js play
node arena.js full
```

默认使用内置 `Tactical Flow` 对模板 `Submission Starter`，600 tick、换边两局；详细说明见
`Dev/MVP/正式比赛测试/README.md`。

它验证的是 JS 测试链路；正式赛跑的是 `war_of_desert/` 的 Go 执行链，两者不要混用。

---

## 迁移待办

1. 生成并保存 golden trace；冻结 trace 生成器版本。
2. 补独立 JSON Schema 与一致性测试（Go DTO 已在 `../war_of_desert/internal/protocol` 落地）。
3. 实现 Go 引擎本体并通过 `Plan_Beta.md` §6.2 门禁，替换 `../war_of_desert/engine/oracle_host.js`。
4. 把坡度、通行、视线与弹道公式收敛为单一共享实现。
5. 将 256² `hello` 从 113 KB 收敛到 64 KiB。
6. 建立不依赖修改快照的覆盖率/分支覆盖报告链。
7. 攻守模式：参考 Bot 的组合缺陷（集结门槛固定 6 台、廉价 LAV 占满人口 24）待修；
   修好后 HP / 工事预算 / tickLimit 需要重新标定。

## 重要纪律

- 不修改 `engine/oracle/` 快照。`MVP/` 可以改（已解冻），但改动不回灌快照；进正式赛必须新出快照 + 新 `rulesHash`。
- 历史计划中的 10 TPS、512²、双资源、人类可玩交互都不是当前需求。
- 正式赛引擎仍是只读快照 oracle；Go 引擎通过 §6.2 门禁前，不得宣称已替代 oracle。
- 不把 MVP 的 UMD、同进程 driver、JS 哈希或 `Game` God Class 照搬进 Go 正式版。
- 任何规则变化必须先更新 `Plan_Beta.md`（或 `Plan_Siege.md`），再更新实现与测试。

## 赛事计时与收卷（指向正式赛目录）

- 选手自查时间：`python -X utf8 war_of_desert/workspace/clock.py`
- 组织方开赛/暂停/恢复/收卷：`python -X utf8 war_of_desert/开赛.py`
- 默认 90 分钟目标、120 分钟强制收卷；
- 权威记录：`war_of_desert/赛务记录.json`；
- 收卷对象：`war_of_desert/workspace/submission/decide.go`；
- 详细契约见 [`Plan_Beta.md`](Plan_Beta.md) §3.5。

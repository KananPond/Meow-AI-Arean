# plan.md · Desert Arena 改造方案（v0.11 → v1.0-candidate）

> **文档性质**：可执行的改造工程方案，不是愿景稿。每条工作项都给了「现状 → 目标 → 改哪个文件哪一行 → 怎么验收」。
> **基线版本**：`Dev/MVP` v0.11（2026-11-13 修复后），实测见 [`reviews/MVP_测试报告_20261113.md`](reviews/MVP_测试报告_20261113.md)。
> **上游依据**：[`Plan_Beta.md`](Plan_Beta.md)（v1.0 规则意图）、[`ORACLE.md`](ORACLE.md)（快照纪律）、
> `Meow-AI-Arean/docs/open-source-scope.md`（正式集数规范）、`community/contests/rmuc2026/`（社区竞赛范式）。
> **本文不含**：`war_of_desert/`（Go 移植半成品）的功能开发。它只保留「编译通过 + 单位口径正确」，不再追规则。
> **状态**：全部工作项为 `待执行`。本文只描述方案，未改任何代码。

---

## 目录

- [0. 怎么用这份文档](#0-怎么用这份文档)
- [1. 基线事实表](#1-基线事实表)
- [2. 改造目标与验收口径](#2-改造目标与验收口径)
- [3. 总体路线与依赖](#3-总体路线与依赖)
- [4. 阶段 0 · 前置清理（0.5 周）](#4-阶段-0--前置清理05-周)
- [5. 阶段 1 · 玩法层（1.5 周）](#5-阶段-1--玩法层15-周)
- [6. 阶段 2 · 评测层（1 周）](#6-阶段-2--评测层1-周)
- [7. 阶段 3 · 赛制与统计（1 周）](#7-阶段-3--赛制与统计1-周)
- [8. 阶段 4 · 引擎与协议（1 周）](#8-阶段-4--引擎与协议1-周)
- [9. 阶段 5 · 交付与合规（1 周）](#9-阶段-5--交付与合规1-周)
- [10. 阶段 6 · 参考 Bot 与模型试赛（0.5 周）](#10-阶段-6--参考-bot-与模型试赛05-周)
- [11. 文件级改动清单](#11-文件级改动清单)
- [12. 测试计划](#12-测试计划)
- [13. 排期与人力](#13-排期与人力)
- [14. 风险登记册](#14-风险登记册)
- [15. 回滚与兼容](#15-回滚与兼容)
- [16. 附录](#16-附录)

---

## 0. 怎么用这份文档

### 0.1 三种执行粒度

| 你需要什么 | 读哪几节 | 产出 |
|---|---|---|
| 只想知道「这游戏要改成什么样」 | §1 → §2 → §5 的每节「目标」 | 一页设计变更清单 |
| 要动手改代码 | §5～§8 全节 + §11 + §12 | 可提交的 PR 序列 |
| 要投稿 / 交给评审 | §2 + §9 + §16.4 检查清单 | 合规的竞赛包 |

### 0.2 工作项编号规则

`阶段字母 + 序号`。`A`=玩法、`B`=评测、`C`=赛制、`D`=引擎、`E`=交付、`F`=试赛。每个工作项包含固定的五段：

```text
现状   —— 现在是什么（带文件:行号或实测数字）
目标   —— 改完是什么（可测量）
改法   —— 具体改哪里、怎么改（贴关键代码）
验收   —— 用什么断言/脚本证明改对了
风险   —— 改坏了会怎样，怎么兜
```

### 0.3 三条不可违反的约束

1. **快照纪律**：`Dev/MVP` 是源码事实源；`war_of_desert/engine/oracle/` 是只读快照。任何规则改动都必须同时升 `rulesHash` 与协议版本，不允许「只改 MVP 就宣称正式赛变了」。
2. **不改历史数据**：`Dev/MVP/formal-match-tests/archive/`、`algorithm-contest/tests/history-seasons/formal/` 下的既有结果是历史证据，只读。新规则的新结果写到新目录。
3. **先证伪再调参**：任何「数值不合理」的判断，必须先有探针（probe bot / 脚本）证明「在当前数值下不可行」，再改数值。反例：RAILGUN 320 CU 看似贵，但 1200 tick 的总收入约 2300～6000 CU，真正原因是参考 Bot 不造它——这种情况改 Bot，不改数值。

---

## 1. 基线事实表

### 1.1 代码规模

| 文件 | 行数 | 职责 | 状态 |
|---|---:|---|---|
| `referee/rules.js` | 1,740 | 规则、单位表、科技、`Game`（56 个方法）、计分卡 | 需拆分（D1） |
| `referee/referee.js` | 399 | 驱动、超时、红牌、确定性复核 | 稳定 |
| `referee/mapgen.js` | 1,479 | 地貌生成、VP/资源点布点、对称化 | 需改（A1） |
| `referee/mapgen_mc.js` | 442 | 另一套生成器（MC 风格） | 保留，不用于排名赛 |
| `referee/ballistics.js` | 266 | 弹道（RK4）、伤害、弹种 | 需校准（A7） |
| `referee/match.js` | 367 | 浏览器渲染层 | 改为回放播放器（E7） |
| `referee/tournament.js` | 179 | 循环赛 + 积分榜 | 需替换为 BT（C3） |
| `referee/bots/tactical.js` | 289 | 参考 Bot A（BFS 认领 + 科技） | 需扩（F1） |
| `referee/bots/greedy.js` | 80 | 参考 Bot B（暴兵） | 需扩（F1） |
| `formal-match-tests/workspace/arena.js` | 371 | 本地 runner（check/play/batch/full） | 稳定 |
| `test/*.js` | 2,855 | 8 个套件 | 需扩（§12） |
| `referee/match_test.js` / `web_smoke.js` | 524 / 135 | 裁判 / 渲染套件 | 稳定 |

### 1.2 运行时常量（`referee/rules.js:34-64`）

| 常量 | 值 | 含义 | 备注 |
|---|---:|---|---|
| `GRID` | 256 | 网格边长（格） | |
| `CELL` | 6 | 每格像素 | `WORLD = 1536 px` |
| `SCALE` | 4 | `GRID/64`，v0.10 → v0.11 的格密度倍数 | **所有「格」单位都被 ×4** |
| `TICK_MS` | 50 | 20 tick/s | |
| `TICK_LIMIT` | 1,200 | 单局 60 s | v0.11 由 600 上调 |
| `DECIDE_MS` | 120 | 单次决策预算 | |
| `INIT_MS` | 2,000 | 开局预算 | |
| `ORDER_LATENCY` | 1 | 指令晚 1 tick 生效 | |
| `MAX_LINE` | 131,072 | 单行上限 128 KiB | hello 实测 113,525 B |
| `PROTO.version` | 5 | 协议版本 | v1.0 升 6（D2） |
| `VP_RADIUS` | 100.8 px | = 16.8 格 | 半径偏大，见 A1 |
| `RES_RADIUS` | 57.6 px | = 9.6 格 | |
| `REPAIR_RADIUS` | 96 px | = 16 格 | |
| `MODE_RETREAT_CELLS` | 8 格 | 名义 2 格 | 同样被 ×4 |

### 1.3 经济与单位

| 常量 | 值 |
|---|---:|
| `START_CU` | 500 |
| `BASE_INCOME` | 1.5 CU/tick（1200 tick = 1,800 CU） |
| `SALVAGE_RATE` | 2.0 CU/tick/采集车 |
| `RES_AMOUNT` / `RES_RADIUS` | 60 次 / 9.6 格 |
| `POP_MAX` | 24 |
| `WEAPON_COST` / `BUILD` / `QUEUE_MAX` | 120 CU / 20 tick / 5 门 |

| 单位 | HP | 速度（格/s） | 射程（格） | 伤害 | ROF（tick） | 造价 | 建造 | 人口 |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| LAV | 90 | 12.8 | 12.8 | 8 | 5 | 100 | 25 | 2 |
| AAV | 260 | 7.6 | 12.0 | 14 | 6 | 240 | 45 | 3 |
| RAILGUN | 110 | 6.4 | 38.0 | 60 | 20 | 320 | 60 | 3 |
| ARTILLERY | 150 | 5.6 | 56.0 | 45 | 30 | 950 | 90 | 5 |
| SALVAGER | 130 | 8.8 | 0 | 0 | 0 | 100 | 20 | 1 |
| SCOUT | 40 | 18.4 | 0 | 0 | 0 | 80 | 15 | 1 |
| CARRIER | 800 | 6.4 | 20.0 | 10 | 15 | 300 | 50 | 4 |
| MOTHERSHIP | 3,000 | 2.2 | 28.0 | 22 | 12 | — | — | 0 |

> 表内速度/射程已含 `SCALE`。造价/血量不受影响。

### 1.4 科技（20 节点）

| 树 | 节点 | 总 tick | 总 CU |
|---|---|---:|---:|
| fire（3 级线性） | F1→F2→F3 | 460 | 440 |
| armor（3 级线性） | A1→A2→A3 | 460 | 440 |
| maneuver（5 级交叉） | M1→M2a/b→M3a/b→M4→M5 | 840 | 800 |
| industry（5 级交叉） | I1→I2a/b→I3a/b→I4→I5 | 840 | 800 |

### 1.5 计分卡（`rules.js:1435-1460`）

| # | 域 | 权重 | 公式（现行） | v0.11 实测均值 | 结论 |
|---:|---|---:|---|---:|---|
| 0 | VP 控制 | 25% | `100·vpTicks/(vps·turn)` | 31.68 | 有效 |
| 1 | 歼敌 | 15% | `100·killValue/敌 initialCost` | 3.82 | 分母错 |
| 2 | 保存兵力 | 15% | `100·aliveCost/producedCost` | 96.18 | 近常数 |
| 3 | 母舰完整度 | 15% | `100·hp/maxHp` | 100.00 | **常数** |
| 4 | KD 差 | 10% | `clamp(50+10·(kills−losses),0,100)` | 50.00 | 近常数 |
| 5 | 战场控制 | 5% | `100·controlTicks/(turn·vps·1.5)` | 91.64 | 偏常数 |
| 6 | 推进效率 | 5% | `100·(vpScore+killValue/60)/(turn/6)` | 95.03 | 与 VP 共线 |
| 7 | 算力表现 | 10% | `100·(1−avgMs/limit−罚)` | 99.96 | **常数** |

`Σ权重=1.00`；约 **49.2 分是常数/准常数**；`corr(VP, 总分)=0.647`。

### 1.6 实测（v0.11，50 seed / 100 局）

| 指标 | 值 |
|---|---|
| 胜负 | Tactical 92 / Greedy 8 / 平 0 |
| 判定层级 | L2 100 / L1 0 / L5 0 |
| 开火 / 命中 | 2,051 / 1,776（86.6%） |
| 击杀 | 171（44 局有击杀，单局最多 16） |
| 分 Bot | Tactical 126 发 / 55 杀；Greedy 1,925 发 / 116 杀 |
| VP 差 | p10 204 / 中位 895 / p90 1,582（满分 2,400） |
| 机制 | 5 兵种、7/20 科技、220,315 单位帧全 `obey` |

### 1.7 hello 体积（`R.Game.header()` 实测）

| 层 | wire 字节（现） | RLE 流字节 | 原始格数 | deflate-9 | deflate+b64 |
|---|---:|---:|---:|---:|---:|
| `map.base` | 9,394 | 7,042 | 65,536 | 3,601 | 4,804 |
| `map.surf` | 10,250 | 7,686 | 65,536 | 3,959 | 5,280 |
| `map.elev` | 87,386 | 65,539 | 65,536 | 27,955 | 37,276 |
| **header 合计** | **113,525** | — | — | — | **≈53,855** |

**关键发现**：`elev` 是噪声层（0～255 逐格起伏），RLE 几乎压不动——65,536 字节的原始网格编出 65,539 字节的 RLE 流，再过 base64 变 87,386 字节，**比原图还大 33%**。`base`/`surf` 成片，RLE 有效（65,536 → 7,042 / 7,686）。

换成三层统一 deflate 后：hello 从 **113,525 B 降到 ≈53,855 B（52.6 KB）**，直接回到 `Plan_Beta` 原本的 64 KiB 契约内，不需要分块，也不需要降地图精度。

---

## 2. 改造目标与验收口径

### 2.1 定位决策（必须先定）

| 选项 | 内容 | 需要的投入 | 建议 |
|---|---|---|---|
| A. 正式赛季 | 多集、多模型、长期排名 | 3～6 个月 + 参赛方 | 不现实 |
| **B. 社区竞赛（规则引擎 + 评测环境）** | 1 集、可复现、可扩展 | **7～9 周**（见 §13） | **推荐** |
| C. 技术提案 | 只交引擎与测试 | 2 周 | 备选 |

**选 B**：它保住了项目最强的部分（引擎、10 套件、覆盖率、manifest、本地 runner），又不需要假装已有赛季和模型榜。

### 2.2 Definition of Done（12 条硬指标）

| # | 维度 | 指标 | 当前 | 目标 | 验收脚本 |
|---:|---|---|---:|---:|---|
| 1 | 玩法 | 斩舰（L1）占比 | 0% | **10%～25%** | `scripts/accept_l1.js` |
| 2 | 玩法 | Tactical:Greedy 胜率差（120 局） | 92:8 | **≤40** | `scripts/accept_balance.js` |
| 3 | 玩法 | 出现过的可生产兵种 | 4/7 | **≥6/7** | `scripts/accept_coverage.js` |
| 4 | 玩法 | 出现过的科技节点 | 7/20 | **≥12/20** | 同上 |
| 5 | 玩法 | 行动模式覆盖 | 1/3 | **3/3** | 同上 |
| 6 | 评测 | 常数域合计 | 49.2 分 | **≤15 分** | `scripts/accept_scoring.js` |
| 7 | 评测 | `corr(VP, 总分)` | 0.647 | **< 0.80**（且 >0.4） | 同上 |
| 8 | 评测 | 每域标准差 | 母舰/算力 0 | **全部 ≥5** | 同上 |
| 9 | 赛制 | 每人局数 | 2 | **≥400** | `standings.json` |
| 10 | 赛制 | 前 5 名 95% CI 宽度 | 无 | **< 120** | `rating/bt.js` |
| 11 | 协议 | `hello` 字节数 | 113,525 | **≤ 65,536** | `test/protocol_test.js` |
| 12 | 引擎 | 单局耗时（单核） | ≈0.36 s | **≤1.0 s** | `scripts/bench.js` |

> 12 条全绿才允许打 `v1.0-candidate`。任一条不达标，按 §14 的降级方案处理。

### 2.3 明确不做（控制范围）

- 不做人类可玩版本（无 UI 操作、无音效）。
- 不做 512² 地图（先解决 256² 的编码与性能，512² 是 v1.1）。
- 不做服务器/匹配系统（本地 runner + 批处理即可）。
- 不做 Go 引擎重写（`war_of_desert/` 只修口径，不追规则）。
- 不做多模式（攻守 `Plan_Siege` 仍在 `experiments/`，不随本次投稿）。

---

## 3. 总体路线与依赖

### 3.1 阶段图

```text
阶段0 清理口径(0.5w)
   │
   ├──> 阶段1 玩法(A1 VP ─> A2 斩舰 ─> A3 击杀收益)
   │         │                │
   │         └──> A4 兵种 ──> A5 科技 ──> A6 模式 ──> A7 弹道
   │                          │
   ├──> 阶段2 计分卡(B1 公式 ─> B2 门禁 ─> B3 迁移)
   │         │
   ├──> 阶段3 赛制(C1 地图池 ─> C2 局数 ─> C3 BT ─> C4 并列 ─> C5 判罚 ─> C6 schema)
   │
   ├──> 阶段4 引擎(D1 拆分 ─> D2 协议v6 ─> D3 hello压缩 ─> D4 性能 ─> D5 确定性 ─> D6 沙箱)
   │
   └──> 阶段5 交付(E1 目录 ─> E2 许可 ─> E3 文档 ─> E4 CONDITIONS ─> E5 prompt
                  ─> E6 workspace/challenge ─> E7 回放 ─> E8 manifest/CI)
                          │
                     阶段6 试赛(F1 三个Bot ─> F2 模型试赛 ─> F3 校准)
```

### 3.2 关键路径

`A1 → A2 → B1 → C3 → E1 → E2 → F2`。

- **A1 必须先做**：VP 半径 16.8 格会让后面所有平衡测试都测不准。
- **B1 依赖 A1/A2**：计分卡的方差要用新玩法才测得出来。
- **C3 依赖 B1**：BT 的输入是胜负，输出的稳定性依赖「胜负不再由常数分决定」。
- **E2 是投稿阻断项**：没有 LICENSE，前四项做完也不能发布。

### 3.3 里程碑

| 里程碑 | 内容 | 判定 |
|---|---|---|
| M0 | 阶段 0 完成 | `npm test` 绿 + 口径表签字 |
| M1 | 玩法闭环 | A1～A3 验收通过，L1 ≥10% |
| M2 | 评测可信 | B1～B3 验收通过，常数 ≤15 分 |
| M3 | 赛制可跑 | 14 个模拟 Bot 跑完 3,640 局，出 BT 榜 |
| M4 | 包可交付 | E1～E8 全绿，`challenge.js` 一键复现 |
| M5 | 试赛完成 | 4 个模型 ×80 局，事故表为 0 阻断项 |

---

## 4. 阶段 0 · 前置清理（0.5 周）

### 0.1 统一「格」口径（阻断项）

**现状**：对外单位有三套并存——像素（`vpRadius`）、格（`vpRadiusCells`）、v0.10 格（`SCALE` 前的语义）。文档写「4.2 格」，实际 `VP_RADIUS/CELL = 16.8`。Go 版曾因此把 `range=12.8` 当像素用，射程小 6.3 倍。

**目标**：**协议以「格」为唯一距离单位**，像素只在渲染层出现。

**改法**：
1. `rules.js` 的 `header()`（约 918-930 行）增加显式单位声明：

```js
rules: {
  units: { distance: 'cell', speed: 'cell/s', time: 'tick', money: 'CU' },
  cell: CELL,               // 每格像素，仅渲染用
  scale: SCALE,             // 格密度倍数，已在数值内部生效
  vpRadiusCells: VP_RADIUS / CELL,
  resRadiusCells: RES_RADIUS / CELL,
  repairRadiusCells: REPAIR_RADIUS / CELL,
  retreatCells: MODE_RETREAT_CELLS,
  ...
}
```
2. 删除 `header.rules.vpRadius`（像素值）或改名 `vpRadiusPx` 并标 `deprecated`。
3. `workspace/RULES.md`、`CONDITIONS.md`、`赛事手册.md` 同步：所有距离一律「格」，并给换算公式 `px = cell × 6`。

**验收**：
- `test/protocol_test.js`：断言 `header.rules.units.distance === 'cell'`，且 `vpRadiusPx` 不存在或带 `deprecated: true`。
- 全仓 grep `vpRadius` 只剩 `vpRadiusCells` 与 `vpRadiusPx`。
- Go `botkit` 的 `Type.range` 注释改为「格」，`Stride` 用 `speed*cell*0.1`（已在 v0.11 修）。

**风险**：改动协议字段会换 `rulesHash`，旧回放无法复核。缓解：`rulesHash` 里加入 `units` 声明，旧数据标注 `proto:5` 不可比。

---

### 0.2 修掉两处 600 tick 残留

**现状**：
- `referee/tournament.js:26` `DEFAULTS.tickLimit = 600`（生产默认值，与 v0.11 的 1200 不一致）。
- `referee/match.js:23` `tickLimit: window.WOD_TICK_LIMIT || 600`（浏览器演示默认）。

**改法**：
```js
// tournament.js
tickLimit: Ref.R ? Ref.R.TICK_LIMIT : 1200,
```
```js
// match.js
tickLimit: (typeof window !== 'undefined' && window.WOD_TICK_LIMIT) || 1200,
```
其余测试里显式传 `tickLimit: 600` 的用例**保留**——它们是「短局契约」测试，故意用短局跑得快。在注释里写清「刻意短局，不代表正式时长」。

**验收**：`node referee/tournament.js` 输出的 `turns` 达到 1200；`node test/all.js` 10/10。

**风险**：循环赛耗时翻倍（约 2 倍）。缓解：测试用例继续用 600；只有 CLI 默认变。

---

### 0.3 归档 `_archive_skeleton`

**现状**：`Dev/MVP/_archive_skeleton/` 里是上一轮的 `README_root`、`war_desert/` 等骨架，含大量 600 tick / 10 TPS 的过期口径，容易被误读为现行规范。

**改法**：整目录移动到 `Dev/archive/_archive_skeleton/`，在 `Dev/archive/README.md` 登记一行「已被 v0.11 取代，只读」。

**验收**：`grep -r "600 tick" Dev/MVP --include=*.md` 不再命中（除测试注释与历史归档）。

---

### 0.4 建立术语表与单位卡

新增 `Dev/docs/术语.md`，固定 12 个词：格、像素、tick、逻辑秒、预算、红牌、判定层级（L1～L5）、快照、`rulesHash`、`manifest`、收卷、冻结。

新增 `Dev/docs/单位卡.md`：一张 A4 大小的速查表（§1.2 + §1.3 的内容），选手包与评审各一份。

**验收**：两份文档被 `Dev/README.md` 索引；`Dev/MVP/formal-match-tests/workspace/RULES.md` 首段链接到单位卡。

---

### 0.5 阶段 0 出口条件

- [ ] 0.1 单位口径统一，协议字段冻结
- [ ] 0.2 两处 600 tick 修完
- [ ] 0.3 `_archive_skeleton` 归档
- [ ] 0.4 术语表 + 单位卡
- [ ] `npm run check && node test/all.js` 10/10
- [ ] `git tag baseline/v0.11-units`（回滚锚点）

---

## 5. 阶段 1 · 玩法层（1.5 周）

> 目标：让「占点 / 交战 / 斩舰」三条胜利路径都真实存在，且互相制衡。

### A1 · VP 几何重做（最高优先）

**现状**
- 4 个 VP，两两成对（`mapgen.js:1188`：`for (const p of pairs) vpCells.push(p[0], p[1])`）。
- 半径 `VP_RADIUS = 4.2·CELL·SCALE = 100.8 px = 16.8 格`（`rules.js:63`）。
- 占领是**瞬时**的：`updateVP()`（`rules.js:1413-1430`）每 tick 数人头，`c0>0 && c1==0` 即归零方，**没有占领进度**。
- 后果：16.8 格 = 地图宽度的 6.6%，一个路过的 LAV 就能「占住」；双方各拿各的，`vpTicks` 单调累积，`VP 差中位 895`。

**目标**

| 项 | 现值 | 目标值 | 理由 |
|---|---:|---:|---|
| VP 数量 | 4 | **3**（1 中 + 2 侧） | 中心点制造必争之地，侧翼保留分兵选择 |
| 控制半径 | 16.8 格 | **6 格**（36 px） | 必须驻留，不能路过 |
| 占领方式 | 瞬时 | **进度 60 tick（3 s）** | 允许反抢，制造「守点」决策 |
| 争夺（双方都在圈内） | 直接 `owner=-1` | **进度冻结**（不回退） | 避免拉锯抖动 |
| 中心点到双方出生点 | — | 距离差 **≤2 格** | 公平性断言 |
| VP 到出生点最近距离 | 实测 39～123 格（中位 75） | **≥40 格**（现状已满足，作回归约束） | 防止开局秒占 |

**改法**

1. `mapgen.js` 新增 `placeVP3(map, seen, rng)`，替换 1188 行的成对布点：

```js
function placeVP3(map, seen, rng) {
  const cands = [];
  for (let gy = 24; gy < GRID - 24; gy++) {
    for (let gx = 24; gx < GRID - 24; gx++) {
      if (!flatEnough(map, gx, gy, 7)) continue;      // 7×7 内高差 ≤ 6
      if (!seen[ci(gx, gy)]) continue;
      if (nearRes(map, gx, gy, 6)) continue;
      cands.push({ gx, gy });
    }
  }
  // 中心：到两个出生点距离差最小，且 ≥ 地图宽度 25%
  const s0 = map.spawns[0], s1 = map.spawns[1];
  cands.sort((a, b) =>
    Math.abs(d2(a, s0) - d2(a, s1)) - Math.abs(d2(b, s0) - d2(b, s1)));
  const center = cands.find(c => d2(c, s0) > GRID * 0.25) || cands[0];
  // 侧翼：与中心、出生点都拉开，且左右镜像
  const wings = mirrorWings(center, cands);           // 关于中心-出生点轴镜像
  return [center, wings[0], wings[1]].map(c => [c.gx, c.gy]);
}
```

2. `rules.js` 的 `VP_RADIUS` 改为 `6 * CELL`（36 px），并新增：

```js
const VP_CAPTURE_TICKS = 60;   // 3 s
```

3. `updateVP()` 重写为带进度的版本：

```js
updateVP() {
  for (const vp of this.vps) {
    let c0 = 0, c1 = 0;
    for (const u of this.units) {
      if (u.dead || u.kind === 'MOTHERSHIP') continue;
      if (Math.hypot(u.x - vp.x, u.y - vp.y) <= VP_RADIUS) (u.side ? c1++ : c0++);
    }
    const contest = c0 > 0 && c1 > 0;
    vp.contested = contest;
    vp.presence = [c0, c1];                 // 新字段，进 tick 广播
    if (contest || (c0 === 0 && c1 === 0)) { /* 冻结，不增不减 */ }
    else {
      const s = c0 > 0 ? 0 : 1;
      if (vp.owner === s) vp.progress = VP_CAPTURE_TICKS;
      else if (vp.owner === 1 - s) {
        vp.progress -= 1;                    // 先中和
        if (vp.progress <= 0) { vp.owner = -1; vp.progress = 0; }
      } else {
        vp.progress += 1;
        if (vp.progress >= VP_CAPTURE_TICKS) {
          vp.owner = s; vp.progress = VP_CAPTURE_TICKS;
          this.emit({ t: this.turn, type: 'vp', id: vp.id, owner: s });
        }
      }
    }
    if (vp.owner >= 0) { this.sides[vp.owner].vpScore++; this.sides[vp.owner].vpTicks++; }
  }
}
```

4. `header().vp[]` 增加 `progressTicks`；`encodeTick().vp[]` 增加 `owner`、`progress`（0～60）、`presence`（双方人数）。

**验收**（新增 `test/vp_test.js`）
- 3 个 VP；中心点到两出生点距离差 ≤2 格（对 32 个 seed 全成立）。
- 单个 LAV 进入空点，第 59 tick `owner` 仍为 -1，第 60 tick 变成本方。
- 敌方进入后 `progress` 从 60 开始下降，降到 0 时 `owner=-1`（中和），再 60 tick 才易主。
- 双方同时进圈：`progress` 不变（冻结），`contested=true`。
- 控制半径断言：距 VP 6.0 格的单位计入，6.1 格不计入。

**风险**
- 3 点可能加剧滚雪球（少一个点可偷）。缓解：占领 3 s + 争夺冻结，偷点需要真实兵力；A3 再给落后方补正。
- 60 tick 占领会让对局更长（当前 100% 打满 1200）。缓解：VP 分值不变，改的是「拿不拿得住」，不是「拿多久」。

---

### A2 · 让斩舰成为真实威胁

**现状**
- 100 局 **0 次 L1**（母舰被毁），L2 占 100%。
- 母舰 3,000 HP、`attack=35%` 时 1 门炮（`weaponCapOf`）、22 伤害 / 12 tick。
- LAV 对母舰 `RPS=0.8`，单发 6.4；打满 3,000 HP 需要 **469 发**。

**数值推导（必须先算清再改）**

| 场景 | 计算 | 结果 |
|---|---|---|
| 4 辆 LAV 持续输出 | 4 × 8 × 0.8 / 0.25 s | 102 dps |
| 打空 3,000 HP | 3000 / 102 | **29.4 s**（理论可行） |
| 母舰反击 | 22 / 0.6 s | 36.7 dps，2.5 s 杀 1 辆 LAV |
| 实际 | 母舰在出生点、Bot 不 targeting | **0 次尝试** |

结论：**不是数值不可达，是行为不可达**——没有任何 Bot 把母舰当目标，母舰也不在前线。所以 A2 要同时改「行为诱导」与「数值窗口」。

**方案对比**

| 方案 | 改动 | 优点 | 缺点 | 采用 |
|---|---|---|---|---|
| a. 降 HP | 3000 → 1800 | 一行改动 | 只改数值，Bot 仍不打 | 部分 |
| b. 提 RPS | LAV 0.8 → 1.0；ARTILLERY 0.8 → 2.5 | 让攻城单位有专属定位 | 正面战场也会变 | 是 |
| c. 侧后弱点 | 从母舰后半球命中 ×1.5 | 奖励包抄，考空间推理 | 需要朝向判定 | 是 |
| d. 护盾阶段 | HP<50% 时武器槽 −50%、速度 ×0.5 | 给「打残」正反馈，不是二元 | 实现成本中等 | 是 |
| e. 攻城模式 | 新增 `siege` 单位 | 新机制 | 超出本次范围 | 否 |

**目标**

| 项 | 目标 |
|---|---|
| 母舰 HP | **2,000** |
| LAV / AAV 对母舰 RPS | 0.8 → **1.0** |
| ARTILLERY 对母舰 RPS | 0.8 → **2.5**（攻城定位） |
| 侧后命中 | ×1.5（以母舰朝向为基准 ±90° 外） |
| 护盾阶段 | `hp < 50%` 时：武器槽 ×0.5、移速 ×0.5、修复速率 ×2 |
| L1 占比 | **10%～25%** |

**改法**

1. `TYPES.MOTHERSHIP.hp`: `3000 → 2000`。
2. `RPS` 表：`LAV.MOTHERSHIP: 0.8 → 1.0`、`AAV.MOTHERSHIP: 0.8 → 1.0`、`ARTILLERY.MOTHERSHIP: 0.8 → 2.5`。
3. `hitUnit()`（约 1340-1360 行）加入侧后判定：

```js
function facingMul(src, target) {
  if (target.kind !== 'MOTHERSHIP') return 1;
  const ang = Math.atan2(src.y - target.y, src.x - target.x);
  let d = Math.abs(((ang - target.heading + Math.PI) % (2 * Math.PI)) - Math.PI);
  return d > Math.PI / 2 ? 1.5 : 1.0;      // 后半球 ×1.5
}
```
> 母舰需要新增 `heading` 字段（跟随移动方向，静止时保持上次朝向），并进 `hello`/`tick` 广播。

4. 护盾阶段在 `weaponCapOf` / `speedMulOf` 调用处判定：

```js
const damaged = cv && cv.hp < cv.maxHp * 0.5;
const slots = damaged ? Math.floor(weaponCapOf(a.attack) / 2) : weaponCapOf(a.attack);
const spd = damaged ? 0.5 : 1;
```

**验收**（新增 `test/siege_test.js`）
- 4 辆 LAV 贴脸打静止母舰：≤32 s 打空 2,000 HP。
- 侧后命中伤害 = 正面 ×1.5（±2%）。
- HP 降到 999 时武器槽从 1 变 0、移速 ×0.5。
- 探针 Bot `RushMS`（全军直冲母舰 + 简单绕后）在 20 局里 **≥2 局 L1**。
- 常规 Bot 对战 120 局：L1 占比 **10%～25%**（>30% 说明太脆，<8% 说明诱导失败）。

**风险**
- 斩舰过快 → 对局变成「谁先 rush 谁赢」。缓解：护盾阶段 + 母舰 1 门炮的反击 + 出生点防空（母舰周围 8 格内敌方单位受伤 ×1.2，可选项）。
- 侧后判定需要 `heading`，会改协议。缓解：`heading` 进 `tick` 的 `units[]`，协议 v6 一起改（D2）。

---

### A3 · 让击杀有战略收益

**现状**：`killValue` 只进两个计分域（`rules.js:1354`），不影响战局。Greedy 1,925 发 / 116 杀 / 8 胜——**击杀与胜负脱钩**。

**目标**：击杀能转化为「经济 → 兵力 → VP」的正循环，让「会打仗的 Bot」有胜率。

**方案（三选一先做 A/B 测试，不要全上）**

| 方案 | 机制 | 预期效果 | 实现成本 |
|---|---|---|---|
| **A. 残骸回收** | 击杀掉落 `30% × cost` CU（单次上限 60），需 SALVAGER 或单位停留 2 s 拾取 | 强化「打扫战场」，SALVAGER 有用了 | 低 |
| B. 人口压制 | 击杀使对方 `POP_MAX` 临时 −1（60 s 恢复） | 直接削弱暴兵 | 中 |
| C. VP 挂钩 | 占领速度 ∝ 己方在圈内单位数 / 对方 | 兵力优势转 VP | 中 |

**推荐先做 A**（最低侵入），跑 120 局看 Greedy 胜率是否从 8% 升到 ≥35%；不够再叠 B。

**改法（方案 A）**
1. `this.sides[bySide].killValue += TYPES[e.kind].cost;` 之后追加：

```js
const drop = Math.min(60, Math.round(TYPES[e.kind].cost * 0.3));
this.wrecks.push({ x: e.x, y: e.y, cu: drop, t: this.turn, side: bySide });
```
2. `step()` 里处理拾取：任一己方非母舰单位在 1.5 格内停留 1 tick 即拾取；SALVAGER 半径 3 格。
3. `encodeTick` 增加 `wrecks: [{x,y,cu}]`（只广播视野内的）。

**验收**
- `test/economy_test.js`：击杀 1 辆 LAV（100 CU）产生 30 CU 残骸；SALVAGER 3 格内 1 tick 拾取；敌方不能拾取。
- 120 局：Greedy 胜率 **≥35%**，Tactical 仍 ≥50%（不能一边倒）。
- `Σ killValue` 与 `Σ 拾取 CU` 对账一致（无凭空造钱）。

**风险**：残骸让雪球更快。缓解：单次上限 60 CU、总量上限 `POP_MAX×20`；若 Greedy 胜率 >65%，把掉落降到 20%。

---

### A4 · 兵种可用性：先证伪，再调数值

**现状**：7 个可生产兵种，实测只出现 LAV/AAV/SALVAGER/CARRIER（+母舰）；RAILGUN / ARTILLERY / SCOUT **0 次**。

**第一步：证伪（不改数值）**
写 3 个探针 Bot，各专造一种，各跑 20 局（seed 20261201 起）：

| 探针 | 策略 | 观察 |
|---|---|---|
| `ProbeRail` | 开局 2 采集 + 1 兵营，直冲 RAILGUN，攒 3 辆后远程点名 | 造得出吗？打得到吗？ |
| `ProbeArty` | 同上，攒 ARTILLERY | 950 CU 在 1200 tick 内够吗？ |
| `ProbeScout` | 4 SCOUT 开图 + LAV 跟进 | 视野收益能转化为胜率吗？ |

**判据**：
- 若探针在 1200 tick 内**造不出**该单位 → 数值问题，按第二步调。
- 若能造出但**打不出效果** → 机制/定位问题，改单位属性或参考 Bot。
- 若能造出且有效，但参考 Bot 不造 → **改 Bot（F1），不改数值**。

**第二步：仅在「造不出」时调数值**

| 单位 | 现价 | 建议价 | 现建造 | 建议 | 理由 |
|---|---:|---:|---:|---:|---|
| RAILGUN | 320 | **260** | 60 | 50 | 3 辆成型时间从 180 tick 降到 150 |
| ARTILLERY | 950 | **620** | 90 | 70 | 总收入约 2,300～6,000 CU，950 占比过高 |
| SCOUT | 80 | **60** | 15 | 12 | 侦察应该便宜到「随手造」 |

**验收**：120 局中 ≥6/7 兵种出现，且每个兵种至少出现在 5% 的对局里。

**风险**：降价导致暴兵。缓解：`POP_MAX 24` 不变，用人口卡住；改价后重跑 A3 的平衡验收。

---

### A5 · 科技树与 60 秒的匹配

**现状**：20 节点；单条 5 级路线需 840 tick + 800 CU；1200 tick 内只走到 7 个节点。研究槽最多 3（`produce/research` 15% → 3 槽）。

**测算**

| 路线 | tick | CU | 占 1200 tick |
|---|---:|---:|---:|
| fire 3 级 | 460 | 440 | 38% |
| maneuver 5 级 | 840 | 800 | 70% |
| 三条并行（3 槽） | 最长 840 | 1,240 | 可行但没钱 |

**目标**：12/20 节点在 1200 tick 内可达；单条 5 级路线 ≤600 tick。

**改法（推荐「全线 −35% 时间、−25% 花费」）**

| 节点 | 现 tick / CU | 新 tick / CU |
|---|---|---|
| F1 / A1 / M1 / I1 | 90 / 80（M1、I1 为 80/70） | **55 / 60**（M1、I1 为 50/55） |
| F2 / A2 / M2 | 150 / 140（M2 为 120/110） | **95 / 105**（M2 为 80/85） |
| F3 / A3 / M3 | 220 / 220（M3 为 160/160） | **140 / 165**（M3 为 110/120） |
| M4 / I4 | 210 / 200 | **140 / 150** |
| M5 / I5 | 270 / 260 | **180 / 195** |

改完后：fire 3 级 = 290 tick；maneuver 5 级 = 560 tick；三条并行 560 tick 可完成。

**验收**：`scripts/accept_coverage.js` 统计 120 局，出现过的节点 ≥12；且「研究完成事件」的中位时间 ≤700 tick。

**风险**：科技太快导致后期同质化。缓解：CU 只降 25%，保持「研究 vs 造兵」的取舍。

---

### A6 · 三种行动模式必须被用上

**现状**：220,315 个单位帧**全部 `obey`**。引擎已实现 `neutral`（遇敌停步、被压制后撤）与 `aggressive`（自动索敌追击），但没有 Bot 使用。

**目标**：3/3 模式在正式对局中出现；`aggressive` 在至少 1 个参考 Bot 里是主模式。

**改法**
1. 参考 Bot 扩展（F1）：`Vanguard` 用 `aggressive` 的前锋 + `obey` 的主力；`Raider` 用 `neutral` 做骚扰。
2. `botkit`（Go）与 JS 模板里给出模式切换的最小示例：

```js
// 前 3 辆 LAV 设为 aggressive 做尖兵，其余 obey
for (const u of myUnits) {
  const want = (u.kind === 'LAV' && idx++ < 3) ? 'aggressive' : 'obey';
  if (u.mode !== want) actions.push({ id: u.id, type: 'mode', mode: want });
}
```

**验收**：`scripts/accept_coverage.js` 统计 `units[].mode` 三种都出现；`aggressive` 单位的存活时间中位数不低于 `obey` 的 80%（不能是送死）。

**风险**：`aggressive` 会让单位脱队。缓解：只在尖兵上用；用 A6 的验收卡住「存活时间」。

---

### A7 · 弹道与视野校准

**现状**：命中率 1,776/2,051 = **86.6%**（偏高，接近「指哪打哪」）；战争迷雾默认**关闭**，排名赛没有信息差。

**目标**

| 项 | 现值 | 目标 |
|---|---:|---:|
| 命中率（对移动目标） | 86.6% | **55%～75%** |
| 迷雾（排名赛） | 关 | **开** |
| 视野采样步长 | 0.5 格 | 0.5 格（不变） |
| 高地加成 | 弹道天然优势 | 保留 |

**改法**
1. `ballistics.js`：给 `v0` 加 ±3% 随机散布（用 `rng`，确定性种子），或把 `HIT_R_M` 从当前值下调 20%。
2. `referee.js` 的 `runMatchSync` 默认 `vision: true`（排名赛）；`arena.js check` 仍可用 `vision:false` 做快速自检。
3. `hello.rules.vision` 明确写 `defaultRanked: true`。

**验收**
- 120 局命中率落在 55%～75%。
- 开/关迷雾各 60 局，胜率变化 ≤5%（说明视野不是靠运气）。
- `test/vision_test.js` 增加「迷雾下 tick 里看不到敌方单位」的断言（已有类似用例，补 `defaultRanked`）。

**风险**：散布让高手 Bot 的瞄准失效。缓解：散布只作用于 `v0`，提前量算法仍有效；若命中率 <50%，把散布降到 ±1.5%。

---

### 5.1 阶段 1 出口条件

- [ ] A1 VP 三点的对称性与占领进度测试全绿
- [ ] A2 L1 占比 10%～25%
- [ ] A3 Greedy 胜率 ≥35%
- [ ] A4 ≥6/7 兵种出现
- [ ] A5 ≥12/20 科技出现
- [ ] A6 3/3 模式出现
- [ ] A7 命中率 55%～75%，迷雾默认开
- [ ] `git tag baseline/v0.12-gameplay`

---

## 6. 阶段 2 · 评测层（1 周）

> 目标：把「约 49 分是常数」的计分卡，改成能区分对局的 8 个域。

### B1 · 计分卡重写

**现状**：见 §1.5。核心问题三个：
1. **常数域**：母舰 15 分（100/100 局）、算力 10 分（99.96）、KD 5 分（50.00）、保存兵力 14.4 分（96.18）。
2. **分母错**：歼敌用 `敌 initialCost`，而 `initialCost` 随对手生产增长，导致早击杀被稀释。
3. **共线**：推进效率 = `(vpScore + killValue/60)/(turn/6)`，与 VP 域同源。

**新计分卡（8 域，权重合计 100%）**

| # | 域 | 权重 | 新公式 | 说明 |
|---:|---|---:|---|---|
| 0 | VP 控制 | **30%** | `100·vpTicks/(vps·turn)` | 不变，但 vps=3、含占领进度 |
| 1 | 歼敌效率 | **15%** | `100·killValue / max(1, 敌 producedCost)` | 分母改「对手总投入」 |
| 2 | 兵力保存 | **15%** | `100·aliveCost / max(1, initialCost + producedCost)` | 分母加初始，压掉「不造兵=满分」 |
| 3 | 母舰完整度 | **10%** | `100·hp/maxHp`，`hp` 取对局内最低值加权 | 让「被打残」可区分 |
| 4 | 交换比 | **10%** | `100·kills/(kills+losses)`，`0/0` 记 0.5 | 替代 KD 差，值域 0～1 |
| 5 | 战场控制 | **10%** | `100·uniqueHoldTicks/(vps·turn)`，争夺 tick ×0.5 | 用「唯一控制」替代人头累加 |
| 6 | 地图压强 | **5%** | `100·clamp((meanFrontX − ownBaseX)/(enemyBaseX − ownBaseX), 0, 1)` | 替代推进效率，独立于 VP |
| 7 | 决策预算 | **5%** | `100·clamp(1 − p95Ms/limit − 0.25·faults, 0, 1)` | 用 p95 不用均值；超时率 >1% 另判负 |

**改法（`rules.js:1435-1460` 的 `computeCards()` 整体替换）**

```js
computeCards() {
  const denom = Math.max(1, this.turn);
  const vps = this.vps.length;
  return [0, 1].map(s => {
    const S = this.sides[s], cv = this.mothership(s), foe = this.sides[1 - s];
    const alive = this.units.filter(u => u.side === s && u.kind !== 'MOTHERSHIP');
    const aliveCost = alive.reduce((a, u) => a + TYPES[u.kind].cost, 0);
    const p95 = this.botP95[s] || 0;
    const limit = this.cfg.decideMs;
    const faultPenalty = Math.min(1, (this.botFaults[s] || 0) * 0.25);
    const f = [
      100 * clamp(S.vpTicks / (vps * denom), 0, 1),
      100 * clamp(S.killValue / Math.max(1, foe.producedCost), 0, 1),
      100 * clamp(aliveCost / Math.max(1, S.initialCost + S.producedCost), 0, 1),
      100 * (cv ? this.msIntegrity(s) : 0),                 // 见下
      S.kills + S.losses === 0 ? 50 : 100 * S.kills / (S.kills + S.losses),
      100 * clamp(S.uniqueHoldTicks / (vps * denom), 0, 1),
      100 * clamp(this.frontPressure(s), 0, 1),
      100 * clamp(1 - p95 / limit - faultPenalty, 0, 1)
    ];
    let total = 0; for (let i = 0; i < 8; i++) total += f[i] * WEIGHTS[i];
    return { fields: f, total, vpScore: S.vpScore, kills: S.kills, losses: S.losses,
             botMs: this.botMs[s], botP95: p95 };
  });
}
```

配套改动：
- `this.sides[s]` 增加 `uniqueHoldTicks`（每 tick 统计「只被己方控制的 VP 数」的累加）。
- `this.botP95[s]` 在驱动层维护（滑窗 256 次的 p95）；`referee.js` 的 `botMs` 统计处扩展。
- `msIntegrity(s)`：`minHp/maxHp` 与「护盾阶段触发次数」的组合：

```js
msIntegrity(s) {
  const cv = this.mothership(s);
  if (!cv) return 0;
  const lowest = this.msLowestHp[s] != null ? this.msLowestHp[s] : cv.hp;
  return 0.6 * (lowest / cv.maxHp) + 0.4 * (cv.hp / cv.maxHp);
}
```
- `frontPressure(s)`：每 tick 记录「己方战斗单位平均 x」，对局结束取时间平均，再按出生点线性归一。

**WEIGHTS 改为**：`[0.30, 0.15, 0.15, 0.10, 0.10, 0.10, 0.05, 0.05]`（和 = 1.00）。

**预期效果（用 v0.11 数据代入的估算）**

| 域 | 旧均值/σ | 新预期均值/σ | 常数分变化 |
|---|---|---|---:|
| 母舰 | 100 / 0 | 82 / 18（被打残会掉） | 15 → 约 8 |
| 算力 | 99.96 / 0.04 | 96 / 6（p95 口径） | 10 → 约 4.8 |
| 保存 | 96.18 / 8.73 | 68 / 16 | 14.4 → 10.2 |
| KD/交换比 | 50 / 20.6 | 52 / 14 | 5 → 5.2 |
| 合计常数 | **49.2** | **约 28**（还需 A2/A3 产生真实方差） | 目标 ≤15 |

> 注意：**B1 单独做完只能降到 28 左右**。要到 15，必须配合 A2（母舰会被打）与 A3（击杀有收益）。这是为什么阶段 1 在阶段 2 之前。

**验收**（新增 `test/scoring_test.js`）
- 8 个域都在 `[0,100]`。
- 构造 4 个极端局面（速攻/龟缩/斩舰/平局），每域至少 1 个局面偏离 50 分 >20。
- `Σ weight = 1.0`。
- 200 方样本：常数域（σ<1）数量 ≤2。
- `corr(VP, total)` ∈ (0.4, 0.8)。

**风险**：新公式让「龟缩」也能高分。缓解：地图压强 + 歼敌效率两条腿；用 4 个极端局面测试卡住。

---

### B2 · 方差门禁（写进 CI）

新增 `scripts/accept_scoring.js`，对最近一次 200 方样本做门禁：

```js
const gates = [
  ['常数域 ≤2', () => fields.filter(f => stdev(f) < 1).length <= 2],
  ['每域 σ ≥ 5', () => fields.every((f, i) => i === 3 || stdev(f) >= 5)],
  ['常数分 ≤15', () => constantScore <= 15],
  ['corr(VP,total) < 0.8', () => Math.abs(corr(vp, total)) < 0.8],
  ['corr(VP,total) > 0.4', () => Math.abs(corr(vp, total)) > 0.4]
];
```

`npm run accept` 串起全部 12 条 DoD（§2.2），任一失败退出码 1；GitHub Actions 里跑。

**验收**：故意把 `WEIGHTS` 改回旧值，`npm run accept` 必须变红（门禁有效性测试）。

---

### B3 · 数据迁移与版本标记

- `result` 里新增 `scoreVersion: 2`、`rulesHash`、`proto: 6`。
- 旧结果目录标注：`Dev/MVP/formal-match-tests/archive/` 与 `algorithm-contest/tests/history-seasons/formal/` 首行加「scoreVersion:1，不可与 v2 比较」。
- `results/README.md` 说明「跨版本只比胜负，不比分数」。

**验收**：随机抽 3 个旧结果文件，`scoreVersion` 为 1；新跑的为 2。

---

### 6.1 阶段 2 出口条件

- [ ] B1 新计分卡上线，`test/scoring_test.js` 全绿
- [ ] B2 `npm run accept` 12 条全绿
- [ ] B3 版本标记完成
- [ ] `git tag baseline/v0.13-scoring`

---

## 7. 阶段 3 · 赛制与统计（1 周）

> 目标：从「2 局 smoke + 积分榜」升级到「固定地图池 + 400 局/人 + BT + CI + 并列」。

### C1 · 固定地图池

**现状**：`tournament.js` 用 `seeds: [20261109]`；正式测试 2 局 1 seed；每对只打 1 张图，地图是混淆变量。

**目标**：**16 张固定地图**，赛前冻结，所有对局共用。

**改法**：新增 `scripts/build_map_pool.js`

```js
// 从 64 个候选 seed 里筛 16 张
// 条件：① VP 对称（中心到两出生点距离差 ≤2 格）
//      ② 出生点间距 ∈ [p25, p75]（去掉过近/过远）
//      ③ 3 个 VP 均可达（BFS，从两个出生点都能到）
//      ④ 资源点 4 个，且每个出生点最近资源点距离差 ≤3 格
//      ⑤ 无出生点被山封死（可达格数 ≥ 地图 30%）
// 输出 maps/pool.json：{ version, seeds: [...], hash, metrics: [...] }
```

**验收**
- 16 张全部通过 5 条条件，输出 `metrics` 表（每张的对称误差、出生距、可达面积）。
- `test/map_pool_test.js`：池子哈希稳定；同一 seed 两次生成字节一致。

**风险**：地图太少导致过拟合。缓解：16 张已足够让「背图」失效；池子版本化，下季换池。

---

### C2 · 局数与对阵

**目标**

| 项 | 值 |
|---|---|
| 地图 | 16 张 |
| 对阵 | 双循环（每对 2 次）× 每张 2 个座位 |
| 每人局数（n=14） | `2 × 13 × 16 = 416` |
| 总场次 | `C(14,2) × 16 × 2 = 2,912` 局 |
| 单局预算 | 0.36 s（实测 12 局 4.36 s）→ 2,912 局 ≈18 min 单核 / ≈2.5 min 8 核 |

**改法**：`arena.js` 新增 `pool` 命令：

```bash
node arena.js pool --bots bots/*.js --maps maps/pool.json --out results/season.jsonl
```
内部按 `(map, pair, seat)` 三层循环，逐局写一行 JSONL（可断点续跑）。

**验收**：14 个模拟 Bot 跑完 2,912 局，`results/season.jsonl` 行数正确；中断后 `--resume` 能从最后一行继续。

---

### C3 · BT 强度模型

**现状**：`tournament.js` 是积分榜（胜 1 / 平 0.5 / 负 0，再按总分排序），没有 BT、没有区间。

**目标**：BT 定名次，Elo 只做展示。

**实现**：新增 `rating/bt.js`（零依赖）

```js
// Bradley-Terry：p(i 胜 j) = 1 / (1 + exp(-(r_i - r_j)/400))
// 用 MM 算法迭代：r_i ← 解 Σ_j n_ij / (r_i + r_j) = w_i
// 先验 N(0, 200²)；锚点：基准 Bot 固定 0
// bootstrap：按「地图」整块重采样 1000 次，取 2.5%/97.5% 分位
function fitBT(games, opts) { ... }
function bootstrap(games, maps, B = 1000) { ... }
```

**关键设定**

| 项 | 值 | 理由 |
|---|---|---|
| 比较单位 | 1 局 = 1 次比较 | 平局记 0.5 |
| 缩放 | 400 | 与 Elo 同刻度，便于阅读 |
| 锚 | 基准 Bot = 0 | 跨赛季可比 |
| bootstrap | 按地图整块，1000 次 | 地图是最大方差源，不能按局独立重采样 |
| CI | 95% | |

**验收**（新增 `test/rating_test.js`）
- 合成数据：3 个 Bot，真实强度 0 / 100 / 200，各 400 局，BT 估计误差 <30。
- 锚点：基准 Bot 的估计恒为 0。
- bootstrap：CI 覆盖真值 ≥90%（100 次重复）。
- 输入不足（<10 局/人）时返回 `insufficient: true`，不输出排名。

**风险**：BT 假设「可比性」，地图差异大时会偏。缓解：地图整块 bootstrap + 固定池。

---

### C4 · 并列与排序规则

**规则**
1. 按 BT 均值降序。
2. **相邻两名 CI 重叠 → 记并列**（同分位，显示 `T3`）。
3. 并列内部排序：VP 差中位 → 击杀数 → 种子（确定性）。
4. 前 3 名必须给出 CI；CI 宽度 >120 时标 `low-confidence`。

**验收**：合成「两个 Bot 真实同强度」的数据，输出必须是并列；`standings.json` 的 `rank` 字段允许重复。

---

### C5 · 判罚与故障

**现状**：`referee.js` 有 `maxStrikes=3` 与 `faults`，但赛制层没有「判负/重跑」规则。

**规则表**

| 事件 | 处理 | 记录字段 |
|---|---|---|
| 单次决策超预算 | 记 1 次 fault，本 tick 指令丢弃 | `faults[]` |
| 累计 3 次 fault | 本局判负（forfeit） | `reason: 'strikes'` |
| 崩溃 / 非零退出 | 本局判负；同 seed 重跑 ≤2 次 | `reason: 'crash'` |
| 非法输出（越界/NaN/超行） | 丢该条指令，记 fault | `rejected[]` |
| 确定性复核失败 | 该局作废，重跑同 seed ≤2 次；仍失败判 forfeit | `reason: 'nondeterministic'` |
| 技术故障（宿主） | 整场重跑，不计入选手 | `incident: true` |

**验收**：注入 4 类故障 Bot（超时/崩溃/越界/不确定），每种 10 局，判罚结果与规则表一致。

---

### C6 · 结果 schema

`results/season.jsonl` 每行：

```json
{"season":"2026Q4","map":3,"seed":20270103,"pair":["botA","botB"],"seat":0,
 "winner":0,"level":"L2","turns":1200,"vp":[812,655],"scoreVersion":2,
 "cards":[{"fields":[...],"total":63.2},{"fields":[...],"total":58.7}],
 "faults":[[],[]],"rejected":[0,2],"p95Ms":[11.2,38.4],
 "rulesHash":"...","proto":6}
```

`results/standings.json`：`[{rank, name, bt, ci:[lo,hi], games, wins, draws, losses, tieGroup}]`。

**验收**：`scripts/report.js` 能只读这两个文件生成 `成绩.md` + `成绩_加权总分.md`（沿用现有格式）。

---

### 7.1 阶段 3 出口条件

- [ ] C1 16 张地图池冻结，哈希入库
- [ ] C2 2,912 局跑通，可断点续跑
- [ ] C3 BT + bootstrap + CI 通过合成数据校验
- [ ] C4 并列规则生效，出现 ≥1 组并列
- [ ] C5 4 类故障判罚正确
- [ ] C6 `standings.json` 可生成成绩表
- [ ] `git tag baseline/v0.14-season`

---

## 8. 阶段 4 · 引擎与协议（1 周）

### D1 · 拆分 `rules.js`

**现状**：1,740 行、`Game` 56 个方法，单文件改一处要读全文。

**目标结构**

```text
referee/
├── rules.js            # facade：re-export 全部公开 API（保持 require 兼容）
├── engine/
│   ├── constants.js    # GRID/CELL/TICK_MS/WEIGHTS/...
│   ├── units.js        # TYPES/RPS/MODES
│   ├── tech.js         # TECHS/TECH_TREES
│   ├── map.js          # 地形读取、groundH、视线
│   ├── combat.js       # hitUnit/explode/弹道调用
│   ├── scoring.js      # computeCards
│   └── game.js         # Game 类（step/apply*/updateVP）
```

**约束**
- `rules.js` 的公开导出**一个不少**（`R.Game`、`R.TYPES`、`R.rulesHash`…）。用 `test/public_api_test.js` 固化导出清单。
- 拆分不改任何数值与行为：拆分前后跑同一 100 局，逐帧 `orderLog` 哈希必须一致。

**验收**：`node test/all.js` 10/10；100 局逐帧哈希与拆分前一致；`rules.js` ≤120 行。

---

### D2 · 协议 v6

**变更清单**

| 位置 | 字段 | 变更 | 兼容 |
|---|---|---|---|
| `hello.rules` | `units` | 新增单位声明 | 新 |
| `hello.rules` | `vpRadiusPx` | 删除（改 `vpRadiusCells`） | 破坏 |
| `hello.rules` | `vpCount`、`vpCaptureTicks` | 新增 | 新 |
| `hello.map` | `encoding: 'deflate'` | 新增，替代隐式 RLE | 破坏 |
| `tick.vp[]` | `progress`、`presence` | 新增 | 兼容 |
| `tick.units[]` | `heading` | 新增（母舰朝向） | 兼容 |
| `tick.wrecks[]` | `{x,y,cu}` | 新增 | 兼容 |
| `actions[]` | `mode` | 已有（v0.9），文档补 | 兼容 |
| `result` | `scoreVersion`、`p95Ms` | 新增 | 兼容 |

`PROTO.version = 6`；`rulesHash` 自动变化（含新字段）。

**验收**：`test/protocol_test.js` 校验字段存在/缺失；旧 v5 回放能被 `verifyDeterminism` 识别为「版本不兼容」而不是静默算错。

---

### D3 · hello 压缩（实测可省 53%）

**现状**：113,525 B；`elev` 的 RLE 反而膨胀到 87,386 B。

**改法**
1. `encodeMapLayer(buf)` 改为 `deflateRawSync(buf, {level:9}).toString('base64')`。
2. `hello.map.encoding = 'deflate'`；`decodeMapLayer` 按字段分流（兼容旧 `rle`）。
3. 三层实测：`base` 9,394→4,804、`surf` 10,250→5,280、`elev` 87,386→37,276。
4. header 预计 **113,525 → 53,855 B**。

**验收**
- `test/protocol_test.js`：`utf8Length(hello) <= 65536`；encode/decode 往返逐字节相等（3 个 seed）。
- 选手包 `botkit` 同步支持 deflate（Node 端 `zlib.inflateRawSync`；Go 端 `compress/flate`）。
- 旧 RLE 回放仍可读（`encoding` 分流）。

**风险**：deflate 在浏览器端需要 `pako` 或 `DecompressionStream`。缓解：回放播放器用 `DecompressionStream('deflate-raw')`（Chrome 103+），并在 README 标注浏览器要求。

---

### D4 · 性能预算

**现状**：单局约 0.36 s（12 局 4.36 s，含回放帧）；`decide` 均值 0.06 ms（远低于 120 ms 预算）。

**目标**

| 指标 | 目标 | 测法 |
|---|---:|---|
| 单局（1200 tick，含回放帧） | ≤1.0 s | `scripts/bench.js`，20 局取中位 |
| `tick` 编码 | ≤2 ms | 同上 |
| 单进程内存 | ≤256 MB | `process.memoryUsage().rss` |
| `hello` 解析（Bot 侧） | ≤50 ms | 探针 Bot 计时 |

**改法**：`scripts/bench.js` 输出 p50/p95；CI 里设 20% 容忍带（防机器抖动）。

**验收**：bench 全绿；把 `TICK_LIMIT` 临时改 2400 时 bench 变红（说明预算有效）。

---

### D5 · 确定性加固

**现状**：40/40 + 100/100 均 `verify.ok`，但依赖「同进程 + 同顺序」。

**风险点**
- `Math.hypot` 在多参数时的实现差异（V8 内部）。
- `Set`/`Map` 迭代顺序（插入序，但重构后可能变）。
- 浮点累加顺序（`units.filter` 后 `reduce`）。

**改法**
1. 所有距离统一用 `Math.sqrt(dx*dx+dy*dy)`（不用 `Math.hypot` 多参数）。
2. 单位数组按 `id` 升序处理，不依赖插入序。
3. `result` 里浮点统一保留 6 位（`Number(v.toFixed(6))`）。
4. `verifyDeterminism` 增加「跨进程」模式：同 seed 跑两个子进程，比对 `orderLog` 哈希。

**验收**：跨进程 200 局 0 不一致；`test/determinism_test.js` 覆盖。

---

### D6 · 沙箱

**现状**：Bot 与裁判同进程（`deepFreeze` 只防误改，不是安全边界）。

**改法**：`docker/Dockerfile.bot`

```dockerfile
FROM node:22-slim
RUN useradd -m -u 10001 bot
USER bot
WORKDIR /app
COPY --chown=bot bot.js /app/bot.js
ENTRYPOINT ["node", "/app/bot.js"]
```

运行参数：
```bash
docker run --rm -i --network none --cpus 1 --memory 512m \
  --read-only --tmpfs /tmp:size=64m --pids-limit 128 wod-bot
```

**验收**：恶意 Bot 4 例（写文件/起子进程/读 `/etc/passwd`/无限循环）全部被拦或超时；正常 Bot 性能损失 <10%。

**风险**：Docker 在评委环境不可用。缓解：保留「同进程 + 超时」降级路径，README 写明两种模式的隔离等级差异。

---

### 8.1 阶段 4 出口条件

- [ ] D1 拆分完成，100 局逐帧哈希不变
- [ ] D2 协议 v6 字段表落地
- [ ] D3 hello ≤64 KiB
- [ ] D4 bench 达标
- [ ] D5 跨进程确定性 200/200
- [ ] D6 沙箱 4 类恶意 Bot 全拦
- [ ] `git tag baseline/v0.15-engine`

---

## 9. 阶段 5 · 交付与合规（1 周）

> 目标：从「能跑的仓库」变成「别人能一键复现的比赛包」。

### E1 · 目标目录结构（对齐 `rmuc2026`）

```text
desert-arena/
├── README.md                 # 首屏：是什么/不是什么/5 分钟跑起来
├── README_en.md              # 英文版（正式集数规范要求）
├── LICENSE                   # MIT（代码）
├── THIRD_PARTY_NOTICES.md    # MicroRTS / OpenRA 参照与许可
├── CONDITIONS.md             # 14 项条件表
├── prompt.md                 # 给模型的任务书
├── arena.js                  # 本地 runner（check/play/batch/pool/challenge）
├── challenge.js              # 一键：16 局 + 输出分数
├── workspace/                # 选手包（自包含）
│   ├── RULES.md
│   ├── botkit.js / botkit.go
│   ├── bots/{tactical,greedy,vanguard}.js
│   ├── engine/               # 裁判快照（自包含，可独立 require）
│   └── submission/bot.js     # 模板
├── maps/pool.json            # 16 张固定地图
├── replays/                  # 精选回放（HTML）
├── results/                  # 成绩与 standings
├── docs/{术语.md,单位卡.md,scoring.md,rating.md}
└── docker/Dockerfile.bot
```

**验收**：把 `workspace/` 单独拷到空目录，`node -e "require('./engine/rules.js')"` 能跑；`node challenge.js` 能出分。

---

### E2 · 许可与溯源（投稿阻断项）

**现状**：无 `LICENSE`；`Dev/Docs/LICENSE.md` 只覆盖资料区；`mapgen.js` 注明参照 MicroRTS（GPL-3.0）。

**改法**
1. 根 `LICENSE`：**MIT**（自有代码）。
2. `THIRD_PARTY_NOTICES.md`：逐条列出
   - MicroRTS（`@72f2fd9`，GPL-3.0）：只参照了循环赛组织方式与地图生成思路，**未复制代码**；若评审认为构成演绎，则本作品整体改为 GPL-3.0。
   - OpenRA / 红警相关：只参照单位命名与手感，无代码。
   - 地图生成噪声：自实现（值噪声 + 侵蚀），无第三方。
3. `Dev/Docs/ATTRIBUTION.md` 与根 `THIRD_PARTY_NOTICES.md` 合并，避免两份。
4. 在 README 首屏写「非官方、非商业、与任何游戏厂商无关」。

**验收**：`scripts/check_license.js` 检查根目录有 LICENSE、有 NOTICES、README 有非官方声明；缺一报错。

---

### E3 · README 首屏（三段式）

```markdown
# Desert Arena · 社区竞赛（规则引擎 + 评测环境）

> 非官方社区项目，与任何商业游戏无关。**没有人类可玩版本**，没有赛季，没有模型榜。

## 这是什么
一套「Bot 对抗 + 无头裁判」的规则引擎：选手写 init/decide，母舰分配、生产、科技、部队行动全由 Bot 决定。

## 5 分钟跑起来
node arena.js check
node arena.js play
node challenge.js            # 16 局，输出 BT 分与 CI

## 不是什么
- 不是正式赛季（1 集，无长期排名）
- 不是模型能力榜（尚无模型参赛）
- 不是 512²（当前 256²，512² 需先解决编码）
```

**验收**：新读者按 README 在 5 分钟内跑出 `CHECK_OK`（找 2 人实测）。

---

### E4 · CONDITIONS（14 项）

沿用正式集数规范的 14 项表头，逐项填「本集的值 + 是否有偏差 + 偏差说明」：

| # | 项 | 值 |
|---:|---|---|
| 1 | 引擎版本 | v1.0-candidate，`rulesHash` 见 §附录 |
| 2 | 协议版本 | 6 |
| 3 | 地图 | 256²，16 张固定池 |
| 4 | 单局时长 | 1200 tick = 60 s |
| 5 | 局数 | 每人 416 局（n=14） |
| 6 | 判定链 | L1 斩舰 → L2 VP → L3 击杀 → L4 残存 HP → L5 平局 |
| 7 | 计分 | 8 域加权，scoreVersion 2 |
| 8 | 排名 | BT + 地图整块 bootstrap + 相邻 CI 并列 |
| 9 | 预算 | 决策 120 ms，p95 进计分；超时率 >1% 判负 |
| 10 | 隔离 | Docker（推荐）/ 同进程（降级） |
| 11 | 禁止 | 改规则表、读文件、起进程、网络、睡死 |
| 12 | 故障 | 3 红牌判负；崩溃判负；不确定重跑 ≤2 |
| 13 | 已知偏差 | 无（v0.11 的 hello 超限已由 D3 修掉） |
| 14 | 复现 | `node challenge.js` + `npm run accept` |

**验收**：14 项全部有值，无「待定」。

---

### E5 · prompt（给模型的任务书）

结构：背景 200 字 → 目标（写 `init/decide`）→ 可用 API 表 → 三个示例（LAV 流 / 科技流 / 侦察流）→ 禁止事项 → 评分说明 → 交付格式。

**验收**：找 1 个模型按 prompt 在 30 分钟内产出能跑的 `bot.js`（不要求赢）。

---

### E6 · workspace 自包含 + `challenge.js`

**现状**：`workspace/` 已自包含（`engine/` 是快照，`botkit` 不依赖外部）；缺 `challenge.js`。

**改法**：`challenge.js` = `check` + 16 局 `pool` + 输出：

```text
CHALLENGE_OK
  地图: 16 张（pool v1，hash 3f9c…）
  局数: 32（16 图 × 2 座位）
  胜: 19 平: 2 负: 11
  BT: 78.4  CI: [42.1, 114.7]
  八领域: 62.3 ± 9.1
```

**验收**：空目录 + `workspace/` + `challenge.js` 即可跑；无网络依赖。

---

### E7 · HTML 回放

**现状**：`referee/match.js` 能在浏览器跑实时对局（`match.html`），但归档回放没有播放器。

**改法**
1. 新增 `replays/index.html`：拖拽/选择 JSON 回放文件 → 用 `match.js` 的渲染层逐帧播放。
2. 顶部信息条：seed、双方、判定层级、VP、`rulesHash`、`scoreVersion`。
3. 时间轴可拖动，支持 0.5×/1×/2×。
4. 精选 8 局（4 个「多数结果」+ 2 个「最接近」+ 2 个「斩舰」）随包发布。

**验收**：Chrome/Edge 打开无报错；8 局都能播完；旧 v5 回放提示「版本不兼容」而不是白屏。

---

### E8 · manifest / 冻结链 / CI

**现状**：`sync_formal_test.js` + `workspace_manifest.json`（13 文件）+ `--frozen` 已可用。

**补强**
1. manifest 增加 `rulesHash`、`proto`、`poolHash`、`generatedAt`。
2. `scripts/freeze.js`：编译/打包选手提交，写 `submissions/交卷记录.json`（含 SHA-256、时间、提交者）。
3. GitHub Actions：

```yaml
- run: npm run check
- run: node test/all.js
- run: npm run coverage
- run: node arena.js full
- run: npm run accept
```

**验收**：PR 上 5 个 job 全绿；故意改 `rules.js` 一个数值，`workspace` 的 manifest 校验变红（防止「改了引擎没同步」）。

---

### 9.1 阶段 5 出口条件

- [ ] E1 目录结构对齐
- [ ] E2 LICENSE + NOTICES + 非官方声明
- [ ] E3 README 三段式，2 人 5 分钟跑通
- [ ] E4 CONDITIONS 14 项无待定
- [ ] E5 prompt 可用
- [ ] E6 `challenge.js` 一键出分
- [ ] E7 8 局回放可播
- [ ] E8 CI 5 job 全绿
- [ ] `git tag v1.0-candidate`

---

## 10. 阶段 6 · 参考 Bot 与模型试赛（0.5 周）

### F1 · 三个独立参考 Bot

**现状**：`tactical.js`（BFS 认领 + 科技）与 `greedy.js`（暴兵）同源思路，都是「最近目标 + 一步贪心」。

| Bot | 定位 | 核心机制 | 预期 |
|---|---|---|---|
| `Vanguard` | 正面推进 | LAV 集群 + 集火 + `aggressive` 尖兵 | 对 Greedy ≥60% |
| `Bastion` | 阵地/科技 | AAV + RAILGUN + `neutral` 守点 | 对 Vanguard 40%～60% |
| `Raider` | 骚扰/斩舰 | SCOUT 开图 + ARTILLERY 远程 + 绕后打母舰 | 至少 10% 局 L1 |

**要求**：三个 Bot 不共享决策代码（各自独立文件），只共享 `botkit` 的工具函数。

**验收**：三者两两对战各 64 局，胜率矩阵无「一超多弱」（最高胜率 ≤70%）。

---

### F2 · 模型试赛

**目标**：4 个模型 × 80 局（5 张图 × 2 座位 × 8 轮）。

**记录表**（每模型一行）：模型名/版本、工具/渠道、温度、平均用时、事故数、BT、CI、典型失败模式。

**验收**：`results/models-2026Q4.md` 生成；至少 1 个模型能跑完 80 局无 forfeit。

---

### F3 · 校准

- 用 F2 的 BT 与 F1 的 Bot 做参照表：`Raider`=0（锚），其余相对位置。
- 若 4 个模型 BT 全落在 ±40 内，说明题目区分度不足 → 回到 A2/A3 加强「战略分叉」。

**验收**：BT 极差 ≥120（否则题目太简单/太难）。

---

### 10.1 阶段 6 出口条件

- [ ] F1 三 Bot 胜率矩阵合理
- [ ] F2 4 模型 ×80 局完成，事故表可读
- [ ] F3 BT 极差 ≥120
- [ ] 12 条 DoD 全绿 → 打 `v1.0` 标签

---

## 11. 文件级改动清单

### 11.1 新增

| 路径 | 内容 | 阶段 |
|---|---|---|
| `referee/engine/{constants,units,tech,map,combat,scoring,game}.js` | 拆分后的模块 | D1 |
| `rating/bt.js` | BT + bootstrap + CI | C3 |
| `maps/pool.json` | 16 张固定地图 | C1 |
| `scripts/build_map_pool.js` | 地图池生成/校验 | C1 |
| `scripts/accept_l1.js` | L1 占比验收 | A2 |
| `scripts/accept_balance.js` | 胜率差验收 | A3 |
| `scripts/accept_coverage.js` | 兵种/科技/模式覆盖 | A4-A6 |
| `scripts/accept_scoring.js` | 计分卡门禁 | B2 |
| `scripts/bench.js` | 性能预算 | D4 |
| `scripts/freeze.js` | 收卷与 SHA-256 | E8 |
| `scripts/check_license.js` | 许可检查 | E2 |
| `challenge.js` | 一键挑战 | E6 |
| `docker/Dockerfile.bot` | 沙箱镜像 | D6 |
| `replays/index.html` | 回放播放器 | E7 |
| `docs/{术语.md,单位卡.md,scoring.md,rating.md}` | 文档 | 0.4/B1/C3 |
| `test/{vp,siege,scoring,rating,protocol,isolation,map_pool,determinism,public_api,economy}_test.js` | 10 个新套件 | 各阶段 |
| `README_en.md`、`CONDITIONS.md`、`prompt.md`、`LICENSE`、`THIRD_PARTY_NOTICES.md` | 交付文件 | E2-E5 |

### 11.2 修改

| 路径 | 改动 | 阶段 |
|---|---|---|
| `referee/rules.js` | `VP_RADIUS`、`VP_CAPTURE_TICKS`、`updateVP`、`computeCards`、`WEIGHTS`、`TYPES.MOTHERSHIP.hp`、`RPS`、`TECHS` 时间/花费、`header()`、`PROTO.version` | A1/A2/A5/B1/D2 |
| `referee/mapgen.js` | `placeVP3`、`vpCount 4→3`、`vpRadiusCells 4.2→6` | A1 |
| `referee/ballistics.js` | `v0` 散布 | A7 |
| `referee/referee.js` | 默认 `vision:true`、`botP95` 维护 | A7/B1 |
| `referee/tournament.js` | `tickLimit` 默认值；输出改 JSONL | 0.2/C2 |
| `referee/match.js` | 默认 tickLimit；回放模式 | 0.2/E7 |
| `formal-match-tests/workspace/arena.js` | `pool` 命令、`--resume` | C2 |
| `formal-match-tests/workspace_manifest.json` | 增加 4 个哈希字段 | E8 |
| `botkit`（JS/Go） | deflate 解码、`heading`、`mode` 示例 | D3/D2 |
| 全部 `*.md` | 600 tick / 64 KiB / 5:1 等旧口径 | 0.x/E3 |

### 11.3 删除/归档

| 路径 | 处理 | 阶段 |
|---|---|---|
| `Dev/MVP/_archive_skeleton/` | → `Dev/archive/` | 0.3 |
| `referee/tournament.js` 的积分榜输出 | 保留代码，排名改用 BT | C3 |
| `Dev/experiments/Plan_Siege.md` | 不动（不随投稿） | — |
| `war_of_desert/` 的引擎快照 | 标注「v1.0 历史，不追 v2 规则」 | E3 |

---

## 12. 测试计划

### 12.1 新增套件与断言数

| 套件 | 断言 | 覆盖点 |
|---|---:|---|
| `test/vp_test.js` | 12 | 3 点对称、占领进度、争夺冻结、半径边界 |
| `test/siege_test.js` | 9 | 斩舰时长、侧后 ×1.5、护盾阶段、L1 触发 |
| `test/economy_test.js` | 6 | 残骸掉落、拾取半径、对账 |
| `test/scoring_test.js` | 14 | 8 域值域、极端局面、权重和、常数门禁 |
| `test/rating_test.js` | 10 | BT 收敛、锚点、bootstrap 覆盖、并列 |
| `test/protocol_test.js` | 10 | hello ≤64 KiB、deflate 往返、v6 字段、v5 拒绝 |
| `test/isolation_test.js` | 8 | 冻结表、原型篡改、文件/进程访问 |
| `test/map_pool_test.js` | 8 | 16 张条件、哈希稳定、生成确定性 |
| `test/determinism_test.js` | 4 | 跨进程 200 局一致 |
| `test/public_api_test.js` | 1 | 导出清单快照 |
| **合计** | **82** | 现有 454 条（`ok(` 计数），新增约 +18% |

### 12.2 回归矩阵

| 层级 | 命令 | 频率 | 门禁 |
|---|---|---|---|
| 单元 | `npm run check && node test/all.js` | 每次提交 | 10+10 套件全绿 |
| 覆盖率 | `npm run coverage` | 每次 PR | 8 文件过阈值 |
| 验收 | `npm run accept` | 每次 PR | 12 条 DoD |
| 端到端 | `node arena.js full` | 每次 PR | `CHECK_OK` + 4 局 0 故障 |
| 赛季 | `node arena.js pool --bots ...` | 每周 | 2,912 局，0 forfeit 异常 |
| 沙箱 | `docker run ...` × 4 恶意样本 | 每次发布 | 全拦 |

### 12.3 变异测试（防「自指断言」）

对 `rules.js` 注入 20 个变异（改 `dmg`、改 `rof`、改 `VP_CAPTURE_TICKS`…），要求 **≥18 个被测试杀死**。现有 v0.10 已达 17/17，新代码要保持。

---

## 13. 排期与人力

按 1 名主程 + 1 名兼职场（文档/回放）估算。

| 周 | 主程 | 兼职 | 里程碑 |
|---|---|---|---|
| W1 | 0.1～0.5 + A1 | 术语表/单位卡 | M0 |
| W2 | A2 + A3 | 回放原型 | — |
| W3 | A4～A7 | CONDITIONS 草稿 | M1 |
| W4 | B1 + B2 | scoring.md | M2 |
| W5 | C1～C3 | rating.md | — |
| W6 | C4～C6 + D1 | README/prompt | M3 |
| W7 | D2～D6 | 回放播放器 | — |
| W8 | E1～E8 | 英文 README | M4 |
| W9 | F1～F3 + 缓冲 | 试赛记录 | M5 |

**总计 9 周**（比 §2.1 的 6～7 周多 2 周，因为 D6 沙箱与 E7 回放被低估）。若压缩：砍 D6（用同进程降级）与 E7（只放 JSON），可回到 7 周，但交付分会下降。

**人力瓶颈**：A2（斩舰）与 C3（BT）各需要 2～3 天连续思考，不宜拆碎。

---

## 14. 风险登记册

| # | 风险 | 概率 | 影响 | 触发信号 | 缓解 | 兜底 |
|---:|---|---|---|---|---|---|
| 1 | VP 改 3 点后更滚雪球 | 中 | 高 | 120 局胜率差 >60 | 占领 3 s + 落后方补正 | 回退 4 点，只改半径 |
| 2 | 斩舰过快导致 rush 一边倒 | 中 | 高 | L1 >35% 且 median turn <400 | 护盾阶段 + 母舰反击 | HP 回调 2400 |
| 3 | 击杀收益让雪球失控 | 中 | 中 | Greedy 胜率 >65% | 掉落降到 20% | 关闭 A3 |
| 4 | 新计分卡仍有常数域 | 高 | 中 | 常数分 >20 | 逐域替换，先删推进效率 | 只保留 5 域 |
| 5 | BT 样本不足导致 CI 过宽 | 中 | 中 | CI 宽度 >200 | 增加地图到 24 张 | 标 `low-confidence` |
| 6 | deflate 在浏览器不可用 | 低 | 中 | 回放白屏 | `DecompressionStream` + 降级 pako | 回放预解码成 JSON |
| 7 | 拆分 rules.js 引入行为漂移 | 中 | 高 | 逐帧哈希不一致 | 拆分前后 100 局哈希比对 | 回滚单文件 |
| 8 | 沙箱不可用 | 中 | 中 | Docker 缺失 | 同进程 + 超时降级 | 明示隔离等级 |
| 9 | 许可争议（GPL 传染） | 低 | 高 | 评审质疑 | NOTICES 逐条说明 | 整体改 GPL-3.0 |
| 10 | 工期超 9 周 | 高 | 中 | W4 未到 M2 | 砍 D6/E7 | 降级为「技术提案」 |

---

## 15. 回滚与兼容

### 15.1 标签序列

```text
baseline/v0.11-units      阶段 0 完成
baseline/v0.12-gameplay   阶段 1 完成
baseline/v0.13-scoring    阶段 2 完成
baseline/v0.14-season     阶段 3 完成
baseline/v0.15-engine     阶段 4 完成
v1.0-candidate            阶段 5 完成
v1.0                      阶段 6 + 12 条 DoD
```

任何阶段出问题，`git checkout <tag>` + 重跑该阶段验收即可；不允许「跳过验收往下做」。

### 15.2 数据兼容

| 数据 | 兼容策略 |
|---|---|
| v0.11 旧回放 | 只读；播放器提示 `proto 5 不兼容`；不参与新排名 |
| 旧 `algorithm-contest/tests/history-seasons/formal/` | 标注 `scoreVersion 1`；跨版本只比胜负 |
| Go `war_of_desert` | 标注「v1.0 历史快照，不追 v2」；不参与新赛季 |
| `botkit` | 同时支持 RLE（旧）与 deflate（新），按 `encoding` 分流 |

### 15.3 规则指纹纪律

`rulesHash` 的输入必须包含：常量、单位表、RPS、科技表、权重、弹道表、协议版本、`vpCount`、`vpCaptureTicks`。**改任何一项都必须换哈希**，并在 `CONDITIONS.md` 记录。

---

## 16. 附录

### 16.1 常量速查（改造后目标值）

| 常量 | v0.11 | v1.0 目标 |
|---|---:|---:|
| `GRID` / `CELL` | 256 / 6 | 不变 |
| `TICK_MS` / `TICK_LIMIT` | 50 / 1200 | 不变 |
| `DECIDE_MS` | 120 | 不变 |
| `MAX_LINE` | 131,072 | **65,536**（压缩后） |
| `PROTO.version` | 5 | **6** |
| `VP` 数量 | 4 | **3** |
| `VP_RADIUS` | 16.8 格 | **6 格** |
| `VP_CAPTURE_TICKS` | — | **60** |
| `MOTHERSHIP.hp` | 3,000 | **2,000** |
| `WEIGHTS` | `[.25,.15,.15,.15,.10,.05,.05,.10]` | `[.30,.15,.15,.10,.10,.10,.05,.05]` |
| 地图池 | 无 | **16 张** |
| 每人局数 | 2 | **416**（n=14） |
| 排名 | 积分榜 | **BT + CI + 并列** |

### 16.2 关键公式

```text
伤害      = base × RPS[src][tgt] × 落速²系数 × 侧后系数(1.0|1.5)
BT        p(i>j) = 1 / (1 + exp(-(r_i - r_j)/400))
CI        按地图整块 bootstrap 1000 次，取 2.5% / 97.5%
总分      Σ fields[i] × WEIGHTS[i]
常数分    各域在 200 方样本中 σ<1 的域 × 权重 × 100 之和
```

### 16.3 命令速查

```bash
# 日常
npm run check && node test/all.js
npm run coverage
npm run accept                       # 12 条 DoD

# 正式测试包
npm run sync:formal
cd formal-match-tests/workspace && node arena.js full

# 地图池
node scripts/build_map_pool.js --out maps/pool.json

# 赛季
node arena.js pool --bots bots/*.js --maps maps/pool.json --out results/season.jsonl

# 性能
node scripts/bench.js --games 20

# 沙箱
docker build -f docker/Dockerfile.bot -t wod-bot .
docker run --rm -i --network none --cpus 1 --memory 512m wod-bot < tick.json
```

### 16.4 投稿前 30 项检查清单

**合规（8）**
- [ ] 根 `LICENSE`（MIT）
- [ ] `THIRD_PARTY_NOTICES.md` 覆盖 MicroRTS/OpenRA
- [ ] README 首屏「非官方、无赛季、无模型榜」
- [ ] `README_en.md`
- [ ] `CONDITIONS.md` 14 项无「待定」
- [ ] `prompt.md`
- [ ] 目录结构对齐正式集数规范
- [ ] `.gitignore` 不排除replays/成绩

**复现（7）**
- [ ] 空目录 + `workspace/` 能独立跑
- [ ] `node challenge.js` 出 BT 分
- [ ] `node arena.js full` 输出 `CHECK_OK`
- [ ] manifest 13+ 文件校验通过
- [ ] `--frozen` 拒绝无效收卷
- [ ] `npm run accept` 12 条全绿
- [ ] CI 5 个 job 全绿

**公平（8）**
- [ ] 16 张地图池哈希冻结
- [ ] 每人 ≥400 局
- [ ] BT + CI + 并列
- [ ] 3 红牌判负规则写明
- [ ] 崩溃/超时/不确定判罚写明
- [ ] 锚点 Bot 固定 0
- [ ] 无「按总分排序」的隐式规则
- [ ] 模型条件表（若做试赛）

**质量（7）**
- [ ] 10+10 套件全绿
- [ ] 覆盖率 8 文件过阈值
- [ ] `hello` ≤64 KiB
- [ ] 单局 ≤1.0 s
- [ ] 跨进程确定性 200/200
- [ ] 沙箱 4 类恶意样本全拦
- [ ] 8 局 HTML 回放可播

---

**文档结束。** 执行顺序按 §3.1，验收以 §2.2 的 12 条为准；任何一条不达标，不要打 `v1.0` 标签。

# MVP 审查报告 · 测试有效性 / 需求符合度 / 架构可行性

> 审查对象：`社区比赛__WoD/MVP/`（MVP v0.2，独立 git 仓库，外层 `/MVP/` 已 gitignore）
> 审查时间：2026-11-10 ｜ 环境：Windows + Node v22.22.2
> 对照基准：`Plan_Beta.md`（目标架构）、`MVP_plan.md`（落地计划）、`Meow-AI-Arean/meow-ai-arena`（主仓库真实约定）
> 方法：读源码 + 实跑全部测试 + V8 函数级覆盖率 + 定点探针复现（不改动被测代码）

---

## 0. 结论摘要

| 问题 | 结论 |
|---|---|
| 测试能跑通吗 | 能。4 个脚本全绿，约 90 条断言，无外部依赖 |
| 覆盖够吗 | 不够。裁判侧函数覆盖 94%，但**战斗链路 0 执行**；人类版 76%，**输入层 0 执行** |
| 测试有效吗 | 部分无效。头号用例（两局换边比赛）实测**零开火零击杀**；"3 个 seed" 实为**2 张地图** |
| 符合 WoD 需求吗 | 核心玩法假设（地形修正链路）已验证；**比赛侧需求（进程隔离/512²/经济/迷雾/选手包/成绩表/BT）缺失** |
| 能用于主项目吗 | **不能直接搬运行时代码**；可作为「规则与协议契约」蓝本做**重写式迁移** |
| 可行性判定 | 可行，但需先补 5 项 P0（运行形态、规则单源、replays/成绩契约、Game(n)、seed 语义）才能动工 |

---

## 0.5 复测（v0.3，报告落盘当天实施）

报告写完后，在 `MVP/` 内按 P0/P1 做了一轮修复。本节记录**哪些结论已失效**、哪些仍然成立。复核命令：`cd MVP && node test/all.js`（6 套件全绿）。

| 原问题 | 状态 | 修法与验证 |
|---|---|---|
| seed 被改写、两 seed 同图 | ✅ 已修 | 地图由 `(seed, attempt)` + 噪声盐值唯一确定；`match_test` §12 断言"相邻 seed 不再是同一张图"，实测相同格 620/4096 |
| 跨 seed 山脉占比 0.9%~28.7% | ✅ 已修 | 生成约束 5%~15%（目标 10%），超标标 `outOfBand`；§12 断言 4 个 seed 全在区间 |
| 裁判战斗链路 0 执行 | ✅ 已修 | 强制接火覆盖射程/冷却/克制/高地/视线；新增"常规对局至少一次交火"（20261110:6、20261111:8） |
| 两局零开火 | ✅ 已修 | 根因在 Bot 策略而非引擎：全军压一点 → 目标摆动 → 认领即走 → 认领即守，`tactical.js` 改了 4 轮 |
| 计分卡三项恒定、"赢局低分"从未出现 | ✅ 已修 | 区分度用例 + 斩舰速胜对照；三 seed 循环赛首次 `divergent=true`（Tactical 4 胜但总分 355.7 < Greedy 406.4） |
| 人口只增不减 | ✅ 已修 | 阵亡回收 `pop`；`contract_test` §2 断言 `pop == 存活 + 队列`（6/6、22/22） |
| AAV 造价两套不一致 | ✅ 已修 | 统一 240；`contract_test` §1 对 6 单位 × 8 字段逐项比对 |
| `Game(n)` 忽略 n | ✅ 已修 | `n != 2` 显式抛错，§12 断言 |
| replays/成绩 schema 三套不兼容 | ✅ 裁判侧已修 | `result` 补 `mode/ranks/forfeit/timeouts/errors/max_ms/turns/vp`；`result.replays` 改为 `{header,names,result,frames}` |
| 输入层 0 覆盖 | ✅ 已修 | 新增 `test/harness.js`（可合成事件）+ `test/input_test.js`（28 条断言） |
| 判定链只到第 2 级 | ✅ 已修 | `contract_test` §4 逐级覆盖 1~5 级 |
| 摘要漏字段 | ✅ 已修 | 摘要混入 `pop/power/queue/state/path/targetId/cd` |
| 母舰维修会把 0 血救活 | ✅ 顺带修掉 | 维修只对 `hp > 0` 生效 |
| 测试无统一入口 | ✅ 已修 | `package.json` + `test/all.js`，6 套件统一退出码 |

覆盖率：`game.js` 76.4% → **90.4%**，`rules.js` → **97.3%**，`referee.js` → **95.5%**，两个 Bot 100%。

**仍然成立、且不该在原型里硬凑的**：

1. 规则仍是**两份实现**（`game.js` / `rules.js`），现在靠"跨端一致性断言"钉住，而不是单源生成；
2. 仍是浏览器 JS、无进程隔离、无 60ms 死线、无 512²、无经济/迷雾（裁判侧）；
3. 回放顶层已对齐，但 `frames` 字段语义与主仓库 `viewer.html` 不同，仍需 WoD 专用渲染层；
4. 无 `workspace/` 选手包、无 `CONDITIONS.md`、无 community 注册、无 CI。

§8 的结论不变：**可作为契约蓝本，不能作为主项目运行时代码**。P0 清单由 5 项收敛为 4 项：运行形态、规则单源、viewer 适配、选手包。

---

## 0.6 复测（v0.4：母舰重做 + 移除人类版）

需求变更：母舰从「装甲/维修/火力/射程 四个能量点」改为**四项百分比分配**
（生产 / 攻击 / 研究 / 移动，整数、和恒 100、每 tick 最多移动 5 点）；
其余单位降级为执行者；**人类可玩版整体移除**，只保留 Bot 对抗链路。

这一版的改动会推翻前两节的若干前提，逐条标注：

| 项 | 变化 |
|---|---|
| 人类可玩版 | **已删除**（`game.js` / `index.html` / `style.css` 及其 3 个测试套件）。需要回看：`git show 3ff5700:game.js` |
| 测试规模 | 6 套件 → **4 套件**；新增 `test/rules_test.js`（41 条）覆盖分配/槽位/生产/科技/武器数/经济 |
| 经济与生产 | 裁判侧从「砍掉」变为**已实现**：500 CU 起手、1.5 CU/tick、采集车、生产队列、人口 24、槽位并行 |
| 科技 | 新增 4 棵树：2 条线性（3 级）+ 2 条交叉（5 级，2 级分叉 4 级汇合，同层互斥） |
| 协议 | v1 → **v2**：`actions` 新增 `alloc` / `produce` / `research` 三类阵营动作；`tick.me` 带全部决策所需的己方状态 |
| 跨端一致性测试 | 随人类版一起消失（只剩一套引擎），改为「规则单源自证」：`rules_test` 直接断言常量与效果的对应关系 |
| 平衡结论 | 600 tick 只能走完 4 个科技节点；「科技流」在现有权重下不一定赢（Greedy 不研究却总分更高）；**4 门炮的收益没有 Bot 去用** |

仍然成立的问题：仍是同页 JS（无进程隔离、无 60ms 死线）、64² 地块、全图可见、无 viewer 适配；
`frames` 与主仓库 `viewer.html` 语义不同。结论不变：**契约蓝本，不是可搬迁的运行时代码**。

---

## 0.7 复测（v0.5：母舰/运载舰拆分 + 槽位细化）

需求再变一次，并顺带修正了 v0.4 里两个不符合直觉的地方：

| 项 | v0.4 | v0.5 |
|---|---|---|
| 舰级 | 只有 `CARRIER`（母舰） | **MOTHERSHIP**（旗舰，被毁即判负）+ **CARRIER**（可制造的运载舰，被毁不判负） |
| 运载舰能力 | — | 800 HP、速度 1.6、火力弱、**修理周围 4 格 +2 HP/tick**、自带 1 槽只能造 LAV/Salvager |
| 生产/研究槽位 | 每 25% 一档，最多 4 | **每 5% 一档，最多 3**（0-4→0，5-9→1，10-14→2，≥15→3） |
| 攻击 | 每 25% 一门，最多 4，免费 | **连续槽位** `attack/100×5`（0~5），炮本身走独立队列 120 CU/20 tick |
| 移动 | 连续 0.4×~1.6× | 不变 |
| 协议 | v2 | **v3**：新增 `weapon`，`produce` 支持 `by:<运载舰 id>` |
| 测试 | rules_test 41 条 | 57 条（新增武器槽位/独立队列/运载舰三组） |

**新的平衡观察（值得下一轮处理）**：槽位在 15% 就封顶，意味着「3 个研究槽」只要 15% 预算。
结果科技变成几乎免费的投资 —— 600 tick 能走完 6 个节点，不研究的 Greedy 直接 1:5 落后。
如果目标是让「科技流 vs 兵力流」互相制衡，应该调 `SLOT_STEP`（5→10）或 `MAX_SLOTS`，
而不是改科技数值；这条已经写进 `MVP/README.md` 的实测段。

仍未变的结论：同页 JS、无进程隔离、64²、全图可见、无 viewer 适配 —— **契约蓝本，不是可搬迁的运行时代码**。

---

## 0.8 复测（v0.6：决策与 tick 解耦）

需求变更：**决策时间与引擎 tick 解耦**，单次决策上限 **120ms**，决策完成后在
**下一个 tick** 执行；tick 长度由实现自定。

| 项 | v0.5 | v0.6 |
|---|---|---|
| 时序 | 无概念；`timeLimits=[2000,50]`，每 tick 同步收发一行 | `tickMs=50`（20 tick/s，600 tick=30s）、`decideMs=120`、`initMs=2000`、`orderLatency=1` |
| 指令生效 | 本 tick 决策，本 tick 生效（同步） | **一律晚 1 tick 落地**，任何情况下都不会"看到就改" |
| 超时 | 单回合 50ms | 单次决策 120ms（encode→decide→decode 全程），超了记 `decision timeout` |
| 新统计 | 只有 `avgMs` | `late`（思考超过 1 tick 的次数）、`decide{calls,avg_ms,max_ms,late,faults}`、`timing{tickMs,decideMs,initMs,orderLatency,wallMs}` |
| 协议 | v3 | **v4**：`hello.rules.timing` + `tick.timing{applyAt}`；时序常量进 `rulesHash` |
| 测试 | match_test 74 条 | **86 条**，新增 §15：`t=1` 无指令 / 首条指令在 `t=2` / 思考 0ms 与 60ms 逐帧哈希一致 / 60ms 不判超时 / 20ms 预算下慢 Bot 只记在超预算一方 |

两条设计判断，值得在迁移时保留：

1. **延迟是协议保证，不是实测值。** 若按 `ceil(实测ms / tickMs)` 算落地时刻，同一 seed 的胜负
   会依赖机器负载，`frameHashes` 复核立刻失效。现在的分工是：延迟固定 1 tick（可复现），
   超时用 120ms 硬配额管（可判定），两者互不污染。
2. **120ms 预算 > 50ms tick 是有意为之。** 它逼着引擎不能等 Bot —— 这正是"解耦"的实质；
   若预算 ≤ tick，"解耦"就退化成"每 tick 同步一次"，看不出任何区别。

对审查结论的影响：原报告"缺失项"里的**「60ms 死线」部分闭合**——决策预算与延迟语义已经落地，
仍缺的是**真进程隔离**（超预算只能事后判、不能中断死循环）与 **kill 语义**。
其余结论不变：64²、全图可见、无 viewer 适配、契约蓝本而非可搬迁运行时。

---

## 0.9 复测（v0.7：命中改成外弹道结算）

需求变更：**杀伤按「炮弹速度 + 弹种」结算**，初速与弹种挂在科技树上，
弹道用**带空气阻力的抛物线**积分，阻力取常数。

| 项 | v0.6 | v0.7 |
|---|---|---|
| 命中 | 开火即扣血（`dmg × RPS × 高地系数`） | 开火生成炮弹 → 后续 tick 飞行 → 落地/撞上才结算 |
| 弹道 | 无 | RK4 积分 `dv/dt = g − k|v|v`，10 ms 步长；`k=4.0e-4 1/m`，`g=300 m/s²`（游戏标度），1 格 = 40 m |
| 伤害 | 固定值 + 克制表 | `表定 × 弹种系数 × (落速/基础初速)^指数 × 装甲修正 × 克制` |
| 弹种 | 无 | AP／HE（溅射 30 m）／APCR；F2、F3 解锁 |
| 科技（火力树） | F1~F3 各 +10/12/15% 伤害 | F1~F3 改**初速** +10/12/15%，F2/F3 兼**解锁弹种**（伤害 ∝ v²，+10% 初速 ≈ +21% 伤害） |
| 地形 | 2D 射线 `hasLOS` + 高地 +25% 系数 | 弹道积分撞山判定；高地加成由重力势能自然产生（物理），系数删除 |
| 协议 | v4 | **v5**：`tick.shells`（在飞炮弹）、`hello.rules.ballistics`（阻力/重力/弹种表）、`result.shots/hits` |
| 测试 | 4 套件 | **5 套件**，新增 `test/ballistics_test.js`（34 条：收敛性、求解误差、双解、挡弹道、v² 律、科技链路、飞行延迟） |

三条实现踩过的坑，值得写进迁移清单：

1. **单位换算**：`s` 是米，方向向量是像素空间单位向量，位置必须 `sx + dx·s/M_PER_PX`。
   少除这一下，炮弹会「飞得比瞄准慢」，表现为「近距离能中、远距离全空」；
2. **上坡不能判成立刻到达**：目标高度高于炮口时，若用 `h <= targetH` 判到达，任何仰角都会立刻返回 `s≈0`，
   求解器于是整局 7000 次开火全部 `out-of-envelope`；
3. **采样密度**：山体遮挡若按「每 N 步采一点」，高速弹在 50 m 网格上会整段跳过山脊。

对审查结论的影响：**「战斗只是掷骰子」这条隐患被消除** —— 现在的伤害有可复核的物理来源，
弹道也真正参与判定（挡与不挡、俯射与仰射都能算出差异）。
仍未做的是：穿深-角度耦合、风场、引信类型、以及「炮弹在 tick 内的并发顺序」。
其余结论不变：64²、全图可见、无 viewer 适配、契约蓝本而非可搬迁运行时。

---

## 1. 测试清单与实测通过情况

| # | 脚本 | 实测结果 | 断言数 | 备注 |
|---|---|---|---:|---|
| 1 | `smoke_test.js`（人类版冒烟） | `SMOKE_OK` | 33 处 `throw` | 1200 tick 无头跑完整局，约 0.65s |
| 2 | `referee/match_test.js`（裁判全流程） | `ALL_TESTS_OK` | 50 处 `ok()` | 11 组用例，114ms 跑完 2 局 |
| 3 | `referee/web_smoke.js`（网页路径） | `WEB_SMOKE_OK` | 12 条页面断言 | DOM 桩驱动 `match.js` 分批循环 40 次 |
| 4 | `referee/tournament.js`（循环赛） | `TOURNAMENT_OK` | 6 局确定性复核 | Tactical 5 胜 1 平 |

关键实测数字（可复现）：

- 人类版：`seed=20261111; units=14; vp=4; walls=756/16384; A* 路径穿山 0 次；maxLateralOffset=7.8 格；mountainEnter=0；tick=1200; winner=1 level=2; total 54.8:50.0`
- 裁判版：两局 `601 帧` 确定性一致；`winner=seat1 / seat0`，`level=2`，`VP 0:471 / 399:0`
- 覆盖率（V8 函数级，跑完上述 4 个脚本后取并集）：

| 文件 | 覆盖 | 未覆盖（节选） |
|---|---|---|
| `game.js` | 126/165 = **76.4%** | 全部输入适配器、`pickUnit/selectAt/selectBox/rightClick/commandAt/handleCmd`、`renderTactical`、`screenToWorld/worldToScreen`、`allocatePower`、`clampCamera`、`doRestart`、`carveCorridor` |
| `referee/rules.js` | 69/73 = 94.5% | **`fire`、`hasLOS`**（全套测试从未调用）、`carveCorridor` |
| `referee/referee.js` | 33/35 = 94.3% | `get games`、`verify()` |
| `tournament.js` / `match.js` | 95.8% / 88.6% | 匿名回调 |
| `bots/*.js` | 100% | — |

> 说明：函数覆盖率不等于分支覆盖率。裁判侧 94% 的漂亮数字里，**权重最高的一条链路（战斗）是零执行**。

---

## 2. 覆盖范围评估：三块最贵的空白

### 2.1 战斗系统在裁判侧完全未被验证

实测 `greedy` vs `tactical` 两局：`kills 0:0 / losses 0:0 / 歼敌字段 0`，`rec.events` 里 `kill` 事件 0 条。也就是说：

- `rules.js` 的 `fire()`、`canShoot()`、`hasLOS()`、`RPS` 克制表、`rof` 冷却、高地 ±25%/+20%、曲射，**没有一条被测试执行过**；
- 决定胜负的唯一机制是"谁走到 VP 上站着"（471 tick vs 0 tick）；
- 定点探针证明战斗逻辑本身没错（把蓝方 LAV 摆到红方 LAV 旁 2 格，12 tick 内互相打掉 108 HP 并产生击杀），**是场景从不让双方接触**。

成因：4 个 VP 分散、每方 5 个单位、Tactical 把全军送同一个点、Greedy 各回各家最近的 VP，600 tick 内两军从不相遇。这既是测试盲区，也是**地图与初始部署的设计缺陷**——它在正式赛里会直接表现为"打不出战斗的对局"。

### 2.2 人类版输入层 0 覆盖

`README.md` 的验收清单（框选、右键、Shift 折线、`A` 攻击移动、编队、滚轮缩放、护卫、`S` 停止、`B` 加速、`Home/F/Backspace` 镜头）**全部是人工勾选项**，测试只通过 `WoD.issue()` 绕过输入适配器直接下发 Order。`smoke_test.js` 用 DOM 桩跑到 `boot` + `render`，但没有一次合成鼠标/键盘事件。

### 2.3 判定链与计分卡的兜底分支未覆盖

- `game.js` 只跑到 **第 2 级**（时间到按 VP）；第 1/3/4/5 级与"双方母舰同时被摧毁"在人类版无自动化（裁判侧第 5 级有覆盖）。
- 八领域中，第 6/7/8 域在 `game.js` 与 `rules.js` 里**公式不同**，且与 `Plan_Beta.md` §4.1 的口径也不同（第 6 域 Beta 是"视野覆盖率 / VP 周边控制时长"，实现是纯视野格数累计；第 7 域 Beta 是"单位 tick 得分"，实现是 `(vpScore + killValue/60)/(denom/12)`）。文档没有标注这两处偏离。
- `REJECT_ORDER` 的 10 个拒绝分支只测了 1 个（`target on mountain`）；`decode()` 的 10 个结构性错误分支只测了 4 个（`bad json`、`bad unit id`、`bad action type`、`bad coords`、`coords out of range`、`duplicate unit action` 均未覆盖）。

---

## 3. 有效性评估：通过 ≠ 有效

### 3.1 【严重】seed 重映射导致"3 个 seed"实为 2 张地图

`rules.js:190 generateMap()` 在连通性校验失败时会 `seed+a` 重试，最终 `this.seed = this.map.seed`（`rules.js:232`）。实测：

```
requested 20261109 -> resolved 20261110
requested 20261110 -> resolved 20261110   ← 与上一条落到同一张图
requested 20261111 -> resolved 20261111
两图逐格对比：4096/4096 完全相同
```

后果：

1. `tournament.js` 的"3 seed × 换边 6 局"里有两局在**同一张地图**上重复，`45.0:56.2` 两次完全相同就是这个证据；"5 胜 1 平"的样本多样性被高估。
2. 初始兵力 `count = 4 + |resolvedSeed| % 3`（`rules.js:248`）由**解析后的 seed** 决定，调用方传 `seed` 无法控制兵力规模（实测 4/5/6 个/方），跨局不可比。
3. 测试没有一条断言"不同 seed → 不同地图"或"header.seed == 请求的 seed"，所以这个 bug 静默通过。

### 3.2 【严重】计分卡在当前对局里近乎常数，设计意图未被检验

两局实测八领域（seat0）：`0, 0, 100, 100, 50, 0/26, 0/100, 100`。
八项里有三项恒定（歼敌 0、保存兵力 100、KD 差 50、母舰完整度 100），实际只有 4 项在动。更关键的是：

- `Plan_Beta.md` §4.1 的核心论断"**赢了这局但加权总分更低**"，在**所有**测试中 `divergent=false`，从未出现——这条最需要验证的设计主张，恰好没有任何用例覆盖；
- `match_test.js` 里 `八领域权重和为 1` 是恒真断言，不能证明计分卡有区分度。

### 3.3 【中】确定性校验只能证明"同一实现重复执行一致"

`game.js:verifyDeterminism()` 与 `rules.js:verifyDeterminism()` 都是**同进程、同引擎**重跑，能抓"日志注入错误"（README 演示的篡改用例有效），但抓不到：

- 跨语言/跨引擎差异（迁移到 Go/Python 后 `Math.imul`、`Float64`、`Map` 迭代顺序全部要重算，`referee/README.md` 已承认）；
- 单线程 JS 里不可能出现的调度类不确定性；
- `stateDigest()`（`game.js:1053`）没有混入 `pop / power / queue.remain / path / unit.state / effects`，生产队列或状态机分歧可能被摘要漏掉。

### 3.4 【中】测试通过，但业务有真 bug：人口只增不减

`game.js:422`（初始）与 `game.js:447`（生产）给 `S.pop` 累加，**单位阵亡时没有任何地方减回去**。实测 1200 tick 后：

```
side 0: pop=12, 实际存活 1 个单位
side 1: pop=43, 实际存活 9 个
```

即"被打光的一方永久丧失补兵能力"，与"保存兵力 / 翻盘"设计直接冲突，而 smoke_test 只断言"生产指令扣资源"，没有断言人口守恒。

### 3.5 【中】两套规则引擎已经发生数值漂移

`game.js` 与 `rules.js` 各自定义一份 `TYPES / RPS / 地形生成 / LOS / 判定链 / 计分卡`。实测对比：

| 单位 | 人类版造价（cu+ru×2） | 裁判版 `cost` | 结论 |
|---|---:|---:|---|
| LAV | 100 | 100 | 一致 |
| **AAV** | **240** | **210** | **漂移 14%** |
| RAILGUN | 320 | 320 | 一致 |
| ARTILLERY | 950 | 950 | 一致 |

AAV 造价直接进入"歼敌"（击杀按造价加权）与"保存兵力"两个 15% 权重域。另外 `rof` 在 `game.js` 是秒、在 `rules.js` 是 tick，靠 `rof/0.1` 手工换算，没有任何断言把两侧钉住。

### 3.6 【低】文档与实现的口径不一致

- `referee/match.js:13` 提示"必须通过静态服务打开（不要只单独打开 HTML）"，与 `MVP/README.md` 首行"双击 `index.html` 即可运行"矛盾（普通 `<script>` 在 `file://` 下是可用的）；
- `MVP/README.md` 写"地图 128²、10 TPS、tick 上限 1200"，`referee/` 是"64²、600 tick、无 TPS 概念"，同名规则两套尺寸，文档未在同一处置顶说明；
- `referee/README.md` 宣称"全程零故障"，但 `match_test.js` 里"零故障"只是打印，未断言。

---

## 4. 与 `社区比赛__WoD` 需求的符合度

需求来源：`Plan_Beta.md` §0 决策台账 + §2 协议 + §3 Bot 架构 + §4 排名；`MVP_plan.md` §1.1 P0；主仓库 `meow-ai-arena` 的真实约定（`arena/core.py`、`land/` 目录契约、`community/check.py`）。

| 需求 | 状态 | 证据 / 差距 |
|---|---|---|
| 零依赖双击即玩 | ✅ | `index.html` 单脚本，无构建 |
| 程序生成等高线 + 严格点对称 + BFS 连通 | ✅ | `point symmetry=true`，`walls=756`，`carveCorridor` 兜底 |
| 地形影响移动/射程/伤害/视线 | ✅（人类版） | A* 穿山 0 次、绕行 7.8 格、山脉挡 LOS=true、`mountainEnter=0` |
| 直射/曲射、高地加成、三元克制 | ⚠️ 部分 | 人类版有实测击杀；**裁判版零执行** |
| Order 纯数据 + 能力表（Trait） | ✅ | 8 种 Order、`TRAITS`、`issueOrders→flushOrders→applyOrder` |
| 三段式协议 + 规则指纹 | ✅ | `hello/tick/end`、`rulesHash=ad99fa68`、版本不匹配判负 |
| 非法动作两层容错（结构错=故障 / 语义错=丢弃） | ✅（设计）／⚠️（覆盖） | 撞山只测 1 类；结构错 6 个分支未测 |
| 五级判定链 + 第 0 级故障出局 | ✅ | 裁判侧全 5 级有覆盖 |
| 八领域加权总分 | ⚠️ 部分 | 公式与 Beta 有 2 处偏离且未标注；区分度未验证 |
| 换边镜像 + 每局重新 init | ✅ | `initCount=2`、座位互换、兵力与造价对称 |
| 确定性重放 + 篡改检测 | ✅（同引擎） | 601 帧逐回合一致；篡改后报错 |
| 循环赛跑分 | ⚠️ 部分 | 有逐局表/积分榜/CSV；**无 BT/Elo、无 `key` 去重、无 gz jsonl** |
| 地图规模 512² | ❌ | 裁判 64²、人类版 128² |
| 60ms 死线 + 真进程隔离 + kill | ❌ | 同页闭包 + 墙钟；**无法中断死循环**，`README` 已如实标注 |
| 经济 / 生产 / 采集 / 迷雾 | ❌（裁判侧） | 固定兵力、全图可见，与 §2.3 信息集不符 |
| `workspace/` 选手包（RULES/starter/anchors/manifest 哈希） | ❌ | 无 |
| `bots/` 封存 + `results/games.jsonl` + `replays/viewer` | ❌ | 无；回放 schema 与主仓库 `{header,names,result,frames}` 不兼容 |
| `CONDITIONS.md` / `prompt.md` / `notes/` | ❌ | 无 |
| `community/check.py` 注册（`EPISODES` 含 wod） | ❌ | 主仓库仅 `land/bomberman/ctf/warehouse/tank/tron` |
| CI / 测试 runner / 覆盖率门槛 | ❌ | 无 `package.json`、无 `.github/workflows` |

**小结**：MVP 精准命中了它自己声明的假设（"地形能否同时影响移动/射程/伤害/视线"），但**比赛侧需求几乎全缺**。两者不是同一件事：MVP 是"玩法原型"，`Plan_Beta` 描述的是"可托管、可排名、可复现的评测系统"。

---

## 5. 架构评估

### 5.1 模块划分：分区注释 ≠ 模块

`game.js` 1786 行单 IIFE，13 个 `§` 编号注释把代码切成 Config/Terrain/Path/Entities/Combat/Fog/AI/Score/Render/Input/Replay/Loop。这是**阅读顺序**，不是**依赖边界**：

- 全部状态挂在模块级可变单例上：`G`（游戏）、`terr`（地形）、`nextId`、`logicMsSum/Count`（`game.js:367-369`）；
- 任何函数都能直接读写任何状态，没有模块私有性、没有接口、没有注入点；
- 新增一个单位类型要改 `TYPES`、`TRAITS`、`RPS`、`drawUnit` 的 `shape` 分支、`PRODUCIBLE`、生产 UI 六处。

`referee/` 的划分明显更好：`rules.js`（纯规则）/ `referee.js`（driver + 生命周期）/ `bots/`（选手）/ `tournament.js`（赛制）职责清晰，UMD 双端可用，是**这套代码里最值得继承的部分**。

### 5.2 依赖关系：逻辑反向依赖表现层

| 位置 | 问题 |
|---|---|
| `game.js:1161` | 模块加载即 `document.getElementById('view').getContext('2d')` → Node 里必须 `vm` + `Proxy` 桩才能跑，测试无法 `require` |
| `game.js:1045` | `endGame()` 内直接 `showResult()`（判 `headless` 分支）→ 胜负逻辑依赖 DOM 是否可用 |
| `game.js:445` | `enqueue()` 资源不足时 `toast()` → 经济逻辑里弹 UI |
| `game.js:473` | `allocatePower()` 内部 `updateHUD()` → 指令层直接刷视图 |
| `game.js:1076/1098` | `tick()` 里 `performance.now()` → 时钟未注入，无头跑分与真实跑分共用一套计时 |

依赖方向是 `逻辑 → 表现`，这与"引擎被 Bot 复算、被回放复算"的目标正好相反。

### 5.3 可维护性

正面：命名一致、注释密度高、`README`/`referee/README` 把与主仓库的对齐关系写得很清楚（这在同类项目里少见）。

负面（按代价排序）：

1. **规则双源**：`TYPES/RPS/地形/LOS/判定链/计分` 在两份文件里各写一遍，已漂移 1 处（AAV 210 vs 240），且无一致性测试；
2. **回放双 schema**：`game.js` 的 `{version,map,config,events,orders,digests,result}` vs `referee` 的 `{version,config,names,games[]}` vs 主仓库期望的 `{header,names,result,frames}`，三套互不兼容；
3. **测试不可组合**：4 个脚本各自手搭 DOM 桩（两份几乎相同），无 runner、无 fixture、无临时目录隔离；
4. `_archive_skeleton/` 作为上一轮快照整体入库（含 `__pycache__` 已忽略），检索时容易误入。

### 5.4 扩展性

- **双方硬编码**：`[0,1]` 在 `game.js`/`rules.js`/`referee.js` 共 8 处；`SIDE_COLOR` 2 项、`G.fog` 2 层、`buildResult()` 按 2 座统计。多方可直接说改不动。
- **`Game(n, seed)` 的 `n` 被忽略**（实测 `new Game(4)` 仍生成 2 方）。而主仓库 `run_match()` 传的是 `len(commands)`，一旦做"混战"会**静默降级成 1v1**，不报错——这是接口层面的契约谎言。
- **无单位/指令注册表**：全靠 `if (kind === ...)`/`switch`，加一个"空中单位""技能"要动十几处。
- **无事件总线**：叙事榜需要的"逐单位阵亡时间/坐标、母舰受损曲线、VP 易手时间线"目前只能从 `G.events` 里捞，且 `events` 与 `digestLog` 混在一个数组里，无 schema。

---

## 6. 潜在风险（按严重度）

| 级别 | 风险 | 影响 |
|---|---|---|
| P0 | 运行形态不兼容（浏览器 JS vs `arena/core.py` + 子进程） | 代码复用率极低，迁移实为重写 |
| P0 | 规则双源漂移 | 人类版、裁判版、正式版三方数值不一致，成绩不可解释 |
| P0 | replays/成绩 schema 三套 | 主仓库 `viewer.html`、`rate.py`(BT) 直接读不了 |
| P0 | `Game(n)` 假接口 | 混战模式静默降级，赛后才发现 |
| P0 | seed 重映射碰撞 | 样本重复、兵力规模不可控，排名置信区间失真 |
| P1 | 战斗链路 0 验证 + 对局从不交火 | 冻结规则前无法保证平衡；正式赛可能出现"零交战"局 |
| P1 | 计分卡三项恒定、"赢局低分"从未出现 | 整个排名口径的核心主张未经验证 |
| P1 | 时限不可强制（无法 kill） | 死循环 Bot 会卡死整个评测 |
| P1 | 无选手包/无哈希清单 | 无法保证"发给每位选手的包逐字节一致" |
| P2 | 输入层 0 自动化 | 交互回归全靠人肉，重构即失守 |
| P2 | 确定性只靠同引擎重跑 | 换语言后需重新建立信任 |
| P2 | 人口泄漏 | 战术空间被压缩，平衡数据失真 |
| P2 | `_archive_skeleton` 与主目录同名概念并存 | 维护者认知负担 |

---

## 7. 迁移前必须补充 / 调整的事项

### P0（阻断项，不做完不应动工）

1. **定运行形态**：主项目沿用主仓库范式——`arena/core.py` 跑 `challenge.py` 定义的 `Game` 类 + 选手子进程（C++/Go），MVP 只保留"规则原型 + 交互体感"定位。明确写一张"复用 / 重写"表。
2. **规则单源**：把 `TYPES / RPS / 权重 / 阈值 / 判定链` 抽成一份 `rules.json`（或直接以 Go/Python 为单源，JS 侧由脚本生成），并加**跨端一致性测试**（对 AAV 这类漂移直接断言）。
3. **契约对齐主仓库**：回放改成 `{header, names, result, frames}`，`result` 补 `ranks / forfeit / timeouts / errors / max_ms`；成绩落 `results/games.jsonl.gz`，逐局带 `key / swap / mode`，能被 `tournament/rate.py` 直接消费。
4. **`Game(n, seed)` 要么真支持 n，要么删参数**并在 `run_match` 处显式断言 `n == 2`，禁止静默降级。
5. **seed 语义拆开**：`requestedSeed`（外部）与 `resolvedSeed`（实际地图）分开记录，写进 `header` 与成绩表；加断言"不同 seed → 不同地图"，种子列表自动去重。

### P1（正式赛前必须有）

6. **战斗与平衡基线**：加"强制接火"用例（把单位摆进射程，断言掉血/击杀/RPS/高地加成/曲射越障），并让 600 tick 的正式配置**真的产生交火**（调 VP 数量、初始部署、视野或集结逻辑）；否则先解决"两军不相遇"。
7. **计分卡区分度**：构造 4 种结局（斩舰速胜 / VP 优势 / 龟缩平局 / 全灭），断言各域变化，并让"赢局低分"至少出现一次；附权重校准脚本（跑 N 局看各域分布再定权重）。
8. **时限可强制**：用子进程 + `kill` + CPU 时间（`core.py` 现成），把 50ms 统一到 `Plan_Beta` 的 60ms 或明确改口径，写进 `CONDITIONS.md`。
9. **选手包**：`workspace/{RULES.md, engine/本地裁判, starter, anchors, viewer, workspace_manifest.json}` + `bots/` 封存哈希。
10. **社区接入**：`community/check.py` 的 `EPISODES/BOT_EPISODES` 加 `wod`，补 `_example`、`LEADERBOARD`、`prompt.md`、`AGENTS` 提示词。

### P2（质量债务，可与上线并行）

11. **测试工程化**：`package.json` + `node --test`（或统一 runner），统一退出码与失败摘要，加覆盖率门槛（逻辑层 ≥ 80%），接 CI。
12. **逻辑/表现解耦**：把 `rules + AI + 计分` 抽成无 DOM 模块（ESM/UMD），`game.js` 只留渲染与输入；输入层用"录制指令 → 回放断言"的方式测。
13. **修 `pop` 泄漏**；把 `toast/updateHUD/showResult` 从逻辑层移出（回调/事件）；`stateDigest` 补 `power/queue/state/path`。
14. **冻结引擎行为**：JS 版固定 Node/浏览器版本与浮点行为写进 `CONDITIONS`；确定性校验改为**跨进程**重放一次。
15. **版本归属**：`MVP/` 目前被外层 `.gitignore` 忽略且自带独立仓库。迁移时应复制文件到 `war_desert/`，不要让主项目代码继续躺在一个被忽略的实验仓里。

---

## 8. 可行性结论

**判定：可行（Feasible），但属于"重写式迁移"，不是"搬迁式复用"。**

可以带走的（价值最高，建议原样固化为契约）：

1. **Order 模型**：`{type, ids, x, y, targetId, queued}` + 能力表分派 + 下一 tick 统一 flush + 两层容错。这是整套设计里最经得起推敲的部分。
2. **三段式协议与 `rulesHash`**：`hello` 一次给静态图、`tick` 只给增量、`end` 给计分卡；改一个平衡数值就让旧 Bot 明确出局，而不是跑出无法解释的结果。
3. **确定性基础设施**：`orderLog + frameHashes + verify()`，以及"故意篡改必须报错"这类自证式测试写法。
4. **判定链 + 八领域 + 换边镜像 + 循环赛**：赛制骨架已端到端跑通。
5. **地形修正链路**：点对称 + BFS 连通 + 山脉同时挡路挡视线 + 直射/曲射不对称，已被实测数据支撑（`pathOnMountain=0`、`maxLateralOffset=7.8`、`mountainEnter=0`）。

不能带走、必须重写的：

1. 渲染与输入（Canvas 2D + DOM，约 700 行）——主项目走 Three.js 回放，这套 UI 只在原型期有价值；
2. 进程隔离、时限、kill、CPU 计时（`rules.js/referee.js` 的同页模拟）；
3. 规则数值表与地图生成（须以 Go/Python 为单源，否则漂移会继续）；
4. 回放与成绩 schema（须按 `{header,names,result,frames}` 与 `games.jsonl` 重建）；
5. 双方硬编码的状态结构（`sides`/`fog`/`buildResult`）。

**行动建议（按顺序）**：

1. 把 MVP 冻结为"规则原型 v0.2"，只做 §7 的 P0 + P1 第 6/7 项（战斗、seed、计分卡），让它成为可信的**规则参考实现**；
2. 以 `war_desert/` 为落点，按 `land/` 的目录契约搭 `challenge.py + workspace/ + bots/ + tournament/ + results/ + replays/`；
3. 引擎用 Go（`Plan_Beta` §3）或 Python（复用 `arena/core.py`）重写，把上述 5 项契约作为**跨实现回归测试**：同一 `orderLog` 在 JS 原型与目标引擎上必须产生相同 `frameHashes`——这条测试通过之日，迁移才算真正完成；
4. 在此之前，不要用 MVP 的成绩做任何平衡或排名结论：它现在能证明"地形好玩"，还不能证明"排名可信"。

---

## 附：证据索引

| 结论 | 位置 |
|---|---|
| seed 重映射 | `referee/rules.js:190-200`、`:232`、`:248` |
| 战斗零执行 | `referee/rules.js:374-393`（`canShoot/fire`），覆盖率数据 |
| AAV 造价漂移 | `game.js:38`（180/30）vs `referee/rules.js:32`（210） |
| 人口泄漏 | `game.js:422`、`:447`（只加不减） |
| 加载即碰 DOM | `game.js:1161-1164` |
| 逻辑依赖表现 | `game.js:445`、`:473`、`:1045` |
| `Game(n)` 忽略 n | `referee/rules.js:225`（`constructor(n, seed, cfg)` 未使用 `n`） |
| 双方硬编码 | `game.js:396-404`、`referee/rules.js:236`、`referee/referee.js:181` |
| 回放 schema 分歧 | `game.js:1519-1538` vs `referee/referee.js:213-221` vs 主仓库 `arena/core.py:252` |
| 摘要字段不全 | `game.js:1053-1073` |
| 计分口径偏离 | `game.js:1007-1016` vs `Plan_Beta.md:172-182` |

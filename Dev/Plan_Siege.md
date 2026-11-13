# Desert Arena · 攻守对战改造与母舰 HP 实验 · 实施方案

> 状态：待执行
> 目标：把对称 VP 对局改成“一方守擂、一方进攻”的不对称对局；胜负只看母舰是否被摧毁；用梯度 HP 实验找出使摧毁率落在 **20%～45%** 的母舰 HP。
> 约束：不修改 `Dev/MVP/referee/rules.js` 冻结预言机；新建 `war_of_desert/engine/oracle/rules_siege.js` 作为独立变体，旧赛季继续按 `rulesHash=906f1b14` 复现。

---

## 1. 规则改动点

### 1.1 阵营与目标

| 项 | 旧规则 | 新规则 |
|---|---|---|
| 对局形式 | 红蓝对称 1v1 | **守方（side 0）vs 攻方（side 1）** 不对称 1v1 |
| 守方 | 与攻方相同 | 母舰 + 初始部队 + **工事预算**；工事只能布置在母舰周围 |
| 攻方 | 与守方相同 | 母舰 + 更多初始部队 + 更高起始 CU；**没有任何工事** |
| 胜负 | 斩舰 → VP → KD → 母舰 HP → 平局 | **守方母舰被摧毁 = 攻方胜**；**攻方母舰被摧毁 = 守方胜**；**一方单位全灭 = 对方胜**；**时间到 = 守方胜**（攻方未能摧毁母舰） |
| 时间 | 600 tick（30 s） | **1800 tick（90 s）**，给攻城留出时间 |
| VP | 4 个 VP，占点累计分 | **删除 VP 与 VP 判定** |

### 1.2 矿场（替换 VP）

- 原 4 个 VP 点改为 **4 个矿场**：
  - 半径沿用 `VP_RADIUS`（4.2 逻辑格 = 16.8 世界格）。
  - 占领规则：矿场半径内只有一方非结构单位 → 该方占领；双方都有 → contested，无收入。
  - 收入：每 tick 给占领方 **0.8 CU**；矿场内每有 1 台 SALVAGER，额外 **+1.2 CU/tick**。
  - 每个矿场有 **600 CU** 储量，采完变为枯竭（owner 仍保留，但无收入）。
  - 矿场收入直接进 `cu`，用于生产单位；不再有独立的 `res` 采集点。
- 删除 `updateVP` 的 VP 计分；新增 `updateMines`。
- `hello` / `tick` / `frame` / `result` 的 `vp` 字段改为 `mines`；`res` 置空。

### 1.3 工事

新增两种结构单位，只能由守方在 **布置阶段（前 80 tick）** 放置：

| 工事 | HP | 射程 | 伤害/ROF | 造价（工事点） | 作用 |
|---|---:|---:|---|---:|---|
| `WALL` | 520 | 0 | 无 | 1 | 阻挡移动与炮弹，逼迫攻方破墙 |
| `TURRET` | 240 | 9 逻辑格（36 世界格） | 12 / 10 tick | 3 | 固定火力，守方的主要输出 |

- 守方工事预算 **12 点**；每 tick 最多放 1 个；布置半径 **34 世界格**（母舰周围）。
- 放置校验：不能在山脉、不能与其他单位重叠、必须在半径内、必须在布置阶段、预算足够。非法放置只记 `rejected`，不判故障。
- 攻方工事预算 0，`fort` 动作直接拒绝。
- 工事不占人口、不参与生产、不参与“单位全灭”以外的经济计算；`WALL` 无武器，`TURRET` 在 `neutral` 模式下自动开火。

### 1.4 初始配置

| 项 | 守方 | 攻方 |
|---|---|---|
| 母舰 | 1 台，HP = 实验变量 | 1 台，HP = 3000（固定） |
| LAV | 3 台 | 4 台 |
| SALVAGER | 1 台 | 1 台 |
| 起始 CU | 700 | 900 |
| 工事预算 | 12 | 0 |

- 双方基础收入仍为 1.5 CU/tick；矿场收入是额外经济。
- 守方工事 + 母舰火力 + 3 LAV 构成防御；攻方 4 LAV + 更高起始 CU 构成进攻波次。

### 1.5 结束条件（按优先级）

1. 守方母舰被摧毁 → 攻方胜（`level=1`）。
2. 攻方母舰被摧毁 → 守方胜（`level=1`）。
3. 守方所有单位死亡（含工事）→ 攻方胜（`level=2`）。
4. 攻方所有单位死亡 → 守方胜（`level=2`）。
5. `turn >= tickLimit` 且守方母舰存活 → 守方胜（`level=3`，时间到）。
6. 双方同时满足 → 平局（`level=0`，只在极端同归于尽时出现）。

> “单位全部死亡也算结束”覆盖两种情况：攻方打光了守方所有单位（含母舰），或守方反推打光了攻方所有单位。若守方只剩工事、母舰已死，条件 1 已触发；若守方只剩工事、母舰还活着，游戏继续，攻方仍需摧毁母舰。

### 1.6 计分卡（诊断用，不参与 HP 实验判定）

八领域改为：矿场控制、歼敌、保存兵力、母舰完整度、KD、矿区控制、矿场收入、算力。权重 `[0.25,0.20,0.15,0.15,0.10,0.05,0.05,0.05]`。HP 实验只看“攻方是否摧毁守方母舰”，计分卡仅用于复盘。

---

## 2. 母舰 HP 梯度设计

### 2.1 梯度

守方母舰 HP 从 3000 逐级削弱：

```text
3000, 2400, 1900, 1500, 1200, 950, 750, 600, 480, 380, 300
```

- 先做 **pilot**：3000 / 1500 / 800 / 400，各 8 个 seed，确认曲线单调、没有 0% 或 100% 的平台。
- 若 3000 时摧毁率已 >45%，整体下移（2400 起）；若 300 时仍 <20%，整体上移或加强攻方。
- 正式 sweep 用 11 个 HP × 40 个 seed = **440 局**；同一 seed 在所有 HP 上复用（配对设计，降低地图方差）。

### 2.2 实验固定项

- 攻方 Bot、守方 Bot、工事布局、起始 CU、矿场参数、seed 集合全部固定；**只变守方母舰 HP**。
- 攻方母舰 HP 固定 3000，避免守方反推能力随 HP 变化。
- 每局 `verify=true`，丢弃 `verify.ok=false` 的局（正常应为 0）。
- 先修单位口径（射程/步长），否则攻方不会开火，实验无意义。

### 2.3 统计方法

- 每个 HP 的摧毁率 `p = 攻方胜局 / 有效局数`。
- 95% 区间用 **Wilson score interval**（小样本比正态近似稳）。
- 主判据：`p ∈ [0.20, 0.45]`。
- 若多个 HP 落在区间内，取 **满足条件的最高 HP**（守方最强、仍可被攻破），并报告整个区间。
- 辅助：对 `logit(p) = a + b·HP` 做加权最小二乘/逻辑回归，解 `p=0.20` 与 `p=0.45` 的交点，给出连续 HP 区间；若观测曲线非单调，以观测区间为准并注明。
- 同步记录：平均摧毁 tick、攻方剩余单位、守方工事存活数、矿场归属、双方损失。

### 2.4 判定“找到”的标准

- 至少一个 HP 的 95% CI 覆盖 20%～45% 区间内的点，且点估计在区间内；
- 相邻 HP 的摧毁率单调下降（允许 1 个反例，需在报告中说明）；
- 报告给出“推荐 HP”与“可接受 HP 区间”，不只给一个点。

---

## 3. 实现步骤

### 3.1 规则变体

1. `cp engine/oracle/rules.js engine/oracle/rules_siege.js`；只改副本，冻结文件不动。
2. 新增常量：`MINE_RATE`、`MINE_AMOUNT`、`MINE_SALVAGE_BONUS`、`FORT_BUDGET`、`FORT_COST`、`FORT_RADIUS`、`PLACE_TICKS`、`SIEGE_WEIGHTS`。
3. `TYPES` 增加 `WALL` / `TURRET`（`structure: true`）。
4. 构造函数：`mode='siege'`、`mothershipHp`、`attackerMothershipHp`、`fortBudget`、`placeTicks`、`attackerCu`、`defenderCu`、`attackerStartLav`、`defenderStartLav`；初始化矿场、守方工事预算、初始兵力。
5. `updateVP` → `updateMines`（占领、收入、枯竭、contested）。
6. `checkEnd` 改为攻守判定；删除 VP/KD/HP 比链。
7. `header` / `encodeTick` / `frame`：`vp` → `mines`，增加 `mode`、`roles`、`siege` 块、`me.fortBudget`/`phase`。
8. `decode` / `step`：新增 `fort` 动作、`placeFort`、`orderLog` 记录。
9. `computeCards` 改为攻守版；`resultData` 增加 `mode`、`mothershipHp`、`destroyed`、`destroyedAt`。
10. `rulesHash` 纳入 `mode`、`mothershipHp`、`fortBudget`、`placeTicks` 等，保证不同 HP 不同指纹。
11. `verifyDeterminism` 重建时带上 siege 配置。

### 3.2 宿主与 Go 执行链

1. `oracle_host.js` 支持 `--rules` 参数（默认旧 `rules.js`），`new` 命令透传 siege 配置；`verify` 回传同一配置。
2. `internal/engine.HostConfig` 增加 `Mode`、`MothershipHP`、`AttackerMothershipHP`、`FortBudget`、`PlaceTicks`、`AttackerCU`、`DefenderCU`、`AttackerStartLav`、`DefenderStartLav`；`Start` 支持 rules 路径。
3. `judge.Config` 增加 `RulesPath`；`RunMatch` 透传。
4. `protocol.Result` 增加 `Mode`、`MothershipHP`、`Destroyed`、`DestroyedAt`，供 sweep 读取。

### 3.3 选手包与新 Bot

1. `botkit` 增加 `Mines`、`Hello.Mode/Roles/Siege`、`State.Phase`、`Me.FortBudget/FortsPlaced/MinesOwned/MineIncome`。
2. 新增 `workspace/siege/attacker` 与 `workspace/siege/defender`：
   - 攻方：生产 LAV/AAV → 沿地图推进 → 优先打挡路工事 → 集火守方母舰；分 1 台 SALVAGER 占矿。
   - 守方：布置阶段放 6 墙 + 2 炮（或 9 墙 + 1 炮）→ 生产 LAV → 守在本方母舰与矿场之间 → `neutral` 模式自动开火。
   - 修复旧的射程/步长口径：射程判断用 `t.Range * Cell`，步长用 `t.Speed * Cell * 0.1`（引擎当前移动积分）。
3. `cmd/siege`：并行跑 HP × seed，输出 `results/siege/sweep.csv`、`sweep.md`，含 Wilson CI 与推荐 HP。

### 3.4 实验流程

1. 编译两个 Bot；跑 1 局 smoke（HP=1500，seed=20261201），确认守方放工事、攻方开火、`verify.ok=true`。
2. pilot：4 个 HP × 8 seed，检查曲线与 0%/100% 平台。
3. 调整攻方起始 CU / 工事预算（只调一次，记录原因），使 3000 时 <45%、300 时 >20%。
4. 正式 sweep：11 HP × 40 seed × 8 并发；输出 CSV/Markdown。
5. 拟合/查表，给出推荐 HP 与区间；把结果写入 `results/siege/结论.md`。

### 3.5 验收标准

- `rules_siege.js` 可在 Node 下独立 `new Game`，1 局 1800 tick 无异常。
- 守方能在前 80 tick 放下工事；攻方 `fort` 被拒；矿场收入可见。
- 至少 1 个 HP 的摧毁率落在 20%～45%，且 95% CI 与区间有交集。
- 所有局 `verify.ok=true`；`rulesHash` 随 HP 变化。
- 报告给出推荐 HP、可接受区间、实验局数、每 HP 的 p 与 CI。

---

## 4. 风险与回退

| 风险 | 处理 |
|---|---|
| 攻方 Bot 太弱，所有 HP 0% | 提高攻方起始 CU/LAV 数；给攻方 AAV；先修射程/步长；降低炮塔伤害 |
| 攻方 Bot 太强，所有 HP 100% | 增加工事预算/炮塔数量；降低攻方起始 CU；提高守方母舰武器 |
| 工事完全封死，攻方无法破墙 | 降低 WALL HP；给攻方 RAILGUN/ARTILLERY；守方布局留缺口 |
| 1800 tick 太短，攻方走不到 | 当前单位速度约 25 世界格/s，255 格约 10 s；90 s 足够 |
| 旧赛季被污染 | 只新增 `rules_siege.js`，旧 `rules.js` 与 `rulesHash` 不动；新配置走 `tournament/siege.json` |
| 结果不单调 | 增加 seed 数；检查是否有地图异常（工事卡山、出生点过近）；报告注明 |

---

## 5. 交付物

- `war_of_desert/engine/oracle/rules_siege.js` + `MANIFEST_siege.json`
- `war_of_desert/engine/oracle_host.js`（支持 `--rules`）
- `war_of_desert/internal/engine/host.go`、`internal/judge/judge.go`、`internal/protocol/protocol.go` 的兼容字段
- `war_of_desert/workspace/siege/attacker/`、`defender/`、`siegebot/`
- `war_of_desert/cmd/siege/main.go`
- `war_of_desert/results/siege/sweep.csv`、`sweep.md`、`结论.md`
- `war_of_desert/赛事手册_攻守.md`（规则说明）

---

## 6. 执行结果（2026-11-13）

### 6.1 实际参数（与方案初稿的差异）

| 项 | 方案初稿 | 实际执行 | 原因 |
|---|---|---|---|
| tickLimit | 1800 | **1200** | 1800 下 1500 HP 的摧毁率已在 15%～25%，目标带被推到 1200 以下；1200 更能体现“破防后剩余输出窗口” |
| 工事预算 | 12 | **24** | 12 点只能放 6 墙 + 2 炮，攻方 7/8 seed 破防；24 点放 8 墙 + 5 炮，才让 HP 成为第二变量 |
| TURRET | 240 HP / 12 dmg | **280 HP / 14 dmg** | 提高守方火力，避免攻方 LAV 海无脑淹没 |
| HP 梯度 | 3000→300 | **3000→1000** | 3000/2200 在 1200 tick 下 0%，1800 5%，1600～1000 进入 12.5%～47.5%；目标带在 1200～1300 |
| seed 数 | 40 | **40** | 10 个 HP × 40 = 400 局；0 故障，400/400 复核 |

### 6.2 结果

| HP | 局数 | 攻方胜 | 摧毁率 | 95% CI |
|---:|---:|---:|---:|---|
| 3000 | 40 | 0 | 0.0% | 0.0% ~ 8.8% |
| 2200 | 40 | 0 | 0.0% | 0.0% ~ 8.8% |
| 1800 | 40 | 2 | 5.0% | 1.4% ~ 16.5% |
| 1600 | 40 | 5 | 12.5% | 5.5% ~ 26.1% |
| 1500 | 40 | 5 | 12.5% | 5.5% ~ 26.1% |
| 1400 | 40 | 7 | 17.5% | 8.8% ~ 31.9% |
| 1300 | 40 | 10 | 25.0% | 14.2% ~ 40.2% |
| **1250** | 40 | **13** | **32.5%** | **20.1% ~ 48.0%** |
| 1200 | 40 | 14 | 35.0% | 22.1% ~ 50.5% |
| 1000 | 40 | 19 | 47.5% | 32.9% ~ 62.5% |

- **目标带（点估计）**：1200～1300；**推荐 1250 HP（32.5%）**。
- 1400（17.5%）低于 20%，1000（47.5%）高于 45%，作为边界参照。
- 7/40 个 seed 在 1400/1300/1250/1200 之间发生胜负翻转；HP 主要影响“破防后能否杀完”，不是“能否破防”。

### 6.3 执行中修掉的问题

1. **装甲科技覆盖母舰 HP**：`applyTech` 用 `TYPES.MOTHERSHIP.hp=3000` 重算 maxHp，导致所有 HP 梯度在 A1/A2 后收敛到 3660；改为用 `cfg.mothershipHp` 作基础值。
2. **工事越界**：守方母舰靠地图边缘时，`fortPlan` 会生成负坐标/超界坐标，`decode` 判 `coords out of range`（结构性故障）；改为跳过越界候选，并给防守站位加 `clampWorld`。
3. **攻方攻击阈值过大**：Bot 用 `range*1.15` 发 `attack`，但引擎按 `range` 判定，导致单位站在 85 px 处“攻击”却不发炮、也不前进；改为 `range*0.92`。
4. **工事候选索引**：用 `FortsPlaced` 当索引用，候选被地形拒绝时会反复重试同一个点；改为本地 `fortIdx` 递增。

### 6.4 产物

- `war_of_desert/engine/oracle/rules_siege.js` + `MANIFEST_siege.json`
- `war_of_desert/workspace/siege/{siegebot,attacker,defender}`
- `war_of_desert/cmd/siege/main.go`
- `war_of_desert/results/siege/{hp*_clean.csv,summary.csv,结论.md}`
- `war_of_desert/赛事手册_攻守.md`

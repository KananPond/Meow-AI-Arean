# Desert Arena · 正式赛事（Go 执行链 + Go 原生引擎）

> **状态：可开赛**（当前正式赛 = **v1.4 · 第 5 季**：母舰削弱（HP/攻击/移速/坡限）+ 对称经济 + 脱战自愈 + 攻坚对舰倍率 + 进程沙箱；v1.0 第 1 季 / v1.1 第 2 季 / v1.3 第 4 季 / v1.4 第 5 季已完赛，见 [`LEADERBOARD.md`](LEADERBOARD.md)）。
>
> **另有 v1.5 实验分支**（非正式赛季）：胜利分三本账、遗产物、插旗、CEP／射界、单位等级，由 `GameCfg.Ruleset == "v15"`（`sim.RulesetV15`）开关隔离，v1.4 语义与指纹 `553785d4` 不变。规则与 90 局标定见 [`../Dev/实验/Plan_V15.md`](../Dev/实验/Plan_V15.md)，产物见 [`results/exp-v15/`](results/exp-v15/README.md)，配置 [`tournament/experiment_v15.json`](tournament/experiment_v15.json)。**参赛仍以 v1.4 为准。**
>
> **AI 参与声明**：本仓库的引擎移植、参考 Bot、测试与文档由 AI（ZCode 智能体）在人类指导下参与编写；
> 规则设计、赛制与验收由人类主导，全部对局结果均经引擎内置的确定性复核门禁（golden trace + orderLog 重放）。
>
> **赛事手册**：[`赛事手册.md`](赛事手册.md) ｜ **规则基线（v1.0 历史）**：[`../Dev/Plan_Beta.md`](../Dev/Plan_Beta.md) 快照
> **引擎双轨**：`engine.kind` 二选一——`oracle`（只读快照 `MVP@288db07f`，`engine/oracle_host.js` 承载，校验 `python -X utf8 scripts/verify_oracle.py`）
> 或 `native`（**Go 原生引擎**：规则语义在 `internal/sim`，宿主 `cmd/enginehost`，当前赛季配置 [`tournament/formal_v14.json`](tournament/formal_v14.json)）
> **执行链**：Go 标准库，零第三方依赖。当前正式赛规则 = **v1.4**（四子系统分配 / CU-RU 运输线经济 / 1200 tick / 七领域计分 / 队列跳过 / 母舰 RU 基础收入 / 资源成对同类 / 脱战自愈 / 母舰削弱）

---

## 这个目录是什么

`war_of_desert/` 是正式赛事所在目录：能开赛、能判罚、能出成绩。

- 选手交一个 Go 程序，官方编译、封存源码哈希；
- 裁判为每个座位拉起独立进程，执行 120 ms 决策死线、连续 3 次红牌、超时 kill 与重启、换边重开；
- 赛程先冻结再执行，赛后产出逐局结果、完整回放、确定性复核、BT 实力值与七领域加权总分；
- v1.0 赛季的规则语义来自冻结快照 oracle；**v1.1 起规则引擎为 Go 原生**
  （移植自 MVP 合并版，golden trace 回归：`engine/golden/` + `internal/sim/golden_test.go`）。

第 1 季基准赛结果：5 名选手、20 场、40 局、零故障、40/40 确定性复核通过。

```text
第 1 季（oracle，600 tick）BT：Nomad → Vanguard（锚）→ Swarm → Bulwark → Starter
第 2 季（native，1200 tick）BT：Nomad → Vanguard（锚）→ Bulwark → Starter → Swarm
第 4 季（native，v1.3，6 人）BT：Nomad → Bulwark → Sapper → Starter → Swarm → Vanguard（锚）
第 5 季（native，v1.4，6 人）BT：与第 4 季逐局相同（母舰削弱未被任何对局触及）
```

两季 Nomad 都是 BT 第一；v1.1 运输线经济重置环境后 Swarm 从第 3 跌到第 5。
两榜不一致是设计目标：胜负与质量是两条独立信号。详见 [`LEADERBOARD.md`](LEADERBOARD.md)。

**开发与历史资料在 [`../Dev/`](../Dev/README.md)**：MVP 规则源码（2026-11-13 已解冻）、原作调研、Plan/ORACLE、历次审查报告、第三方参照模板。它们不参与正式赛执行。

---

## 快速开始

裁判台在仓库根目录（`社区比赛__WoD/裁判台.bat`），不在本目录：

```bash
双击 ../裁判台.bat         # 或 py -3 -X utf8 ../referee_desk.py
```

它有两个页签：「赛季」跑本目录里冻结好的正式赛季；「工作区」扫描
`../算法赛/RTS算法赛@<模型名>/`，勾选任意几个临时编排一场评测。

窗口里选赛季配置，按顺序点「环境自检 → 编译封存 → 冻结赛程 → 开赛 → 评分」，
进度条按场次走，日志实时滚动；右上角封存灯直接反映 `arena seal` 的结果。

组织方旧菜单（功能与裁判台重叠，保留给脚本化场景）：

```bash
python -X utf8 start.py          # 交互菜单：build / plan / run / rate / check / selftest / season / status
python -X utf8 start.py season   # 整季流水线：build → plan → run → rate
```

在 **`war_of_desert/`** 执行：

```bash
CFG=tournament/formal_v14.json   # 当前赛季（v1.4）
go run ./cmd/arena selftest -config $CFG   # 验证超时 kill / 崩溃重启 / 非法结构红牌
go run ./cmd/arena build    -config $CFG   # 编译 6 名选手（5 个参考 Bot + Starter）并封存源码哈希；v1.5 实验的 Hunter 不在此配置内
go run ./cmd/arena plan     -config $CFG   # 冻结赛程 → results/formal-v14/赛程.json
go run ./cmd/arena run      -config $CFG   # 跑完 45 场 / 90 局 → 结果.jsonl + 回放（开赛前校验封存）
go run ./cmd/arena rate     -config $CFG   # BT + 加权总分 → 成绩.md / 成绩_加权总分.md / 成绩.json
```

单项自检（选手包默认入口）：

```bash
go run ./cmd/arena check -config tournament/formal_v14.json -seed 20261109
```

观赛：把 `results/*/回放/match-NNN.json` 拖进 [`回放查看器.html`](回放查看器.html)
（零依赖单文件，浏览器直接打开；地形 / 单位 / VP / 记分 / 终局复核徽标）。

---

## 攻守对战（实验）

v1.0 之外的独立变体：一方守擂、一方进攻，守方有工事、攻方没有；VP 改为矿场，胜负只看母舰是否被摧毁。
规则与协议见 [`赛事手册_攻守.md`](赛事手册_攻守.md)，标定过程见 [`results/siege/结论.md`](results/siege/结论.md)。

**当前采纳参数**（40 局/档 + 样本外验证标定）：

| 参数 | 值 |
|---|---|
| 母舰 HP | 1250（双方相同） |
| 守方工事预算 | 16（5 炮塔 + 1 墙） |
| tickLimit | 1260（63 s） |
| 攻方胜率 | 45.0%（80 局）/ 46.0%（100 局），0 故障 |

只读快照 `engine/oracle/rules.js` 未改动；攻守规则在 `engine/oracle/rules_siege.js`（`MANIFEST_siege.json` 记哈希），
旧赛季仍按 `rulesHash=906f1b14` 复现。

```bash
# 参考 Bot
go build -o bots/bin/attacker.exe ./workspace/siege/attacker
go build -o bots/bin/defender.exe ./workspace/siege/defender

# 单局 / 批量标定（默认就是上面的参数）
go run ./cmd/siege -smoke -seed 20261201
go run ./cmd/siege -seeds 20261201-20261240 -workers 4 -out results/siege/run.csv

# 契约自检：三模式（obey / neutral / aggressive）的移动与索敌边界
node engine/oracle/selftest_modes.js
```

**BT 评分**：攻守不对称，用攻/守双参数模型，不是单值实力值。

```bash
# 编一组风格不同的参考 Bot（同一份源码，-ldflags 注入变体）
for v in v1 v2 v3 v4; do
  go build -ldflags "-X desertarena/workspace/siege/siegebot.Variant=$v" \
    -o bots/bin/pool_$v.exe ./workspace/siege/pool
done

# 循环赛 + 拟合（有序对 × seed 各一局）
go run ./cmd/siegepool \
  -pool "v1=bots/bin/pool_v1.exe,v2=bots/bin/pool_v2.exe,v3=bots/bin/pool_v3.exe,v4=bots/bin/pool_v4.exe" \
  -seeds 20261301-20261316 -workers 4 -out results/siege/bt_pool

# 只重算评分，不重跑对局
go run ./cmd/siegepool -from results/siege/bt_pool.csv -out results/siege/bt_pool
```

口径见 [`赛事手册_攻守.md`](赛事手册_攻守.md) §10，结果见 [`results/siege/bt_pool.md`](results/siege/bt_pool.md)。

---

## v1.5 规则实验（非正式赛季）

独立于正式赛的规则验证分支，**不改 v1.4 语义，也不发给选手**。开关是
`GameCfg.Ruleset == "v15"`（`sim.RulesetV15`），配置 [`tournament/experiment_v15.json`](tournament/experiment_v15.json)，
规则实现 `internal/sim/rules15.go`，设计与参数见 [`../Dev/实验/Plan_V15.md`](../Dev/实验/Plan_V15.md)。

- 胜负改为**胜利分三本账**（击杀／遗产／插旗），删除被动 VP 加分；判胜顺序：母舰毁 → VS≥300 → 时间到比 VS → KD → 母舰 HP → 平。
- 遗产物（刷新／运输入库）、可摧毁的旗（160 HP）、CEP 与射界／炮塔转速、火控科技 C1–C3、5 级单位等级。
- 1500 tick、6 Bot × 3 seed（含新增 [`workspace/bots/hunter/`](workspace/bots/hunter/)）、90 局，规则指纹 `24d4935b`；v1.4 指纹仍是 `553785d4`。

```bash
CFG=tournament/experiment_v15.json
go run ./cmd/arena build -config $CFG
go run ./cmd/arena plan  -config $CFG
go run ./cmd/arena run   -config $CFG
py -3 -X utf8 scripts/exp_metrics.py results/exp-v15
```

产物与标定数据见 [`results/exp-v15/README.md`](results/exp-v15/README.md)。实验批次**不跑 `arena rate`**，
因此没有 BT 榜／加权总分。

---

## 目录

```text
社区比赛__WoD/
├── war_of_desert/            ← 正式比赛（本文件所在目录）
│   ├── README.md            本文件
│   ├── 赛事手册.md           正式赛制、流程、判罚、申诉、运营清单
│   ├── LEADERBOARD.md       两季榜单与读法
│   ├── 回放查看器.html       零依赖回放播放器（拖入 match-NNN.json 观赛）
│   ├── go.mod
│   ├── cmd/
│   │   ├── arena/           build / plan / run / rate / check / selftest
│   │   ├── siege/           攻守：单局 smoke + 母舰 HP 扫描
│   │   ├── siegepool/       攻守：循环赛 + 双参数 BT 评分
│   │   └── faultbot/        自检用故障注入 Bot
│   ├── internal/
│   │   ├── sim/             Go 原生规则引擎（rules15.go = v1.5 实验分支；v14 语义不变）
│   │   ├── protocol/        hello / tick / actions / end 的线上契约与地图解码
│   │   ├── engine/          引擎宿主客户端（行分隔 JSON）
│   │   ├── judge/           进程隔离、死线、红牌、重启、回放、确定性复核
│   │   ├── schedule/        赛程冻结（换边、seed 递增）
│   │   ├── rating/          Bradley–Terry（v1.0 单值）+ 攻守双参数
│   │   └── contest/         配置加载与路径解析
│   ├── engine/
│   │   ├── oracle_host.js   宿主：命令流 → 快照 oracle（可按参数加载规则变体）
│   │   └── oracle/          只读快照 + MANIFEST.json + rules_siege.js
│   ├── tournament/formal.json       第 1 季（v1.0，历史）
│   ├── tournament/formal_v11.json   第 2 季（v1.1，历史）
│   ├── tournament/formal_v12.json   第 3 季（v1.2，赛程作废）
│   ├── tournament/formal_v13.json   第 4 季（v1.3）
│   ├── tournament/formal_v14.json   第 5 季（v1.4，当前正式赛）
│   ├── tournament/experiment_v15.json  v1.5 实验（非正式赛季，1500 tick）
│   ├── workspace/           引擎侧参考工作区（Starter 选手 + botkit/refbot 共用库）
│   │   ├── bots/sapper/     攻坚参考 Bot（RAILGUN 拆舰，v1.3 新增）
│   │   ├── bots/hunter/     遗产／插旗参考 Bot（v1.5 实验新增）
│   │   └── siege/           攻守参考 Bot：siegebot 库 + attacker / defender / pool
│   ├── bots/                封存记录与编译产物（bin 不入库）
│   ├── results/formal/      第 1 季：赛程、结果、成绩、回放
│   ├── results/formal-v14/  第 5 季（v1.4，当前）：赛程、结果、成绩
│   ├── results/exp-v15/     v1.5 实验：赛程、结果、回放（本地产物，回放不入库）
│   ├── results/siege/       攻守：HP 扫描、工事预算标定、BT 榜单与结论
│   ├── replays/             历史赛季归档（预留）
│   ├── notes/               赛前狠话、赛后复盘（预留）
│   ├── start.py             组织方菜单（默认走当前赛季 formal_v14.json）
│   ├── 开赛.py               旧版计时脚本，已被根目录 赛务.py 取代（保留备查）
│   └── workspace/clock.py   选手侧计时展示
└── Dev/                     开发/历史资料区（见 ../Dev/README.md）
```

`../Dev/` 内部：

```text
Dev/
├── README.md                 资料区索引
├── Plan_Beta.md              v1.1 规则基线与迁移规格
├── 实验/Plan_Siege.md        攻守改造与标定记录
├── 审查/设计审查_正式版v1.0_20261113.md
├── ORACLE.md                 预言机运行与复现规范
├── Docs/                     原作调研、事实卡片、版权与术语
│   └── archive/              已归档：Plan_Alpha、MVP_plan、历次审查/修复/评估报告
├── MVP/                      JS 规则实现（正式版，含 正式比赛测试/）
└── reference/模板/           第三方只读参照
```

---

## 架构

```text
cmd/arena ──► judge ──► engine(宿主客户端) ──► node engine/oracle_host.js
                 │                                   │
                 ├──► Bot 进程 A（stdin/stdout）      └──► engine/oracle/rules.js（只读）
                 └──► Bot 进程 B（stdin/stdout）
```

- **规则唯一来源**：`engine/oracle/` 是冻结提交 `288db07f` 的 `rules.js / mapgen.js / mapgen_mc.js / ballistics.js` 快照，SHA-256 记在 `MANIFEST.json`。宿主不修改规则，不读墙钟，不写文件。
- **进程边界**：引擎宿主、每个 Bot 都是独立操作系统进程；裁判用真实管道收发 UTF-8 行分隔 JSON。Bot 拿不到裁判内存，也无法通过原型链改规则。
- **失败隔离**：Bot 超时/崩溃 → kill → 重启 → 重发 hello；引擎宿主异常 → 整场失败并记录，不产出半截成绩。
- **未来替换**：Go 引擎实现后，只需替换 `engine/oracle_host.js`，`internal/*`、选手包与成绩契约不变。

### 与 `Plan_Beta.md` 的关系

- 引擎迁移验收（§5.2/§6.2）三件套已就位并通过：Go 自确定性（裁判逐局重放复核，第 2 季 40/40）、golden trace 回归（`engine/golden/` + `internal/sim/golden_test.go`）、绝对锚点与 JS 契约测试（`internal/sim/sim_contract_test.go` / `game_test.go`）；
- 第 1 季的 `hello` 113 KB 超 64 KiB 是 §11 已登记偏差，v1.1 原生引擎的 hello 是否收窄见 `internal/protocol`；
- 赛制层（BT、赛程、收卷、榜单）两季均已落地。

---

## 产物契约

以当前赛季目录 `results/formal-v14/` 为例（历史赛季目录结构相同，v1.0 为八领域）：

| 文件 | 内容 |
|---|---|
| `results/formal-v14/赛程.json` | 冻结的选手、场次、seed、规则与判罚口径 |
| `results/formal-v14/结果.jsonl` | 每局一行：场次、座位、胜者、层级、VP、七领域总分、故障、确定性、规则指纹 |
| `results/formal-v14/成绩.md` | BT 实力值、95% 区间、胜平负、总分 |
| `results/formal-v14/成绩_加权总分.md` | 七领域合计与场均 |
| `results/formal-v14/成绩.json` | 机器可读的 BT 结果 |
| `results/formal-v14/回放/match-NNN.json` | 原始 hello + 逐局终局 + 每 5 tick 帧 + 复核结果 |
| `bots/封存记录.json` | 选手源码与共用库的 SHA-256、编译产物路径 |

---

## 已知边界

1. **两代引擎并存**。第 1 季成绩由 JS oracle 引擎产出，复现需 Node 运行时；第 2 季起 `engine.kind=native`（Go 原生，`internal/sim`），oracle 仅作 v1.0 历史复现与对照。
2. **hello 约 110 KB**。v1.3 契约单行上限 128 KiB，hello 实测约 110 KB，在契约内。
3. **视野关闭**。`rules.vision=false`，全图可见；开雾需要另跑一个赛季，不能与本赛季混排。
4. **参考 Bot 只用于基准**。它们只有流场寻路和简单交战规则，不代表规则上限；第 1–5 季无人斩舰，属选手强度不足，不是规则问题。v1.3 起「斩舰」在数值上可达（`internal/sim/victory_test.go` 用 3 台 RAILGUN 在 25 格距离验证），但参考 Bot 的推进仍会在基地口被地形卡住。
5. **回放不入库**。`results/*/回放/` 可由 `arena run` 重建；`bots/bin/` 同理。
6. **攻守模式仍是实验**。规则、协议、标定都齐了，但缺选手包、许可、回放查看器与沙箱，不能直接当正式赛季。
7. **攻守的 BT 样本偏少**。每对 16 局时头名区间仍有 ±117 分；出正式榜单建议每对 ≥40 局（4 人池约 480 局）。
8. **母舰削弱在实战中不可感知**。v1.4 把母舰 HP/攻击/移速/坡限全部下调，但第 5 季 90 局与第 4 季逐局完全相同——参考 Bot 既不把兵送进敌方母舰射程，也不指挥自家母舰移动，这四项在当前打法下是「无人触碰的数值」。削弱本身有门禁保证（`mobility_test.go`：母舰 400 拍内必须能驶出基地 60 px），但要有体感得先修 Bot 的攻坚执行。
9. **参考 Bot 的攻坚能力仍然很弱**。v1.3 修掉了「出生即被困」（出兵点不校验可通行）与「建造轮转饿死高价兵」（SALVAGER 无限下单、付不起就跳项）两个缺陷，Sapper 也能稳定造出 RAILGUN，但它的推进仍可能被基地周边地形卡住。斩舰要成为赛季常态，还需要选手侧的真实攻坚打法。
10. **沙箱只覆盖进程层，且只支持 Windows**。Windows Job Object 挡住子进程派生、内存爆炸与孤儿进程，工作目录是一次性临时目录；**文件读权限与出网没有隔离**，需要独立低权账户或容器。非 Windows 平台没有等价沙箱，裁判会**拒绝开赛并报错**，而不是无沙箱裸跑或伪造平局记录。手册 §3.2 已写明这一点。
11. **规则指纹=规则身份**。改动任何影响对局语义的代码都必须递增 `sim.RulesVersion`，否则 `rulesHash` 不会变，历史成绩就失去可区分性。指纹本身补齐遗漏常量后，`rulesHash` 也会变化，旧成绩需要用旧版本引擎复核。

## 禁止事项

- 不修改 `engine/oracle/` 快照；发现疑似缺陷先登记到 `../Dev/Plan_Beta.md` §11。
- 不在裁判进程内跑选手代码；进程隔离是硬约束。
- 不把 `赛程.json` 当草稿改；要改就重新 `plan` 并注明原因。
- 不在未更新 `赛事手册.md` 的情况下改判罚或计分口径。

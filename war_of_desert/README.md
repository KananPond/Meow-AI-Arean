# Desert Arena · 正式版

> **状态：正式版 v1.0 已落地并跑完第 1 季基准赛**（2026-11-12）
> **赛事手册**：[`赛事手册.md`](赛事手册.md) ｜ **规则基线**：[`../Dev/Plan_Beta.md`](../Dev/Plan_Beta.md) v1.0
> **引擎**：冻结 oracle `MVP@288db07f` 的只读快照，由 `engine/oracle_host.js` 在独立进程承载
> **执行链**：Go 1.27 标准库，零第三方依赖

---

## 这个仓库是什么

`war_of_desert/` 就是**正式比赛的执行目录**：能开赛、能判罚、能出成绩，不再只是空骨架。

- 选手交一个 Go 程序，官方编译、封存源码哈希；
- 裁判为每个座位拉起独立进程，执行 120 ms 决策死线、连续 3 次红牌、超时 kill 与重启、换边重开；
- 赛程先冻结再执行，赛后产出逐局结果、完整回放、确定性复核、BT 实力值与八领域加权总分；
- 规则语义不另写一份，全部来自冻结 oracle 快照。

第 1 季基准赛结果：5 名选手、20 场、40 局、零故障、40/40 确定性复核通过。

```text
BT 名次：Nomad → Vanguard（锚）→ Swarm → Bulwark → Starter
加权总分：Nomad → Swarm → Starter → Vanguard → Bulwark
```

两榜不一致是设计目标：胜负与质量是两条独立信号。详见 [`results/formal/成绩.md`](results/formal/成绩.md)。

**开发与历史资料在 [`../Dev/`](../Dev/README.md)**：冻结 MVP 预言机、原作调研、Plan/ORACLE、历次审查报告、第三方参照模板。它们不参与正式赛执行。

---

## 快速开始

组织方菜单（等价于下面的命令）：

```bash
python -X utf8 start.py          # 交互菜单：build / plan / run / rate / check / selftest / season / status
python -X utf8 start.py season   # 整季流水线：build → plan → run → rate
```

在 **`war_of_desert/`** 执行：

```bash
go run ./cmd/arena selftest    # 验证超时 kill / 崩溃重启 / 非法结构红牌
go run ./cmd/arena build       # 编译 5 名选手 + 封存源码哈希
go run ./cmd/arena plan        # 冻结赛程 → results/formal/赛程.json
go run ./cmd/arena run         # 跑完 20 场 / 40 局 → 结果.jsonl + 回放
go run ./cmd/arena rate        # BT + 加权总分 → 成绩.md / 成绩_加权总分.md / 成绩.json
```

单项自检（选手包默认入口）：

```bash
go run ./cmd/arena check -seed 20261109
```

---

## 攻守对战（实验）

在 v1.0 之外新增的独立变体：一方守擂、一方进攻，守方有工事，攻方没有；VP 改为矿场，胜负只看母舰是否被摧毁。规则、协议与实验结果见 [`赛事手册_攻守.md`](赛事手册_攻守.md) 和 [`results/siege/结论.md`](results/siege/结论.md)。

```bash
go build -o bots/bin/attacker.exe ./workspace/siege/attacker
go build -o bots/bin/defender.exe ./workspace/siege/defender
go run ./cmd/siege -smoke -hp 1250 -seed 20261201
go run ./cmd/siege -tickLimit 1200 -fortBudget 24 -hps 1250 -seeds 20261201-20261240 -workers 4 -out results/siege/hp1250_clean.csv
```

冻结的 `engine/oracle/rules.js` 未改动；攻守规则在 `engine/oracle/rules_siege.js`，旧赛季仍按 `rulesHash=906f1b14` 复现。

---

## 目录

```text
社区比赛__WoD/
├── war_of_desert/            ← 正式比赛（本文件所在目录）
│   ├── README.md            本文件
│   ├── 赛事手册.md           正式赛制、流程、判罚、申诉、运营清单
│   ├── LEADERBOARD.md       第 1 季榜单与读法
│   ├── go.mod
│   ├── cmd/
│   │   ├── arena/           build / plan / run / rate / check / selftest
│   │   └── faultbot/        自检用故障注入 Bot
│   ├── internal/
│   │   ├── protocol/        hello / tick / actions / end 的线上契约与地图解码
│   │   ├── engine/          引擎宿主客户端（行分隔 JSON）
│   │   ├── judge/           进程隔离、死线、红牌、重启、回放、确定性复核
│   │   ├── schedule/        赛程冻结（换边、seed 递增）
│   │   ├── rating/          Bradley–Terry + bootstrap
│   │   └── contest/         配置加载与路径解析
│   ├── engine/
│   │   ├── oracle_host.js   宿主：命令流 → 冻结 oracle
│   │   └── oracle/          只读快照 + MANIFEST.json
│   ├── tournament/formal.json
│   ├── workspace/           选手包（规则、模板、工具库、参考 Bot、计时器）
│   ├── bots/                封存记录与编译产物（bin 不入库）
│   ├── results/formal/      赛程、结果、成绩、回放
│   ├── replays/             历史赛季归档（预留）
│   ├── notes/               赛前狠话、赛后复盘（预留）
│   ├── start.py             组织方菜单
│   ├── 开赛.py               组织方计时、暂停、强制收卷
│   └── workspace/clock.py   选手侧计时展示
└── Dev/                     开发/历史资料区（见 ../Dev/README.md）
```

`../Dev/` 内部：

```text
Dev/
├── README.md                 资料区索引
├── MVP/                      冻结 JS 规则预言机（只读，含 正式比赛测试/）
├── Docs/                     原作调研、事实卡片、版权与术语
├── Plan_Beta.md              当前规则与迁移规格
├── Plan_Alpha.md             历史构想（归档）
├── MVP_plan.md               历史 MVP 计划（废止）
├── ORACLE.md                 oracle 运行与复现规范
├── MVP_*报告*.md             历次审查/修复/评估证据
└── 模板/                     第三方只读参照
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

- 规则、协议字段、计分口径遵循 [`../Dev/Plan_Beta.md`](../Dev/Plan_Beta.md) v1.0；
- 引擎迁移门禁（§6.2）**尚未宣称通过**：本季仍是 oracle 引擎；
- `hello` 113 KB 超 64 KiB 是 §11 已登记偏差，本季沿用；
- 赛制层（BT、赛程、收卷、榜单）已落地，属于 §1.2 所说“不阻断引擎迁移”的部分。

---

## 产物契约

| 文件 | 内容 |
|---|---|
| `results/formal/赛程.json` | 冻结的选手、场次、seed、规则与判罚口径 |
| `results/formal/结果.jsonl` | 每局一行：场次、座位、胜者、层级、VP、八领域总分、故障、确定性、规则指纹 |
| `results/formal/成绩.md` | BT 实力值、95% 区间、胜平负、总分 |
| `results/formal/成绩_加权总分.md` | 八领域合计与场均 |
| `results/formal/成绩.json` | 机器可读的 BT 结果 |
| `results/formal/回放/match-NNN.json` | 原始 hello + 逐局终局 + 每 5 tick 帧 + 复核结果 |
| `bots/封存记录.json` | 选手源码与共用库的 SHA-256、编译产物路径 |

---

## 已知边界

1. **引擎仍是 JS oracle**。Go 引擎尚未实现，`../Dev/Plan_Beta.md` §6.2 门禁未过；正式成绩依赖 Node 运行时。
2. **hello 113 KB**。协议契约仍是 64 KiB，本季按已登记偏差执行。
3. **视野关闭**。`rules.vision=false`，全图可见；开雾需要另跑一个赛季，不能与本赛季混排。
4. **参考 Bot 只用于基准**。它们只有流场寻路和简单交战规则，不代表规则上限；第 1 季无人斩舰，属选手强度不足，不是规则问题。
5. **回放不入库**。`results/*/回放/` 可由 `arena run` 重建；`bots/bin/` 同理。

---

## 禁止事项

- 不修改 `engine/oracle/` 快照；发现疑似缺陷先登记到 `../Dev/Plan_Beta.md` §11。
- 不在裁判进程内跑选手代码；进程隔离是硬约束。
- 不把 `赛程.json` 当草稿改；要改就重新 `plan` 并注明原因。
- 不在未更新 `赛事手册.md` 的情况下改判罚或计分口径。

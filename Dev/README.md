# Dev · 正式版开发区

> **正式版 = `Dev/MVP`**：`MVP/referee/` 是规则实现，`MVP/正式比赛测试/` 是自包含测试环境。
> `war_of_desert/` 是围绕旧快照 `MVP@288db07` 的 Go 移植**半成品**，不是规则事实源。
> 规则以 [`Plan_Beta.md`](Plan_Beta.md) 为准；历史锚点 `baseline/288db07` 只作对照。

---

## 目录

```text
Dev/
├── README.md                 本文件
├── plan.md                   改造方案（v0.11 → v1.0-candidate，逐项到文件/行号/验收）
├── MVP/                      正式版规则实现（独立 git 仓库）
│   ├── referee/              rules / referee / match / tournament / 两个 Bot
│   ├── test/                 10 个测试套件
│   ├── scripts/              coverage + sync_formal_test
│   └── 正式比赛测试/         自包含 workspace、manifest、收卷、全流程报告
├── Plan_Beta.md              v1.0/v1.1 规则规格（历史基线；当前事实源 = war_of_desert/internal/sim，v1.2）
├── ORACLE.md                 快照校验、复现、覆盖率与 golden trace 规范
├── Docs/                     原作调研、事实卡片、术语、来源（CC BY 4.0）
│   └── archive/              历史计划与审查（只读）
├── 实验/
│   ├── Plan_Siege.md         攻守不对称实验（不随正式版投稿）
│   └── Plan_V15.md           v1.5 规则实验：遗产／插旗／胜利分／CEP／等级（非正式赛季）
├── 审查/
│   ├── WoD_设计审查_20261113.md           v1.1 现行审查
│   └── 设计审查_正式版v1.0_20261113.md    v1.0 历史审查
├── 归档/
│   └── README.md             历史资料索引
└── reference/
    └── 模板/                 第三方只读参照（本地克隆，不入库）
```

## 现行文档

| 文档 | 作用 | 状态 |
|---|---|---|
| [`plan.md`](plan.md) | **改造方案**：12 条 DoD、A/B/C/D/E/F 六阶段工作项、文件级清单、测试计划、排期与风险 | **执行中** |
| [`MVP/README.md`](MVP/README.md) | 正式版规则、数值、设计取舍、v0.11 修复记录 | 现行 |
| [`MVP/正式比赛测试/README.md`](MVP/正式比赛测试/README.md) | 自包含 workspace、manifest、收卷与本地 runner | 现行 |
| [`Plan_Beta.md`](Plan_Beta.md) | v1.0/v1.1 规则基线：时序、协议、架构边界、验收门禁 | 历史（Go 链现为 v1.4 正式赛 + v1.5 实验，事实源 `war_of_desert/internal/sim`） |
| [`ORACLE.md`](ORACLE.md) | 快照校验、复现、覆盖率、golden trace | 现行 |
| [`审查/MVP_测试报告_20261113.md`](审查/MVP_测试报告_20261113.md) | v0.11 修复后全面测试：10 套件、覆盖率、100 局基准、遗留问题 | **现行** |
| [`审查/WoD_设计审查_20261113.md`](审查/WoD_设计审查_20261113.md) | v0.10 设计审查：评估协议、计分卡、机制覆盖、规范对照 | 历史（v0.11 已修部分代码问题） |
| [`实验/Plan_Siege.md`](实验/Plan_Siege.md) | 攻守改造与标定记录 | 实验，不随投稿 |
| [`实验/Plan_V15.md`](实验/Plan_V15.md) | v1.5 规则实验：胜利分三本账／遗产／插旗／CEP／等级，含 90 局标定（`war_of_desert` Go 链，指纹 `24d4935b`） | 实验，**非正式赛季** |
| [`审查/设计审查_正式版v1.0_20261113.md`](审查/设计审查_正式版v1.0_20261113.md) | v1.0 审查：单位口径、计分卡、名次统计力 | 历史 |
| [`Docs/`](Docs/) | 原作调研、事实卡片、术语、来源 | 参考 |
| [`归档/README.md`](归档/README.md) | 历史计划/审查索引 | 只读 |
| [`reference/模板/`](reference/模板/) | 第三方 RTS 参照（MicroRTS/OpenRA），只读 | 参考 |

## 正式版入口

```bash
# 规则测试（10 套件）
cd MVP && npm test

# 覆盖率门槛
cd MVP && npm run coverage

# 同步正式测试包 + 刷新 manifest
cd MVP && npm run sync:formal

# 本地正式测试（默认 1200 tick / 换边两局）
cd MVP/正式比赛测试/workspace && node arena.js full
```

`arena.js` 启动时校验 `workspace_manifest.json`；`--frozen` 使用 `交卷/bot.js`（必须 `accepted=true` 且 SHA-256 一致）。改完 `referee/` 后必须跑一次 `npm run sync:formal`，否则 `npm test` 测新引擎、`arena.js` 跑旧快照。

## 事实源与纪律

1. **规则**：改 `MVP/referee/` → `npm test` → `npm run sync:formal` → manifest 与 `rulesHash` 更新。
2. **历史快照**：`war_of_desert/engine/oracle/` 是 v1.0 只读快照，不回灌；`verify_oracle.py` 只证明历史字节未变。
3. **实验分支不随正式版投稿**：攻守实验见 `实验/Plan_Siege.md` 与 `war_of_desert/engine/oracle/rules_siege.js`；
   v1.5 规则实验见 `实验/Plan_V15.md` 与 `war_of_desert/internal/sim/rules15.go`，由
   `GameCfg.Ruleset == "v15"` 开关隔离，`war_of_desert/tournament/experiment_v15.json` 驱动，
   **不进入正式赛季、不改 v1.4 语义与指纹**；正式赛事实源仍是 `war_of_desert/internal/sim`（v1.4）。
4. **作品/报告**：正式审查放 `审查/`；历史计划放 `Docs/archive/`；第三方参照放 `reference/`。

## 当前已知缺口（详见审查报告）

- `hello` 在 256² 下约 110 KB；512² 前必须先做压缩/分块（当前 `MAX_LINE=128KB`）。
- 100 局里 0 斩舰；Tactical 92:8 Greedy；Greedy 116 杀仍只赢 8 局——VP 单轴、击杀不转化为胜利。
- 排名仍是积分榜，没有 BT/CI/并列；正式赛需要补统计模型。
- 八领域约 49.2 分是常数/准常数；VP 与总分相关 0.647；需重写计分卡。
- 没有模型比赛条件、许可、沙箱、归档 HTML 回放、一键挑战。
- `war_of_desert` 的 Go 执行链仍是半成品，单位口径/选手包/冻结链问题见审查附录 C。

## 核对

```bash
# 历史快照字节
python -X utf8 war_of_desert/scripts/verify_oracle.py

# v1.0 锚点
git -C Dev/MVP rev-parse baseline/288db07^{commit}

# 正式版测试
npm --prefix Dev/MVP test
```

## 赛事计时与收卷

- 正式测试：`python Dev/MVP/正式比赛测试/开赛.py`（90 分钟目标 / 120 分钟强制收卷）。
- 历史 Go 链：`python war_of_desert/开赛.py`（仅作移植参考，不再作为正式版入口）。

# 社区比赛__WoD

Desert Arena 1v1 算法对抗赛：红蓝双方在程序生成的沙漠地形上作战，Bot 以独立进程提交。
仓库里有两套规则，**互不影响**：

| 规则 | 形态 | 当前参数 | 状态 |
|---|---|---|---|
| **v1.0 正式赛** | 对称 VP 对局，八领域加权总分 + Bradley–Terry 双榜 | 600 tick ｜ 120 ms 决策 | 已跑完第 1 季基准赛 |
| **攻守实验** | 一方守擂、一方进攻，守方布工事，VP 改矿场，胜负只看母舰 | HP 1250 ｜ 工事 16 ｜ 1260 tick ｜ 攻方胜率 45% | 规则与标定完成，未转正式赛季 |

攻守是冻结规则之上的**独立变体**（`engine/oracle/rules_siege.js`），`rules.js` 一字未改，
旧赛季仍按 `rulesHash=906f1b14` 复现。

## 两个目录

| 目录 | 内容 |
|---|---|
| [`war_of_desert/`](war_of_desert/README.md) | **可执行部分**：赛事手册（正式 / 攻守）、Go 执行链、选手包、参考 Bot、赛程、成绩与标定数据 |
| [`Dev/`](Dev/README.md) | **开发与历史资料**：冻结 MVP 预言机、原作调研、规格与计划、设计审查、归档文档 |

## 入口

```bash
cd war_of_desert
python -X utf8 start.py          # 组织方菜单
go run ./cmd/arena check -seed 20261109    # v1.0 单局自检
go run ./cmd/siege -smoke -seed 20261201   # 攻守单局
```

## 文档地图

| 想知道什么 | 看哪里 |
|---|---|
| v1.0 怎么组织一场比赛、怎么判罚 | [`war_of_desert/赛事手册.md`](war_of_desert/赛事手册.md) |
| 攻守的规则、协议、参数与标定 | [`war_of_desert/赛事手册_攻守.md`](war_of_desert/赛事手册_攻守.md) |
| 攻守的完整标定过程与结论 | [`war_of_desert/results/siege/结论.md`](war_of_desert/results/siege/结论.md) |
| 第 1 季成绩 | [`war_of_desert/results/formal/成绩.md`](war_of_desert/results/formal/成绩.md) |
| v1.0 的规则基线、协议、架构边界 | [`Dev/Plan_Beta.md`](Dev/Plan_Beta.md) |
| 攻守的改造方案与全部实验记录 | [`Dev/Plan_Siege.md`](Dev/Plan_Siege.md) |
| 最近一次设计审查（单位口径、计分卡、名次） | [`Dev/设计审查_正式版v1.0_20261113.md`](Dev/设计审查_正式版v1.0_20261113.md) |
| 被取代的历史计划与旧审查报告 | [`Dev/Docs/archive/`](Dev/Docs/archive/README.md) |
| 原作调研与事实卡片 | [`Dev/Docs/`](Dev/Docs/README.md) |

## 现状与边界

- v1.0 引擎仍是**冻结 oracle**（Node 承载），Go 引擎未过 `Plan_Beta.md` §6.2 门禁。
- `hello` 113 KB 超出 64 KiB 契约，属已登记偏差。
- 攻守模式**尚未产品化**：缺选手包、许可、回放查看器与沙箱。
- 攻守的参考 Bot 有已知组合缺陷（集结门槛固定、廉价单位占满人口），修好后参数需重新标定。

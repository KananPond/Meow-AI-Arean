# 社区比赛__WoD

Desert Arena 1v1 算法对抗赛：红蓝双方在程序生成的沙漠地形上作战，Bot 以独立进程提交。

现在有两套规则：
- **v1.0 正式赛**：对称 VP 对局，八领域加权总分 + Bradley–Terry 双榜。
- **攻守实验**：一方守擂、一方进攻，守方布置工事，VP 改为矿场，胜负只看母舰是否被摧毁；用于母舰 HP 平衡标定。

## 两个目录

| 目录 | 内容 |
|---|---|
| [`war_of_desert/`](war_of_desert/README.md) | **正式比赛 + 攻守实验**：赛事手册、Go 执行链、选手包、参考 Bot、赛程与成绩；攻守规则见 [`赛事手册_攻守.md`](war_of_desert/赛事手册_攻守.md) |
| [`Dev/`](Dev/README.md) | **开发与历史资料**：冻结 MVP 预言机、原作调研、Plan/ORACLE、历次审查报告、第三方参照 |

## 入口

```bash
cd war_of_desert
python -X utf8 start.py          # 组织方菜单
go run ./cmd/arena check -seed 20261109
```

赛季结果见 [`war_of_desert/results/formal/成绩.md`](war_of_desert/results/formal/成绩.md)。

攻守实验（矿场 / 工事 / 母舰 HP）见 [`war_of_desert/赛事手册_攻守.md`](war_of_desert/赛事手册_攻守.md) 与 [`results/siege/结论.md`](war_of_desert/results/siege/结论.md)。

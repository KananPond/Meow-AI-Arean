# 社区比赛__WoD

Desert Arena 1v1 算法对抗赛：红蓝双方在程序生成的沙漠地形上争夺 VP、保护母舰，Bot 以独立进程提交。

## 两个目录

| 目录 | 内容 |
|---|---|
| [`war_of_desert/`](war_of_desert/README.md) | **正式比赛**：赛事手册、Go 执行链、选手包、参考 Bot、赛程与成绩 |
| [`Dev/`](Dev/README.md) | **开发与历史资料**：冻结 MVP 预言机、原作调研、Plan/ORACLE、历次审查报告、第三方参照 |

## 入口

```bash
cd war_of_desert
python -X utf8 start.py          # 组织方菜单
go run ./cmd/arena check -seed 20261109
```

赛季结果见 [`war_of_desert/results/formal/成绩.md`](war_of_desert/results/formal/成绩.md)。

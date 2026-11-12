# results · 赛事产物

> 结果目录按赛季分：`results/formal/` 是当前正式赛，历史赛季另建目录并保留原始文件。

## 文件

| 文件 | 生产者 | 说明 |
|---|---|---|
| `赛程.json` | `arena plan` | 冻结的选手、场次、seed、规则与判罚口径；`run` 只认它 |
| `结果.jsonl` | `arena run` | 每局一行：场次、座位、胜者、层级、VP、八领域总分、故障、确定性、规则指纹 |
| `成绩.md` | `arena rate` | BT 实力值、95% 区间、胜平负、总分 |
| `成绩_加权总分.md` | `arena rate` | 八领域合计与场均 |
| `成绩.json` | `arena rate` | 机器可读的 BT 结果 |
| `回放/match-NNN.json` | `arena run` | 原始 hello + 每局结果 + 每 5 tick 帧 + 复核 |

## 重建

```bash
cd war_desert
go run ./cmd/arena plan
go run ./cmd/arena run      # 默认覆盖结果；-resume 可续跑
go run ./cmd/arena rate
```

`回放/` 体积较大且完全可由 `run` 重建，默认不入库；需要归档时整目录打包。

## 审计

1. `结果.jsonl` 行数 = `赛程.json` 场次数 × 2；
2. 每行 `verifyOk=true`、`rulesHash` 一致；
3. `成绩.md` 的局数 = 结果行数；
4. 任何缺场、重跑、作废都要在 `workspace/CONDITIONS.md` 里留记录。

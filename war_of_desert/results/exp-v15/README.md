# exp-v15 · v1.5 实验基线

> 本地实验产物，不是正式赛季。规则指纹 `24d4935b`，1500 tick，6 Bot × 3 seed，
> 45 场 / 90 局。规则与参数说明见 `Dev/实验/Plan_V15.md`。

## 文件

| 文件 | 说明 |
|---|---|
| `赛程.json` | `arena plan` 冻结的 45 场对阵 |
| `结果.jsonl` | 每局一行：胜者、层级、VS、七领域总分、故障、复核 |
| `回放/` | 90 局回放，每 5 tick 一帧（含 `relics` / `flags` / `vs` / 单位等级） |
| `封存记录.json` | 本季源码/共用库 SHA-256（`arena build` 生成） |

## 关键结果

- 阈值胜 59（66%）、时间到 28（31%）、斩舰 3（3%）；平局 11（12%）。
- 场均击杀 1.21、开火 13.0、命中率 57%；平均 1305 tick。
- VS 均值 247 / 219：击杀 22 / 21（9%）、遗产 199 / 173（75%）、插旗 25 / 25（10%）。
- 等级：LAV L2 ×21、L3 ×1、L4 ×2；RAILGUN L3 ×1。
- 母舰终局 HP 最低 8%，3 局被摧毁。

## 复现

```bash
cd war_of_desert
go run ./cmd/arena build -config tournament/experiment_v15.json
go run ./cmd/arena plan  -config tournament/experiment_v15.json
go run ./cmd/arena run   -config tournament/experiment_v15.json
py -3 -X utf8 scripts/exp_metrics.py results/exp-v15
```

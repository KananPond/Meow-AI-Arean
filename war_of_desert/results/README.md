# results · 赛事产物

> 按赛季/规则分目录，互不通用：
> - `formal-v14/`：**当前赛季 · v1.4 正式赛**（对称 VP、七领域计分、BT 双榜），由 `arena` 生成
> - `formal-v13/`：第 4 季 v1.3 正式赛（母舰削弱前）
> - `formal-v12/`：第 3 季 v1.2 的冻结赛程（仅 `赛程.json`，未执行；公平性缺陷修复后作废）
> - [`formal-v11/`](formal-v11/)：历史 · 第 2 季 v1.1（七领域计分）
> - [`formal/`](formal/)：历史 · 第 1 季 v1.0（对称 VP、八领域计分、BT 双榜）
> - [`siege/`](siege/)：**攻守实验**（矿场、工事、母舰 HP 标定、攻守双参数 BT），索引见 [`siege/README.md`](siege/README.md)
>
> 历史赛季目录保留原始文件，不随当前版本改动。

## 赛季目录文件（以 formal-v12/ 为例）

| 文件 | 生产者 | 说明 |
|---|---|---|
| `赛程.json` | `arena plan` | 冻结的选手、场次、seed、规则与判罚口径；`run` 只认它 |
| `结果.jsonl` | `arena run` | 每局一行：场次、座位、胜者、层级、VP、七领域总分、故障、确定性、规则指纹 |
| `成绩.md` | `arena rate` | BT 实力值、95% 区间、胜平负、总分 |
| `成绩_加权总分.md` | `arena rate` | 七领域合计与场均 |
| `成绩.json` | `arena rate` | 机器可读的 BT 结果 |
| `回放/match-NNN.json` | `arena run` | 原始 hello + 每局结果 + 每 5 tick 帧 + 复核 |

（v1.0 的 `formal/` 为八领域计分，其余字段口径相同。）

## 重建（当前赛季）

```bash
cd war_of_desert
CFG=tournament/formal_v12.json
go run ./cmd/arena plan -config $CFG
go run ./cmd/arena run  -config $CFG   # 默认覆盖结果；-resume 可续跑
go run ./cmd/arena rate -config $CFG
```

`回放/` 体积较大且完全可由 `run` 重建，默认不入库；需要归档时整目录打包。

## 审计

1. `结果.jsonl` 行数 = `赛程.json` 场次数 × 2；
2. 每行 `verifyOk=true`、`rulesHash` 一致；
3. `成绩.md` 的局数 = 结果行数；
4. 任何缺场、重跑、作废都要在 `workspace/CONDITIONS.md` 里留记录。

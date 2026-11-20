# bots · 封存与编译产物

> 本目录只放“已经收卷的东西”。选手在编码期改的是 `workspace/submission/`，不是这里。

## 目录

```text
bots/
├── bin/                 编译产物（.exe），不入库，可随时重建
├── 封存记录.json         源码 SHA-256、共用库哈希、编译产物路径、生成时间（生成物，不入库）
└── README.md            本文件
```

`封存记录.json` 每次 `arena build` 都会重写：源码哈希稳定，`binSha256` 随编译器/
环境变化。所以它不入库，只为「当前这台机器上跑的就是这份源码」服务。
**赛季正式跑过的那一份**按赛季归档到 `results/<赛季>/封存记录.json` 并入库，
那是赛后审计的依据——详见 [`../results/README.md`](../results/README.md#审计)。

## 封存记录口径

`arena build` 会：

1. 对每个 entry 执行 `go build -o bots/bin/<name>.exe ./<source>`；
2. 递归计算 `source` 下所有 `.go` 文件的 SHA-256；
3. 计算共用库 `workspace/botkit`、`workspace/refbot` 的 SHA-256；
4. 写入 `封存记录.json`。

只有“源码哈希 + 共用库哈希 + 编译产物”三者一致，才认为这名选手的参赛版本没有被替换。

## 重建

```bash
cd war_of_desert
go run ./cmd/arena build
```

编译失败、源码缺失、哈希不符，都按“未提交”处理，不进入赛程。

## 已封存选手（第 1 季）

| 选手 | 来源 | 说明 |
|---|---|---|
| Vanguard | `workspace/bots/vanguard` | 正面推进，BT 锚 |
| Bulwark | `workspace/bots/bulwark` | 经济防守 |
| Swarm | `workspace/bots/swarm` | 数量压制 |
| Nomad | `workspace/bots/nomad` | 机动抢点 |
| Starter | `workspace/submission` | 选手提交模板 |

共用库：`workspace/botkit`（协议与寻路）、`workspace/refbot`（参考 Bot 策略）。

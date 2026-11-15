# 选手规则与协议 v1.1（Go 原生引擎）

> 你写一个独立程序，裁判用 stdin/stdout 和你对话。本文是完整契约；`botkit/` 是可选工具库。
> v1.1 规则变更：四子系统分配（armour/repair/attack/range）、CU/RU 双资源运输线经济、
> 生产线由工业科技驱动、单局 1200 tick、七领域确定性计分（移除墙钟「算力表现」）。

---

## 1. 你要交什么

`submission/decide.go`，`package main`，编译成单个可执行文件。官方编译命令：

```bash
go build -o bots/bin/submission.exe ./workspace/submission
```

正式赛收卷时只冻结这个文件；编译由主办方统一执行。

---

## 2. 消息流

```text
裁判 → 你   {"type":"hello", ...}          开局一次
你  → 裁判  {"ready":true}                  初始化完成
裁判 → 你   {"type":"tick", ...}            每 tick 一次
你  → 裁判  {"actions":[...]}               每 tick 一次
裁判 → 你   {"type":"end","result":{...}}   终局一次，随后退出
```

- 所有消息是 **UTF-8、单行 JSON**，行尾 `\n`。
- 每行上限 128 KiB（`hello` 实测约 110 KB）。
- 收到 `end` 后请自行退出；不退出会被裁判 kill，不影响成绩。

---

## 3. hello

字段（完整定义见 `internal/protocol.Hello`）：

| 字段 | 含义 |
|---|---|
| `protoVersion` | 协议版本，必须为 `5` |
| `rulesHash` | 规则指纹，写进成绩与回放 |
| `seed` / `mapAttempt` | 请求 seed 与地图重试次数，两者唯一确定一张图 |
| `generator` | `minecraft` 或 `geomorphic` |
| `w` / `h` / `cell` / `world` | 256 / 256 / 6 / 1536（像素；1 格 = 6 px，物理 10 m） |
| `map` | 三层地图编码，见 §4 |
| `spawns` | 双方出生点（格坐标） |
| `vp` / `res` | VP 与资源点世界坐标 |
| `rules` | 单位表、科技表、经济、时序、弹道常数、视野、模式 |

`rules.types` 里的 `speed` / `range` / `vision` 单位是**世界格**（已乘 `scale`，但没乘 `cell`）；像素距离 = 格数 × `cell`（6）。不要把 `range=12.8` 直接当像素用。

---

## 4. 地图解码

`map` 形如：

```json
{"w":256,"h":256,"enc":"rle+b64/1","base":"...","surf":"...","elev":"..."}
```

- `base` / `surf`：base64 解码后按 `(值, 次数-1)` 成对还原，每对展开 `次数` 个格；
- `elev`：base64 解码后就是 256×256 个字节；
- 行优先，索引 `y*256 + x`。

`base` 取值：`0` 平地、`1` 沙地、`2` 岩地、`3` 山脉（**绝对不可通行**）、`4` 平台。

不想自己写解码就用 `botkit.Load()`，它已经处理好。

---

## 5. tick

关键字段：

| 字段 | 含义 |
|---|---|
| `turn` / `tickLimit` | 当前 tick 与总 tick（1200） |
| `side` | 你是 0（红）还是 1（蓝） |
| `timing` | `tickMs=50`、`decideMs=120`、`applyAt`、`orderLatency=1` |
| `units` | 你方全部单位 + 你视野内的敌方单位。敌方单位不带 `mode`；己方采集车带 `harvest`/`hstate`/`carry` |
| `vp` | `{id,x,y,owner,contested}`；`owner=-1` 表示无主 |
| `res` | 资源点 `{id,x,y,kind,amount}`；`kind` 是 `cu`（4 个里 3 个）或 `ru`（1 个） |
| `score` | 双方 `{vp,kills,losses}` |
| `me` | 你的 CU、人口、四项分配、槽位、武器、科技、生产队列 |
| `shells` | 在飞的炮弹（能看到来袭） |
| `events` | 最近 12 条事件（视野开启时只给可见事件） |

`units[]` 每项：`{id, side, kind, x, y, hp, maxHp, mode?}`。坐标是世界像素，不是格。

---

## 6. actions

每拍返回：

```json
{"actions":[
  {"id":7,"type":"move","x":1280,"y":900},
  {"id":9,"type":"attack","targetId":12},
  {"type":"alloc","alloc":{"armour":25,"repair":25,"attack":30,"range":20}},
  {"type":"produce","kind":"LAV"},
  {"type":"produce","kind":"CARRIER"},
  {"type":"research","tech":"I1"},
  {"type":"weapon"},
  {"type":"mode","mode":"neutral"},
  {"id":12,"type":"harvest","resId":2}
]}
```

| 动作 | 字段 | 说明 |
|---|---|---|
| `move` | `id,x,y` | 移动到世界坐标；**引擎不替你寻路**，撞山/陡坡会减速或走不动 |
| `attack` | `id,targetId` | 原地指定优先目标；射程/视线/弹道仍由引擎裁决 |
| `idle` | `id` | 原地待命 |
| `harvest` | `id,resId` | **采集车专用**：指派运输线资源点，引擎托管「装载→回程→卸货」；`resId:null` 取消。**只发一次**——每拍重发会把状态机打回 toResource。采 `kind:"cu"` 节点回本方 CU、`kind:"ru"` 节点回本方 RU |
| `alloc` | `alloc` | 母舰四子系统分配（armour/repair/attack/range），整数、和必须 100，每拍最多 1 条 |
| `produce` | `kind` | 入生产队列；生产线数量由工业科技决定（1→I1→I3a/I3b 共 3 条）；`by` 可指定自己的运载舰代造小单位。**队列严格先进先出**：只有队首出队，队首 CU/RU 不足时**整队等待、不丢单、不可取消**——下单前先看 `me.cu`/`me.ru`，高阶单位务必排在 SALVAGER 之后 |
| `research` | `tech` | 开始研究；前置、互斥、槽位（固定 2）、CU 由引擎校验 |
| `weapon` | — | 造一门母舰主炮，独立队列（炮位上限固定 5） |
| `mode` | `mode` | 不带 `id` 改全队与后续新造单位；带 `id` 只改一台 |

规则上限：每台单位每拍最多 1 条单位动作；阵营动作每拍各最多 1 条；动作总数不超过 `己方存活单位数 × 2 + 4`。

**两类非法要分清**：

- 结构性非法（不是 JSON、缺 `actions`、id 不属于你、坐标越界、重复单位动作、动作过多）→ 记 1 次故障；
- 语义性非法（撞山、目标已死、槽位满、钱不够、分配和不为 100）→ 只丢这条动作，不记故障。

---

## 7. 时限与红牌

- 每次 `decide` 从收到 tick 到交出完整一行为 120 ms；
- 开局 `init`（读 hello、解地图、预计算）独立 2000 ms；
- 超时 → kill 进程、记故障、本拍空动作、重启后继续；
- 崩溃 → 同样记故障并重启；
- **连续 3 次故障判负**；成功一次即清零；
- 思考 50–120 ms 不算故障，只记 `late`。

进程会被 kill 后重启，重启时重发 hello。**不要假设进程活到终局**，也不要把关键状态只放在内存里——不过正式赛也不允许你写文件，所以正确做法是：每次 tick 都从 `state` 重建决策。

---

## 8. 胜负与计分

五级判定链：

1. 母舰被毁；
2. 时间到比 VP 累计分；
3. VP 相同比 KD 差；
4. KD 相同比母舰剩余 HP 比例；
5. 全相同判平局。

七领域确定性计分卡（0–100 加权；v1.1 移除了随机器负载漂移的「算力表现」域，
决策耗时只作诊断记录，不进计分——计分完全可逐帧复现）：

| 领域 | 权重 |
|---|---:|
| VP 控制 | 30% |
| 歼敌 | 20% |
| 保存兵力 | 15% |
| 母舰完整度 | 15% |
| KD 差 | 10% |
| 战场控制 | 5% |
| 推进效率 | 5% |

**斩舰只决定这一局谁赢，不额外加分。** 赢得快但打光家底，总分可能反而低。

---

## 9. 自检

```bash
cd war_of_desert
go build -o bots/bin/submission.exe ./workspace/submission
go run ./cmd/arena check -config tournament/formal_v11.json -seed 20261109
```

期望：两局跑满 1200 tick、`故障=0/0`、`确定性复核：true`。`formal_v11.json` 的
`engine.kind` 是 `native`——规则引擎是 Go 原生 `cmd/enginehost`，不再依赖 Node 快照。

想换对手：

```bash
go run ./cmd/arena check -config tournament/formal_v11.json -a bots/bin/submission.exe -b bots/bin/nomad.exe -seed 20261110
```

---

## 10. 常见坑

- 坐标是**世界像素**（0–1536），不是格；`x/6` 才是格。
- `attack` 不会替你靠近目标；射程外请先 `move`。
- `move` 目标是格内任意点，但山脊不可通行；直线撞山会卡住，自己绕。
- `alloc` 和必须正好 100；每次最多改 5 个百分点。生产/研究**不再由分配驱动**：
  生产线 = 1 + 工业科技（I1→2 条、I3a/I3b→3 条，封顶 3），研究槽固定 2，炮位固定 5。
- 高阶单位兼耗 **RU**（AAV 30 / RAILGUN 60 / ARTILLERY 250 / CARRIER 100），
  RU 只来自 `kind:"ru"` 的资源点；没有 RU 就只能造 LAV/SALVAGER/SCOUT。
- **生产队列是死板的 FIFO**：队首付不起（CU 或 RU）时整队冻结，且**没有取消/改序手段**。
  把 AAV 排在 SALVAGER 前面、或让一个永远付不起的单位卡在队首，后面全部陪葬
  （50 轮实测：某参考 Bot 因此 1200 tick 零制造，重单位全场 0 下线）。
  下单前自查 `me.cu`/`me.ru`，付得起才排；`me.queue`/`me.active` 可看自己队列。
- `harvest` 只发一次：每拍重发会把采集车状态机打回 `toResource`，一局都装不满一车。
  第一台采集车先派去 `cu` 节点（基础收入为 0，CU 全靠采集），第二台派 `ru` 解锁高阶单位。
- 母舰是**固定慢速**移动基地，四子系统只影响 减伤/自愈/增伤/增程。
- 母舰被毁立刻结束，别把它当普通坦克用。
- 别在 `decide` 里 sleep；120 ms 是硬死线。

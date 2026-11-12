# 比赛条件 · 第 1 季基准赛

> 对齐 UP 主原版每集 `CONDITIONS.md` 的定位：记录工具、版本、时限、渠道、用时，以及**所有中断、补时和事故**。本季是自动化基准赛，无人工参赛会话，因此“模型 / 工具 / 思考档”记为不适用，但引擎、裁判与评分侧的每一次选择都如实记录。

## 一、环境

| 项 | 值 |
|---|---|
| 运行平台 | Windows 11（Git Bash），离线 |
| 执行链 | Go 1.27.3 标准库；`cmd/arena` + `internal/*` |
| 引擎 | 冻结 oracle 快照 `engine/oracle/`，由 `engine/oracle_host.js` 在独立 Node 进程承载 |
| Node | v22.22.2（仅标准库） |
| 规则来源 | `MVP@288db07f164bc6c680a2d878c47153804d3fdcab` 的 `referee/{rules,mapgen,mapgen_mc,ballistics}.js` |
| 规则指纹 | `906f1b14` |
| 随机性 | 地图由 seed 确定性生成；无 `Math.random()`、无系统时间参与模拟 |
| 并发 | 单进程顺序执行，无并发局，不存在负载抖动 |
| 第三方依赖 | 无（Go 与 Node 两侧都只用标准库） |

## 二、时限与判罚

| 项 | 值 | 来源 |
|---|---|---|
| 单次决策预算 | 120 ms | `Plan_Beta.md` §2 |
| 开局初始化 | 2000 ms（hello → ready） | 同上 |
| 逻辑 tick | 50 ms（20 tick/s） | 同上 |
| 单局时长 | 600 tick | 同上 |
| 单行上限 | 64 KiB（hello 例外） | 同上 |
| 红牌 | 连续 3 次 | `maxStrikes=3`, `strikeMode=consecutive` |
| 故障后处理 | kill → 重启 → 重发 hello，故障计数保留 | `internal/judge` |
| 计时口径 | 墙钟（写 tick → 读完 actions） | 与 oracle 的 `decideMs` 同义 |

自检结果（`arena selftest`）：

```text
timeout  PASS  胜者=座位1 层级=0 故障=3 判定=故障出局：连续失败达到上限（红牌）
crash    PASS  胜者=座位1 层级=0 故障=3 判定=故障出局：连续失败达到上限（红牌）
badjson  PASS  胜者=座位1 层级=0 故障=3 判定=故障出局：连续失败达到上限（红牌）
```

## 三、赛季参数

| 项 | 值 |
|---|---|
| 选手 | Vanguard / Bulwark / Swarm / Nomad / Starter |
| 基准 seed | 20261109、20261110 |
| 实际 seed | 20261109 – 20261129（按场次递增） |
| 赛制 | 5 人循环 × 2 个基准 seed，每场换边两局 |
| 场次 / 局数 | 20 场 / 40 局 |
| 地图 | 256×256，`minecraft`，视野关闭 |
| 每局抽帧 | 5 tick |

## 四、成绩

```text
BT：Nomad 175.3 → Vanguard 0.0（锚） → Swarm -29.6 → Bulwark -59.4 → Starter -74.6
加权总分：Nomad 927.5 → Swarm 926.8 → Starter 884.1 → Vanguard 870.2 → Bulwark 859.8
```

- 40 局全部 `verify.ok=true`；
- 故障总数 0；
- 判定层级分布：L2（VP）35 局、L5（全相同平局）5 局；无斩舰局、无故障出局局；
- 平均单局墙钟 1.2 s（含 600 tick 决策往返与引擎复核）。

## 五、中断、补时与事故记录（完整）

| # | 阶段 | 事件 | 处理 | 是否影响成绩 |
|---:|---|---|---|---|
| 1 | 引擎快照 | 冻结 oracle 的 `hello` 约 113 KB，超出正式契约 64 KiB | 登记为 `Plan_Beta.md` §11 已存在的偏差，本季沿用 RLE+base64；Go 引擎迁移时收敛 | 否（所有选手同一份 hello，信息集一致） |
| 2 | 参考 Bot | 首版 `Vanguard` 的 `Aggression>=1` 分支会让单位无限追击、完全放弃 VP，16 局全败 | 修正交战半径判定，并给参考 Bot 补流场寻路；重跑全季，旧结果不保留 | 是（修正后 Vanguard 7 胜 2 平 7 负，重新成为有效基准） |
| 3 | 统计口径 | BT 以名册第一名 Vanguard 为锚，锚定选手的实力值恒为 0，区间退化为 0~0 | 在 `成绩.md` 中显式标注“基准锚”；不伪造区间 | 否（数学定义，非成绩异常） |
| 4 | 回放体积 | 每场回放含完整 hello 与 240 帧，单文件约 600 KB | 去掉对局内重复 header，全季回放 12 MB；回放可由 `arena run` 重建 | 否 |
| 5 | 对局过程 | 无超时、无崩溃、无红牌、无重跑、无补时 | 40 局自然打满 600 tick | 否 |
| 6 | 评分口径 | 算力表现域按墙钟耗时折算，重跑整季时总分会有 ±0.x 浮动（第 1 季 Nomad 在 927.4–927.5 之间） | 明确该域不参与确定性复核；胜负、状态、事件、VP 完全一致 | 否（四舍五入到 0.1 后几乎不可见） |

## 六、与冻结 MVP 的差异（如实标注）

- 裁判从“同进程 JS 调用”改为“独立进程 + 真实 stdin/stdout 行协议”，新增 `ready` 握手；
- 引擎仍是冻结 oracle，但被封装为独立进程宿主；Go 引擎尚未实现，`Plan_Beta.md` §6.2 门禁未宣称通过；
- 选手提交从 `bot.js` 改为 Go 程序；
- 正式赛执行链为 Go 标准库，赛制层不再依赖 Node（仅引擎宿主需要 Node）；
- 不做赛前狠话与赛后复盘收集，`notes/` 仅保留占位说明。

## 七、复现

```bash
cd war_desert
go run ./cmd/arena selftest
go run ./cmd/arena build
go run ./cmd/arena plan
go run ./cmd/arena run
go run ./cmd/arena rate
```

期望：`SELFTEST_OK`；20 场 / 40 局；`结果.jsonl` 40 行；`成绩.md` 与 `成绩_加权总分.md` 生成。

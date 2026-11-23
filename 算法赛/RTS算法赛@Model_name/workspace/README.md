# 选手工作区

> 当前比赛规则：**v1.9.2**（ruleset `v15`，1500 tick）。`RULES.md` 是协议与经济基线，
> `RULES_V15.md` 是 v1.9.2 的增量，两者冲突时以 `RULES_V15.md` 为准。

> 编码期你只需要动 `submission/<选手名>_bot.go`。其余目录是规则、裁判和陪练。

## 目录

```text
workspace/
├── RULES.md                   完整规则与行协议（先读）
├── RULES_V15.md               v1.9.2 增量规则（当前比赛）
├── prompt.md                  发给模型的原始任务书
├── README.md                  本文件
├── clock.py                   编码期计时器（只读）
├── go.mod                     本地编译用的模块定义（指向 arena/）
├── arena/                     正式赛的引擎 + 裁判 + 比赛程序（源码，只读）
│   ├── cmd/arena              比赛入口：local / check / plan / run / rate / version
│   ├── cmd/enginehost         引擎宿主
│   ├── cmd/sandbox            沙箱自检：用赛场限制跑你的 Bot
│   ├── internal/              规则、裁判、沙箱、评分
│   └── workspace/botkit/      协议库（import "desertarena/workspace/botkit"）
├── bots/
│   ├── skeleton/              骨架 Bot：只演示接口协议，不含任何策略
│   ├── baseline@1.exe         基线一：中等战术（预编译，无源码）
│   └── baseline@2.exe         基线二：极强·多轮调试（预编译，无源码）
├── tournament/local.json      本地赛季配置（两个基线 + 你的 Bot）
├── 本地对局.bat               双击：编译 + 跟两个基线打
├── 本地赛季.bat               双击：编译 + 完整赛季评分
└── submission/
    └── <选手名>_bot.go        你要改的那一个文件
```

`arena/` 里不是模拟器，是**正式赛那条执行链的源码**：同样的规则（`internal/sim`）、
同样的裁判（`internal/judge`：协议校验、超时杀进程、Job Object 沙箱、确定性复核）、
同样的比赛入口（`cmd/arena`）。你本地跑出来的判罚，就是赛场上的判罚。

## 三条命令

都在这个 `workspace/` 目录里执行。

```bash
# 1. 编译（工作区自带 go.mod，不用把代码拷进引擎目录）
go build -o bot.exe ./submission

# 2. 和两个基线对打：2 个 seed × 换边两局
cd arena
go run ./cmd/arena local -bot ..\bot.exe -seeds 2

# 3. 只跑一局，看细节（判定层级、故障、决策耗时、确定性复核）
go run ./cmd/arena check -a ..\bot.exe -b ..\bots\baseline@1.exe -seed 20261109
```

`local` 的输出：

```
── 对手 baseline@1 ──
  seed 20261109 第1局 座位0  VP  761:1089  负  层级2  故障0  决策0.7ms
  seed 20261109 第2局 座位1  VP  761:1089  负  层级2  故障0  决策0.6ms
合计：0 胜 0 平 4 负（4 局）｜平均 VP 761 : 1089｜最大决策 0.7ms｜故障 0 条
回放 → matches\local\回放\
```

回放是逐帧 JSON（单位位置、血量、VP 归属、生产队列），卡在哪一拍、兵死在哪，
直接翻它，比看日志快。

## 完整赛季

和正式赛同一个 `plan → run → rate`：

```bat
本地赛季.bat
```

或者手工：

```bash
cd arena
go run ./cmd/arena plan -config ..\tournament\local.json
go run ./cmd/arena run  -config ..\tournament\local.json -skip-seal
go run ./cmd/arena rate -config ..\tournament\local.json
```

产出在 `matches\season\`：`成绩.md`（Bradley–Terry 排名 + 95% 区间）、
`成绩_加权总分.md`（七领域）、`结果.jsonl`、`回放\`。

`-skip-seal` 是「跳过封存校验」。正式赛开跑前会拿源码哈希跟封存记录比对，对不上
直接中止；本地没有封存记录，所以要跳过。正式赛不带这个参数。

## 沙箱（正式赛会对你的程序做的事）

裁判用 Windows Job Object 给你的进程加硬约束，本地用**同一份实现**验证：

```bash
cd arena
go run ./cmd/sandbox                                # 验证本机沙箱可用
go run ./cmd/sandbox -exe ../bot.exe -turns 200     # 在沙箱里跑你的 Bot 200 拍
```

| 项 | 值 | 说明 |
|---|---|---|
| 活跃进程数 | 1 | 派生子进程会被系统拒绝 |
| 单进程内存 | 512 MiB | 超了分配失败或被 kill |
| Job 内存 | 1024 MiB | 同上，整组上限 |
| 工作目录 | 一次性临时目录 | 相对路径写不到仓库，也读不到别人 |
| 环境变量 | 白名单 | 只有 SystemRoot/TEMP/TMP/PATH 等 |

**没有覆盖的**：文件系统读权限与出网。别指望沙箱能挡住 `os.Open("C:/...")`——
那两条是选手义务，裁判侧靠赛后封存哈希校验兜底。

## 计时

```bash
python -X utf8 clock.py
```

显示已用时间、90 分钟目标、120 分钟强制收卷时刻。最终以组织方的赛务记录为准。

## 提交纪律

- 只改 `submission/<选手名>_bot.go`；
- `arena/`、`RULES.md`、`clock.py`、`bots/` 都是只读的：**改了也不会跟卷走**，
  收卷只带走你那一个文件，本地能过、赛场会编译失败；
- 不要试图写文件、联网、起子进程；
- 收卷时冻结的是 `submission/<选手名>_bot.go`。没有它，这届就没有你。

## 参考 Bot 怎么用

- `bots/skeleton/`：**只有协议，没有算法**。它示范 hello→ready、tick→actions、
  end→退出，以及 `handleTick` 该返回什么形状。你要做的是在里面填策略。
- `bots/baseline@1.exe`、`bots/baseline@2.exe`：两个真实可跑的对手，没有源码。
  拿它们当尺子——先别输得太难看，再去想怎么赢。

骨架不含任何策略是刻意的：它只回答「线上传什么」，不回答「该怎么打」。

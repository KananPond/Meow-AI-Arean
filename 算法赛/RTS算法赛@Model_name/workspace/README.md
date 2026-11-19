# 选手工作区

> 正式赛编码期，你只需要动 `submission/<选手名>_bot.go`。其余目录是规则、工具和陪练。

## 目录

```text
workspace/
├── RULES.md                   完整规则与行协议（先读）
├── prompt.md                  发给模型的原始任务书
├── README.md                  本文件
├── clock.py                   编码期计时器（只读）
├── go.mod                     本地编译用的模块定义（指向引擎）
├── botkit/                    可选工具库：协议、地图解码、一步寻路（只读）
├── sandbox/                   本地沙箱自检（确认赛场限制下你的程序能跑）
├── bots/
│   ├── skeleton/              骨架 Bot：只演示接口协议，不含任何策略
│   ├── baseline@1.exe         基线一：正面推进（预编译，无源码）
│   └── baseline@2.exe         基线二：机动抢点（预编译，无源码）
└── submission/
    └── <选手名>_bot.go        你要改的那一个文件
```

参考 Bot 只有三个。两个基线是**预编译的 exe**，没有源码，但可以随时拉出来对打。

## 三条命令

都在这个 `workspace/` 目录里执行：

```bash
# 1. 编译。工作区自带 go.mod，不用把代码拷进引擎目录
go build -o bot.exe ./submission

# 2. 和基线打一场（换边两局，打印判定 / VP / 总分 / 故障 / 确定性复核）
cd ../../../war_of_desert
go run ./cmd/arena check -a ../算法赛/RTS算法赛@HY-4/workspace/bot.exe \
                         -b ../算法赛/baseline@1.exe -seed 20261109

# 3. 换对手、换 seed
go run ./cmd/arena check -a ../算法赛/RTS算法赛@HY-4/workspace/bot.exe \
                         -b ../算法赛/baseline@2.exe -seed 20261110
```

把上面的 `RTS算法赛@HY-4` 换成你自己的目录名。

## 沙箱（正式赛会对你的程序做的事）

正式赛不是「随手起个进程跑你的 Bot」：裁判用 Windows Job Object 给它加了硬约束。
这些约束在本地也能验证，用的**和赛场是同一份实现**：

```bash
cd ../../../war_of_desert
go run ./workspace/sandbox                                    # 验本机沙箱可用
go run ./workspace/sandbox -exe ../算法赛/RTS算法赛@HY-4/workspace/bot.exe -turns 200
```

| 项 | 值 | 说明 |
|---|---|---|
| 活跃进程数 | 1 | 派生子进程会被系统拒绝 |
| 单进程内存 | 512 MiB | 超了分配失败或被 kill |
| Job 内存 | 1024 MiB | 同上，整组上限 |
| 工作目录 | 一次性临时目录 | 相对路径写不到仓库，也读不到别人 |
| 环境变量 | 白名单 | 只有 SystemRoot/TEMP/TMP/PATH 等 |

**没有覆盖的**：文件系统读权限与出网。别指望沙箱能挡住 `os.Open("C:/...")`——
手册 §3.2 把这两条写成选手义务，裁判侧靠赛后封存哈希校验兜底。

## 计时

```bash
python -X utf8 clock.py
```

显示已用时间、90 分钟目标、120 分钟强制收卷时刻。最终以组织方的赛务记录为准。

## 提交纪律

- 只改 `submission/<选手名>_bot.go`；
- 不要动 `RULES.md`、`clock.py`、`botkit/`、`bots/` 里的官方文件；
- 不要试图写文件、联网、起子进程；
- 收卷时冻结的是 `submission/<选手名>_bot.go`。没有它，这届就没有你。

## 参考 Bot 怎么用

- `bots/skeleton/`：**只有协议，没有算法**。它示范 hello→ready、tick→actions、
  end→退出，以及 `handleTick` 该返回什么形状。你要做的是在里面填策略。
- `bots/baseline@1.exe`、`bots/baseline@2.exe`：两个真实可跑的对手，没有源码。
  拿它们当尺子——先别输得太难看，再去想怎么赢。

骨架不含任何策略是刻意的：它只回答「线上传什么」，不回答「该怎么打」。

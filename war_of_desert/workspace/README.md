# 选手工作区

> 这是**引擎侧**的参考工作区：正式赛的 Starter 选手，加上 `botkit/`、`refbot/` 共用库。
> 发给选手的完整包在 [`../../算法赛/RTS算法赛@Model_name/workspace/`](../../算法赛/RTS算法赛@Model_name/workspace/)，
> 里面带 `arena/`（引擎与裁判源码）、骨架 + 两个预编译基线、`本地对局.bat`。

> 正式赛编码期，你只需要动 `submission/`。其余目录是规则、工具和陪练。

## 目录

```text
workspace/
├── RULES.md               完整规则与行协议（先读）
├── prompt.md              发给模型的原始任务书
├── README.md              本文件
├── clock.py               编码期计时器（只读）
├── submission/decide.go   你的提交物
├── botkit/                可选工具库：协议、地图解码、一步寻路
├── refbot/                参考 Bot 共用策略库（只读）
├── sandbox/               轻量沙箱自检工具（本地验证赛场限制）
└── bots/                  五个参考 Bot
    ├── vanguard/          正面推进
    ├── bulwark/           经济防守
    ├── swarm/             数量压制
    ├── nomad/             机动抢点
    └── sapper/            攻坚拆舰（v1.3 新增）
                          （第 6 名选手 Starter 就是 submission/decide.go）
```

## 三条命令

```bash
# 1. 编译你的 Bot
go build -o bots/bin/submission.exe ./workspace/submission

# 2. 自检：换边两局，打印判定、VP、总分、故障、确定性
go run ./cmd/arena check -seed 20261109

# 3. 换对手
go run ./cmd/arena check -a bots/bin/submission.exe -b bots/bin/nomad.exe -seed 20261110
```

以上命令都在 `war_of_desert/` 目录执行。

## 沙箱（正式赛会对你的程序做的事）

正式赛不是「随手起个进程跑你的 Bot」：裁判用 Windows Job Object 给它加了硬约束。
这些约束在本地也能验证，用的**和赛场是同一份实现**：

```bash
# 1. 先确认本机沙箱可用（沙箱内必须无法派生子进程）
go run ./workspace/sandbox

# 2. 用正式赛的限制把你的 Bot 跑起来：真实 hello + 逐拍 tick
go run ./workspace/sandbox -exe bots/bin/submission.exe -turns 200
```

限制清单：

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

显示已用时间、90 分钟目标、120 分钟强制收卷时刻。它与正式赛务记录同源，但最终以组织方的 `赛务记录.json` 为准。

## 提交纪律

- 只改 `submission/`（以及你自己新增的包）；
- 不要动 `RULES.md`、`clock.py`、`botkit/`、`refbot/`、`bots/` 里的官方文件；
- 不要试图写文件、联网、起子进程；
- 收卷时冻结的是 `submission/decide.go`。没有它，这届就没有你。

## 参考 Bot 的用法

读源码比读文档快：

- `vanguard`：重甲正面推进，看它怎么用 `AAV + 母舰火力`；
- `bulwark`：先经济后装甲，看它怎么排科技顺序；
- `swarm`：轻甲海，看它怎么把产能拉满；
- `nomad`：侦察 + 抢点，看它怎么用 `SCOUT` 换 VP；
- `submission`：基线模板，注释最多，适合从它改。

想快速验证一个想法，直接改 `submission/decide.go` 里的一行常量再跑 `check`，比空想快。

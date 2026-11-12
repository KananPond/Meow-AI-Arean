# 选手工作区

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
└── bots/                  四个参考 Bot
    ├── vanguard/          正面推进
    ├── bulwark/           经济防守
    ├── swarm/             数量压制
    └── nomad/             机动抢点
                          （第 5 名选手 Starter 就是 submission/decide.go）
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

以上命令都在 `war_desert/` 目录下执行。

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

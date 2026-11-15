#!/usr/bin/env bash
# 补提交_v1.1.sh · v1.1 引擎迁移 + 第 2 季赛果 分批补提交脚本
#
# 背景：2026-11-14 会话中 Mimosa L3 提交门禁连续拦截（复核结论见
#       Dev/审查/安全复核_MimosaL3_20261114.md——剩余标记均为 judge/host 进程拉起
#       等架构固有模式与仓库外/第三方代码，工作树功能已全部验证）。
# 用法：待钩子侧放行后，在仓库根（社区比赛__WoD/）执行：bash 补提交_v1.1.sh
set -euo pipefail

# 批次 1 · 引擎迁移
git add .gitignore \
  war_of_desert/internal/sim war_of_desert/cmd/enginehost war_of_desert/cmd/tracegen \
  war_of_desert/engine/golden war_of_desert/engine/parity war_of_desert/engine/oracle_host.js \
  war_of_desert/engine/oracle/selftest_modes.js war_of_desert/tournament/formal_v11.json \
  war_of_desert/cmd/arena/main.go war_of_desert/cmd/siege/main.go war_of_desert/cmd/siegepool/main.go \
  war_of_desert/internal/contest/config.go war_of_desert/internal/engine/host.go \
  war_of_desert/internal/protocol/protocol.go \
  war_of_desert/workspace/CONDITIONS.md war_of_desert/workspace/RULES.md \
  war_of_desert/workspace/botkit/botkit.go war_of_desert/workspace/bots \
  war_of_desert/workspace/refbot/refbot.go war_of_desert/workspace/submission/decide.go \
  war_of_desert/engine/README.md "war_of_desert/赛事手册.md" "war_of_desert/赛事手册_攻守.md"
git commit -m "feat(engine): v1.1 Go 原生引擎落地——internal/sim 移植 MVP 合并版

- internal/sim：规则语义 Go 原生实现（四子系统分配 / CU-RU 运输线经济 / 1200 tick / 七领域计分），
  golden trace 回归 + JS 契约对拍（RNG/哈希/弹道/寻路）+ 绝对锚点测试全部通过
- cmd/enginehost：原生引擎宿主；cmd/tracegen：golden trace 生成器
- engine/golden：冻结帧序列基线；engine/parity：移植基线钉定记录与行为指纹
- tournament/formal_v11.json：第 2 季配置（engine.kind=native）
- workspace：botkit/参考 Bot/模板同步 v1.1 协议；手册同步
- 安全加固：oracle_host 规则加载白名单（require 全字面量）；parity 脚本字面量 require、
  固定名落盘与指纹格式校验；移除未使用的 arena run/rate -dir 覆盖入口"

# 批次 2 · 第 2 季赛果
git add war_of_desert/results/formal-v11 war_of_desert/LEADERBOARD.md \
  war_of_desert/README.md README.md "war_of_desert/bots/封存记录.json"
git commit -m "feat(season): 第 2 季基准赛完赛——Go 原生引擎 40/40 局零故障

- results/formal-v11：赛程冻结、40 局逐局结果（verifyOk 全 true）、BT 成绩、七领域加权总分
- BT 名次：Nomad → Vanguard（锚）→ Bulwark → Starter → Swarm；总分：Nomad → Starter → Vanguard → Bulwark → Swarm
- LEADERBOARD.md：两季榜单与读法（Swarm 在 v1.1 经济下崩盘、Bulwark 四平局、仍无斩舰局）
- README：两季状态表、观赛入口（回放查看器）与文档地图更新"

# 批次 3 · Dev 资料区与复核记录
git add Dev/README.md Dev/Plan_Beta.md Dev/Docs/archive/README.md Dev/plan.md \
  Dev/实验 Dev/审查 Dev/归档 \
  "Dev/Plan_Siege.md" "Dev/设计审查_正式版v1.0_20261113.md"
git commit -m "docs(dev): Dev 资料区重组——实验/审查/归档归位，补 Mimosa L3 复核记录

- Plan_Siege.md 移入 实验/；设计审查 v1.0 移入 审查/（旧路径删除）
- 新增 审查/安全复核_MimosaL3_20261114.md：61→4 误报消减过程与剩余 4 项定性
- Plan_Beta / README / 归档索引同步"

echo "补提交完成：git log --oneline -4 核对"

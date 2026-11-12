# 社区比赛__WoD · Desert Arena

> **当前阶段：正式版迁移准备**
> **当前有效规格**：[Plan_Beta.md](Plan_Beta.md) v1.0
> **当前规则预言机**：`MVP@288db07f164bc6c680a2d878c47153804d3fdcab`，只读冻结
> **当前状态**：MVP 常规测试通过；覆盖率采集失败已登记为冻结预言机的已知限制；`war_desert/` 仍保持空骨架，等待 Go 正式版迁移。

---

## 这个项目是什么

Desert Arena 是面向本地 AI 的 1v1 算法对抗赛：红蓝双方在程序生成的沙漠地形中争夺 VP 点、保护母舰，并在严格决策预算下提交 Bot 代码。项目同时保留叙事榜方向，但叙事功能不属于当前引擎迁移阻断项。

正式版目标是 Go 标准库实现，采用独立 Bot 进程、真实行分隔 JSON 协议、确定性重放和八领域计分卡。

## 当前事实源

| 文件/目录 | 作用 |
|---|---|
| [Plan_Beta.md](Plan_Beta.md) | **当前唯一正式规格与迁移计划**：规则基线、协议契约、架构边界、验收门禁、偏差登记 |
| [ORACLE.md](ORACLE.md) | 冻结 MVP 的运行、复现、覆盖率记录和 golden trace 规范 |
| `MVP/` | 冻结 JS 规则预言机；只读，不得修改 |
| `war_desert/` | 未来 Go 正式版空骨架；当前无 Go 实现 |
| `Docs/` | 原作调研、事实卡片、版权与术语资料 |
| `Plan_Alpha.md` | 历史构想，已归档 |
| `MVP_plan.md` | 历史 MVP 计划，已废止 |
| `MVP_*审查/评估报告` | 历史审查证据，不作当前规则事实源 |

## 快速核对冻结预言机

```bash
git -C MVP rev-parse HEAD
npm --prefix MVP run check
npm --prefix MVP test
```

期望：HEAD 为 `288db07f164bc6c680a2d878c47153804d3fdcab`；语法检查和 10 个测试套件全部通过。详细口径见 [ORACLE.md](ORACLE.md)。

## 当前规则快照

以 [Plan_Beta.md](Plan_Beta.md) §2 为准：

- 256×256 地图，默认 `minecraft` 生成器；
- 20 tick/s，单局 600 tick；
- 120ms 决策预算，指令固定晚 1 tick 生效；
- 连续 3 次故障红牌；
- 单 CU、四项母舰分配、四条科技树、RK4 外弹道；
- 五级判定链与八领域加权计分；
- 视野可配置，但只改变信息集，不改变模拟。

## 当前主要待办

1. 将外层基线文档纳入版本控制并打基线提交/tag。
2. 建立不修改冻结 MVP 的 golden trace 生成与保存流程。
3. 定义机器可读协议 schema 和 `map.json/replay.json/end` 契约。
4. 完成 Go `protocol/engine/scoring/io` 的目录边界设计。
5. 建立独立覆盖率/分支覆盖报告链，替代当前冻结 MVP 内不稳定的插桩门禁。

## 重要纪律

- 不修改冻结 MVP。
- 不把历史计划中的 10 TPS、512²、双资源、人类可玩交互当成当前需求。
- 不在 Go 正式版中直接照搬 MVP 的 UMD、同进程 driver、JS 哈希或 `Game` God Class。
- 任何规则变化必须先更新 `Plan_Beta.md`，再更新实现与测试。

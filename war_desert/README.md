# war_desert · Go 正式版目录

> **状态：空骨架，等待迁移实现**
> **当前有效规格**：[`../Plan_Beta.md`](../Plan_Beta.md) v1.0
> **规则预言机**：`../MVP@288db07f164bc6c680a2d878c47153804d3fdcab`，只读冻结

---

## 当前为什么没有 Go 代码

本目录按正式版规划保留，但本轮只做迁移准备，不启动 Go 实现。在协议 schema、golden trace、进程隔离设计和测试门禁完成前，不先写引擎代码，避免把冻结 MVP 的结构债翻译进正式版。

## 未来代码边界

正式实现应按以下层次组织，详细依赖规则见 `Plan_Beta.md` §4：

```text
protocol/   协议 DTO、schema、规范化序列化、地图编解码
engine/     地图、地形、移动、弹道、视野、经济、战斗、终局
scoring/    八领域计分与算力域归一
io/         os/exec、stdin/stdout、deadline、kill、换边重启、归档
```

`engine` 必须保持纯确定性；进程、墙钟和文件系统只能进入 `io`。

## 子目录规划

| 目录 | 未来用途 |
|---|---|
| `workspace/` | 选手包、规则文档、模板、本地裁判和 manifest |
| `bots/` | 选手提交的 `decide.go` 原样封存与哈希记录 |
| `tournament/` | 赛程配置、评分脚本、未来 BT/Elo 实现 |
| `results/` | 逐局结果、完整计分卡、成绩表 |
| `replays/` | `map.json`、`replay.json` 和未来回放页资源 |
| `notes/` | 赛前狠话、赛后复盘、叙事榜素材 |

## 迁移进入条件

`Plan_Beta.md` §7 的 P0 项完成后，再按 P1 顺序进入 Go 实现。至少先完成：

1. golden trace 生成器与保存格式；
2. 机器可读协议 schema；
3. Go 包边界与依赖图；
4. 不修改冻结 MVP 的覆盖率/分支覆盖报告链；
5. 疑似 MVP 缺陷的“修正或有意复刻”决策记录。

## 禁止事项

- 不复制 `MVP/` 的 JS 文件结构或 UMD 包装方式。
- 不把 Bot 调用留在引擎同进程。
- 不直接使用 JS 对象键序哈希作为跨语言确定性依据。
- 不在通过 `Plan_Beta.md` §6.2 门禁前宣布 Go 版可替代冻结 oracle。

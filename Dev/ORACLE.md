# Oracle 使用与复现规范

> **状态：已生效**（2026-11-12）
> **上游契约**：[`Plan_Beta.md`](Plan_Beta.md) v1.0
> **冻结提交**：`288db07f164bc6c680a2d878c47153804d3fdcab`

---

## 1. 角色

`Dev/MVP/` 是只读 JS 规则预言机，用途限定为：

1. 规则语义参考；
2. golden trace 生成；
3. Go 候选实现对拍；
4. 平衡实验与回归观察。

它不是正式比赛引擎，不再接收功能、修复、格式化或版本号调整。

## 2. 冻结纪律

- 禁止直接修改冻结 MVP 的工作区。
- 禁止为了让覆盖率、lint 或新需求通过而调整 MVP。
- 发现疑似 bug 时，登记到 `Plan_Beta.md` §11，由 Go 版决定修正或有意复刻。
- 若必须实验性修改，先复制到 `Dev/MVP/` 之外的新目录，并在实验记录中写明来源提交。

## 3. 最小复现步骤

在仓库根目录执行：

```bash
git -C Dev/MVP rev-parse HEAD
npm --prefix Dev/MVP run check
npm --prefix Dev/MVP test
```

要求：

- HEAD 必须为 `288db07f164bc6c680a2d878c47153804d3fdcab`，或另有正式变更记录的新冻结提交。
- `npm run check` 通过。
- `npm test` 10/10 通过并输出 `ALL_SUITES_OK`。

## 4. 当前已知运行状态

截至 2026-11-12，本机 Node `v22.22.2`、npm `10.9.7` 下：

- `npm run check`：通过。
- `npm test`：通过，10/10，约 64 秒。
- `npm run coverage -- --verbose`：失败；插桩触发地图生成的 25 秒墙钟断言，并受采集脚本 300 秒超时影响。
- 该覆盖率失败属于冻结预言机的采集/性能门槛限制，不代表常规规则测试失败。

因此覆盖率数字不得作为当前 oracle 的验收结果，也不得通过修改冻结 MVP 修复。

## 5. 覆盖率与性能记录口径

在 P1 独立报告链建立前，记录覆盖率时必须同时写明：

- 是否设置 `NODE_V8_COVERAGE`；
- Node/npm 版本；
- 机器型号与负载；
- `npm test` 原始结果；
- coverage 命令及退出码；
- 失败来自超时、性能断言还是阈值不足；
- 是否修改过任何被测文件。

不得只写“覆盖率失败/通过”。

## 6. Golden trace 输出要求

未来从冻结 oracle 生成 trace 时，每批必须记录：

- oracle commit；
- Node 版本；
- seed、generator、tickLimit、vision；
- Bot 名称、版本或脚本路径；
- 原始 `orderLog`；
- 每 tick 规范化状态快照；
- 终局 winner/level/reason；
- 八领域计分卡；
- 生成时间与工具版本。

跨语言对拍不得直接比较 JS `fnv1a(JSON.stringify(...))`；应比较规范化后的状态、事件和确定性计分字段。

## 7. 复现记录模板

```text
日期：
操作人：
机器：
Node：
npm：
MVP HEAD：
命令：
结果：
退出码：
耗时：
失败/偏差说明：
是否修改冻结 MVP：否
关联记录/附件：
```

## 8. 与正式版的关系

Go 实现通过全部迁移门禁前，任何“与规则不一致”的争议都回到冻结 oracle 复核；Go 实现通过门禁后，正式比赛仍以 `Plan_Beta.md` 和机器可读 schema 为契约，oracle 只作为历史基线和对拍工具。

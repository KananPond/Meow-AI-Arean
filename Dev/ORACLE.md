# Oracle 使用与复现规范

> **状态：已生效**（2026-11-12）｜**解冻调整**（2026-11-13）
> **上游契约**：[`Plan_Beta.md`](Plan_Beta.md) v1.0
> **基线锚点**：`Dev/MVP` 的 tag `baseline/288db07`（= `288db07f164bc6c680a2d878c47153804d3fdcab`）
> **只读对象**：`war_of_desert/engine/oracle/` 的字节快照，SHA-256 记在 `MANIFEST.json`

---

## 1. 角色

**语义权威是快照，不是源仓库。** `war_of_desert/engine/oracle/` 保存 `baseline/288db07` 四个文件
（`rules.js / mapgen.js / mapgen_mc.js / ballistics.js`）的原始字节，用途限定为：

1. 规则语义参考；
2. golden trace 生成；
3. Go 候选实现对拍；
4. 平衡实验与回归观察。

`Dev/MVP/` 已于 **2026-11-13 解冻**，恢复为可继续开发的规则源：修缺陷、试机制、加规则都在这里做，
不必再复制到外部目录。但它**不再是**语义锚点 —— 解冻后的任何提交都不等于 v1.0 规则。

## 2. 快照纪律

- **禁止就地修改 `war_of_desert/engine/oracle/`**。它是只读字节，改了第 1 季结果就无法复现。
- `Dev/MVP/` 可以改，但改动**不会自动回灌**快照。要让新规则进入正式赛，必须显式走：
  改 MVP → 跑测试 → 新出快照（新目录或新版本号）→ 更新 `MANIFEST.json` → 新 `rulesHash` → 记入 `Plan_Beta.md`。
- 发现疑似缺陷：解冻前只能登记；现在可以直接在 MVP 修，但须在 `Plan_Beta.md` §11 写清
  「影响哪个版本、是否需要一个新快照」，避免“修了却没人知道正式赛仍是旧行为”。
- **解冻不包括运行时防篡改**：`rules.js` 里 `TYPES / RPS / TECHS / SHELLS` 与 `Game.prototype`
  的 `Object.freeze` 仍保留。那是同进程 Bot 的隔离契约（`referee/match_test.js` §16），与“仓库冻结”是两件事。

## 3. 最小复现步骤

在仓库根目录执行：

```bash
python -X utf8 war_of_desert/scripts/verify_oracle.py   # 快照字节 vs MANIFEST
git -C Dev/MVP rev-parse baseline/288db07^{commit}      # 基线锚点
npm --prefix Dev/MVP run check
npm --prefix Dev/MVP test
```

要求：

- 快照校验输出 `ORACLE_SNAPSHOT_OK`（四个文件的哈希与字节数全部一致）。
- `npm run check` 通过。
- `npm test` 10/10 通过并输出 `ALL_SUITES_OK`。
- `Dev/MVP` 的 HEAD **不再要求**等于 `288db07`（已解冻，可前进）；但任何用 MVP 生成
  golden trace 的记录，必须写明当时 `git -C Dev/MVP rev-parse HEAD` 的结果。

> **Windows / `core.autocrlf=true` 注意**：checkout 会把 LF 写成 CRLF，四个快照文件的字节数各多出
> 「行数」个字节（实测 `rules.js` 101609 → 103347），哈希全错。仓库根 `.gitattributes` 已对
> `war_of_desert/engine/oracle/*` 标记 `-text`，禁止任何 EOL 转换——**不要删那两行**，也不要
> 用会改写行尾的编辑器批量保存快照。删了之后，复现性只在“从未重新 checkout 的机器”上成立。

## 4. 当前已知运行状态

截至 2026-11-12，本机 Node `v22.22.2`、npm `10.9.7` 下：

- `npm run check`：通过。
- `npm test`：通过，10/10，约 64 秒。
- `npm run coverage -- --verbose`：失败；插桩触发地图生成的 25 秒墙钟断言，并受采集脚本 300 秒超时影响。
- 该覆盖率失败属于预言机侧的采集/性能门槛限制，不代表常规规则测试失败。

因此覆盖率数字不得作为当前 oracle 的验收结果，也不得靠修改快照来“修复”。

## 5. 覆盖率与性能记录口径

在 P1 独立报告链建立前，记录覆盖率时必须同时写明：

- 是否设置 `NODE_V8_COVERAGE`；
- Node/npm 版本；
- 机器型号与负载；
- `npm test` 原始结果；
- coverage 命令及退出码；
- 失败来自超时、性能断言还是阈值不足；
- 是否修改过任何被测文件（若改了 MVP，写明当时 HEAD）。

不得只写“覆盖率失败/通过”。

## 6. Golden trace 输出要求

从快照（或用解冻后的 MVP，但须注明 HEAD）生成 trace 时，每批必须记录：

- 快照版本（`baseline/288db07`）与 `rulesHash`；
- 若用 MVP 直接生成，额外记录 `git -C Dev/MVP rev-parse HEAD`；
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
快照版本：baseline/288db07（verify_oracle: OK/FAIL）
MVP HEAD：（若用解冻后的 MVP 生成 trace 则必填）
命令：
结果：
退出码：
耗时：
失败/偏差说明：
快照是否被修改：否
关联记录/附件：
```

## 8. 与正式版的关系

Go 实现通过全部迁移门禁前，任何“与规则不一致”的争议都回到快照复核；Go 实现通过门禁后，正式比赛仍以
`Plan_Beta.md` 和机器可读 schema 为契约，快照只作为历史基线和对拍工具。

# engine · 规则引擎

> 本目录只有一件事：把冻结 oracle 变成裁判可调用的独立进程。

## 文件

| 文件 | 说明 |
|---|---|
| `oracle_host.js` | 宿主：stdin 收命令，stdout 回单行 JSON；内部 `require('./oracle/rules.js')` |
| `oracle/` | 冻结快照（`rules.js` / `mapgen.js` / `mapgen_mc.js` / `ballistics.js`）+ `MANIFEST.json` |

## 命令流

```text
{"cmd":"new", seed, generator, tickLimit, tickMs, decideMs, initMs, vision}
  → {ok, hello, seed, generator, mapAttempt, rulesHash, protoVersion, tickLimit}
{"cmd":"encode","seat":0}                  → {ok, line}
{"cmd":"apply","seat":0,"line":"..."}       → {ok, count} | {ok:false,error}
{"cmd":"step"}                              → {ok, done, turn, frame?}
{"cmd":"note","seat":0,"ms":12.5,"fault":false} → {ok}
{"cmd":"forfeit","seat":0,"reason":"..."}   → {ok, done}
{"cmd":"end","winner":-1,"level":0,"reason":"..."} → {ok, done}
{"cmd":"result"}                            → {ok, result}
{"cmd":"verify"}                            → {ok, verify}
{"cmd":"bye"}                               → 退出
```

## 纪律

- 不读墙钟、不写文件、不取系统随机数；同一 `seed + orderLog` 必得同一结果。
- 不修改 `oracle/`。发现疑似缺陷登记到 `Plan_Beta.md` §11，由 Go 引擎决定修正或有意复刻。
- 唯一输出通道是 stdout 单行 JSON；任何调试输出都会破坏协议。

## 替换为 Go 引擎

满足 `Plan_Beta.md` §6.2 门禁后：

1. 新引擎实现同一套命令流（`new/encode/apply/step/note/forfeit/end/result/verify`）；
2. `formal.json` 的 `engine.host` 指向新程序；
3. 用同一批 seed 与 golden trace 对拍；
4. 选手包、裁判、赛程、成绩契约不动。

这就是把引擎放在独立进程里的收益：换引擎不需要改赛制。

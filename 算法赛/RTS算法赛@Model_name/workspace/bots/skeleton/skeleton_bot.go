// 骨架 Bot —— 只演示 line-JSON 协议，**不含任何策略或算法**。
//
// 它只做三件事：
//   1. 读到 hello，回一行 {"ready":true}
//   2. 每读到一行 tick，回一行 {"actions":[]}（空动作，所有单位原地不动）
//   3. 读到 end，退出
//
// 协议细节见 RULES.md 第 2 节。要写策略，就在下面 handleTick 里返回动作。
//
// 这里刻意不 import botkit：好让你看清 stdin/stdout 上来回传的到底是什么。
// 需要地图解码、坐标换算、流场寻路时，再照 RULES.md 第 5 节换成 botkit.Run。
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	// hello 约 110 KB，行缓冲开到 1 MiB 以免被截断。
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	for {
		line, err := in.ReadBytes('\n')
		if err != nil {
			return // 裁判关了管道
		}

		// 先只看 type，别急着整体反序列化：hello 很大，tick 每拍一行。
		var head struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(line, &head) != nil {
			continue // 坏行忽略；反复返回坏 JSON 会被记故障
		}

		switch head.Type {
		case "hello":
			// hello 里带地图（RLE+base64）、规则常量、时序；这里不解析，只握手。
			// ready 必须在 2000 ms 内发出。
			fmt.Fprintln(out, `{"ready":true}`)
			out.Flush()

		case "tick":
			// 每拍必须回一行 {"actions":[...]}，120 ms 内交不齐记一次故障。
			// tick 里有 turn / units / vp / res / me / events 等字段，
			// 完整 schema 见 RULES.md 第 3 节。
			fmt.Fprintln(out, handleTick(line))
			out.Flush()

		case "end":
			// 终局结果在这里；回不回都行，直接退出最省事。
			return
		}
	}
}

// handleTick 是你真正要写的地方。现在它返回空动作，也就是什么都不做。
//
// 一行合法回包的样子（完整字段见 RULES.md 第 3 节）：
//
//	{"actions":[
//	   {"id":3,"type":"move","x":768,"y":768},
//	   {"type":"produce","kind":"LAV"}
//	]}
//
// id 是你的单位 id；不带 id 的阵营动作（produce / research / alloc / weapon）
// 由母舰执行。
func handleTick(tickLine []byte) string {
	_ = tickLine // 可以在这里 json.Unmarshal 出单位列表再决策
	return `{"actions":[]}`
}

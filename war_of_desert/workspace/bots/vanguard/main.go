// Vanguard · 正面推进：重甲开路，火力优先。
package main

import (
	"fmt"
	"os"

	"desertarena/workspace/refbot"
)

func main() {
	err := refbot.Run(refbot.Strategy{
		Alloc: map[string]int{"armour": 20, "repair": 20, "attack": 40, "range": 20},
		// v1.1 队列是死板 FIFO：AAV 需 30 RU 且 RU 只能靠 SALVAGER 采 ru 节点。
		// 先 SALVAGER（CU+RU 两个经济源），再轻单位，AAV 垫在后面等 RU 到账。
		Build:      []string{"SALVAGER", "AAV", "LAV", "LAV"},
		Techs:      []string{"I1", "A1", "F1", "I3a", "A2", "F2", "A3"},
		Aggression: 1.4,
		Weapons:    3,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

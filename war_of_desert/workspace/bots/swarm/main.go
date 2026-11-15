// Swarm · 数量压制：轻甲海，靠产能与射速滚雪球。
package main

import (
	"fmt"
	"os"

	"desertarena/workspace/refbot"
)

func main() {
	err := refbot.Run(refbot.Strategy{
		Alloc: map[string]int{"armour": 15, "repair": 15, "attack": 40, "range": 30},
		// v1.1 基础收入为 0：没有 SALVAGER 采 cu，开局 500 CU 花完后经济死、队列冻结。
		// 先上采集车供血，再铺轻甲海（阵容全 0 RU，不派车去 ru 节点）。
		Build:      []string{"SALVAGER", "LAV", "LAV", "SCOUT", "LAV"},
		Techs:      []string{"I1", "F1", "I3a", "F2", "F3", "I4"},
		Aggression: 1.6,
		Push:       true,
		Weapons:    2,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

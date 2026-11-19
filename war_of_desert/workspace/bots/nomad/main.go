// Nomad · 机动抢点：高移动、先出侦察，靠 VP 累计分赢。
//
// 版本：v1.2（官方参考 Bot；两季基准赛 BT 第一）。
// 功能：交战半径 0.8×、母舰前出占点、造 2 门主炮；
// SCOUT/SALVAGER 优先，M 系机动科技路线。
package main

import (
	"fmt"
	"os"

	"desertarena/workspace/refbot"
)

func main() {
	err := refbot.Run(refbot.Strategy{
		Alloc:      map[string]int{"armour": 15, "repair": 15, "attack": 30, "range": 40},
		Build:      []string{"SCOUT", "SALVAGER", "LAV"},
		Techs:      []string{"I1", "M1", "M2a", "I3a", "M3a", "M4"},
		Aggression: 0.8,
		HoldVP:     true,
		Weapons:    2,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

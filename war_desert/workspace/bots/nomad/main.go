// Nomad · 机动抢点：高移动、先出侦察，靠 VP 累计分赢。
package main

import (
	"fmt"
	"os"

	"desertarena/workspace/refbot"
)

func main() {
	err := refbot.Run(refbot.Strategy{
		Alloc:      map[string]int{"produce": 20, "attack": 25, "research": 15, "move": 40},
		Build:      []string{"SCOUT", "SALVAGER", "LAV"},
		Techs:      []string{"M1", "M2a", "M3a", "M4"},
		Aggression: 0.8,
		HoldVP:     true,
		Weapons:    2,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

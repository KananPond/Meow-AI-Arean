// Vanguard · 正面推进：重甲开路，火力优先。
package main

import (
	"fmt"
	"os"

	"desertarena/workspace/refbot"
)

func main() {
	err := refbot.Run(refbot.Strategy{
		Alloc:      map[string]int{"produce": 30, "attack": 45, "research": 10, "move": 15},
		Build:      []string{"AAV", "LAV", "LAV"},
		Techs:      []string{"A1", "F1", "A2", "F2", "A3"},
		Aggression: 1.4,
		Weapons:    3,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

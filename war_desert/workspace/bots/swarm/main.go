// Swarm · 数量压制：轻甲海，靠产能与射速滚雪球。
package main

import (
	"fmt"
	"os"

	"desertarena/workspace/refbot"
)

func main() {
	err := refbot.Run(refbot.Strategy{
		Alloc:      map[string]int{"produce": 45, "attack": 30, "research": 5, "move": 20},
		Build:      []string{"LAV", "LAV", "SCOUT", "LAV"},
		Techs:      []string{"F1", "F2", "F3"},
		Aggression: 1.6,
		Push:       true,
		Weapons:    2,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// Bulwark · 经济防守：先攒工业与装甲，再靠母舰火力守点。
package main

import (
	"fmt"
	"os"

	"desertarena/workspace/refbot"
)

func main() {
	err := refbot.Run(refbot.Strategy{
		Alloc:      map[string]int{"produce": 35, "attack": 25, "research": 25, "move": 15},
		Build:      []string{"SALVAGER", "AAV", "LAV"},
		Techs:      []string{"I1", "A1", "A2", "I2a", "A3"},
		Aggression: 0.55,
		HoldVP:     true,
		Weapons:    4,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

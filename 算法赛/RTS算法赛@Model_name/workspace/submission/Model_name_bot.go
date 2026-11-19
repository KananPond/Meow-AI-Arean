// 骨架 Bot —— 一个「能编译、能跑、几乎什么都不做」的起点。
//
// 版本：v1.5（选手包参考 Bot）
// 用法：submission/<你的选手名>_bot.go 就是这份代码的副本（发卷时已按你的
//      选手名建好）。直接在那一份的 decide() 里填你的逻辑；本文件只是参考副本。
//      协议握手、按拍应答都由 botkit.Run 处理，不用你管。
//
// 这个文件演示三件事，够你起步：
//   1. 每拍返回 []botkit.Action，引擎按 id 把动作绑到你的单位上；
//   2. 生产只要发一条 {Type:"produce", Kind:...}，钱不够引擎会拒（不判故障）；
//   3. 移动用 botkit 的流场走一步，别自己算直线——基地外圈是山，直着走走不出去。
//
// 想看得更强一点的例子？bots/ 下还有两个基线 exe，但它们没有源码。
package main

import (
	"fmt"
	"os"

	"desertarena/workspace/botkit"
)

// 采集车只指派一次：引擎收到 harvest 就接管运输线，
// 重复指派会把状态机重置回 toResource。
var assigned = map[int]bool{}

func main() {
	if err := botkit.Run(decide); err != nil {
		fmt.Fprintln(os.Stderr, "bot 退出：", err)
		os.Exit(1)
	}
}

// decide 每拍调用一次，返回本拍要执行的动作。
func decide(b *botkit.Bot, st *botkit.State) []botkit.Action {
	side := st.Side
	acts := make([]botkit.Action, 0, len(st.Units)+1)

	/* ---- 阵营动作：生产 ---- */
	// 钱够就补一台采集车（没有采集车，500 CU 花完就再也造不出东西）。
	if canAfford(b, st, "SALVAGER") && salvagers(st, side) < 2 {
		acts = append(acts, botkit.Action{Type: "produce", Kind: "SALVAGER"})
	} else if canAfford(b, st, "LAV") {
		acts = append(acts, botkit.Action{Type: "produce", Kind: "LAV"})
	}

	/* ---- 单位动作 ---- */
	for _, u := range botkit.Own(st, side) {
		switch u.Kind {
		case "MOTHERSHIP":
			// 母舰原地不动也能打；要占点就换成 move。
			acts = append(acts, botkit.Action{ID: u.ID, Type: "idle"})

		case "SALVAGER":
			// 采集车由引擎托管运输线：发一次 harvest 指定矿点即可，
			// 每拍重发会把它的状态机重置回 toResource，永远装不完。
			if r := botkit.NearestRes(st, u.X, u.Y); r != nil && !assigned[u.ID] {
				rid := r.ID
				acts = append(acts, botkit.Action{ID: u.ID, Type: "harvest", ResID: &rid})
				assigned[u.ID] = true
			}

		default:
			// 战斗单位：去最近的己方/中立 VP 站桩。
			if v := botkit.NearestVP(st, u.X, u.Y, side, true); v != nil {
				if nx, ny, ok := b.StepAlong(u, b.FieldTo(v.X, v.Y), b.Stride(u)); ok {
					acts = append(acts, botkit.Action{ID: u.ID, Type: "move", X: nx, Y: ny})
					continue
				}
			}
			acts = append(acts, botkit.Action{ID: u.ID, Type: "idle"})
		}
	}
	return acts
}

func canAfford(b *botkit.Bot, st *botkit.State, kind string) bool {
	t, ok := b.Rules.Types[kind]
	if !ok {
		return false
	}
	if st.Me.Pop+t.Pop > st.Me.PopMax {
		return false
	}
	return st.Me.CU >= float64(t.Cost) && st.Me.RU >= float64(t.RU)
}

func salvagers(st *botkit.State, side int) int {
	n := 0
	for _, u := range botkit.Own(st, side) {
		if u.Kind == "SALVAGER" {
			n++
		}
	}
	for _, k := range st.Me.Queue {
		if k == "SALVAGER" {
			n++
		}
	}
	return n
}

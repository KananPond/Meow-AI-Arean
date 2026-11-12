// Desert Arena 正式赛 · 选手提交模板
//
// 你只需要改这个文件（以及你愿意加的自己的包），把它编译成独立程序交上来：
//
//	go build -o bots/bin/submission.exe ./workspace/submission
//
// 程序必须遵守 hello → ready → tick/actions → end 的行协议：
//
//	· 读完 hello 后回一行 {"ready":true}
//	· 每收到一行 tick，回一行 {"actions":[...]}
//	· 收到 {"type":"end",...} 后自行退出
//
// 下面这份实现是「能跑通、不犯错」的基线：占点、造兵、研究、开火各做一点。
// 它不追求强度，追求你读得懂、改得动。botkit 里的注释写清了每个字段的含义。
package main

import (
	"fmt"
	"math"
	"os"

	"desertarena/workspace/botkit"
)

// buildOrder 是生产循环；按顺序尝试，钱不够就跳到下一个。
var buildOrder = []string{"LAV", "SALVAGER", "LAV", "AAV"}

// techOrder 是研究优先级；前置不满足或钱不够会自动跳过。
var techOrder = []string{"A1", "F1", "A2", "I1", "F2"}

// alloc 是母舰四项分配，和必须为 100。
var alloc = map[string]int{"produce": 35, "attack": 35, "research": 15, "move": 15}

var buildIdx int

func main() {
	if err := botkit.Run(decide); err != nil {
		fmt.Fprintln(os.Stderr, "bot 退出：", err)
		os.Exit(1)
	}
}

func decide(b *botkit.Bot, st *botkit.State) []botkit.Action {
	side := st.Side
	acts := make([]botkit.Action, 0, len(st.Units)+4)

	/* ---------- 阵营动作 ---------- */
	if !sameAlloc(st.Me.AllocTarget, alloc) {
		acts = append(acts, botkit.Action{Type: "alloc", Alloc: alloc})
	}
	if st.Me.Mode != "neutral" {
		/* neutral：途中遇敌会自己停步开火，被高差压制会后退 —— 对新手最友好 */
		acts = append(acts, botkit.Action{Type: "mode", Mode: "neutral"})
	}
	if kind, ok := nextBuild(b, st); ok {
		acts = append(acts, botkit.Action{Type: "produce", Kind: kind})
		buildIdx++
	}
	if tech := nextTech(b, st); tech != "" {
		acts = append(acts, botkit.Action{Type: "research", Tech: tech})
	}

	/* ---------- 单位动作 ---------- */
	enemies := botkit.Enemies(st, side)
	for _, u := range botkit.Own(st, side) {
		t := b.Rules.Types[u.Kind]

		switch u.Kind {
		case "MOTHERSHIP":
			/* 母舰守家附近的点；有敌人进射程就开火 */
			if e := botkit.Nearest(enemies, u.X, u.Y); e != nil && dist(u, *e) <= t.Range*0.95 {
				acts = append(acts, botkit.Action{ID: u.ID, Type: "attack", TargetID: e.ID})
				continue
			}
			if v := b.HomeVP(st, side); v != nil {
				acts = append(acts, moveAlong(b, u, v.X, v.Y))
				continue
			}
			acts = append(acts, botkit.Action{ID: u.ID, Type: "idle"})

		case "SALVAGER":
			/* 采集车：走到资源点脚下待命，站桩才会产 CU */
			if r := botkit.NearestRes(st, u.X, u.Y); r != nil {
				acts = append(acts, moveAlong(b, u, r.X, r.Y))
			}

		case "SCOUT":
			if v := botkit.NearestVP(st, u.X, u.Y, side, true); v != nil {
				acts = append(acts, moveAlong(b, u, v.X, v.Y))
			}

		default:
			/* 战斗单位：射程内打最近的敌人，否则压上去；没有敌人就去占点 */
			if e := botkit.Nearest(enemies, u.X, u.Y); e != nil {
				d := dist(u, *e)
				if d <= t.Range*0.95 {
					acts = append(acts, botkit.Action{ID: u.ID, Type: "attack", TargetID: e.ID})
					continue
				}
				if d <= t.Range*3 {
					x, y := b.StepToward(u, e.X, e.Y, b.Stride(u))
					acts = append(acts, botkit.Action{ID: u.ID, Type: "move", X: x, Y: y})
					continue
				}
			}
			if v := botkit.NearestVP(st, u.X, u.Y, side, true); v != nil {
				acts = append(acts, moveAlong(b, u, v.X, v.Y))
			}
		}
	}
	return acts
}

/* ------------------------------------------------------------------ */
/* 下面是几个小工具，按需改。                                          */
/* ------------------------------------------------------------------ */

func dist(a, b botkit.Unit) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}

func sameAlloc(have, want map[string]int) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

func nextBuild(b *botkit.Bot, st *botkit.State) (string, bool) {
	for i := 0; i < len(buildOrder); i++ {
		kind := buildOrder[(buildIdx+i)%len(buildOrder)]
		t, ok := b.Rules.Types[kind]
		if !ok {
			continue
		}
		if st.Me.Pop+t.Pop > st.Me.PopMax || len(st.Me.Queue)+len(st.Me.Active) >= 4 {
			return "", false
		}
		if st.Me.CU >= t.Cost {
			return kind, true
		}
	}
	return "", false
}

func nextTech(b *botkit.Bot, st *botkit.State) string {
	done := map[string]bool{}
	for _, id := range st.Me.TechDone {
		done[id] = true
	}
	for _, d := range st.Me.TechDoing {
		done[d.ID] = true
	}
	for _, id := range techOrder {
		if done[id] {
			continue
		}
		t, ok := b.Rules.Techs[id]
		if !ok || st.Me.CU < t.CU {
			continue
		}
		okReq := true
		for _, r := range t.Req {
			if !done[r] {
				okReq = false
				break
			}
		}
		if okReq {
			return id
		}
	}
	return ""
}

// moveAlong 用流场走到固定目标（VP / 资源点）；到达或不可达时待命。
func moveAlong(b *botkit.Bot, u botkit.Unit, x, y float64) botkit.Action {
	if nx, ny, ok := b.StepAlong(u, b.FieldTo(x, y), b.Stride(u)); ok {
		return botkit.Action{ID: u.ID, Type: "move", X: nx, Y: ny}
	}
	return botkit.Action{ID: u.ID, Type: "idle"}
}

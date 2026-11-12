// Package refbot 是官方参考 Bot 的共用策略库。
//
// 它刻意保持「能打但不神」：拿 VP、造兵、研究、开火，四件事都做，但寻路只有一步贪心。
// 目的不是当最强对手，而是给正式赛提供一批可复现、风格不同的基准选手，
// 让 BT 拟合与平衡回归有东西可测。
package refbot

import (
	"math"

	"desertarena/workspace/botkit"
)

// Strategy 是一支队伍的性格。
type Strategy struct {
	Alloc      map[string]int // 母舰四项分配目标
	Build      []string       // 生产循环
	Techs      []string       // 研究优先级
	Aggression float64        // 交战半径 = 自身射程 × 该系数
	HoldVP     bool           // 母舰是否前出占点
	Push       bool           // 战斗单位是否直插敌方半场（制造交战）
	Weapons    int            // 目标炮数
}

// Run 启动标准主循环。
func Run(s Strategy) error {
	buildIdx := 0
	return botkit.Run(func(b *botkit.Bot, st *botkit.State) []botkit.Action {
		side := st.Side
		acts := make([]botkit.Action, 0, len(st.Units)+4)

		/* ---- 阵营动作：分配 / 模式 / 生产 / 研究 / 造炮 ---- */
		if !sameAlloc(st.Me.AllocTarget, s.Alloc) {
			acts = append(acts, botkit.Action{Type: "alloc", Alloc: s.Alloc})
		}
		if st.Me.Mode != "neutral" {
			acts = append(acts, botkit.Action{Type: "mode", Mode: "neutral"})
		}
		if len(s.Build) > 0 {
			kind := s.Build[buildIdx%len(s.Build)]
			if canProduce(b, st, kind) {
				acts = append(acts, botkit.Action{Type: "produce", Kind: kind})
				buildIdx++
			}
		}
		if tech := nextTech(b, st, s.Techs); tech != "" {
			acts = append(acts, botkit.Action{Type: "research", Tech: tech})
		}
		if s.Weapons > 0 && st.Me.WeaponsReady < s.Weapons && st.Me.CU >= 240 {
			acts = append(acts, botkit.Action{Type: "weapon"})
		}

		/* ---- 单位动作 ---- */
		enemies := botkit.Enemies(st, side)
		for _, u := range botkit.Own(st, side) {
			t, _ := b.Rules.Types[u.Kind]
			switch u.Kind {
			case "SALVAGER":
				if r := botkit.NearestRes(st, u.X, u.Y); r != nil {
					acts = append(acts, moveAlong(b, u, r.X, r.Y))
				}
				continue
			case "SCOUT":
				if v := botkit.NearestVP(st, u.X, u.Y, side, true); v != nil {
					acts = append(acts, moveAlong(b, u, v.X, v.Y))
				}
				continue
			case "MOTHERSHIP":
				target := botkit.Nearest(enemies, u.X, u.Y)
				if target != nil && math.Hypot(target.X-u.X, target.Y-u.Y) <= t.Range*0.95 {
					acts = append(acts, botkit.Action{ID: u.ID, Type: "attack", TargetID: target.ID})
					continue
				}
				if s.HoldVP {
					if v := b.HomeVP(st, side); v != nil {
						acts = append(acts, moveAlong(b, u, v.X, v.Y))
						continue
					}
				}
				acts = append(acts, botkit.Action{ID: u.ID, Type: "idle"})
				continue
			}

			/* 战斗单位：射程内开火，否则压向最近敌人；没有敌人就去抢点。 */
			target := botkit.Nearest(enemies, u.X, u.Y)
			if target != nil {
				d := math.Hypot(target.X-u.X, target.Y-u.Y)
				if d <= t.Range*0.95 {
					acts = append(acts, botkit.Action{ID: u.ID, Type: "attack", TargetID: target.ID})
					continue
				}
				if d <= t.Range*s.Aggression {
					x, y := b.StepToward(u, target.X, target.Y, b.Stride(u))
					acts = append(acts, botkit.Action{ID: u.ID, Type: "move", X: x, Y: y})
					continue
				}
			}
			v := botkit.NearestVP(st, u.X, u.Y, side, true)
			if s.Push {
				v = b.EnemyVP(st, side)
			}
			if v != nil {
				acts = append(acts, moveAlong(b, u, v.X, v.Y))
			}
		}
		return acts
	})
}

func sameAlloc(have map[string]int, want map[string]int) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

func canProduce(b *botkit.Bot, st *botkit.State, kind string) bool {
	t, ok := b.Rules.Types[kind]
	if !ok {
		return false
	}
	if st.Me.Pop+t.Pop > st.Me.PopMax {
		return false
	}
	if len(st.Me.Queue)+len(st.Me.Active) >= 4 {
		return false
	}
	return st.Me.CU >= t.Cost
}

func nextTech(b *botkit.Bot, st *botkit.State, order []string) string {
	done := map[string]bool{}
	for _, id := range st.Me.TechDone {
		done[id] = true
	}
	for _, d := range st.Me.TechDoing {
		done[d.ID] = true
	}
	for _, id := range order {
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
		if len(t.ReqAny) > 0 && len(t.Req) == 0 {
			okReq = false
			for _, r := range t.ReqAny {
				if done[r] {
					okReq = true
					break
				}
			}
		}
		blocked := false
		for _, x := range t.Excl {
			if done[x] {
				blocked = true
				break
			}
		}
		if okReq && !blocked {
			return id
		}
	}
	return ""
}

// moveAlong 用流场走到固定目标（VP / 资源点）。到达或不可达时待命。
func moveAlong(b *botkit.Bot, u botkit.Unit, x, y float64) botkit.Action {
	if nx, ny, ok := b.StepAlong(u, b.FieldTo(x, y), b.Stride(u)); ok {
		return botkit.Action{ID: u.ID, Type: "move", X: nx, Y: ny}
	}
	return botkit.Action{ID: u.ID, Type: "idle"}
}

// Package refbot 是官方参考 Bot 的共用策略库。
//
// 版本：v1.2-beta（Desert Arena 正式赛）。
// 功能：提供「能打但不神」的标准策略——拿 VP、造兵、研究、开火四件事都做，
// 寻路只有一步贪心；目的不是当最强对手，而是给正式赛提供一批
// 可复现、风格不同的基准选手，让 BT 拟合与平衡回归有东西可测。
package refbot

import (
	"math"

	"desertarena/workspace/botkit"
)

// Strategy 是一支队伍的性格。
//
// 各字段由 bots/*/main.go 注入，同一份源码不同性格。
type Strategy struct {
	// 母舰四项分配目标
	Alloc map[string]int
	// 生产循环
	Build []string
	// 研究优先级
	Techs []string
	// 交战半径 = 自身射程 × 该系数
	Aggression float64
	// 母舰是否前出占点
	HoldVP bool
	// 战斗单位是否直插敌方半场
	Push bool
	// 目标炮数
	Weapons int
}

// Run 启动标准主循环。
//
// 每拍产出：阵营动作（分配/模式/生产/研究/造炮）+ 逐单位动作；
// 生产带跳过轮转与 RU 自查，harvest 只指派一次。
func Run(s Strategy) error {
	buildIdx := 0
	// 已指派的采集车数
	harvestN := 0
	// harvest 只指派一次，重发会重置 hstate
	harvested := map[int]bool{}
	return botkit.Run(func(b *botkit.Bot, st *botkit.State) []botkit.Action {
		side := st.Side
		acts := make([]botkit.Action, 0, len(st.Units)+4)

		// 生产表里有没有吃 RU 的高阶单位
		needsRU := false
		for _, k := range s.Build {
			if t, ok := b.Rules.Types[k]; ok && t.RU > 0 {
				needsRU = true
				break
			}
		}

		/* ---- 阵营动作：分配 / 模式 / 生产 / 研究 / 造炮 ---- */
		if !sameAlloc(st.Me.AllocTarget, s.Alloc) {
			acts = append(acts, botkit.Action{Type: "alloc", Alloc: s.Alloc})
		}
		if st.Me.Mode != "neutral" {
			acts = append(acts, botkit.Action{Type: "mode", Mode: "neutral"})
		}
		// 生产循环带「跳过轮转」：当前项付不起（CU 或 RU）就试下一项，
		// 避免一个永远付不起的 kind 占住轮转位（引擎队列是死板 FIFO，堵了就整队陪葬）。
		if len(s.Build) > 0 {
			for i := 0; i < len(s.Build); i++ {
				kind := s.Build[(buildIdx+i)%len(s.Build)]
				if canProduce(b, st, kind) {
					acts = append(acts, botkit.Action{Type: "produce", Kind: kind})
					buildIdx += i + 1
					break
				}
			}
		}
		if tech := nextTech(b, st, s.Techs); tech != "" {
			acts = append(acts, botkit.Action{Type: "research", Tech: tech})
		}
		// 自查炮位占用（weaponsReady + 排队中）再发 weapon，槽满绝不重发；
		// CU>=240 才排下一门，别跟生产抢启动资金。
		if s.Weapons > 0 && st.Me.WeaponsReady+len(st.Me.WeaponQueue) < s.Weapons && st.Me.CU >= 240 {
			acts = append(acts, botkit.Action{Type: "weapon"})
		}

		/* ---- 单位动作 ---- */
		enemies := botkit.Enemies(st, side)
		for _, u := range botkit.Own(st, side) {
			t, _ := b.Rules.Types[u.Kind]
			switch u.Kind {
			case "SALVAGER":
				// 采集车由引擎托管运输线，harvest 指派一次即可：
				// 每 tick 重发会把 hstate 重置回 toResource，装载循环永远走不完。
				// 目标节点按指派序分流：第 1 台采 cu（基础收入 0，CU 全靠采集）；
				// 生产表里真有高阶单位时，第 2 台采 ru 解锁，其余全回 cu——
				// 纯轻单位阵容（RU 需求为 0）不去 ru，别浪费 CU 采集力。
				if !harvested[u.ID] {
					kind := "cu"
					if harvestN == 1 && needsRU {
						kind = "ru"
					}
					if r := botkit.NearestResKind(st, u.X, u.Y, kind); r != nil {
						rid := r.ID
						acts = append(acts, botkit.Action{ID: u.ID, Type: "harvest", ResID: &rid})
						harvested[u.ID] = true
						harvestN++
					}
				}
				continue
			case "SCOUT":
				if v := botkit.NearestVP(st, u.X, u.Y, side, true); v != nil {
					acts = append(acts, moveAlong(b, u, v.X, v.Y))
				}
				continue
			case "MOTHERSHIP":
				target := botkit.Nearest(enemies, u.X, u.Y)
				if target != nil && math.Hypot(target.X-u.X, target.Y-u.Y) <= t.Range*0.95*float64(b.Hello.Cell) {
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
				if d <= t.Range*0.95*float64(b.Hello.Cell) {
					acts = append(acts, botkit.Action{ID: u.ID, Type: "attack", TargetID: target.ID})
					continue
				}
				if d <= t.Range*s.Aggression*float64(b.Hello.Cell) {
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

// sameAlloc 比较当前分配与目标分配是否一致。
//
// alloc 动作每拍最多 1 条，一致就不重发。
func sameAlloc(have map[string]int, want map[string]int) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

// canProduce 判断当前拍能否下单生产该单位。
//
// 检查人口、队列上限与 CU；v1.1 起兼查 RU——
// 引擎队列付不起只会整队冻结（不丢单也不可取消），排了就是自堵。
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
	if float64(t.RU) > st.Me.RU {
		return false
	}
	return st.Me.CU >= t.Cost
}

// nextTech 按优先级挑一个本拍可启动的科技。
//
// 前置只认已完成（在研不算，否则引擎会以 missing prereq 拒掉）；
// 槽满本拍直接不发；互斥在研的也跳过。
func nextTech(b *botkit.Bot, st *botkit.State, order []string) string {
	// 真正研究完成的集合（前置只认它）
	done := map[string]bool{}
	doing := map[string]bool{}
	for _, id := range st.Me.TechDone {
		done[id] = true
	}
	for _, d := range st.Me.TechDoing {
		doing[d.ID] = true
	}
	// 槽满（固定 2）就本拍不发，白发还要挨记
	if len(doing) >= st.Me.Slots.Research {
		return ""
	}
	for _, id := range order {
		if done[id] || doing[id] {
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
			if done[x] || doing[x] {
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

// moveAlong 用流场走到固定目标（VP / 资源点）。
//
// 到达或不可达时返回 idle，保证每台单位每拍恰一条动作。
func moveAlong(b *botkit.Bot, u botkit.Unit, x, y float64) botkit.Action {
	if nx, ny, ok := b.StepAlong(u, b.FieldTo(x, y), b.Stride(u)); ok {
		return botkit.Action{ID: u.ID, Type: "move", X: nx, Y: ny}
	}
	return botkit.Action{ID: u.ID, Type: "idle"}
}

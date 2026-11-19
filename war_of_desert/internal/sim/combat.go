// 战斗与行为层：索敌、开火、伤害结算、移动与意图绑定。
//
// 版本：v1.2（Desert Arena · Go 原生引擎 internal/sim）。
// 功能：指令归一化（normalizeOrders→orderLog）、Step 主循环的阵营动作与
// 单位行为、直击/溅射伤害、克制表应用、kill/spawn/vp 事件产出。
// 管线顺序逐条对齐 JS 合并版，禁止单方面重排。
package sim

import (
	"math"
	"sort"
)

// 战斗与终局（rules.js:1328-1791 的移植）：移动、弹道胶水、开火循环、
// 炮弹推进、VP、七领域计分、五级判定链、step 管线。

// applyMove 一步移动：坡度系数 × 地表系数 × TICK_MS/1000 时间基准。
func (g *Game) applyMove(u *Unit, tx, ty float64) {
	dx, dy := tx-u.X, ty-u.Y
	d := math.Hypot(dx, dy)
	if d < 2.5 {
		return
	}
	here := Ci(GxOf(u.X), GyOf(u.Y))
	aheadX := math.Floor((u.X + (dx/d)*Cell) / Cell)
	aheadY := math.Floor((u.Y + (dy/d)*Cell) / Cell)
	aheadX = Clamp(aheadX, 0, Grid-1)
	aheadY = Clamp(aheadY, 0, Grid-1)
	ahead := Ci(int(aheadX), int(aheadY))
	// 24 级 = 100% 坡度；格变细后除数跟缩
	slope := (float64(g.Map.Elev[ahead]) - float64(g.Map.Elev[here])) / (24.0 / Scale)
	f := Clamp(1-0.55*math.Max(0, slope)+0.18*math.Max(0, -slope), 0.35, 1.30)
	b := int(g.Map.Base[here])
	if b < len(BaseSurface) && BaseSurface[b] != 0 {
		f *= BaseSurface[b]
	}
	S := g.Sides[u.Side]
	speed := Types[u.Kind].Speed * (1 + S.Mods.Speed)
	// DoK：母舰固定慢速；时间基准 = TICK_MS/1000（v0.11 修复，不写死 0.1s）
	travel := math.Min(speed*Cell*(float64(TickMS)/1000)*f, d)
	steps := int(math.Max(1, math.Ceil(travel/(Cell*0.5))))
	step := travel / float64(steps)
	for s := 0; s < steps; s++ {
		nx := u.X + (dx/d)*step
		ny := u.Y + (dy/d)*step
		if !StepAllowed(g.Map, u.Kind, u.X, u.Y, nx, ny) {
			break
		}
		u.X = nx
		u.Y = ny
	}
}

// groundH 地形高度（m）：炮口/目标的离地高度。
func (g *Game) groundH(x, y float64) float64 {
	return ElevM * float64(g.Map.Elev[Ci(GxOf(x), GyOf(y))])
}

// solidH 弹道碰撞高度：只有山脉是实体。
func (g *Game) solidH(x, y float64) float64 {
	if g.Map.Base[Ci(GxOf(x), GyOf(y))] == 3 {
		return MountainH
	}
	return 0
}

func (g *Game) muzzleH(u *Unit) float64 {
	return g.groundH(u.X, u.Y) + UnitH
}

// v0Of 初速：基础初速 × 弹种倍率 × (1 + 发射药科技)。
func (g *Game) v0Of(u *Unit, shellID string) float64 {
	t := Types[u.Kind]
	S := g.Sides[u.Side]
	sh, ok := Shells[shellID]
	if !ok {
		sh = Shells["AP"]
	}
	return t.V0 * sh.VMul * (1 + S.Mods.Muzzle)
}

// shellsOf 已解锁弹种。
func (g *Game) shellsOf(side int) []string {
	out := []string{}
	for _, id := range ShellIDs {
		req := Shells[id].Unlock
		if req == nil || g.Sides[side].TechDone[*req] {
			out = append(out, id)
		}
	}
	return out
}

// pickShell 自动选弹：对本目标装甲等级期望最高；溅射弹给 0.12 固定加成。
func (g *Game) pickShell(side int, target *Unit) string {
	best, bestScore := "AP", -1.0
	for _, id := range g.shellsOf(side) {
		sh := Shells[id]
		armor := sh.ArmorOf(Types[target.Kind].Armor)
		score := sh.D0*armor + boolF(sh.Splash > 0)*0.12
		if score > bestScore {
			bestScore = score
			best = id
		}
	}
	return best
}

func boolF(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

type shotSolution struct {
	ok      bool
	reason  string
	theta   float64
	t       float64
	vImpact float64
	apex    float64
	aimX    float64
	aimY    float64
	dx, dy  float64
}

// solveShot 解一发：目标在飞，迭代两次用飞行时间估提前量。
func (g *Game) solveShot(u *Unit, shellID string, ex, ey, evx, evy float64) shotSolution {
	t := Types[u.Kind]
	h0 := g.muzzleH(u)
	v0 := g.v0Of(u, shellID)
	// 提前量：先夹速度，再夹偏移量（防瞬移目标飞出包线）
	vCap := 12.0 * Cell
	lvx := Clamp(evx, -vCap, vCap)
	lvy := Clamp(evy, -vCap, vCap)
	dRaw := math.Hypot(ex-u.X, ey-u.Y)
	if dRaw < MPerPX*2 {
		return shotSolution{ok: false, reason: "too-close"}
	}
	tGuess := 0.6
	var sol SolveResult
	aimX, aimY, dx, dy := ex, ey, 1.0, 0.0
	for it := 0; it < 2; it++ {
		offX, offY := lvx*tGuess, lvy*tGuess
		offLen := math.Hypot(offX, offY)
		maxOff := dRaw * 0.5
		k := 1.0
		if offLen > maxOff {
			k = maxOff / offLen
		}
		aimX = ex + offX*k
		aimY = ey + offY*k
		dPx := math.Hypot(aimX-u.X, aimY-u.Y)
		if dPx < MPerPX*2 {
			return shotSolution{ok: false, reason: "too-close"}
		}
		dx = (aimX - u.X) / dPx
		dy = (aimY - u.Y) / dPx
		d := dPx * MPerPX
		dh := g.groundH(aimX, aimY) - g.groundH(u.X, u.Y)
		sol = Solve(SolveParams{
			V0: v0, H0: h0, D: d, DH: dh,
			ElevMin: float64(t.Elev[0]), ElevMax: float64(t.Elev[1]),
			GroundAt: func(sM float64) float64 {
				return g.solidH(u.X+dx*sM/MPerPX, u.Y+dy*sM/MPerPX)
			},
		})
		if !sol.OK {
			return shotSolution{ok: false, reason: sol.Reason}
		}
		tGuess = sol.T
	}
	return shotSolution{ok: true, theta: sol.Theta, t: sol.T, vImpact: sol.VImpact,
		apex: sol.Apex, aimX: aimX, aimY: aimY, dx: dx, dy: dy}
}

// weaponCount 母舰伤害乘「已造好的炮数」。
func (g *Game) weaponCount(u *Unit) int {
	if u.Kind == "MOTHERSHIP" {
		return g.Sides[u.Side].WeaponsReady
	}
	return 1
}

// fire 开火：解一次弹道，按炮数生成若干发（多门炮 = 同一解轻微散布）。
func (g *Game) fire(u, e *Unit) bool {
	t := Types[u.Kind]
	S := g.Sides[u.Side]
	if g.weaponCount(u) <= 0 {
		return false
	}
	d := math.Hypot(u.X-e.X, u.Y-e.Y) / Cell
	lim := t.Range * (1 + S.Mods.Range)
	if u.Kind == "MOTHERSHIP" {
		lim *= RangeMulBase + RangeMulSpan*float64(S.Alloc.Range)/100
	}
	if d > lim {
		return false
	}
	shellID := g.pickShell(u.Side, e)
	sol := g.solveShot(u, shellID, e.X, e.Y, e.VX, e.VY)
	if !sol.ok {
		return false
	}
	n := int(math.Max(1, float64(g.weaponCount(u))))
	for i := 0; i < n; i++ {
		spread := 0.0
		if n > 1 {
			spread = (float64(i) - (float64(n)-1)/2) * 0.4
		}
		g.spawnShot(u, shellID, sol, spread, e.ID)
	}
	S.Shots += n
	return true
}

func (g *Game) spawnShot(u *Unit, shellID string, sol shotSolution, spreadDeg float64, targetID int) {
	v0 := g.v0Of(u, shellID)
	st := MakeShellState(v0, sol.theta+spreadDeg, g.muzzleH(u))
	g.Projectiles = append(g.Projectiles, &Projectile{
		ID: g.NextShot, Side: u.Side, SrcKind: u.Kind, Shell: shellID,
		SX: u.X, SY: u.Y, DX: sol.dx, DY: sol.dy,
		S: 0, H: st.H, VX: st.Vx, VH: st.Vh, T: 0,
		V0: v0, TargetID: targetID,
	})
	g.NextShot++
}

// hitUnit 伤害结算：落速 + 弹种 + 装甲 + 兵种克制 + 四子系统。
func (g *Game) hitUnit(p *Projectile, target *Unit, factor float64) {
	t := Types[p.SrcKind]
	dmg := DamageOf(p.Shell, t.Dmg, t.V0, p.VImpact, Types[target.Kind].Armor)
	// 克制倍率：显式 0 必须保持 0（SCOUT/SALVAGER 无武器）
	if row, ok := RPS[p.SrcKind]; ok {
		if mul, ok := row[target.Kind]; ok {
			dmg *= mul
		}
	}
	dmg *= factor
	// 四子系统：armour 减伤（目标母舰）、attack 加伤（来源母舰）
	if target.Kind == "MOTHERSHIP" {
		dmg *= 1 - ArmourReduce*(float64(g.Sides[target.Side].Alloc.Armour)/100)
	}
	if p.SrcKind == "MOTHERSHIP" {
		dmg *= AttackDmgBase + float64(g.Sides[p.Side].Alloc.Attack)/100
	}
	g.damageUnit(target, dmg, p.Side)
}

func (g *Game) damageUnit(e *Unit, dmg float64, bySide int) {
	if e.Dead {
		return
	}
	e.LastHit = g.Turn
	e.HP -= dmg
	if e.HP > 0 {
		return
	}
	e.Dead = true
	def := Types[e.Kind]
	if def.Pop > 0 {
		g.Sides[e.Side].Pop = int(math.Max(0, float64(g.Sides[e.Side].Pop-def.Pop)))
	}
	g.Sides[e.Side].Losses++
	g.Sides[bySide].Kills++
	g.Sides[bySide].KillValue += def.Cost
	ev := Event{T: g.Turn, Type: "kill", By: bySide, ID: e.ID, Kind: e.Kind}
	g.LastEvents = append(g.LastEvents, ev)
	g.Events = append(g.Events, ev)
}

// explode 一发落地：直击结算 → 溅射。
func (g *Game) explode(p *Projectile, x, y float64, direct *Unit) {
	sh := Shells[p.Shell]
	p.Dead = true
	if direct != nil {
		g.hitUnit(p, direct, 1)
		g.Sides[p.Side].Hits++
	}
	if sh.Splash <= 0 {
		return
	}
	R := sh.Splash / MPerPX
	for _, u := range g.Units {
		if u.Dead || u == direct {
			continue
		}
		dPx := math.Hypot(u.X-x, u.Y-y)
		if dPx > R {
			continue
		}
		falloff := SplashFactor(dPx, R) * sh.SplashDmg
		friendMul := 1.0
		if u.Side == p.Side {
			friendMul = 0.5
		}
		g.hitUnit(p, u, falloff*friendMul)
	}
}

// stepShots 炮弹推进：每 tick 走 TICK_MS，内部 5 个 10ms 子步，线段 vs 单位圆判命中。
func (g *Game) stepShots() {
	if len(g.Projectiles) == 0 {
		return
	}
	dt := float64(TickMS) / 1000
	const SUB = 5
	for _, p := range g.Projectiles {
		if p.Dead {
			continue
		}
		for i := 0; i < SUB && !p.Dead; i++ {
			s0, h0 := p.S, p.H
			st := Advance(ShellState{S: p.S, H: p.H, Vx: p.VX, Vh: p.VH, T: p.T}, dt/SUB)
			p.S, p.H, p.VX, p.VH, p.T = st.S, st.H, st.Vx, st.Vh, st.T
			mp1 := p.S / MPerPX
			mp0 := s0 / MPerPX
			x1, y1 := p.SX+p.DX*mp1, p.SY+p.DY*mp1
			x0, y0 := p.SX+p.DX*mp0, p.SY+p.DY*mp0
			// 1) 撞山
			if solid := g.solidH(x1, y1); solid > 0 && p.H <= solid {
				p.VImpact = ShellSpeed(ShellState{Vx: p.VX, Vh: p.VH})
				g.explode(p, x1, y1, nil)
				break
			}
			// 2) 落地
			if p.H <= 0 {
				p.VImpact = ShellSpeed(ShellState{Vx: p.VX, Vh: p.VH})
				g.explode(p, x1, y1, nil)
				break
			}
			// 3) 命中单位：线段到圆心最近距离 < 命中半径
			var hit *Unit
			for _, u := range g.Units {
				if u.Dead || u.Side == p.Side {
					// 直击只打敌人（友军只吃溅射）
					continue
				}
				ux, uy := u.X, u.Y
				vx, vy := x1-x0, y1-y0
				len2 := vx*vx + vy*vy
				tt := 0.0
				if len2 > 0 {
					tt = ((ux-x0)*vx + (uy-y0)*vy) / len2
				}
				tt = math.Max(0, math.Min(1, tt))
				cx, cy := x0+vx*tt, y0+vy*tt
				if math.Hypot(ux-cx, uy-cy) > HitRM/MPerPX {
					continue
				}
				hAt := h0 + (p.H-h0)*tt
				if math.Abs(hAt-(g.groundH(ux, uy)+UnitH)) > 6 {
					continue
				}
				hit = u
				break
			}
			if hit != nil {
				p.VImpact = ShellSpeed(ShellState{Vx: p.VX, Vh: p.VH})
				g.explode(p, hit.X, hit.Y, hit)
				break
			}
		}
		if p.T >= BallMaxT {
			p.Dead = true
		}
	}
	var alive []*Projectile
	for _, p := range g.Projectiles {
		if !p.Dead {
			alive = append(alive, p)
		}
	}
	g.Projectiles = alive
}

// updateVP VP 占控结算。
func (g *Game) updateVP() {
	for _, vp := range g.Vps {
		c0, c1 := 0, 0
		for _, u := range g.Units {
			if u.Dead || u.Kind == "MOTHERSHIP" {
				continue
			}
			if math.Hypot(u.X-vp.X, u.Y-vp.Y) <= VPRadius {
				if u.Side == 0 {
					c0++
				} else {
					c1++
				}
			}
		}
		g.Sides[0].ControlTicks += c0
		g.Sides[1].ControlTicks += c1
		prev := vp.Owner
		vp.Contested = c0 > 0 && c1 > 0
		vp.Owner = -1
		if vp.Contested {
			vp.Owner = -1
		} else if c0 > 0 {
			vp.Owner = 0
		} else if c1 > 0 {
			vp.Owner = 1
		}
		if vp.Owner >= 0 {
			g.Sides[vp.Owner].VPScore++
			g.Sides[vp.Owner].VPTicks++
		}
		if prev != vp.Owner {
			ev := Event{T: g.Turn, Type: "vp", ID: vp.ID, Owner: vp.Owner}
			g.LastEvents = append(g.LastEvents, ev)
			g.Events = append(g.Events, ev)
		}
	}
}

// computeCards 七领域确定性计分（墙钟耗时只作诊断，不进计分）。
func (g *Game) computeCards() [2]Card {
	denom := math.Max(1, float64(g.Turn))
	var out [2]Card
	for s := 0; s < 2; s++ {
		S := g.Sides[s]
		cv := g.Mothership(s)
		aliveCost := 0.0
		for _, u := range g.Units {
			if u.Side == s && u.Kind != "MOTHERSHIP" {
				aliveCost += float64(Types[u.Kind].Cost)
			}
		}
		avgBot := 0.0
		if g.BotCalls[s] > 0 {
			avgBot = g.BotMS[s] / float64(g.BotCalls[s])
		}
		f := []float64{
			100 * Clamp(float64(S.VPTicks)/(float64(len(g.Vps))*denom), 0, 1),
			100 * Clamp(float64(S.KillValue)/math.Max(1, float64(g.Sides[1-s].InitialCost)), 0, 1),
			100 * Clamp(aliveCost/math.Max(1, float64(S.ProducedCost)), 0, 1),
		}
		msHP := 0.0
		if cv != nil {
			msHP = Clamp(cv.HP/cv.MaxHP, 0, 1)
		}
		f = append(f,
			100*msHP,
			Clamp(50+10*float64(S.Kills-S.Losses), 0, 100),
			100*Clamp(float64(S.ControlTicks)/(denom*float64(len(g.Vps))*1.5), 0, 1),
			100*Clamp((float64(S.VPScore)+float64(S.KillValue)/60)/(denom/6), 0, 1),
		)
		total := 0.0
		for i := 0; i < 7; i++ {
			total += f[i] * Weights[i]
		}
		out[s] = Card{Fields: f, Total: total, VPScore: S.VPScore, Kills: S.Kills, Losses: S.Losses, BotMS: avgBot}
	}
	return out
}

// EndGame 由裁判直接收局（双方同时失去资格时用）。
func (g *Game) EndGame(winner, level int, reason string) { g.endGame(winner, level, reason) }

func (g *Game) endGame(winner, level int, reason string) {
	if g.ResultData != nil {
		return
	}
	cards := g.computeCards()
	techs := [2][]string{sortStrings(keysOf(g.Sides[0].TechDone)), sortStrings(keysOf(g.Sides[1].TechDone))}
	g.ResultData = &Result{
		Winner: winner, Level: level, Reason: reason, Turn: g.Turn, Seed: g.Seed,
		Mode: "duel", Players: g.Players, Turns: g.Turn, MapAttempt: g.Map.Attempt,
		Score: [2]int{g.Sides[0].VPScore, g.Sides[1].VPScore},
		VP:    [2]int{g.Sides[0].VPScore, g.Sides[1].VPScore},
		Cards: cards, Events: len(g.Events),
		Alloc: [2]Alloc{g.Sides[0].Alloc, g.Sides[1].Alloc},
		Techs: techs,
		Shots: [2]int{g.Sides[0].Shots, g.Sides[1].Shots},
		Hits:  [2]int{g.Sides[0].Hits, g.Sides[1].Hits},
		Timing: ResultTiming{
			TickMS: g.Cfg.TickMS, DecideMS: g.Cfg.DecideMS, InitMS: g.Cfg.InitMS,
			OrderLatency: OrderLatency, WallMS: g.Turn * g.Cfg.TickMS,
		},
	}
	for s := 0; s < 2; s++ {
		d := DecideStat{Calls: g.BotCalls[s], Late: g.BotLate[s], Faults: g.BotFaults[s]}
		if g.BotCalls[s] > 0 {
			d.AvgMS = roundTo(g.BotMS[s]/float64(g.BotCalls[s]), 3)
		}
		d.MaxMS = roundTo(g.BotMax[s], 3)
		g.ResultData.Decide[s] = d
	}
}

func roundTo(v float64, digits int) float64 {
	p := math.Pow(10, float64(digits))
	return math.Round(v*p) / p
}

func keysOf(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// Forfeit 判负（红牌）。
func (g *Game) Forfeit(seat int, reason string) {
	if g.ResultData != nil {
		return
	}
	g.Forfeited = seat
	if reason == "" {
		reason = "故障出局：连续失败达到上限（红牌）"
	}
	g.endGame(1-seat, 0, reason)
}

// checkEnd 五级判定链（rules.js:1621-1638）。
func (g *Game) checkEnd() {
	if g.ResultData != nil {
		return
	}
	if g.Forfeited >= 0 {
		g.endGame(1-g.Forfeited, 0, "故障出局：失败达到上限（红牌）")
		return
	}
	c0, c1 := g.Mothership(0), g.Mothership(1)
	dead0 := c0 == nil || c0.HP <= 0
	dead1 := c1 == nil || c1.HP <= 0
	if dead0 && dead1 {
		g.endGame(-1, 1, "双方母舰同时被摧毁，判为平局")
		return
	}
	if dead0 {
		g.endGame(1, 1, "红方母舰被摧毁")
		return
	}
	if dead1 {
		g.endGame(0, 1, "蓝方母舰被摧毁")
		return
	}
	if g.Turn >= g.Cfg.TickLimit {
		a, b := g.Sides[0], g.Sides[1]
		if a.VPScore != b.VPScore {
			if a.VPScore > b.VPScore {
				g.endGame(0, 2, "时间到 · 按 VP 累计分")
			} else {
				g.endGame(1, 2, "时间到 · 按 VP 累计分")
			}
			return
		}
		kd0, kd1 := a.Kills-a.Losses, b.Kills-b.Losses
		if kd0 != kd1 {
			if kd0 > kd1 {
				g.endGame(0, 3, "时间到 · VP 相同，按 KD 差")
			} else {
				g.endGame(1, 3, "时间到 · VP 相同，按 KD 差")
			}
			return
		}
		h0, h1 := 0.0, 0.0
		if c0 != nil {
			h0 = c0.HP / c0.MaxHP
		}
		if c1 != nil {
			h1 = c1.HP / c1.MaxHP
		}
		if math.Abs(h0-h1) > 1e-6 {
			if h0 > h1 {
				g.endGame(0, 4, "时间到 · 按母舰剩余 HP")
			} else {
				g.endGame(1, 4, "时间到 · 按母舰剩余 HP")
			}
			return
		}
		g.endGame(-1, 5, "时间到 · 全部相同，平局")
	}
}

// Result 终局记录（未结束则先跑判定）。
func (g *Game) Result() *Result {
	if g.ResultData == nil {
		g.checkEnd()
	}
	return g.ResultData
}

// normalizeOrders 归一化一方的指令（进 orderLog 的形态）。
func normalizeOrders(orders []Action) []Action {
	out := make([]Action, 0, len(orders))
	for _, a := range orders {
		switch a.Type {
		case "alloc":
			if a.Alloc != nil {
				ac := *a.Alloc
				out = append(out, Action{Type: "alloc", Alloc: &ac})
			} else {
				out = append(out, Action{Type: "alloc"})
			}
		case "produce":
			if a.By != nil {
				by := *a.By
				out = append(out, Action{Type: "produce", Kind: a.Kind, By: &by})
			} else {
				out = append(out, Action{Type: "produce", Kind: a.Kind})
			}
		case "research":
			out = append(out, Action{Type: "research", Tech: a.Tech})
		case "harvest":
			out = append(out, Action{Type: "harvest", ID: a.ID, ResID: a.ResID})
		case "weapon":
			out = append(out, Action{Type: "weapon"})
		case "mode":
			if a.ID != nil {
				id := *a.ID
				out = append(out, Action{Type: "mode", ID: &id, Mode: a.Mode})
			} else {
				out = append(out, Action{Type: "mode", Mode: a.Mode})
			}
		default:
			out = append(out, Action{Type: a.Type, ID: a.ID, X: a.X, Y: a.Y, TargetID: a.TargetID})
		}
	}
	return out
}

// Step 推进一回合（orders[i] 可为 nil）。管线顺序逐条对齐 JS。
func (g *Game) Step(orders [2][]Action) *Frame {
	if g.Done() {
		return g.buildFrame()
	}
	// 记录归一化指令（确定性重放的全部外部输入）
	rec := [2][]Action{normalizeOrders(orders[0]), normalizeOrders(orders[1])}
	g.OrderLog = append(g.OrderLog, OrderLogEntry{T: g.Turn + 1, O: rec})
	g.Turn++
	g.LastEvents = nil

	// 1. 阵营动作
	for s := 0; s < 2; s++ {
		for i := range orders[s] {
			a := &orders[s][i]
			switch a.Type {
			case "alloc":
				if a.Alloc != nil {
					g.Sides[s].Target = *a.Alloc
				}
			case "produce":
				if a.By != nil {
					var host *Unit
					for _, u := range g.Units {
						if u.ID == *a.By && !u.Dead && u.Side == s && u.Kind == "CARRIER" {
							host = u
							break
						}
					}
					if host == nil {
						g.noteRejected(s, *a.By, "produce: carrier gone")
						continue
					}
					host.Queue = append(host.Queue, ProdJob{Kind: a.Kind, Remain: float64(Types[a.Kind].Build)})
				} else {
					g.Sides[s].Queue = append(g.Sides[s].Queue, a.Kind)
				}
			case "weapon":
				S := g.Sides[s]
				if S.WeaponsReady+len(S.WeaponQueue) >= MaxWeapons {
					g.noteRejected(s, 0, "weapon: no free weapon slot")
					continue
				}
				S.WeaponQueue = append(S.WeaponQueue, WeaponJob{Remain: WeaponBuild})
			case "research":
				chk := g.techAvailable(s, a.Tech)
				if !chk.ok {
					g.noteRejected(s, 0, "research: "+chk.reason)
					continue
				}
				def := Techs[a.Tech]
				g.Sides[s].CU -= float64(def.CU)
				g.Sides[s].TechDoing = append(g.Sides[s].TechDoing, TechJob{ID: a.Tech, Remain: float64(def.Tick)})
				ev := Event{T: g.Turn, Type: "research-start", Side: s, Kind: a.Tech}
				g.LastEvents = append(g.LastEvents, ev)
				g.Events = append(g.Events, ev)
			case "mode":
				if !inList(Modes, a.Mode) {
					g.noteRejected(s, 0, "mode: unknown "+a.Mode)
					continue
				}
				if a.ID != nil {
					var one *Unit
					for _, u := range g.Units {
						if u.ID == *a.ID && !u.Dead && u.Side == s {
							one = u
							break
						}
					}
					if one == nil {
						g.noteRejected(s, *a.ID, "mode: unit gone")
						continue
					}
					one.Mode = a.Mode
				} else {
					g.Sides[s].Mode = a.Mode
					for _, u := range g.Units {
						if !u.Dead && u.Side == s {
							u.Mode = a.Mode
						}
					}
				}
			}
		}
	}

	// 2. 分配过渡 → 经济 → 生产 → 运载舰 → 武器 → 研究
	for s := 0; s < 2; s++ {
		g.stepAlloc(s)
		g.stepEconomy(s)
		g.stepProduction(s)
		g.stepCarriers(s)
		g.stepWeapons(s)
		g.stepResearch(s)
	}

	// 3. 单位动作绑定 + 行为层意图 + 移动
	for s := 0; s < 2; s++ {
		byID := map[int]*Action{}
		for i := range orders[s] {
			a := &orders[s][i]
			if a.ID != nil {
				byID[*a.ID] = a
			}
		}
		for _, u := range g.Units {
			if !u.Dead && u.Side == s {
				u.Order = byID[u.ID]
			}
		}
	}
	for _, u := range g.Units {
		if u.Dead {
			continue
		}
		u.AITargetID = 0
		if u.Order != nil && u.Order.Type == "harvest" {
			if u.Order.ResID == nil {
				u.Harvest = nil
			} else {
				res := *u.Order.ResID
				u.Harvest = &res
			}
			if u.Harvest == nil {
				u.HState = "idle"
			} else {
				u.HState = "toResource"
			}
		}
		// 引擎托管的采集车不走 Bot 移动指令
		if u.Kind == "SALVAGER" && u.Harvest != nil {
			continue
		}
		it := g.unitIntent(u)
		if it.hasMove {
			g.applyMove(u, it.moveX, it.moveY)
		}
		u.AITargetID = it.targetID
	}
	// 3b. 速度估计（弹道提前量）
	for _, u := range g.Units {
		if u.Dead {
			continue
		}
		u.VX = (u.X - u.PX) / (float64(TickMS) / 1000)
		u.VY = (u.Y - u.PY) / (float64(TickMS) / 1000)
		u.PX, u.PY = u.X, u.Y
	}
	// 4. 开火
	for _, u := range g.Units {
		if u.Dead {
			continue
		}
		t := Types[u.Kind]
		if t.Arc == "none" || t.Range <= 0 {
			continue
		}
		if u.CD > 0 {
			u.CD--
			continue
		}
		if g.weaponCount(u) <= 0 {
			continue
		}
		// 首选：Bot 显式目标 → 行为层目标 → 射程内最近敌人（都要过视线）
		var prefID int
		hasPref := false
		if u.Order != nil && u.Order.TargetID != nil {
			prefID, hasPref = *u.Order.TargetID, true
		} else if u.AITargetID != 0 {
			prefID, hasPref = u.AITargetID, true
		}
		var pref *Unit
		if hasPref {
			for _, e := range g.Units {
				if e.ID == prefID && !e.Dead && e.Side != u.Side {
					pref = e
					break
				}
			}
		}
		fired := false
		if pref != nil && (!g.Vision.Enabled || g.canSeeBySide(u.Side, pref.X, pref.Y)) {
			fired = g.fire(u, pref)
		}
		if !fired {
			lim := t.Range * (1 + g.Sides[u.Side].Mods.Range)
			type candT struct {
				e    *Unit
				d    float64
				adx  float64
				ady  float64
			}
			var cands []candT
			for _, e := range g.Units {
				if e.Dead || e.Side == u.Side {
					continue
				}
				dx, dy := u.X-e.X, u.Y-e.Y
				d := math.Hypot(dx, dy) / Cell
				if d > lim {
					continue
				}
				if g.Vision.Enabled && !g.canSeeBySide(u.Side, e.X, e.Y) {
					continue
				}
				cands = append(cands, candT{e, d, math.Abs(dx), math.Abs(dy)})
			}
			// 平局判定必须镜像不变：距离之后用 |dx|、|dy|，不能用 ID——
			// 全局 ID 两侧分属不同区间，用 ID 决胜会让镜像对局失去协变性（座位偏差）。
			sort.SliceStable(cands, func(a, b int) bool {
				if cands[a].d != cands[b].d {
					return cands[a].d < cands[b].d
				}
				if cands[a].adx != cands[b].adx {
					return cands[a].adx < cands[b].adx
				}
				return cands[a].ady < cands[b].ady
			})
			for i := 0; i < len(cands) && i < 3 && !fired; i++ {
				fired = g.fire(u, cands[i].e)
			}
		}
		if fired {
			rof := float64(t.Rof) * (1 - math.Min(0.5, g.Sides[u.Side].Mods.Rof))
			u.CD = math.Max(1, rof)
		}
	}
	// 5. 炮弹推进 + 清尸 + VP + 终局 + 帧哈希
	g.stepShots()
	var alive []*Unit
	for _, u := range g.Units {
		if !u.Dead {
			alive = append(alive, u)
		}
	}
	g.Units = alive
	g.updateVP()
	g.checkEnd()
	f := g.buildFrame()
	g.FrameHashes = append(g.FrameHashes, g.HashFrame(f))
	return f
}

func inList(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

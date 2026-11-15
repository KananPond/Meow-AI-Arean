// 对局状态机：单位/阵营/队列/科技/事件的权威持有者。
//
// 版本：v1.2-beta（Desert Arena · Go 原生引擎 internal/sim；
// v1.2 起生产队列为跳过式 + 母舰 RU 基础收入）。
// 功能：Game 构造与终局判定、经济（采集车运输线）、生产（跳过式队列）、
// 武器/研究/分配子系统、事件流与被拒指令记录。
package sim

import (
	"errors"
	"math"
	"sort"
)

// Game —— 规则引擎有状态核心（rules.js Game 类的移植，v1.1 合并版语义）。
// step 管线顺序逐条对齐 JS；所有「可能进帧哈希」的数值保持 double 口径，
// 排序全部带 index 平手裁决，遍历顺序固定 —— 这是自确定性的三根支柱。

// GameCfg 对局配置（对应 JS cfg）。
type GameCfg struct {
	TickLimit int    `json:"tickLimit"`
	TickMS    int    `json:"tickMs"`
	DecideMS  int    `json:"decideMs"`
	InitMS    int    `json:"initMs"`
	Generator string `json:"generator"`
	Vision    Vision `json:"vision"`
}

// SideState 一方状态。
type SideState struct {
	CU           float64
	RU           float64
	Pop          int
	VPScore      int
	VPTicks      int
	Kills        int
	Losses       int
	Shots        int
	Hits         int
	KillValue    int
	ProducedCost int
	InitialCost  int
	ControlTicks int
	Alloc        Alloc
	Target       Alloc
	TechDone     map[string]bool
	TechDoing    []TechJob
	Queue        []string
	Active       []ProdJob
	Mode         string
	WeaponsReady int
	WeaponQueue  []WeaponJob
	Mods         Mods
}

// Mods 科技加成累计。
type Mods struct {
	Muzzle  float64 `json:"muzzle"`
	HP      float64 `json:"hp"`
	Range   float64 `json:"range"`
	Speed   float64 `json:"speed"`
	Rof     float64 `json:"rof"`
	Income  float64 `json:"income"`
	Produce float64 `json:"produce"`
	Harvest float64 `json:"harvest"`
	Cost    float64 `json:"cost"`
}

// TechJob 正在研究的科技。
type TechJob struct {
	ID     string  `json:"id"`
	Remain float64 `json:"remain"`
}

// ProdJob 正在生产/待生产的单位。
type ProdJob struct {
	Kind   string  `json:"kind"`
	Remain float64 `json:"remain"`
}

// WeaponJob 正在造的炮。
type WeaponJob struct {
	Remain float64 `json:"remain"`
}

// Unit 一个单位。
type Unit struct {
	ID   int     `json:"id"`
	Side int     `json:"side"`
	Kind string  `json:"kind"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	HP   float64 `json:"hp"`
	// 渲染/协议用：满血值
	MaxHP float64 `json:"maxHp"`
	// 开火冷却：浮点计数（JS 口径，rof 科技后非整数）
	CD         float64
	Dead       bool
	Order      *Action
	Mode       string
	AITargetID int
	PX, PY     float64
	VX, VY     float64
	// CARRIER 专用
	Queue   []ProdJob
	Harvest *int
	CarryCU float64
	CarryRU float64
	HState  string
	Path    []PathPt
	PathKey string
}

// PathPt 采集车路径路点。
type PathPt struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// VPoint 一个 VP 点。
type VPoint struct {
	ID        int     `json:"id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Owner     int     `json:"owner"`
	Contested bool    `json:"contested"`
}

// Resource 一个资源点。
type Resource struct {
	ID     int     `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Kind   string  `json:"kind"` // cu | ru
	Amount float64 `json:"amount"`
}

// Projectile 一发在飞的炮弹。
type Projectile struct {
	ID       int
	Side     int
	SrcKind  string
	Shell    string
	SX, SY   float64
	DX, DY   float64
	S, H     float64
	VX, VH   float64
	T        float64
	V0       float64
	VImpact  float64
	TargetID int
	Dead     bool
}

// Event 对局事件。
type Event struct {
	T     int    `json:"t"`
	Type  string `json:"type"`
	Side  int    `json:"side,omitempty"`
	ID    int    `json:"id,omitempty"`
	Kind  string `json:"kind,omitempty"`
	By    int    `json:"by,omitempty"`
	Ready int    `json:"ready,omitempty"`
	Owner int    `json:"owner,omitempty"`
}

// Action 一条（归一化后的）指令。指针字段区分「缺省」与「零值」（对齐 JS null/undefined）。
// Alloc 用于对称赛（armour/repair/attack/range），SiegeAlloc 用于攻守（produce/attack/research/move）。
type Action struct {
	Type       string      `json:"type"`
	ID         *int        `json:"id,omitempty"`
	Kind       string      `json:"kind,omitempty"`
	By         *int        `json:"by,omitempty"`
	Tech       string      `json:"tech,omitempty"`
	Alloc      *Alloc      `json:"alloc,omitempty"`
	SiegeAlloc *SiegeAlloc `json:"siegeAlloc,omitempty"`
	X          *float64    `json:"x,omitempty"`
	Y          *float64    `json:"y,omitempty"`
	TargetID   *int        `json:"targetId,omitempty"`
	ResID      *int        `json:"resId,omitempty"`
	Mode       string      `json:"mode,omitempty"`
}

// OrderLogEntry 确定性重放的一条记录。
type OrderLogEntry struct {
	T int         `json:"t"`
	O [2][]Action `json:"o"`
}

// Rejected 非法动作记录。
type Rejected struct {
	Turn   int    `json:"turn"`
	Seat   int    `json:"seat"`
	ID     int    `json:"id"`
	Reason string `json:"reason"`
}

// Card 计分卡。
type Card struct {
	Fields  []float64 `json:"fields"`
	Total   float64   `json:"total"`
	VPScore int       `json:"vpScore"`
	Kills   int       `json:"kills"`
	Losses  int       `json:"losses"`
	BotMS   float64   `json:"botMs"`
}

// Result 终局记录（对齐 JS result 字段）。
type Result struct {
	Winner     int           `json:"winner"` // 0/1 胜方，-1 平局
	Level      int           `json:"level"`
	Reason     string        `json:"reason"`
	Turn       int           `json:"turn"`
	Seed       int           `json:"seed"`
	Mode       string        `json:"mode"`
	Players    int           `json:"players"`
	Turns      int           `json:"turns"`
	MapAttempt int           `json:"mapAttempt"`
	Score      [2]int        `json:"score"`
	VP         [2]int        `json:"vp"`
	Cards      [2]Card       `json:"cards"`
	Events     int           `json:"events"`
	Alloc      [2]Alloc      `json:"alloc"`
	Techs      [2][]string   `json:"techs"`
	Shots      [2]int        `json:"shots"`
	Hits       [2]int        `json:"hits"`
	Timing     ResultTiming  `json:"timing"`
	Decide     [2]DecideStat `json:"decide"`
}

// ResultTiming / DecideStat 终局统计块。
type ResultTiming struct {
	TickMS       int `json:"tickMs"`
	DecideMS     int `json:"decideMs"`
	InitMS       int `json:"initMs"`
	OrderLatency int `json:"orderLatency"`
	WallMS       int `json:"wallMs"`
}

type DecideStat struct {
	Calls  int     `json:"calls"`
	AvgMS  float64 `json:"avg_ms"`
	MaxMS  float64 `json:"max_ms"`
	Late   int     `json:"late"`
	Faults int     `json:"faults"`
}

// Game 一局对战的全部状态。
type Game struct {
	Cfg         GameCfg
	Vision      Vision
	Map         *GameMap
	Seed        int
	Players     int
	Turn        int
	Units       []*Unit
	NextID      int
	Vps         []*VPoint
	Resources   []*Resource
	Sides       [2]*SideState
	BotMS       [2]float64
	BotMax      [2]float64
	BotCalls    [2]int
	BotFaults   [2]int
	BotLate     [2]int
	Events      []Event
	LastEvents  []Event
	Projectiles []*Projectile
	NextShot    int
	ResultData  *Result
	Forfeited   int
	OrderLog    []OrderLogEntry
	FrameHashes []string
	Rejected    []Rejected
}

// NewGame 构造一局（JS Game constructor 的移植）。
func NewGame(seed int, cfg GameCfg) (*Game, error) {
	if cfg.TickLimit <= 0 {
		cfg.TickLimit = TickLimit
	}
	if cfg.TickMS <= 0 {
		cfg.TickMS = TickMS
	}
	if cfg.DecideMS <= 0 {
		cfg.DecideMS = DecideMS
	}
	if cfg.InitMS <= 0 {
		cfg.InitMS = InitMS
	}
	if cfg.Generator == "" {
		cfg.Generator = "minecraft"
	}
	v := NormalizeVision(cfg.Vision)
	cfg.Vision = v
	m, err := GenerateMap(seed, &GenOptions{Generator: cfg.Generator})
	if err != nil {
		return nil, err
	}
	g := &Game{
		Cfg: cfg, Vision: v, Map: m, Seed: m.Seed, Players: 2,
		NextID: 1, NextShot: 1, Forfeited: -1,
	}
	for _, c := range m.VPCells {
		g.Vps = append(g.Vps, &VPoint{
			ID:    len(g.Vps),
			X:     float64(c[0]*Cell) + Cell/2,
			Y:     float64(c[1]*Cell) + Cell/2,
			Owner: -1,
		})
	}
	for i, c := range m.ResCells {
		kind := "cu"
		if i%3 == 2 {
			kind = "ru"
		}
		g.Resources = append(g.Resources, &Resource{
			ID:   i,
			X:    float64(c[0]*Cell) + Cell/2,
			Y:    float64(c[1]*Cell) + Cell/2,
			Kind: kind, Amount: ResAmount,
		})
	}
	for s := 0; s < 2; s++ {
		g.Sides[s] = &SideState{
			CU: StartCU, RU: StartRU,
			Alloc: startAlloc(), Target: startAlloc(),
			TechDone: map[string]bool{}, Mode: DefaultMode,
		}
	}
	// 初始兵力：母舰 + 2 LAV
	for s := 0; s < 2; s++ {
		sp := m.SpawnCells[s]
		cx := float64(sp[0]*Cell) + Cell/2
		cy := float64(sp[1]*Cell) + Cell/2
		g.spawn(s, "MOTHERSHIP", cx, cy)
		for i := 0; i < 2; i++ {
			a := math.Pi*float64(s) + (float64(i)/2)*math.Pi*2
			g.spawn(s, "LAV", cx+math.Cos(a)*Cell*2.8*Scale, cy+math.Sin(a)*Cell*2.8*Scale)
		}
	}
	g.FrameHashes = append(g.FrameHashes, g.HashFrame(nil))
	return g, nil
}

// Done 对局是否已结束。
func (g *Game) Done() bool { return g.ResultData != nil }

func (g *Game) spawn(side int, kind string, x, y float64) *Unit {
	t := Types[kind]
	S := g.Sides[side]
	maxHP := math.Round(t.HP * (1 + S.Mods.HP))
	u := &Unit{
		ID: g.NextID, Side: side, Kind: kind, X: x, Y: y,
		HP: maxHP, MaxHP: maxHP,
		Mode: S.Mode,
		PX:   x, PY: y,
		HState:  "idle",
		CarryCU: 0, CarryRU: 0,
		AITargetID: 0,
	}
	g.NextID++
	g.Units = append(g.Units, u)
	if kind != "MOTHERSHIP" {
		S.Pop += t.Pop
		S.InitialCost += t.Cost
		S.ProducedCost += t.Cost
	}
	return u
}

// Mothership 返回某方存活母舰（无则 nil）。
func (g *Game) Mothership(side int) *Unit {
	for _, u := range g.Units {
		if u.Side == side && u.Kind == "MOTHERSHIP" && !u.Dead {
			return u
		}
	}
	return nil
}

// NoteBotTime / NoteFault 决策耗时与故障记账（诊断用，不进计分）。
func (g *Game) NoteBotTime(seat int, ms float64) {
	g.BotMS[seat] += ms
	g.BotCalls[seat]++
	if ms > g.BotMax[seat] {
		g.BotMax[seat] = ms
	}
	if ms > float64(g.Cfg.TickMS) {
		g.BotLate[seat]++
	}
}

func (g *Game) NoteFault(seat int) { g.BotFaults[seat]++ }

// noteRejected 记一条被拒指令。Turn 记该指令本应生效的 tick（= orderLog 的 T：
// 解码发生在 N 拍 step 之后，指令原定 N+1 拍生效），便于与 orderLog 逐拍对照。
// 上限 200 条/局（超出静默丢弃），口径见 engine/README.md。
func (g *Game) noteRejected(seat, id int, reason string) {
	if len(g.Rejected) < 200 {
		g.Rejected = append(g.Rejected, Rejected{Turn: g.Turn + 1, Seat: seat, ID: id, Reason: reason})
	}
}

// dist 单位间距（px）。
func unitDist(a, b *Unit) float64 { return math.Hypot(a.X-b.X, a.Y-b.Y) }

// distXY 任意两点间距（资源点判定用）。
func distXY(ax, ay, bx, by float64) float64 { return math.Hypot(ax-bx, ay-by) }

// ---------------- 行为层（obey / neutral / aggressive） ----------------

func (g *Game) canFightKind(kind string) bool {
	t, ok := Types[kind]
	return ok && t.Arc != "none" && t.Range > 0 && t.Dmg > 0
}

func (g *Game) inWeaponRange(u, e *Unit) bool {
	t, ok := Types[u.Kind]
	if !ok || t.Arc == "none" || t.Range <= 0 {
		return false
	}
	return math.Hypot(u.X-e.X, u.Y-e.Y)/Cell <= t.Range*(1+g.Sides[u.Side].Mods.Range)
}

func (g *Game) nearestVisibleEnemy(u *Unit) (*Unit, float64) {
	best, bestD := (*Unit)(nil), math.Inf(1)
	for _, e := range g.Units {
		if e.Dead || e.Side == u.Side {
			continue
		}
		if g.Vision.Enabled && !g.canSeeBySide(u.Side, e.X, e.Y) {
			continue
		}
		d := math.Hypot(u.X-e.X, u.Y-e.Y)
		if d < bestD {
			bestD = d
			best = e
		}
	}
	if best == nil {
		return nil, 0
	}
	return best, bestD / Cell
}

func (g *Game) clampToWorld(x, y float64) (float64, float64) {
	lo := Cell * 0.5
	hi := World - Cell*0.5
	return Clamp(x, lo, hi), Clamp(y, lo, hi)
}

type intent struct {
	moveX, moveY float64
	hasMove      bool
	// 0 = 无
	targetID int
}

func (g *Game) unitIntent(u *Unit) intent {
	mode := u.Mode
	if mode == "" {
		mode = DefaultMode
	}
	order := u.Order
	t := Types[u.Kind]
	canFight := g.canFightKind(u.Kind)
	if !canFight {
		if order != nil && order.Type == "move" && order.X != nil {
			return intent{moveX: *order.X, moveY: *order.Y, hasMove: true}
		}
		return intent{}
	}
	if mode == "obey" {
		it := intent{}
		if order != nil && order.Type == "move" && order.X != nil {
			it.hasMove, it.moveX, it.moveY = true, *order.X, *order.Y
		}
		if order != nil && order.TargetID != nil {
			it.targetID = *order.TargetID
		}
		return it
	}
	foe, foeD := g.nearestVisibleEnemy(u)
	if foe == nil {
		it := intent{targetID: 0}
		if order != nil && order.Type == "move" && order.X != nil {
			it.hasMove, it.moveX, it.moveY = true, *order.X, *order.Y
		}
		return it
	}
	engage := g.inWeaponRange(u, foe)
	if mode == "neutral" {
		if engage {
			myH := TerrainHeightAt(g.Map, u.X, u.Y)
			hisH := TerrainHeightAt(g.Map, foe.X, foe.Y)
			if hisH-myH >= HeightDiffMin {
				dx, dy := u.X-foe.X, u.Y-foe.Y
				d := math.Hypot(dx, dy)
				if d == 0 {
					d = 1
				}
				px, py := g.clampToWorld(u.X+(dx/d)*ModeRetreatCells*Cell, u.Y+(dy/d)*ModeRetreatCells*Cell)
				return intent{moveX: px, moveY: py, hasMove: true, targetID: foe.ID}
			}
			return intent{targetID: foe.ID}
		}
		it := intent{targetID: foe.ID}
		if order != nil && order.Type == "move" && order.X != nil && !g.hasHeightAdvantage(u, foe) {
			it.hasMove, it.moveX, it.moveY = true, *order.X, *order.Y
		}
		return it
	}
	// aggressive
	if engage {
		return intent{targetID: foe.ID}
	}
	vis := float64(t.Vision)
	if g.Vision.Enabled {
		vis += HeightBonusCells
	}
	if foeD > vis {
		it := intent{}
		if order != nil && order.Type == "move" && order.X != nil {
			it.hasMove, it.moveX, it.moveY = true, *order.X, *order.Y
		}
		return it
	}
	keep := math.Max(1, t.Range*0.8) * Cell
	dx, dy := u.X-foe.X, u.Y-foe.Y
	d := math.Hypot(dx, dy)
	if d == 0 {
		d = 1
	}
	px, py := g.clampToWorld(foe.X+(dx/d)*keep, foe.Y+(dy/d)*keep)
	return intent{moveX: px, moveY: py, hasMove: true, targetID: foe.ID}
}

func (g *Game) hasHeightAdvantage(u, e *Unit) bool {
	if e == nil {
		return false
	}
	myH := TerrainHeightAt(g.Map, u.X, u.Y)
	hisH := TerrainHeightAt(g.Map, e.X, e.Y)
	return myH-hisH >= HeightDiffMin
}

// canSee / canSeeUnit：视野判定入口（rules.js canSee 的方法化包装）。
// terrain.go 的通用 canSee 需要 TYPES 视距，这里按 Game 上下文实现同一语义。
func (g *Game) visionRangeOf(kind string) int {
	if r, ok := Types[kind]; ok {
		return r.Vision
	}
	return 0
}

func (g *Game) canSeeUnit(u *Unit, target *Unit) bool {
	return g.canSeePos(u, target.X, target.Y)
}

// canSeePos 对「某个观察单位 → 世界点」的可见性。
func (g *Game) canSeePos(u *Unit, tx, ty float64) bool {
	if !g.Vision.Enabled {
		return true
	}
	rad := g.visionRangeOf(u.Kind)
	if g.Vision.Fog {
		rad += HeightBonusAt(g.Map, u.X, u.Y, tx, ty)
		if math.Hypot(u.X-tx, u.Y-ty) > float64(rad)*Cell {
			return false
		}
	}
	if !g.Vision.LOS || LosClear(g.Map, u.X, u.Y, tx, ty) {
		return true
	}
	return false
}

// canSeeBySide 观察方 side 的任一单位能否看到 target（rules.js:373-385）。
func (g *Game) canSeeBySide(side int, tx, ty float64) bool {
	if !g.Vision.Enabled {
		return true
	}
	for _, u := range g.Units {
		if u.Dead || u.Side != side {
			continue
		}
		if g.canSeePos(u, tx, ty) {
			return true
		}
	}
	return false
}

// pointVisibleSide 空中目标按视距判（rules.js:389-398）。
func (g *Game) pointVisibleSide(side int, x, y float64) bool {
	if !g.Vision.Enabled {
		return true
	}
	for _, u := range g.Units {
		if u.Dead || u.Side != side {
			continue
		}
		rad := g.visionRangeOf(u.Kind)
		if math.Hypot(u.X-x, u.Y-y) <= float64(rad)*Cell {
			return true
		}
	}
	return false
}

// visibleEnemies 本方当前能看到的敌方单位 id 集合。
func (g *Game) visibleEnemies(side int) map[int]bool {
	set := map[int]bool{}
	if !g.Vision.Enabled {
		return set
	}
	for _, e := range g.Units {
		if e.Dead || e.Side == side {
			continue
		}
		for _, u := range g.Units {
			if u.Dead || u.Side != side {
				continue
			}
			if g.canSeePos(u, e.X, e.Y) {
				set[e.ID] = true
				break
			}
		}
	}
	return set
}

// ---------------- 母舰分配 / 科技 / 经济 / 生产 ----------------

func (g *Game) productionLines(side int) int {
	S := g.Sides[side]
	lines := 1
	if S.TechDone["I1"] {
		lines++
	}
	if S.TechDone["I3a"] || S.TechDone["I3b"] {
		lines++
	}
	return minInt(MaxSlots, lines)
}

func (g *Game) stepAlloc(side int) {
	S := g.Sides[side]
	budget := AllocStep
	moved := 0
	for _, k := range AllocKeys {
		if budget <= 0 {
			break
		}
		surplus := S.Alloc.get(k) - S.Target.get(k)
		if surplus <= 0 {
			continue
		}
		take := minInt(surplus, budget)
		S.Alloc.set(k, S.Alloc.get(k)-take)
		budget -= take
		moved += take
	}
	for _, k := range AllocKeys {
		if moved <= 0 {
			break
		}
		need := S.Target.get(k) - S.Alloc.get(k)
		if need <= 0 {
			continue
		}
		give := minInt(need, moved)
		S.Alloc.set(k, S.Alloc.get(k)+give)
		moved -= give
	}
}

type techCheck struct {
	ok     bool
	reason string
}

func (g *Game) techAvailable(side int, id string) techCheck {
	def, ok := Techs[id]
	if !ok {
		return techCheck{false, "unknown tech"}
	}
	S := g.Sides[side]
	if S.TechDone[id] {
		return techCheck{false, "already researched"}
	}
	for _, t := range S.TechDoing {
		if t.ID == id {
			return techCheck{false, "already researching"}
		}
	}
	for _, r := range def.Req {
		if !S.TechDone[r] {
			return techCheck{false, "missing prereq " + r}
		}
	}
	if len(def.ReqAny) > 0 {
		any := false
		for _, r := range def.ReqAny {
			if S.TechDone[r] {
				any = true
				break
			}
		}
		if !any {
			return techCheck{false, "missing prereq (one of " + joinSlash(def.ReqAny) + ")"}
		}
	}
	for _, x := range def.Excl {
		if S.TechDone[x] {
			return techCheck{false, "excluded by " + x}
		}
		for _, t := range S.TechDoing {
			if t.ID == x {
				return techCheck{false, "excluded by " + x}
			}
		}
	}
	if len(S.TechDoing) >= ResearchSlots {
		return techCheck{false, "no free research slot"}
	}
	if S.CU < float64(def.CU) {
		return techCheck{false, "not enough cu"}
	}
	return techCheck{true, ""}
}

func joinSlash(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += "/"
		}
		out += s
	}
	return out
}

func (g *Game) applyTech(side int, id string) {
	S := g.Sides[side]
	eff := Techs[id].Eff
	// 数值效果进 mods；unlock 是非数值（生效靠 techDone 查询）
	for _, k := range []string{"muzzle", "hp", "range", "speed", "rof", "income", "produce", "harvest", "cost"} {
		if v, ok := eff[k].(float64); ok {
			switch k {
			case "muzzle":
				S.Mods.Muzzle += v
			case "hp":
				S.Mods.HP += v
			case "range":
				S.Mods.Range += v
			case "speed":
				S.Mods.Speed += v
			case "rof":
				S.Mods.Rof += v
			case "income":
				S.Mods.Income += v
			case "produce":
				S.Mods.Produce += v
			case "harvest":
				S.Mods.Harvest += v
			case "cost":
				S.Mods.Cost += v
			}
		}
	}
	// 耐久是唯一影响已有单位的效果：按 TYPES 基准重算，避免乘算漂移
	if _, has := eff["hp"]; has {
		for _, u := range g.Units {
			if u.Side != side || u.Dead {
				continue
			}
			ratio := u.HP / u.MaxHP
			u.MaxHP = math.Round(Types[u.Kind].HP * (1 + S.Mods.HP))
			u.HP = u.MaxHP * ratio
		}
	}
}

func (g *Game) stepResearch(side int) {
	S := g.Sides[side]
	var keep []TechJob
	for _, r := range S.TechDoing {
		r.Remain--
		if r.Remain <= 0 {
			S.TechDone[r.ID] = true
			g.applyTech(side, r.ID)
			ev := Event{T: g.Turn, Type: "tech", Side: side, ID: techNumericID(r.ID)}
			g.LastEvents = append(g.LastEvents, ev)
			g.Events = append(g.Events, ev)
		} else {
			keep = append(keep, r)
		}
	}
	S.TechDoing = keep
}

// techNumericID 事件里的 id 字段：JS 把科技 id 放在字符串字段，
// 这里 Event.ID 是 int —— 科技事件用 0 并把字符串放 Kind。
func techNumericID(id string) int { return 0 }

func (g *Game) stepEconomy(side int) {
	S := g.Sides[side]
	S.CU += BaseIncome * (1 + S.Mods.Income)
	// v1.2-beta（C）：母舰微量 RU 基础收入（不吃 mods），
	// 重单位中局即可解锁，不再完全依赖 SALVAGER 采 ru 节点。
	S.RU += RUBaseIncome
	g.stepSalvagers(side)
	cv := g.Mothership(side)
	if cv != nil && !cv.Dead && cv.HP > 0 && cv.HP < cv.MaxHP {
		cv.HP = math.Min(cv.MaxHP, cv.HP+MSRepairRate*(float64(S.Alloc.Repair)/100))
	}
}

func (g *Game) stepSalvagers(side int) {
	S := g.Sides[side]
	for _, u := range g.Units {
		if u.Dead || u.Side != side || u.Kind != "SALVAGER" {
			continue
		}
		if u.Harvest == nil {
			u.HState = "idle"
			continue
		}
		var node *Resource
		for _, r := range g.Resources {
			if r.ID == *u.Harvest {
				node = r
				break
			}
		}
		if node == nil {
			u.Harvest = nil
			u.HState = "idle"
			continue
		}
		switch u.HState {
		case "toResource", "idle":
			if node.Amount <= 0 {
				u.Harvest = nil
				u.HState = "idle"
				continue
			}
			u.HState = "toResource"
			g.followPath(u, node.X, node.Y)
			if distXY(u.X, u.Y, node.X, node.Y) <= ResRadius {
				u.HState = "loading"
			}
		case "loading":
			rate := SalvageRate * (1 + S.Mods.Harvest)
			space := SalvagerCapacity - (u.CarryCU + u.CarryRU)
			take := math.Min(rate, math.Min(space, node.Amount))
			if take > 0 {
				if node.Kind == "ru" {
					u.CarryRU += take
				} else {
					u.CarryCU += take
				}
				node.Amount -= take
			}
			if u.CarryCU+u.CarryRU >= SalvagerCapacity || node.Amount <= 0 {
				u.HState = "toDropoff"
			}
		case "toDropoff":
			drop := g.nearestDropoff(side, u)
			if drop == nil {
				continue
			}
			g.followPath(u, drop.X, drop.Y)
			if unitDist(u, drop) <= ResRadius {
				u.HState = "unloading"
			}
		case "unloading":
			S.CU += u.CarryCU
			S.RU += u.CarryRU
			u.CarryCU, u.CarryRU = 0, 0
			if node.Amount > 0 {
				u.HState = "toResource"
			} else {
				u.Harvest = nil
				u.HState = "idle"
			}
		}
	}
}

func (g *Game) nearestDropoff(side int, u *Unit) *Unit {
	best, bd := (*Unit)(nil), math.Inf(1)
	for _, d := range g.Units {
		if d.Dead || d.Side != side {
			continue
		}
		if d.Kind != "MOTHERSHIP" && d.Kind != "CARRIER" {
			continue
		}
		dd := unitDist(u, d)
		if dd < bd {
			bd = dd
			best = d
		}
	}
	return best
}

func (g *Game) followPath(u *Unit, tx, ty float64) {
	path := g.salvagerPath(u, tx, ty)
	for len(path) > 0 && math.Hypot(u.X-path[0].X, u.Y-path[0].Y) < Cell*0.8 {
		path = path[1:]
	}
	u.Path = path
	wpX, wpY := tx, ty
	if len(path) > 0 {
		wpX, wpY = path[0].X, path[0].Y
	}
	g.applyMove(u, wpX, wpY)
}

func (g *Game) salvagerPath(u *Unit, tx, ty float64) []PathPt {
	gx := GxOf(tx)
	gy := GyOf(ty)
	key := itoa(gx) + ":" + itoa(gy)
	if u.PathKey == key && u.Path != nil {
		return u.Path
	}
	start := GyOf(u.Y)*Grid + GxOf(u.X)
	goal := gy*Grid + gx
	prev := make([]int32, N)
	for i := range prev {
		prev[i] = -1
	}
	seen := make([]uint8, N)
	q := make([]int32, N)
	qh, qt := 0, 0
	q[qt] = int32(start)
	qt++
	seen[start] = 1
	found := start == goal
	for qh < qt && !found {
		cur := int(q[qh])
		qh++
		cx, cy := cur%Grid, cur/Grid
		for d := 0; d < 8; d++ {
			nx, ny := cx+dx8[d], cy+dy8[d]
			if nx < 0 || ny < 0 || nx >= Grid || ny >= Grid {
				continue
			}
			ni := ny*Grid + nx
			if seen[ni] != 0 || g.Map.Base[ni] == 3 {
				continue
			}
			if abs(int(g.Map.Elev[cur])-int(g.Map.Elev[ni])) > UnitSlope {
				continue
			}
			seen[ni] = 1
			prev[ni] = int32(cur)
			q[qt] = int32(ni)
			qt++
			if ni == goal {
				found = true
				break
			}
		}
	}
	var cells []PathPt
	if found {
		for c := goal; c != start && c >= 0; c = int(prev[c]) {
			cells = append(cells, PathPt{
				X: float64(c%Grid)*Cell + Cell/2,
				Y: float64(c/Grid)*Cell + Cell/2,
			})
		}
		// reverse
		for i, j := 0, len(cells)-1; i < j; i, j = i+1, j-1 {
			cells[i], cells[j] = cells[j], cells[i]
		}
	}
	u.Path = cells
	u.PathKey = key
	return u.Path
}

func (g *Game) stepProduction(side int) {
	S := g.Sides[side]
	slots := g.productionLines(side)
	// v1.2-beta（A 队列跳过）：从队首找第一个 CU/RU 付得起的单位出队，
	// 不可负担项原地保留（不丢单、不改相对次序），整队无可负担才等待。
	// 消除 v1.1 队头死锁（50 轮实测：Vanguard 式零制造、重单位整层不可达）。
	for len(S.Active) < slots && len(S.Queue) > 0 {
		pick := -1
		var kind string
		var t UnitType
		var cost float64
		for i, k := range S.Queue {
			t = Types[k]
			cost = math.Round(float64(t.Cost) * (1 - S.Mods.Cost))
			if S.CU >= cost && S.RU >= float64(t.RU) {
				pick = i
				kind = k
				break
			}
		}
		if pick < 0 {
			break
		}
		S.CU -= cost
		S.RU -= float64(t.RU)
		S.Queue = append(S.Queue[:pick], S.Queue[pick+1:]...)
		S.Active = append(S.Active, ProdJob{Kind: kind, Remain: float64(t.Build)})
	}
	for i := range S.Active {
		S.Active[i].Remain -= 1 * (1 + S.Mods.Produce)
	}
	var finished []ProdJob
	var stillActive []ProdJob
	for _, a := range S.Active {
		if a.Remain <= 0 {
			finished = append(finished, a)
		} else {
			stillActive = append(stillActive, a)
		}
	}
	S.Active = stillActive
	if len(finished) == 0 {
		return
	}
	cv := g.Mothership(side)
	if cv == nil {
		return
	}
	for _, f := range finished {
		a := math.Mod(float64(g.Turn)*1.7+float64(g.NextID)*2.3, math.Pi*2)
		u := g.spawn(side, f.Kind,
			cv.X+math.Cos(a)*Cell*2.6*Scale,
			cv.Y+math.Sin(a)*Cell*2.6*Scale)
		ev := Event{T: g.Turn, Type: "spawn", Side: side, Kind: f.Kind, ID: u.ID}
		g.LastEvents = append(g.LastEvents, ev)
		g.Events = append(g.Events, ev)
	}
}

func (g *Game) stepWeapons(side int) {
	S := g.Sides[side]
	if len(S.WeaponQueue) == 0 {
		return
	}
	job := &S.WeaponQueue[0]
	job.Remain--
	if job.Remain > 0 {
		return
	}
	if S.CU < WeaponCost {
		job.Remain = 1
		return
	}
	S.CU -= WeaponCost
	S.WeaponQueue = S.WeaponQueue[1:]
	S.WeaponsReady++
	ev := Event{T: g.Turn, Type: "weapon", Side: side, Ready: S.WeaponsReady}
	g.LastEvents = append(g.LastEvents, ev)
	g.Events = append(g.Events, ev)
}

func (g *Game) stepCarriers(side int) {
	S := g.Sides[side]
	for _, c := range g.Units {
		if c.Dead || c.Side != side || c.Kind != "CARRIER" {
			continue
		}
		for _, u := range g.Units {
			if u.Dead || u.Side != side || u.HP >= u.MaxHP {
				continue
			}
			if math.Hypot(u.X-c.X, u.Y-c.Y) > RepairRadius {
				continue
			}
			u.HP = math.Min(u.MaxHP, u.HP+RepairRate)
		}
		if len(c.Queue) == 0 {
			continue
		}
		job := &c.Queue[0]
		cost := math.Round(float64(Types[job.Kind].Cost) * (1 - S.Mods.Cost))
		ruCost := float64(Types[job.Kind].RU)
		if job.Remain == float64(Types[job.Kind].Build) {
			if S.CU < cost || S.RU < ruCost {
				continue
			}
			S.CU -= cost
			S.RU -= ruCost
		}
		job.Remain--
		if job.Remain > 0 {
			continue
		}
		c.Queue = c.Queue[1:]
		a := math.Mod(float64(g.Turn)*2.1+float64(g.NextID)*1.7, math.Pi*2)
		u := g.spawn(side, job.Kind,
			c.X+math.Cos(a)*Cell*2*Scale,
			c.Y+math.Sin(a)*Cell*2*Scale)
		ev := Event{T: g.Turn, Type: "spawn", Side: side, Kind: job.Kind, ID: u.ID, By: c.ID}
		g.LastEvents = append(g.LastEvents, ev)
		g.Events = append(g.Events, ev)
	}
}

// sortStrings 排序副本（帧/协议里 techDone 列表按字典序）。
func sortStrings(in []string) []string {
	out := append([]string(nil), in...)
	sort.Strings(out)
	return out
}

var errDone = errors.New("game already done")

// v1.5 实验规则：胜利分账本、遗产物、插旗、CEP/射界、单位等级。
//
// 设计目标（2026-11-20 讨论稿）：
//   - 删掉「站在 VP 圈里每 tick 加分」，VP 只作为插旗与遗产入库的据点；
//   - 击杀/遗产/插旗三本账合成 VictoryScore，时间到按胜利分判；
//   - 遗产物由 SALVAGER 运输，插旗后入库；旗被击毁则旗分扣除、库存遗产掉回地面；
//   - 战斗单位有射界与炮塔转速，射击带圆概率误差（CEP），科技树新增火控分支；
//   - 单位等级参考《家园：卡拉克沙漠》：只靠击杀获得经验，5 级，逐级加精度/伤害/减伤，
//     高等级单位击杀获得更多胜利分。
//
// 本文件只在 GameCfg.Ruleset == "v15" 时生效；v14 走原路径，指纹与语义不变。
package sim

import (
	"fmt"
	"math"
)

const (
	RulesetV14 = "v14"
	RulesetV15 = "v15"

	RulesVersionV15 = "1.5"

	// VSWinScore 胜利分即时胜利阈值。时间到仍未达到则比胜利分。
	VSWinScore = 300

	// RelicValue 一件遗产入库的胜利分。
	RelicValue = 60
	// FlagValue 插旗的胜利分；被拔旗时守方扣回，拔旗方得 50%（向下取整）。
	FlagValue       = 25
	FlagStolenShare = 0.5
	// FlagHP 旗的耐久。LAV 单台约 100 tick 拆掉，给守军回防留窗口。
	FlagHP = 160

	// RelicLoadTicks / RelicDropTicks 采矿机装载/入库的读条。
	RelicLoadTicks = 30
	RelicDropTicks = 30
	// RelicSpawnTicks 遗产补充间隔；RelicMaxGround 场上同时存在的无主遗产上限。
	RelicSpawnTicks = 100
	RelicMaxGround  = 2
	// FlagPlaceTicks 采矿机在 VP 圈内静止插旗的读条。
	FlagPlaceTicks = 30

	// MothershipKillVP 母舰被毁的胜利分（终局记录用，击杀本身直接结束）。
	MothershipKillVP = 300

	// V15DamageMul v1.5 全局直伤倍率：提高战斗致命性，避免命中都被血量摊平。
	V15DamageMul = 1.80
	// V15CEPMul v1.5 的 CEP 整体缩放（越小越准）。
	V15CEPMul = 0.75
)

// RelicRadius / RelicCarrySpeedMul 遗产拾取半径与负重减速。
const (
	RelicRadius        = 2.0 * Cell * Scale
	RelicCarrySpeedMul = 0.80
	// DropoffRadius v15 的卸货半径。母舰基地外圈常被山环包住，寻路到母舰
	// 中心格会停在环外，旧口径 ResRadius（2.4 格）永远够不到，采集车在
	// 资源点与基地之间来回摆却从不卸货。v15 放宽到基地外围 6 格。
	DropoffRadius = 8.0 * Cell * Scale
)

// VictoryScore 三本账。Total 必须与三项之和保持一致（所有写入走 addVS）。
type VictoryScore struct {
	Kills  int `json:"kills"`
	Relics int `json:"relics"`
	Flags  int `json:"flags"`
	Total  int `json:"total"`
}

// Relic 一件遗产物。State: ground（无主）/ carried（在采矿机上）/ stored（已入库）。
type Relic struct {
	ID        int     `json:"id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	State     string  `json:"state"`
	CarrierID int     `json:"carrier,omitempty"`
	VPID      int     `json:"vp,omitempty"`
}

// Flag 一面旗。挂在某个 VP 上，有独立耐久，可被击毁。
type Flag struct {
	ID    int     `json:"id"`
	VPID  int     `json:"vp"`
	Side  int     `json:"side"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	HP    float64 `json:"hp"`
	MaxHP float64 `json:"maxHp"`
}

// V15State v1.5 专属状态；v14 为 nil。
type V15State struct {
	Relics      []*Relic
	Flags       []*Flag
	NextRelicID int
	NextFlagID  int
	NextSite    int
	Rng         *RNG
	Sites       []PathPt
}

// FlagType 旗的结构参数（不是可生产单位，不进 Types 表）。
var FlagType = UnitType{
	HP: FlagHP, Speed: 0, Range: 0, Dmg: 0, Rof: 0, Vision: 0,
	Arc: "none", Armor: "heavy", Cost: 0, Build: 0, Pop: 0, Structure: true,
}

// TechTreesV15 在现有四棵树基础上新增「火控」分支。
var TechTreesV15 = func() map[string]struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Levels int    `json:"levels"`
	Desc   string `json:"desc"`
} {
	m := map[string]struct {
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Levels int    `json:"levels"`
		Desc   string `json:"desc"`
	}{}
	for k, v := range TechTrees {
		m[k] = v
	}
	m["firecontrol"] = struct {
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		Levels int    `json:"levels"`
		Desc   string `json:"desc"`
	}{"火控系统", "linear", 3, "提升炮塔转速、降低射击圆概率误差（CEP）"}
	return m
}()

// TechsV15 在现有科技表上追加火控三级。C1 要求 F1、C2 要求 C1+F2，
// 让精度分支接在火力树上，而不是另起一条可裸奔的树。
var TechsV15 = func() map[string]TechDef {
	m := make(map[string]TechDef, len(Techs)+3)
	for k, v := range Techs {
		m[k] = v
	}
	m["C1"] = TechDef{Tree: "firecontrol", Tier: 1, Name: "光学测距仪", Tick: 55, CU: 90,
		Req: []string{"F1"}, Eff: map[string]any{"cep": 0.10, "traverse": 0.15}}
	m["C2"] = TechDef{Tree: "firecontrol", Tier: 2, Name: "双向稳定器", Tick: 90, CU: 150,
		Req: []string{"C1", "F2"}, Eff: map[string]any{"cep": 0.12, "traverse": 0.15}}
	m["C3"] = TechDef{Tree: "firecontrol", Tier: 3, Name: "火控计算机", Tick: 130, CU: 230,
		Req: []string{"C2"}, Eff: map[string]any{"cep": 0.15, "traverse": 0.20}}
	return m
}()

// CEPMax 满射程处的圆概率误差（世界格；1 格 = 10 m）。
// 近距按 r/range 线性缩小，最低 35%，避免贴脸也打飞。
var CEPMax = map[string]float64{
	"LAV": 0.60, "AAV": 0.70, "RAILGUN": 0.35,
	"ARTILLERY": 2.20, "CARRIER": 0.60, "MOTHERSHIP": 0.80,
}

// ArcHalf 车体射界半角（度）。360° 炮塔用 180。
var ArcHalf = map[string]float64{
	"LAV": 120, "AAV": 90, "RAILGUN": 45,
	"ARTILLERY": 180, "CARRIER": 120, "MOTHERSHIP": 150,
}

// TraverseRate 炮塔转速（度/tick，20 tick = 1 s）。
// 近战目标角速度很高：LAV 在 65 px 处横穿时约 17°/tick。转速太低会让
// 「有射界」直接退化成「永远打不中」，所以轻单位给到 12°/tick。
var TraverseRate = map[string]float64{
	"LAV": 12.0, "AAV": 6.0, "RAILGUN": 3.0,
	"ARTILLERY": 2.0, "CARRIER": 6.0, "MOTHERSHIP": 4.0,
}

// VetTitles 五级军衔（参考卡拉克沙漠的 Coalition 命名）。
var VetTitles = [5]string{"新兵", "老兵", "士官", "上尉", "指挥官"}

// VetThresholds 升到下一级所需的累计经验；经验只来自击杀，按目标造价/100 计。
// 分三档：轻/中/重，重单位升级更慢。
var VetThresholds = map[string][5]float64{
	"light":  {0, 1.5, 3.5, 6.0, 9.0},
	"medium": {0, 2.0, 5.0, 8.0, 12.0},
	"heavy":  {0, 3.0, 6.0, 10.0, 15.0},
}

func vetClass(kind string) string {
	switch kind {
	case "LAV", "SCOUT", "SALVAGER":
		return "light"
	case "AAV", "RAILGUN", "CARRIER":
		return "medium"
	case "ARTILLERY", "MOTHERSHIP":
		return "heavy"
	}
	return "light"
}

// VetLevel 按经验返回 1..5 级。
func VetLevel(kind string, xp float64) int {
	th := VetThresholds[vetClass(kind)]
	lv := 1
	for i := 1; i < len(th); i++ {
		if xp+1e-9 >= th[i] {
			lv = i + 1
		}
	}
	return lv
}

// KillVP 击杀一个单位的胜利分：造价/4，母舰固定 300。
func KillVP(kind string) int {
	if kind == "MOTHERSHIP" {
		return MothershipKillVP
	}
	t, ok := Types[kind]
	if !ok {
		return 0
	}
	v := t.Cost / 4
	if v < 1 {
		v = 1
	}
	return v
}

// addVS 统一写账，保证 Total 不漂移。kind: kills/relics/flags。
func (s *SideState) addVS(kind string, delta int) {
	switch kind {
	case "kills":
		s.VS.Kills += delta
	case "relics":
		s.VS.Relics += delta
	case "flags":
		s.VS.Flags += delta
	}
	s.VS.Total += delta
}

func (g *Game) v15() bool { return g.Cfg.Ruleset == RulesetV15 }

// techTable 返回当前规则集的科技表。
func (g *Game) techTable() map[string]TechDef {
	if g.v15() {
		return TechsV15
	}
	return Techs
}

func (g *Game) treeTable() any {
	if g.v15() {
		return TechTreesV15
	}
	return TechTrees
}

// RulesHashFor 按规则集选择指纹。v14 保持历史口径；v15 在 v14 基础上追加新常量。
func RulesHashFor(ruleset string, tickLimit int, vision Vision) string {
	if ruleset != RulesetV15 {
		return RulesHash(tickLimit, vision)
	}
	body := "v15|" + RulesVersionV15 + "|base:" + RulesHash(tickLimit, vision) +
		"|vs:" + itoa(VSWinScore) +
		"|relic:" + itoa(RelicValue) + "," + itoa(RelicSpawnTicks) + "," + itoa(RelicMaxGround) +
		"," + itoa(RelicLoadTicks) + "," + itoa(RelicDropTicks) + "," + fnum(RelicRadius) + "," + fnum(RelicCarrySpeedMul) +
		"|flag:" + itoa(FlagValue) + "," + itoa(FlagHP) + "," + itoa(FlagPlaceTicks) + "," + fnum(FlagStolenShare) +
		"|mskill:" + itoa(MothershipKillVP) +
		"|leth:" + fnum(V15DamageMul) + "," + fnum(V15CEPMul) +
		"|drop:" + fnum(DropoffRadius) +
		"|cep:" + mapFingerprint(CEPMax) +
		"|arc:" + mapFingerprint(ArcHalf) +
		"|traverse:" + mapFingerprint(TraverseRate) +
		"|vet:" + vetFingerprint() +
		"|techs:" + techFingerprintOf(TechsV15) +
		"|killvp:" + killVPFingerprint()
	return FNV1A(body)
}

func mapFingerprint(m map[string]float64) string {
	out := ""
	for _, k := range typeOrder() {
		if v, ok := m[k]; ok {
			out += k + ":" + fnum(v) + ";"
		}
	}
	return out
}

func vetFingerprint() string {
	out := ""
	for _, k := range []string{"light", "medium", "heavy"} {
		th := VetThresholds[k]
		out += k + ":"
		for _, v := range th {
			out += fnum(v) + ","
		}
		out += ";"
	}
	for _, t := range VetTitles {
		out += t + ","
	}
	out += "coef:0.6,0.04,0.03,0.15;"
	return out
}

func killVPFingerprint() string {
	out := ""
	for _, k := range typeOrder() {
		out += k + ":" + itoa(KillVP(k)) + ";"
	}
	return out
}

func killVPTable() map[string]int {
	m := map[string]int{}
	for _, k := range typeOrder() {
		m[k] = KillVP(k)
	}
	return m
}

// techFingerprintOf 与 techFingerprint 同口径，但可指定表（v15 含 C1-C3）。
func techFingerprintOf(table map[string]TechDef) string {
	out := ""
	for _, id := range techOrderV15() {
		d, ok := table[id]
		if !ok {
			continue
		}
		out += fmt.Sprintf("%s:%s,%s,%d,%d,%d,%v,%v,%v,%v;", id, d.Tree, d.Name, d.Tier, d.Tick, d.CU, d.Req, d.ReqAny, d.Excl, d.Eff)
	}
	return out
}

func techOrderV15() []string {
	return append(techOrder(), "C1", "C2", "C3")
}

// ---------------- 状态初始化 ----------------

func (g *Game) initV15() {
	g.V15 = &V15State{
		NextRelicID: 1,
		NextFlagID:  1,
		Rng:         NewRng(g.Seed ^ 0x5F3759DF),
	}
	g.V15.Sites = g.buildRelicSites()
	// 开局每方各有一件遗产，保证第一波运输必然发生。
	g.spawnRelicAt(0)
	g.spawnRelicAt(1)
}

// buildRelicSites 选 4 个镜像遗迹点：以地图中心对称，避开山、VP 与出生点。
func (g *Game) buildRelicSites() []PathPt {
	cx, cy := float64(World)/2, float64(World)/2
	dx, dy := float64(World)*0.10, float64(World)*0.06
	cands := [][2]float64{
		{cx - dx, cy - dy}, {cx + dx, cy + dy},
		{cx - dx, cy + dy}, {cx + dx, cy - dy},
	}
	out := make([]PathPt, 0, 4)
	for _, c := range cands {
		x, y := g.nearestOpenPoint(c[0], c[1], 14*Cell)
		out = append(out, PathPt{X: x, Y: y})
	}
	return out
}

// nearestOpenPoint 从 (x,y) 向外螺旋找可通行格中心。
func (g *Game) nearestOpenPoint(x, y, maxR float64) (float64, float64) {
	ok := func(px, py float64) bool {
		gx, gy := GxOf(px), GyOf(py)
		if gx < 0 || gy < 0 || gx >= Grid || gy >= Grid {
			return false
		}
		return g.Map.Base[Ci(gx, gy)] != 3
	}
	if ok(x, y) {
		return x, y
	}
	for r := float64(Cell); r <= maxR; r += Cell {
		for k := 0; k < 16; k++ {
			a := float64(k) * math.Pi / 8
			px, py := x+math.Cos(a)*r, y+math.Sin(a)*r
			if ok(px, py) {
				return px, py
			}
		}
	}
	return x, y
}

// spawnRelicAt 在指定遗迹点放一件无主遗产（该点已有地面遗产则跳过）。
func (g *Game) spawnRelicAt(site int) bool {
	if g.V15 == nil || len(g.V15.Sites) == 0 {
		return false
	}
	s := g.V15.Sites[site%len(g.V15.Sites)]
	for _, r := range g.V15.Relics {
		if r.State == "ground" && math.Hypot(r.X-s.X, r.Y-s.Y) < Cell*2 {
			return false
		}
	}
	g.V15.Relics = append(g.V15.Relics, &Relic{
		ID: g.V15.NextRelicID, X: s.X, Y: s.Y, State: "ground",
	})
	g.V15.NextRelicID++
	return true
}

func (g *Game) groundRelics() int {
	n := 0
	for _, r := range g.V15.Relics {
		if r.State == "ground" {
			n++
		}
	}
	return n
}

// stepRelicSpawn 定期补遗产，场上最多 RelicMaxGround 件。
func (g *Game) stepRelicSpawn() {
	if g.Turn%RelicSpawnTicks != 0 || g.groundRelics() >= RelicMaxGround {
		return
	}
	for i := 0; i < len(g.V15.Sites); i++ {
		site := (g.V15.NextSite + i) % len(g.V15.Sites)
		if g.spawnRelicAt(site) {
			g.V15.NextSite = (site + 1) % len(g.V15.Sites)
			return
		}
	}
}

func (g *Game) relicByID(id int) *Relic {
	for _, r := range g.V15.Relics {
		if r.ID == id {
			return r
		}
	}
	return nil
}

func (g *Game) flagByID(id int) *Flag {
	for _, f := range g.V15.Flags {
		if f.ID == id {
			return f
		}
	}
	return nil
}

func (g *Game) flagAtVP(vpID int) *Flag {
	for _, f := range g.V15.Flags {
		if f.VPID == vpID {
			return f
		}
	}
	return nil
}

func (g *Game) sideFlagAtVP(side, vpID int) *Flag {
	f := g.flagAtVP(vpID)
	if f != nil && f.Side == side {
		return f
	}
	return nil
}

func (g *Game) unitByID(id int) *Unit {
	for _, u := range g.Units {
		if u.ID == id {
			return u
		}
	}
	return nil
}

// ---------------- 遗产运输 ----------------

// stepRelics 采矿机遗产状态机。由 stepEconomy 调用（在移动与开火之前）。
func (g *Game) stepRelics(side int) {
	for _, u := range g.Units {
		if u.Dead || u.Side != side || u.Kind != "SALVAGER" || u.Relic == nil {
			continue
		}
		r := g.relicByID(*u.Relic)
		if r == nil || r.State == "stored" {
			u.Relic, u.RState = nil, ""
			u.HState = "idle"
			continue
		}
		switch u.RState {
		case "", "toRelic":
			if r.State != "ground" {
				u.Relic, u.RState = nil, ""
				continue
			}
			u.RState = "toRelic"
			g.followPath(u, r.X, r.Y)
			if distXY(u.X, u.Y, r.X, r.Y) <= RelicRadius {
				u.RState = "loading"
				u.RelicTimer = 0
			}
		case "loading":
			if r.State != "ground" {
				u.RState = "toRelic"
				continue
			}
			u.RelicTimer++
			if u.RelicTimer >= RelicLoadTicks {
				r.State = "carried"
				r.CarrierID = u.ID
				r.X, r.Y = u.X, u.Y
				u.RState = "toDeposit"
				u.RelicTimer = 0
				g.emit(Event{Type: "relic-pick", Side: side, ID: r.ID, By: u.ID})
			}
		case "toDeposit":
			vp := g.nearestFriendlyFlagVP(side, u.X, u.Y)
			if vp == nil {
				// 没有己方旗，原地等；采矿机不丢货。
				g.applyMove(u, u.X, u.Y)
				continue
			}
			g.followPath(u, vp.X, vp.Y)
			if distXY(u.X, u.Y, vp.X, vp.Y) <= VPRadius {
				u.RState = "unloading"
				u.RelicTimer = 0
			}
		case "unloading":
			var vp *VPoint
			for _, v := range g.Vps {
				if distXY(u.X, u.Y, v.X, v.Y) <= VPRadius {
					vp = v
					break
				}
			}
			if vp == nil || g.sideFlagAtVP(side, vp.ID) == nil {
				// 目标旗没了或已离开圈：重新找。
				u.RState = "toDeposit"
				u.RelicTimer = 0
				continue
			}
			u.RelicTimer++
			if u.RelicTimer >= RelicDropTicks {
				r.State = "stored"
				r.VPID = vp.ID
				r.CarrierID = 0
				r.X, r.Y = vp.X, vp.Y
				g.Sides[side].addVS("relics", RelicValue)
				u.Relic, u.RState = nil, ""
				u.RelicTimer = 0
				g.emit(Event{Type: "relic-in", Side: side, ID: r.ID, By: u.ID, Owner: vp.ID})
			}
		}
	}
}

// nearestFriendlyFlagVP 找离 (x,y) 最近的、插着本方旗的 VP。
func (g *Game) nearestFriendlyFlagVP(side int, x, y float64) *VPoint {
	var best *VPoint
	bd := math.Inf(1)
	for _, vp := range g.Vps {
		if g.sideFlagAtVP(side, vp.ID) == nil {
			continue
		}
		d := math.Hypot(vp.X-x, vp.Y-y)
		if d < bd {
			bd = d
			best = vp
		}
	}
	return best
}

// dropRelic 把采矿机携带的遗产丢回地面（被击毁/主动取消时）。
func (g *Game) dropRelic(u *Unit) {
	if u.Relic == nil {
		return
	}
	if r := g.relicByID(*u.Relic); r != nil && r.State == "carried" && r.CarrierID == u.ID {
		r.State = "ground"
		r.CarrierID = 0
		r.VPID = 0
		r.X, r.Y = u.X, u.Y
	}
	u.Relic, u.RState = nil, ""
	u.RelicTimer = 0
}

// ---------------- 插旗 ----------------

// stepFlags 采矿机在 VP 圈内静止 FlagPlaceTicks 后自动插旗。
func (g *Game) stepFlags() {
	for _, u := range g.Units {
		if u.Dead || u.Kind != "SALVAGER" || u.Harvest != nil {
			u.FlagTimer = 0
			continue
		}
		var at *VPoint
		for _, vp := range g.Vps {
			if math.Hypot(u.X-vp.X, u.Y-vp.Y) <= VPRadius {
				at = vp
				break
			}
		}
		if at == nil || g.flagAtVP(at.ID) != nil {
			u.FlagTimer = 0
			continue
		}
		u.FlagTimer++
		if u.FlagTimer >= FlagPlaceTicks {
			g.placeFlag(u.Side, at)
			u.FlagTimer = 0
		}
	}
}

func (g *Game) placeFlag(side int, vp *VPoint) *Flag {
	f := &Flag{
		ID: g.V15.NextFlagID, VPID: vp.ID, Side: side,
		X: vp.X, Y: vp.Y, HP: FlagHP, MaxHP: FlagHP,
	}
	g.V15.NextFlagID++
	g.V15.Flags = append(g.V15.Flags, f)
	g.Sides[side].addVS("flags", FlagValue)
	g.emit(Event{Type: "flag-up", Side: side, ID: f.ID, Owner: vp.ID})
	return f
}

// damageFlag 旗受伤；击毁时扣守方旗分、库存遗产掉回地面，攻击方得 50% 旗分。
func (g *Game) damageFlag(p *Projectile, f *Flag, dmg float64) {
	if f == nil || f.HP <= 0 {
		return
	}
	if p.ByUnitID != 0 {
		if u := g.unitByID(p.ByUnitID); u != nil {
			dmg *= g.vetDmgMul(u)
		}
	}
	f.HP -= dmg
	if f.HP > 0 {
		return
	}
	g.destroyFlag(f, p.Side)
}

func (g *Game) destroyFlag(f *Flag, bySide int) {
	owner := f.Side
	// 扣回插旗分（允许为负：这笔分已经花掉了也要扣）。
	g.Sides[owner].addVS("flags", -FlagValue)
	if bySide >= 0 && bySide != owner {
		share := int(math.Floor(float64(FlagValue) * FlagStolenShare))
		g.Sides[bySide].addVS("flags", share)
	}
	// 该点库存遗产全部掉回地面，并从守方账上扣回。
	for _, r := range g.V15.Relics {
		if r.State == "stored" && r.VPID == f.VPID {
			r.State = "ground"
			r.VPID = 0
			r.CarrierID = 0
			r.X, r.Y = f.X, f.Y
			g.Sides[owner].addVS("relics", -RelicValue)
		}
	}
	for i, x := range g.V15.Flags {
		if x.ID == f.ID {
			g.V15.Flags = append(g.V15.Flags[:i], g.V15.Flags[i+1:]...)
			break
		}
	}
	g.emit(Event{Type: "flag-down", Side: owner, ID: f.ID, By: bySide, Owner: f.VPID})
}

// ---------------- 胜利分 / 等级 ----------------

// addKillVS 击杀记账。byUnit 是开火单位（0 表示无归属，例如环境伤害）。
func (g *Game) addKillVS(bySide int, byUnit *Unit, victimKind string) {
	base := KillVP(victimKind)
	mul := 1.0
	if byUnit != nil {
		mul = g.vetVPMul(byUnit)
	}
	v := int(math.Round(float64(base) * mul))
	if v < 1 {
		v = 1
	}
	g.Sides[bySide].addVS("kills", v)
}

// addKillXP 只靠击杀获得经验（卡拉克沙漠口径：伤害/治疗不给经验）。
func (g *Game) addKillXP(u *Unit, victimKind string) {
	if u == nil || u.Dead {
		return
	}
	t, ok := Types[victimKind]
	if !ok {
		return
	}
	u.XP += float64(t.Cost) / 100.0
	if lv := VetLevel(u.Kind, u.XP); lv > u.Level {
		u.Level = lv
		g.emit(Event{Type: "levelup", Side: u.Side, ID: u.ID, Ready: lv, Kind: u.Kind})
	}
}

func (g *Game) vetDmgMul(u *Unit) float64 {
	if u == nil || u.Level <= 1 {
		return 1
	}
	return 1 + 0.04*float64(u.Level-1)
}

func (g *Game) vetDR(u *Unit) float64 {
	if u == nil || u.Level <= 1 {
		return 1
	}
	return 1 - 0.03*float64(u.Level-1)
}

func (g *Game) vetVPMul(u *Unit) float64 {
	if u == nil || u.Level <= 1 {
		return 1
	}
	return 1 + 0.15*float64(u.Level-1)
}

// vetCEPMul 等级带来的 CEP 缩减 = 科技缩减量的 60%（满级）。
// 没有火控科技时，等级不提供额外精度——这是「靠科技树提高的 60%」的字面实现。
func (g *Game) vetCEPMul(u *Unit) float64 {
	if u == nil || u.Level <= 1 {
		return 1
	}
	tech := g.Sides[u.Side].Mods.CEP
	return 1 - 0.6*tech*float64(u.Level-1)/4
}

// ---------------- 射击角度 / CEP ----------------

// aimV15 把炮塔转向目标，返回是否已对准且在车体射界内。
//
// 关键：炮塔方向不能在 180° 附近按「最短路径」反复翻符号，否则会在
// 正后方来回摆、永远对不准。这里锁定一个旋转方向，跨过目标时吸附。
func (g *Game) aimV15(u *Unit, tx, ty float64) bool {
	bearing := math.Atan2(ty-u.Y, tx-u.X) * 180 / math.Pi
	rate := TraverseRate[u.Kind]
	if rate <= 0 {
		return true
	}
	rate *= 1 + g.Sides[u.Side].Mods.Traverse
	d := angleDiff(u.Turret, bearing)
	if math.Abs(d) <= 6 {
		u.TurretDir = 0
		return g.arcOK(u, bearing)
	}
	if u.TurretDir == 0 {
		if d >= 0 {
			u.TurretDir = 1
		} else {
			u.TurretDir = -1
		}
	}
	next := math.Mod(u.Turret+float64(u.TurretDir)*rate+360, 360)
	nd := angleDiff(next, bearing)
	// 跨过目标方位时直接吸附；180° 附近先走出歧义区，避免误判。
	if math.Abs(d) <= 170 && d*nd <= 0 {
		u.Turret = bearing
		u.TurretDir = 0
		return g.arcOK(u, bearing)
	}
	u.Turret = next
	return false
}

func (g *Game) arcOK(u *Unit, bearing float64) bool {
	half := ArcHalf[u.Kind]
	if half > 0 && half < 180 && math.Abs(angleDiff(u.Facing, bearing)) > half {
		return false
	}
	return true
}

func (g *Game) applyCEP(u *Unit, x, y float64) (float64, float64) {
	base, ok := CEPMax[u.Kind]
	if !ok || base <= 0 {
		return x, y
	}
	rangeCells := Types[u.Kind].Range * (1 + g.Sides[u.Side].Mods.Range)
	if rangeCells <= 0 {
		return x, y
	}
	d := math.Hypot(x-u.X, y-u.Y) / Cell
	frac := Clamp(d/rangeCells, 0.35, 1)
	cep := base * frac * (1 - g.Sides[u.Side].Mods.CEP) * g.vetCEPMul(u) * V15CEPMul
	if cep <= 0 {
		return x, y
	}
	sigma := cep * Cell / 1.1774
	n1 := g.gauss()
	n2 := g.gauss()
	return x + n1*sigma, y + n2*sigma
}

// gauss Box-Muller，使用 v1.5 独立随机流。
func (g *Game) gauss() float64 {
	u1 := math.Max(1e-9, g.V15.Rng.Next())
	u2 := g.V15.Rng.Next()
	return math.Sqrt(-2*math.Log(u1)) * math.Cos(2*math.Pi*u2)
}

func angleDiff(a, b float64) float64 {
	d := math.Mod(a-b, 360)
	if d > 180 {
		d -= 360
	}
	if d < -180 {
		d += 360
	}
	return d
}

// ---------------- 帧/协议辅助 ----------------

func (g *Game) emit(ev Event) {
	ev.T = g.Turn
	g.LastEvents = append(g.LastEvents, ev)
	g.Events = append(g.Events, ev)
}

// checkEndV15 胜利分判定链；返回 true 表示已收局。
func (g *Game) checkEndV15(c0, c1 *Unit) bool {
	a, b := g.Sides[0], g.Sides[1]
	if a.VS.Total >= VSWinScore || b.VS.Total >= VSWinScore {
		if a.VS.Total > b.VS.Total {
			g.endGame(0, 2, "胜利分达到阈值")
		} else if b.VS.Total > a.VS.Total {
			g.endGame(1, 2, "胜利分达到阈值")
		} else {
			g.endGame(-1, 5, "双方同时达到胜利分阈值，判为平局")
		}
		return true
	}
	if g.Turn >= g.Cfg.TickLimit {
		if a.VS.Total != b.VS.Total {
			if a.VS.Total > b.VS.Total {
				g.endGame(0, 2, "时间到 · 按胜利分")
			} else {
				g.endGame(1, 2, "时间到 · 按胜利分")
			}
			return true
		}
		kd0, kd1 := a.Kills-a.Losses, b.Kills-b.Losses
		if kd0 != kd1 {
			if kd0 > kd1 {
				g.endGame(0, 3, "时间到 · 胜利分相同，按 KD 差")
			} else {
				g.endGame(1, 3, "时间到 · 胜利分相同，按 KD 差")
			}
			return true
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
			return true
		}
		g.endGame(-1, 5, "时间到 · 全部相同，平局")
		return true
	}
	return false
}

// computeCardsV15 七领域计分：字段 1 改为胜利分，字段 7 改为遗产吞吐。
func (g *Game) computeCardsV15() [2]Card {
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
		f := []float64{
			100 * Clamp(float64(S.VS.Total)/float64(VSWinScore), 0, 1),
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
			100*Clamp(float64(S.VS.Relics)/float64(RelicValue*8), 0, 1),
		)
		total := 0.0
		for i := 0; i < 7; i++ {
			total += f[i] * Weights[i]
		}
		out[s] = Card{Fields: f, Total: total, VPScore: S.VS.Total, Kills: S.Kills, Losses: S.Losses}
	}
	return out
}

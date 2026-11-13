// Package siegebot 是攻守实验用的最小 Bot 框架。
//
// 它刻意不依赖 workspace/botkit：旧 botkit 把 rules.types.range 当像素用，
// 而引擎实际按“世界格”判定射程（d/CELL <= range）。这里统一用：
//
//	rangePx = type.Range * hello.Cell
//	stride  = type.Speed * hello.Cell * 0.1   // 引擎移动积分当前用 0.1s/tick
//
// 这样 Bot 看到的世界和引擎一致。
package siegebot

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"desertarena/internal/protocol"
)

type Type struct {
	HP        float64 `json:"hp"`
	Speed     float64 `json:"speed"`
	Range     float64 `json:"range"`
	Vision    float64 `json:"vision"`
	Damage    float64 `json:"dmg"`
	ROF       float64 `json:"rof"`
	Pop       int     `json:"pop"`
	Cost      int     `json:"cost"`
	Arc       string  `json:"arc"`
	Armor     string  `json:"armor"`
	Structure bool    `json:"structure"`
}

type Siege struct {
	DefenderSide         int            `json:"defenderSide"`
	AttackerSide         int            `json:"attackerSide"`
	MothershipHP         int            `json:"mothershipHp"`
	AttackerMothershipHP int            `json:"attackerMothershipHp"`
	FortBudget           int            `json:"fortBudget"`
	PlaceTicks           int            `json:"placeTicks"`
	FortCost             map[string]int `json:"fortCost"`
	FortRadius           float64        `json:"fortRadius"`
	FortTypes            []string       `json:"fortTypes"`
	MineRate             float64        `json:"mineRate"`
	MineAmount           float64        `json:"mineAmount"`
	MineSalvage          float64        `json:"mineSalvage"`
	MineRadius           float64        `json:"mineRadius"`
}

type Economy struct {
	Income     float64 `json:"income"`
	MineRate   float64 `json:"mineRate"`
	MineAmount float64 `json:"mineAmount"`
}

type Rules struct {
	TickLimit int             `json:"tickLimit"`
	Types     map[string]Type `json:"types"`
	Economy   Economy         `json:"economy"`
}

type Mine struct {
	ID     int     `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Amount float64 `json:"amount"`
}

type Hello struct {
	ProtoVersion int                 `json:"protoVersion"`
	Mode         string              `json:"mode"`
	Roles        []string            `json:"roles"`
	Seed         int                 `json:"seed"`
	W            int                 `json:"w"`
	H            int                 `json:"h"`
	Cell         int                 `json:"cell"`
	World        int                 `json:"world"`
	Map          protocol.EncodedMap `json:"map"`
	Spawns       [][2]int            `json:"spawns"`
	Mines        []Mine              `json:"mines"`
	Siege        Siege               `json:"siege"`
	Rules        Rules               `json:"rules"`
}

type Unit struct {
	ID    int     `json:"id"`
	Side  int     `json:"side"`
	Kind  string  `json:"kind"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	HP    float64 `json:"hp"`
	MaxHP float64 `json:"maxHp"`
	Mode  string  `json:"mode,omitempty"`
}

type MineState struct {
	ID        int     `json:"id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Owner     int     `json:"owner"`
	Contested bool    `json:"contested"`
	Amount    float64 `json:"amount"`
}

type Active struct {
	Kind   string `json:"kind"`
	Remain int    `json:"remain"`
}

type Me struct {
	CU          float64        `json:"cu"`
	Pop         int            `json:"pop"`
	PopMax      int            `json:"popMax"`
	Alloc       map[string]int `json:"alloc"`
	AllocTarget map[string]int `json:"allocTarget"`
	Mode        string         `json:"mode"`
	FortBudget  int            `json:"fortBudget"`
	FortsPlaced int            `json:"fortsPlaced"`
	MinesOwned  int            `json:"minesOwned"`
	MineIncome  float64        `json:"mineIncome"`
	Phase       string         `json:"phase"`
	TechDone    []string       `json:"techDone"`
	TechDoing   []struct {
		ID     string `json:"id"`
		Remain int    `json:"remain"`
	} `json:"techDoing"`
	Queue        []string `json:"queue"`
	Active       []Active `json:"active"`
	WeaponsReady int      `json:"weaponsReady"`
	WeaponQueue  []int    `json:"weaponQueue"`
}

type Tick struct {
	Type      string      `json:"type"`
	Turn      int         `json:"turn"`
	TickLimit int         `json:"tickLimit"`
	Side      int         `json:"side"`
	Units     []Unit      `json:"units"`
	Mines     []MineState `json:"mines"`
	Me        Me          `json:"me"`
}

type Action struct {
	ID       int            `json:"id,omitempty"`
	Type     string         `json:"type"`
	X        float64        `json:"x,omitempty"`
	Y        float64        `json:"y,omitempty"`
	TargetID int            `json:"targetId,omitempty"`
	Alloc    map[string]int `json:"alloc,omitempty"`
	Kind     string         `json:"kind,omitempty"`
	Tech     string         `json:"tech,omitempty"`
	Mode     string         `json:"mode,omitempty"`
}

type Field struct {
	Dist []int32
	W, H int
}

type Bot struct {
	Hello    Hello
	Layers   protocol.MapLayers
	Role     string
	Cell     int
	fields   map[int]*Field
	fortIdx  int
	buildIdx int
}

func Run(role string, decide func(*Bot, *Tick) []Action) error {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	line, err := in.ReadString('\n')
	if err != nil {
		return fmt.Errorf("读 hello: %w", err)
	}
	var h Hello
	if err := json.Unmarshal([]byte(line), &h); err != nil {
		return fmt.Errorf("hello 非法: %w", err)
	}
	if h.ProtoVersion != protocol.ProtoVersion {
		return fmt.Errorf("协议版本不匹配: %d", h.ProtoVersion)
	}
	layers, err := protocol.DecodeMapLayers(h.Map)
	if err != nil {
		return fmt.Errorf("地图解码: %w", err)
	}
	b := &Bot{Hello: h, Layers: layers, Role: role, Cell: h.Cell, fields: map[int]*Field{}}
	out := bufio.NewWriterSize(os.Stdout, 1<<16)
	if _, err := out.WriteString("{\"ready\":true}\n"); err != nil {
		return err
	}
	if err := out.Flush(); err != nil {
		return err
	}
	for {
		line, err := in.ReadString('\n')
		if err != nil {
			return nil
		}
		var probe struct {
			Type string `json:"type"`
		}
		if json.Unmarshal([]byte(line), &probe) != nil {
			continue
		}
		if probe.Type == "end" {
			return nil
		}
		var t Tick
		if err := json.Unmarshal([]byte(line), &t); err != nil {
			continue
		}
		acts := decide(b, &t)
		if acts == nil {
			acts = []Action{}
		}
		raw, _ := json.Marshal(map[string]any{"actions": acts})
		if _, err := out.WriteString(string(raw) + "\n"); err != nil {
			return err
		}
		if err := out.Flush(); err != nil {
			return err
		}
	}
}

/* ---------------- 地图与移动 ---------------- */

func (b *Bot) cellOf(x, y float64) (int, int) {
	gx := int(x / float64(b.Cell))
	gy := int(y / float64(b.Cell))
	if gx < 0 {
		gx = 0
	}
	if gy < 0 {
		gy = 0
	}
	if gx >= b.Hello.W {
		gx = b.Hello.W - 1
	}
	if gy >= b.Hello.H {
		gy = b.Hello.H - 1
	}
	return gx, gy
}

func (b *Bot) walkable(gx, gy int) bool {
	if gx < 0 || gy < 0 || gx >= b.Hello.W || gy >= b.Hello.H {
		return false
	}
	i := gy*b.Hello.W + gx
	return b.Layers.Base[i] != 3
}

func (b *Bot) slopeOK(gx, gy, nx, ny int) bool {
	if !b.walkable(nx, ny) {
		return false
	}
	i := gy*b.Hello.W + gx
	j := ny*b.Hello.W + nx
	d := int(b.Layers.Elev[j]) - int(b.Layers.Elev[i])
	if d < 0 {
		d = -d
	}
	return d <= 104 // UNIT_SLOPE = 26 * SCALE
}

// FieldTo 从目标点做 BFS，得到全图到目标的步数场。目标不可走时退到最近可走格。
func (b *Bot) FieldTo(tx, ty float64) *Field {
	gx, gy := b.cellOf(tx, ty)
	if !b.walkable(gx, gy) {
		found := false
		for r := 1; r < 12 && !found; r++ {
			for dy := -r; dy <= r && !found; dy++ {
				for dx := -r; dx <= r && !found; dx++ {
					if b.walkable(gx+dx, gy+dy) {
						gx, gy, found = gx+dx, gy+dy, true
					}
				}
			}
		}
		if !found {
			return nil
		}
	}
	key := gy*b.Hello.W + gx
	if f, ok := b.fields[key]; ok {
		return f
	}
	w, h := b.Hello.W, b.Hello.H
	dist := make([]int32, w*h)
	for i := range dist {
		dist[i] = -1
	}
	q := make([]int, 0, w*h)
	start := gy*w + gx
	dist[start] = 0
	q = append(q, start)
	dirs := [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	for head := 0; head < len(q); head++ {
		i := q[head]
		cx, cy := i%w, i/w
		for _, d := range dirs {
			nx, ny := cx+d[0], cy+d[1]
			if !b.slopeOK(cx, cy, nx, ny) {
				continue
			}
			j := ny*w + nx
			if dist[j] >= 0 {
				continue
			}
			dist[j] = dist[i] + 1
			q = append(q, j)
		}
	}
	f := &Field{Dist: dist, W: w, H: h}
	b.fields[key] = f
	return f
}

// StepAlong 沿步数场走一步；返回下一个落点。走不动时返回 false。
func (b *Bot) StepAlong(u Unit, f *Field, stride float64) (float64, float64, bool) {
	if f == nil {
		return 0, 0, false
	}
	gx, gy := b.cellOf(u.X, u.Y)
	cur := gy*f.W + gx
	if f.Dist[cur] <= 0 {
		return 0, 0, false
	}
	best, bestD := -1, f.Dist[cur]
	dirs := [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}}
	for _, d := range dirs {
		nx, ny := gx+d[0], gy+d[1]
		if !b.slopeOK(gx, gy, nx, ny) {
			continue
		}
		j := ny*f.W + nx
		if f.Dist[j] >= 0 && f.Dist[j] < bestD {
			bestD, best = f.Dist[j], j
		}
	}
	if best < 0 {
		return 0, 0, false
	}
	bx, by := best%f.W, best/f.W
	tx := (float64(bx) + 0.5) * float64(b.Cell)
	ty := (float64(by) + 0.5) * float64(b.Cell)
	dx, dy := tx-u.X, ty-u.Y
	d := math.Hypot(dx, dy)
	if d < 1e-6 {
		return tx, ty, true
	}
	if d < stride {
		return tx, ty, true
	}
	return u.X + dx/d*stride, u.Y + dy/d*stride, true
}

func (b *Bot) StepToward(u Unit, tx, ty, stride float64) (float64, float64, bool) {
	dx, dy := tx-u.X, ty-u.Y
	d := math.Hypot(dx, dy)
	if d < 1e-6 {
		return u.X, u.Y, false
	}
	s := math.Min(stride, d)
	nx, ny := u.X+dx/d*s, u.Y+dy/d*s
	gx, gy := b.cellOf(u.X, u.Y)
	nxCell, nyCell := b.cellOf(nx, ny)
	if !b.slopeOK(gx, gy, nxCell, nyCell) {
		return u.X, u.Y, false
	}
	return nx, ny, true
}

func (b *Bot) clampWorld(x, y float64) (float64, float64) {
	c := float64(b.Cell)
	w := float64(b.Hello.World)
	if x < c {
		x = c
	}
	if y < c {
		y = c
	}
	if x > w-c {
		x = w - c
	}
	if y > w-c {
		y = w - c
	}
	return x, y
}

func (b *Bot) TypeOf(kind string) Type { return b.Hello.Rules.Types[kind] }

func (b *Bot) RangePx(kind string) float64 {
	t := b.TypeOf(kind)
	return t.Range * float64(b.Cell)
}

func (b *Bot) StridePx(kind string) float64 {
	t := b.TypeOf(kind)
	if t.Speed <= 0 {
		return 0
	}
	return t.Speed * float64(b.Cell) * 0.1 * 0.85
}

func (b *Bot) Own(t *Tick) []Unit {
	out := make([]Unit, 0, len(t.Units))
	for _, u := range t.Units {
		if u.Side == t.Side && u.HP > 0 {
			out = append(out, u)
		}
	}
	return out
}

func (b *Bot) Enemies(t *Tick) []Unit {
	out := make([]Unit, 0, len(t.Units))
	for _, u := range t.Units {
		if u.Side != t.Side && u.HP > 0 {
			out = append(out, u)
		}
	}
	return out
}

func (b *Bot) Mothership(t *Tick, side int) *Unit {
	for i := range t.Units {
		if t.Units[i].Side == side && t.Units[i].Kind == "MOTHERSHIP" && t.Units[i].HP > 0 {
			return &t.Units[i]
		}
	}
	return nil
}

func (b *Bot) NearestMine(t *Tick, u Unit) *MineState {
	var best *MineState
	bd := math.Inf(1)
	for i := range t.Mines {
		m := &t.Mines[i]
		if m.Amount <= 0 {
			continue
		}
		d := math.Hypot(m.X-u.X, m.Y-u.Y)
		if d < bd {
			bd, best = d, m
		}
	}
	return best
}

func (b *Bot) NearestEnemyWithin(t *Tick, u Unit, maxPx float64, structuresFirst bool) *Unit {
	var best *Unit
	bd := maxPx
	for i := range t.Units {
		e := &t.Units[i]
		if e.Side == u.Side || e.HP <= 0 {
			continue
		}
		if structuresFirst && !b.TypeOf(e.Kind).Structure {
			continue
		}
		d := math.Hypot(e.X-u.X, e.Y-u.Y)
		if d <= bd {
			bd, best = d, e
		}
	}
	return best
}

func (b *Bot) NearestEnemy(t *Tick, u Unit, maxPx float64) *Unit {
	var best *Unit
	bd := maxPx
	for i := range t.Units {
		e := &t.Units[i]
		if e.Side == u.Side || e.HP <= 0 {
			continue
		}
		d := math.Hypot(e.X-u.X, e.Y-u.Y)
		if d <= bd {
			bd, best = d, e
		}
	}
	return best
}

/* ---------------- 通用生产/研究 ---------------- */

func (b *Bot) CanProduce(t *Tick, kind string) bool {
	ty := b.TypeOf(kind)
	if ty.Cost <= 0 {
		return false
	}
	if t.Me.Pop+ty.Pop > t.Me.PopMax {
		return false
	}
	if len(t.Me.Queue)+len(t.Me.Active) >= 4 {
		return false
	}
	return t.Me.CU >= float64(ty.Cost)
}

func (b *Bot) NextTech(t *Tick, order []string) string {
	done := map[string]bool{}
	for _, id := range t.Me.TechDone {
		done[id] = true
	}
	for _, d := range t.Me.TechDoing {
		done[d.ID] = true
	}
	for _, id := range order {
		if done[id] {
			continue
		}
		return id
	}
	return ""
}

/* ---------------- 攻方 ---------------- */

func Attack(b *Bot, t *Tick) []Action {
	acts := make([]Action, 0, 12)
	if t.Turn == 0 {
		acts = append(acts,
			Action{Type: "mode", Mode: "neutral"},
			Action{Type: "alloc", Alloc: map[string]int{"produce": 40, "attack": 30, "research": 10, "move": 20}},
		)
	}
	buildOrder := []string{"LAV", "LAV", "AAV", "RAILGUN", "LAV", "AAV"}
	kind := buildOrder[b.buildIdx%len(buildOrder)]
	if b.CanProduce(t, kind) {
		acts = append(acts, Action{Type: "produce", Kind: kind})
		b.buildIdx++
	}
	if tech := b.NextTech(t, []string{"A1", "F1", "A2"}); tech != "" && t.Me.CU >= 80 {
		acts = append(acts, Action{Type: "research", Tech: tech})
	}
	enemyM := b.Mothership(t, 0)
	for _, u := range b.Own(t) {
		if u.Kind == "MOTHERSHIP" {
			if e := b.NearestEnemyWithin(t, u, b.RangePx(u.Kind)*0.95, false); e != nil {
				acts = append(acts, Action{ID: u.ID, Type: "attack", TargetID: e.ID})
			}
			continue
		}
		if u.Kind == "SALVAGER" {
			if m := b.NearestMine(t, u); m != nil {
				if f := b.FieldTo(m.X, m.Y); f != nil {
					if x, y, ok := b.StepAlong(u, f, b.StridePx(u.Kind)); ok {
						acts = append(acts, Action{ID: u.ID, Type: "move", X: x, Y: y})
						continue
					}
				}
			}
			continue
		}
		rng := b.RangePx(u.Kind)
		// 先打挡路的工事，再打射程内的其它敌人，最后推进母舰。
		if e := b.NearestEnemyWithin(t, u, rng*0.92, true); e != nil {
			acts = append(acts, Action{ID: u.ID, Type: "attack", TargetID: e.ID})
			continue
		}
		if e := b.NearestEnemyWithin(t, u, rng*0.95, false); e != nil {
			acts = append(acts, Action{ID: u.ID, Type: "attack", TargetID: e.ID})
			continue
		}
		if enemyM == nil {
			continue
		}
		if f := b.FieldTo(enemyM.X, enemyM.Y); f != nil {
			if x, y, ok := b.StepAlong(u, f, b.StridePx(u.Kind)); ok {
				acts = append(acts, Action{ID: u.ID, Type: "move", X: x, Y: y})
				continue
			}
		}
		// BFS 场可能把墙当可走格；卡住时直接朝最近工事/母舰蹭过去，
		// 进入射程后下一拍就会转成 attack。
		tgt := b.NearestEnemyWithin(t, u, rng*4, true)
		if tgt == nil {
			tgt = enemyM
		}
		if tgt != nil {
			if x, y, ok := b.StepToward(u, tgt.X, tgt.Y, b.StridePx(u.Kind)); ok {
				acts = append(acts, Action{ID: u.ID, Type: "move", X: x, Y: y})
			} else {
				acts = append(acts, Action{ID: u.ID, Type: "idle"})
			}
		}
	}
	return acts
}

/* ---------------- 守方 ---------------- */

type Fort struct {
	Kind string
	X, Y float64
}

func (b *Bot) fortPlan(t *Tick) []Fort {
	m := b.Mothership(t, t.Side)
	if m == nil {
		return nil
	}
	out := make([]Fort, 0, 24)
	// 先放 6 面墙（60° 间隔，r=130），再放 2 座炮（r=155），正好 6*1+2*3=12 点。
	// 墙留出缺口，攻方可以绕，但会被炮和守军覆盖；想强攻就得拆墙。
	add := func(kind string, x, y float64) {
		if x < float64(b.Cell) || y < float64(b.Cell) || x > float64(b.Hello.World-b.Cell) || y > float64(b.Hello.World-b.Cell) {
			return
		}
		out = append(out, Fort{Kind: kind, X: x, Y: y})
	}
	for i := 0; i < 8; i++ {
		a := float64(i) * math.Pi * 2 / 8
		add("WALL", m.X+math.Cos(a)*130, m.Y+math.Sin(a)*130)
	}
	for i := 0; i < 5; i++ {
		a := float64(i)*math.Pi*2/5 + math.Pi/5
		add("TURRET", m.X+math.Cos(a)*155, m.Y+math.Sin(a)*155)
	}
	// 预算有剩（部分候选被地形拒绝）时继续补墙。
	for i := 0; i < 10; i++ {
		a := float64(i)*math.Pi*2/10 + 0.1
		add("WALL", m.X+math.Cos(a)*110, m.Y+math.Sin(a)*110)
	}
	return out
}

func Defend(b *Bot, t *Tick) []Action {
	acts := make([]Action, 0, 12)
	if t.Turn == 0 {
		acts = append(acts,
			Action{Type: "mode", Mode: "neutral"},
			Action{Type: "alloc", Alloc: map[string]int{"produce": 35, "attack": 35, "research": 10, "move": 20}},
		)
	}
	// 布置阶段：每 tick 放一个工事；预算用完就停。
	if t.Me.Phase == "place" && t.Me.FortBudget > 0 {
		plan := b.fortPlan(t)
		idx := b.fortIdx
		if idx < t.Me.FortsPlaced {
			idx = t.Me.FortsPlaced
		}
		if idx < len(plan) {
			f := plan[idx]
			acts = append(acts, Action{Type: "fort", Kind: f.Kind, X: f.X, Y: f.Y})
			b.fortIdx = idx + 1
		}
	}
	defOrder := []string{"LAV", "AAV", "LAV", "AAV"}
	dk := defOrder[b.buildIdx%len(defOrder)]
	if b.CanProduce(t, dk) {
		acts = append(acts, Action{Type: "produce", Kind: dk})
		b.buildIdx++
	}
	if t.Me.WeaponsReady < 2 && t.Me.CU >= 120 {
		acts = append(acts, Action{Type: "weapon"})
	}
	if tech := b.NextTech(t, []string{"A1", "F1", "A2"}); tech != "" && t.Me.CU >= 80 {
		acts = append(acts, Action{Type: "research", Tech: tech})
	}
	m := b.Mothership(t, t.Side)
	if m == nil {
		return acts
	}
	for _, u := range b.Own(t) {
		if u.Kind == "MOTHERSHIP" {
			if e := b.NearestEnemyWithin(t, u, b.RangePx(u.Kind)*0.95, false); e != nil {
				acts = append(acts, Action{ID: u.ID, Type: "attack", TargetID: e.ID})
			}
			continue
		}
		if u.Kind == "SALVAGER" {
			// 守方采集车不出墙，留在母舰旁；经济靠基础收入 + 攻方占矿后的压力。
			continue
		}
		rng := b.RangePx(u.Kind)
		if e := b.NearestEnemyWithin(t, u, rng*0.95, false); e != nil {
			acts = append(acts, Action{ID: u.ID, Type: "attack", TargetID: e.ID})
			continue
		}
		// 防守站位：母舰周围 130 px，按单位 id 分散。
		a := float64(u.ID) * 2.399
		tx, ty := b.clampWorld(m.X+math.Cos(a)*130, m.Y+math.Sin(a)*130)
		if x, y, ok := b.StepToward(u, tx, ty, b.StridePx(u.Kind)); ok {
			acts = append(acts, Action{ID: u.ID, Type: "move", X: x, Y: y})
		} else {
			acts = append(acts, Action{ID: u.ID, Type: "idle"})
		}
	}
	return acts
}

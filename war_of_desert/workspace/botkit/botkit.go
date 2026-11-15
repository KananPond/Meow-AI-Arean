// Package botkit 是选手包自带的工具库：协议读写、地图解码、常用查询与一步寻路。
//
// 它不是「必须用」的框架。选手可以完全不用它，自己读写 stdin/stdout；
// 它只是把每个人都要重写一遍的东西（读 hello、解地图、朝目标走一步）先写好。
package botkit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"desertarena/internal/protocol"
	"desertarena/internal/sim"
)

// Type 是 hello.rules.types 里的单位属性。
// 注意：speed / range / vision 的单位是**世界格**（不是像素）；像素距离要除以 Hello.Cell。
type Type struct {
	HP     float64 `json:"hp"`
	Speed  float64 `json:"speed"`
	Range  float64 `json:"range"`
	Dmg    float64 `json:"dmg"`
	Rof    float64 `json:"rof"`
	Vision float64 `json:"vision"`
	Arc    string  `json:"arc"`
	Armor  string  `json:"armor"`
	Cost   float64 `json:"cost"`
	RU     int     `json:"ru"` // v1.1：高阶单位兼耗 RU（AAV 30 / RAILGUN 60 / ARTILLERY 250 / CARRIER 100）
	Build  float64 `json:"build"`
	Pop    int     `json:"pop"`
}

// Tech 是科技节点。
type Tech struct {
	Tree   string   `json:"tree"`
	Tier   int      `json:"tier"`
	Name   string   `json:"name"`
	Tick   float64  `json:"tick"`
	CU     float64  `json:"cu"`
	Req    []string `json:"req"`
	ReqAny []string `json:"reqAny"`
	Excl   []string `json:"excl"`
}

// Rules 是 hello.rules 中选手真正会用到的部分。
type Rules struct {
	Types   map[string]Type `json:"types"`
	Techs   map[string]Tech `json:"techs"`
	Economy struct {
		StartCU float64 `json:"startCu"`
		PopMax  int     `json:"popMax"`
	} `json:"economy"`
	Alloc struct {
		Keys []string `json:"keys"`
	} `json:"alloc"`
}

// Unit 是 tick 里的单位。
type Unit struct {
	ID    int     `json:"id"`
	Side  int     `json:"side"`
	Kind  string  `json:"kind"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	HP    float64 `json:"hp"`
	MaxHP float64 `json:"maxHp"`
	Mode  string  `json:"mode"`
}

// VP 是胜利点。
type VP struct {
	ID        int     `json:"id"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	Owner     int     `json:"owner"`
	Contested bool    `json:"contested"`
}

// Res 是资源点。
type Res struct {
	ID     int     `json:"id"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Kind   string  `json:"kind"` // "cu" | "ru"：cu 回 CU、ru 回 RU
	Amount int     `json:"amount"`
}

// SideScore 是双方公开比分。
type SideScore struct {
	VP     int `json:"vp"`
	Kills  int `json:"kills"`
	Losses int `json:"losses"`
}

// Me 是本方私有信息。
type Me struct {
	CU          float64        `json:"cu"`
	RU          float64        `json:"ru"` // v1.1：只能靠 SALVAGER 采 ru 节点获得
	Pop         int            `json:"pop"`
	PopMax      int            `json:"popMax"`
	Alloc       map[string]int `json:"alloc"`
	AllocTarget map[string]int `json:"allocTarget"`
	Mode        string         `json:"mode"`
	Slots       struct {
		Produce     int     `json:"produce"`
		Research    int     `json:"research"`
		AttackSlots float64 `json:"attackSlots"`
		AttackCap   int     `json:"attackCap"`
	} `json:"slots"`
	WeaponSlots  float64  `json:"weaponSlots"`
	WeaponsReady int      `json:"weaponsReady"`
	WeaponQueue  []int    `json:"weaponQueue"` // 各排队炮的剩余 tick（自查占用，避免白发被拒的 weapon）
	SpeedMul     float64  `json:"speedMul"`
	TechDone     []string `json:"techDone"`
	TechDoing    []struct {
		ID     string `json:"id"`
		Remain int    `json:"remain"`
	} `json:"techDoing"`
	Queue  []string `json:"queue"`
	Active []struct {
		Kind   string `json:"kind"`
		Remain int    `json:"remain"`
	} `json:"active"`
}

// State 是一行 tick。
type State struct {
	Type      string            `json:"type"`
	Turn      int               `json:"turn"`
	TickLimit int               `json:"tickLimit"`
	Side      int               `json:"side"`
	Seed      int               `json:"seed"`
	Units     []Unit            `json:"units"`
	VP        []VP              `json:"vp"`
	Res       []Res             `json:"res"`
	Score     [2]SideScore      `json:"score"`
	Me        Me                `json:"me"`
	Events    []json.RawMessage `json:"events"`
}

// Action 是一条动作。零值字段会被 omitempty 去掉。
type Action struct {
	ID       int            `json:"id,omitempty"`
	Type     string         `json:"type"`
	X        float64        `json:"x"`
	Y        float64        `json:"y"`
	TargetID int            `json:"targetId,omitempty"`
	ResID    *int           `json:"resId,omitempty"` // harvest 指派（v1.1 运输线）
	Kind     string         `json:"kind,omitempty"`
	Tech     string         `json:"tech,omitempty"`
	Mode     string         `json:"mode,omitempty"`
	Alloc    map[string]int `json:"alloc,omitempty"`
}

// Bot 持有 hello 与地图，是选手侧的只读环境。
type Bot struct {
	Hello  protocol.Hello
	Layers protocol.MapLayers
	Rules  Rules
	in     *bufio.Reader
	out    *bufio.Writer
	fields map[int]*Field
}

// Load 读 hello、解地图、回报 ready。任何一步失败都直接退出：
// 带病开跑只会得到一串故障，不如在开局就说清楚。
func Load() (*Bot, error) {
	in := bufio.NewReaderSize(os.Stdin, 1<<20)
	line, err := in.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("读 hello 失败: %w", err)
	}
	var h protocol.Hello
	if err := json.Unmarshal([]byte(line), &h); err != nil {
		return nil, fmt.Errorf("hello 不是合法 JSON: %w", err)
	}
	if h.ProtoVersion != protocol.ProtoVersion {
		return nil, fmt.Errorf("协议版本不匹配：裁判 v%d，本程序 v%d", h.ProtoVersion, protocol.ProtoVersion)
	}
	layers, err := protocol.DecodeMapLayers(h.Map)
	if err != nil {
		return nil, fmt.Errorf("地图解码失败: %w", err)
	}
	var rules Rules
	if len(h.Rules) > 0 {
		if err := json.Unmarshal(h.Rules, &rules); err != nil {
			return nil, fmt.Errorf("rules 解析失败: %w", err)
		}
	}
	b := &Bot{Hello: h, Layers: layers, Rules: rules, in: in, out: bufio.NewWriterSize(os.Stdout, 1<<16)}
	if _, err := b.out.WriteString("{\"ready\":true}\n"); err != nil {
		return nil, err
	}
	if err := b.out.Flush(); err != nil {
		return nil, err
	}
	return b, nil
}

// Next 读下一行。type=end 时返回 ok=false。
func (b *Bot) Next() (st State, ok bool, err error) {
	line, err := b.in.ReadString('\n')
	if err != nil {
		return st, false, err
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(line), &probe); err != nil {
		return st, false, err
	}
	if probe.Type == "end" {
		return st, false, nil
	}
	if err := json.Unmarshal([]byte(line), &st); err != nil {
		return st, false, err
	}
	return st, true, nil
}

// Send 交回本拍动作。
func (b *Bot) Send(actions []Action) error {
	raw, err := json.Marshal(map[string]any{"actions": actions})
	if err != nil {
		return err
	}
	if _, err := b.out.Write(append(raw, '\n')); err != nil {
		return err
	}
	return b.out.Flush()
}

// Run 是标准主循环：Load → 每拍调用 decide → Send。
func Run(decide func(*Bot, *State) []Action) error {
	b, err := Load()
	if err != nil {
		return err
	}
	for {
		st, ok, err := b.Next()
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := b.Send(decide(b, &st)); err != nil {
			return err
		}
	}
}

/* ------------------------------------------------------------------ */
/* 地图与移动                                                          */
/* ------------------------------------------------------------------ */

// CellOf 返回世界坐标所在格。
func (b *Bot) CellOf(x, y float64) (int, int) {
	gx := int(x / float64(b.Hello.Cell))
	gy := int(y / float64(b.Hello.Cell))
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

// Mountain 判断某格是否绝对不可通行（base=3）。
func (b *Bot) Mountain(x, y float64) bool {
	gx, gy := b.CellOf(x, y)
	return b.Layers.Base[gy*b.Hello.W+gx] == 3
}

// Walkable 判断落点是否在界内且不是山脉。
func (b *Bot) Walkable(x, y float64) bool {
	world := float64(b.Hello.World)
	if x < 0 || y < 0 || x > world || y > world {
		return false
	}
	return !b.Mountain(x, y)
}

// StepToward 返回朝目标走一步的落点。遇到山脉时按 ±23°/±46°/±69°/±92° 试探，
// 全被挡住就原地不动。它不保证最优路径，只保证「不会主动撞山」。
func (b *Bot) StepToward(u Unit, tx, ty, stride float64) (float64, float64) {
	dx, dy := tx-u.X, ty-u.Y
	d := math.Hypot(dx, dy)
	if d < 1 || stride <= 0 {
		return u.X, u.Y
	}
	if d < stride {
		stride = d
	}
	base := math.Atan2(dy, dx)
	for _, off := range []float64{0, 0.4, -0.4, 0.8, -0.8, 1.2, -1.2, 1.6, -1.6} {
		a := base + off
		nx, ny := u.X+math.Cos(a)*stride, u.Y+math.Sin(a)*stride
		if b.Walkable(nx, ny) {
			return nx, ny
		}
	}
	return u.X, u.Y
}

// Stride 是每拍最大位移（像素）。TYPES.speed 是世界格/s，时间基准取共享
// sim 包的 TICK_MS/1000（v1.1 = 0.05s；Plan_Beta §4.1：公式只此一份，禁止复制）。
func (b *Bot) Stride(u Unit) float64 {
	t, ok := b.Rules.Types[u.Kind]
	if !ok || t.Speed <= 0 {
		return 1
	}
	return math.Max(1, t.Speed*float64(b.Hello.Cell)*(sim.TickMS/1000.0))
}

// Own 返回本方存活单位。
func Own(st *State, side int) []Unit {
	out := make([]Unit, 0, len(st.Units))
	for _, u := range st.Units {
		if u.Side == side && u.HP > 0 {
			out = append(out, u)
		}
	}
	return out
}

// Enemies 返回可见敌方单位（视野关闭时即全图）。
func Enemies(st *State, side int) []Unit {
	out := make([]Unit, 0, len(st.Units))
	for _, u := range st.Units {
		if u.Side != side && u.HP > 0 {
			out = append(out, u)
		}
	}
	return out
}

// Nearest 返回离 (x,y) 最近的单位，没有则 nil。
func Nearest(list []Unit, x, y float64) *Unit {
	var best *Unit
	bd := math.Inf(1)
	for i := range list {
		d := math.Hypot(list[i].X-x, list[i].Y-y)
		if d < bd {
			bd = d
			best = &list[i]
		}
	}
	return best
}

// NearestVP 返回最近的目标点；preferNeutral=true 时优先未占领/争夺中的点。
func NearestVP(st *State, x, y float64, side int, preferNeutral bool) *VP {
	var best *VP
	bd := math.Inf(1)
	for i := range st.VP {
		v := &st.VP[i]
		if preferNeutral && v.Owner == side && !v.Contested {
			continue
		}
		d := math.Hypot(v.X-x, v.Y-y)
		if d < bd {
			bd = d
			best = v
		}
	}
	if best == nil && preferNeutral {
		return NearestVP(st, x, y, side, false)
	}
	return best
}

// NearestRes 返回最近资源点。
func NearestRes(st *State, x, y float64) *Res {
	var best *Res
	bd := math.Inf(1)
	for i := range st.Res {
		d := math.Hypot(st.Res[i].X-x, st.Res[i].Y-y)
		if d < bd {
			bd = d
			best = &st.Res[i]
		}
	}
	return best
}

// NearestResKind 返回指定类型（"cu" | "ru"）的最近资源点；该类型已采空时回退任意最近点。
func NearestResKind(st *State, x, y float64, kind string) *Res {
	var best *Res
	bd := math.Inf(1)
	for i := range st.Res {
		if st.Res[i].Kind != kind || st.Res[i].Amount <= 0 {
			continue
		}
		d := math.Hypot(st.Res[i].X-x, st.Res[i].Y-y)
		if d < bd {
			bd = d
			best = &st.Res[i]
		}
	}
	if best == nil {
		return NearestRes(st, x, y)
	}
	return best
}

// HomeVP 返回离本方出生点最近的 VP，用作母舰驻守点。
func (b *Bot) HomeVP(st *State, side int) *VP {
	if side < 0 || side >= len(b.Hello.Spawns) {
		return nil
	}
	sx := float64(b.Hello.Spawns[side][0]*b.Hello.Cell) + float64(b.Hello.Cell)/2
	sy := float64(b.Hello.Spawns[side][1]*b.Hello.Cell) + float64(b.Hello.Cell)/2
	return NearestVP(st, sx, sy, side, false)
}

/* ------------------------------------------------------------------ */
/* 流场寻路                                                            */
/* ------------------------------------------------------------------ */

// Field 是一张「到某个目标格的最短步数」图。目标固定（VP / 资源点）时，
// 一张图全队共用，比每台单位每拍各跑一次 A* 便宜得多。
type Field struct {
	W, H int
	Dist []int32 // -1 表示不可达
}

// FieldTo 生成（或复用）到目标世界坐标的流场。目标格不可达时返回 nil。
func (b *Bot) FieldTo(x, y float64) *Field {
	gx, gy := b.CellOf(x, y)
	key := gy*b.Hello.W + gx
	if b.fields == nil {
		b.fields = map[int]*Field{}
	}
	if f, ok := b.fields[key]; ok {
		return f
	}
	w, h := b.Hello.W, b.Hello.H
	dist := make([]int32, w*h)
	for i := range dist {
		dist[i] = -1
	}
	q := make([]int32, 0, w*h)
	start := int32(key)
	dist[start] = 0
	q = append(q, start)
	for head := 0; head < len(q); head++ {
		cur := q[head]
		cx, cy := int(cur)%w, int(cur)/w
		d := dist[cur]
		for _, n := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := cx+n[0], cy+n[1]
			if nx < 0 || ny < 0 || nx >= w || ny >= h {
				continue
			}
			ni := ny*w + nx
			if dist[ni] != -1 || b.Layers.Base[ni] == 3 {
				continue
			}
			dist[ni] = d + 1
			q = append(q, int32(ni))
		}
	}
	f := &Field{W: w, H: h, Dist: dist}
	b.fields[key] = f
	return f
}

// StepAlong 沿着流场走一步。到达目标格附近时返回 ok=false，调用方应改为待命。
func (b *Bot) StepAlong(u Unit, f *Field, stride float64) (float64, float64, bool) {
	if f == nil {
		return u.X, u.Y, false
	}
	gx, gy := b.CellOf(u.X, u.Y)
	cur := gy*f.W + gx
	if f.Dist[cur] <= 0 {
		return u.X, u.Y, false
	}
	best, bx, by := f.Dist[cur], gx, gy
	for _, n := range [8][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}, {1, 1}, {1, -1}, {-1, 1}, {-1, -1}} {
		nx, ny := gx+n[0], gy+n[1]
		if nx < 0 || ny < 0 || nx >= f.W || ny >= f.H {
			continue
		}
		ni := ny*f.W + nx
		if f.Dist[ni] < 0 {
			continue
		}
		if n[0] != 0 && n[1] != 0 {
			// 不允许斜穿两个山格
			if b.Layers.Base[gy*f.W+nx] == 3 || b.Layers.Base[ny*f.W+gx] == 3 {
				continue
			}
		}
		if f.Dist[ni] < best {
			best, bx, by = f.Dist[ni], nx, ny
		}
	}
	tx := (float64(bx) + 0.5) * float64(b.Hello.Cell)
	ty := (float64(by) + 0.5) * float64(b.Hello.Cell)
	dx, dy := tx-u.X, ty-u.Y
	d := math.Hypot(dx, dy)
	if d < 0.5 {
		return u.X, u.Y, false
	}
	if d > stride {
		tx, ty = u.X+dx/d*stride, u.Y+dy/d*stride
	}
	// 最后一道保险：落点仍是山就原地不动
	if !b.Walkable(tx, ty) {
		return u.X, u.Y, false
	}
	return tx, ty, true
}

// EnemyVP 返回离敌方出生点最近的 VP，用于「压进去打」的策略。
func (b *Bot) EnemyVP(st *State, side int) *VP {
	enemy := 1 - side
	if enemy < 0 || enemy >= len(b.Hello.Spawns) {
		return nil
	}
	ex := float64(b.Hello.Spawns[enemy][0]*b.Hello.Cell) + float64(b.Hello.Cell)/2
	ey := float64(b.Hello.Spawns[enemy][1]*b.Hello.Cell) + float64(b.Hello.Cell)/2
	return NearestVP(st, ex, ey, side, false)
}

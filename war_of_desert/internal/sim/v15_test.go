package sim

import (
	"math"
	"testing"
)

func newV15Game(t *testing.T, seed int) *Game {
	t.Helper()
	g, err := NewGame(seed, GameCfg{Generator: "minecraft", TickLimit: 1200, Ruleset: RulesetV15})
	if err != nil {
		t.Fatalf("NewGame v15: %v", err)
	}
	return g
}

// 遗产运输 + 插旗 + 拔旗掉落的主链路。
func TestV15RelicFlagChain(t *testing.T) {
	g := newV15Game(t, fairnessSeeds[0])
	if g.V15 == nil || len(g.V15.Relics) < 1 {
		t.Fatal("v15 未初始化遗产")
	}
	vp := g.Vps[0]
	var sv *Unit
	for _, u := range g.Units {
		if u.Side == 0 && u.Kind == "SALVAGER" {
			sv = u
			break
		}
	}
	if sv == nil {
		t.Fatal("v15 开局没有采矿机")
	}
	// 直接把采矿机放到 VP 中心，省掉寻路。
	sv.X, sv.Y = vp.X, vp.Y
	sv.PX, sv.PY = vp.X, vp.Y
	for i := 0; i < FlagPlaceTicks+5; i++ {
		g.Step([2][]Action{nil, nil})
	}
	f := g.flagAtVP(vp.ID)
	if f == nil || f.Side != 0 {
		t.Fatalf("插旗失败：flag=%v", f)
	}
	if g.Sides[0].VS.Flags != FlagValue {
		t.Fatalf("插旗分 %d 应为 %d", g.Sides[0].VS.Flags, FlagValue)
	}

	// 把一件遗产挪到采矿机脚下并指派运输。
	r := g.V15.Relics[0]
	r.X, r.Y, r.State = sv.X, sv.Y, "ground"
	rid := r.ID
	sv.Relic = &rid
	sv.RState = "toRelic"
	sv.RelicTimer = 0
	for i := 0; i < RelicLoadTicks+RelicDropTicks+20; i++ {
		g.Step([2][]Action{nil, nil})
	}
	if r.State != "stored" {
		t.Fatalf("遗产未入库：state=%s rstate=%s", r.State, sv.RState)
	}
	if g.Sides[0].VS.Relics != RelicValue {
		t.Fatalf("遗产分 %d 应为 %d", g.Sides[0].VS.Relics, RelicValue)
	}

	// 拔旗：守方扣旗分 + 该点库存遗产分，遗产掉回地面，攻方得 50%。
	g.destroyFlag(f, 1)
	if g.Sides[0].VS.Flags != 0 || g.Sides[0].VS.Relics != 0 {
		t.Fatalf("拔旗后守方账本 %+v 应为零", g.Sides[0].VS)
	}
	if g.Sides[1].VS.Flags != int(math.Floor(FlagValue*FlagStolenShare)) {
		t.Fatalf("拔旗方得分 %d 应为 %d", g.Sides[1].VS.Flags, int(math.Floor(FlagValue*FlagStolenShare)))
	}
	if r.State != "ground" {
		t.Fatalf("拔旗后遗产应为无主，实际 %s", r.State)
	}
}

// 等级：只靠击杀经验，5 级封顶，精度加成不超过科技增益的 60%。
func TestV15Veterancy(t *testing.T) {
	g := newV15Game(t, fairnessSeeds[0])
	var lav *Unit
	for _, u := range g.Units {
		if u.Side == 0 && u.Kind == "LAV" {
			lav = u
			break
		}
	}
	if lav == nil {
		t.Fatal("没有 LAV")
	}
	g.Sides[0].Mods.CEP = 0.37
	// 杀 9 台 LAV 的量（每台 1 XP）→ light 档 9 XP = 5 级。
	for i := 0; i < 9; i++ {
		g.addKillXP(lav, "LAV")
	}
	if lav.Level != 5 {
		t.Fatalf("等级 %d 应为 5", lav.Level)
	}
	if m := g.vetCEPMul(lav); math.Abs(m-(1-0.6*0.37)) > 1e-9 {
		t.Fatalf("5 级精度系数 %v 应为 %v", m, 1-0.6*0.37)
	}
	if g.vetVPMul(lav) < 1.5 {
		t.Fatalf("5 级击杀分倍率 %v 过低", g.vetVPMul(lav))
	}
}

// v15 射界对准后必须能开火（防 CEP/炮塔改造把战斗彻底锁死）。
func TestV15Fire(t *testing.T) {
	g := newV15Game(t, fairnessSeeds[0])
	var a, b *Unit
	for _, u := range g.Units {
		if u.Kind == "LAV" && u.Side == 0 && a == nil {
			a = u
		}
		if u.Kind == "LAV" && u.Side == 1 && b == nil {
			b = u
		}
	}
	if a == nil || b == nil {
		t.Fatal("缺少 LAV")
	}
	// 清出一块平地，避免弹道被山挡。
	for y := 90; y < 110; y++ {
		for x := 90; x < 110; x++ {
			g.Map.Base[Ci(x, y)] = 0
			g.Map.Elev[Ci(x, y)] = 0
		}
	}
	a.X, a.Y, a.PX, a.PY = 600, 600, 600, 600
	b.X, b.Y, b.PX, b.PY = 660, 600, 660, 600
	a.Facing, a.Turret = 0, 0
	b.Facing, b.Turret = 180, 180
	aid, bid := a.ID, b.ID
	for i := 0; i < 40; i++ {
		g.Step([2][]Action{
			{{Type: "attack", ID: &aid, TargetID: &bid}},
			{{Type: "attack", ID: &bid, TargetID: &aid}},
		})
	}
	if g.Sides[0].Shots+g.Sides[1].Shots == 0 {
		t.Fatalf("对准后仍未开火：a(facing=%.1f turret=%.1f) b(facing=%.1f turret=%.1f)",
			a.Facing, a.Turret, b.Facing, b.Turret)
	}
}

// 攻坚兵必须能对母舰造成伤害（证明斩舰路径在规则层是通的）。
func TestV15RailgunVsMothership(t *testing.T) {
	g := newV15Game(t, fairnessSeeds[0])
	ms := g.Mothership(1)
	if ms == nil {
		t.Fatal("没有敌方母舰")
	}
	// 在母舰射程外放一台 railgun（38 格 vs 母舰 7 格）。
	x, y := ms.X-200, ms.Y
	if x < 10 {
		x = ms.X + 200
	}
	rg := g.spawn(0, "RAILGUN", x, y)
	rg.Level = 1
	rg.Facing, rg.Turret = 0, 0
	rg.Mode = "obey"
	id, mid := rg.ID, ms.ID
	before := ms.HP
	for i := 0; i < 400; i++ {
		g.Step([2][]Action{
			{{Type: "attack", ID: &id, TargetID: &mid}},
			nil,
		})
		if g.Done() {
			break
		}
	}
	if ms.HP >= before {
		t.Fatalf("railgun 未对母舰造成伤害：%.0f → %.0f (shots=%d)", before, ms.HP, g.Sides[0].Shots)
	}
	t.Logf("母舰 HP %.0f → %.0f，railgun shots=%d", before, ms.HP, g.Sides[0].Shots)
}

// 拔旗必须走「打旗 HP」而不是站圈读条。
func TestV15FlagShot(t *testing.T) {
	g := newV15Game(t, fairnessSeeds[0])
	vp := g.Vps[0]
	f := g.placeFlag(1, vp) // 敌方旗
	var lav *Unit
	for _, u := range g.Units {
		if u.Side == 0 && u.Kind == "LAV" {
			lav = u
			break
		}
	}
	if lav == nil {
		t.Fatal("没有 LAV")
	}
	lav.X, lav.Y, lav.PX, lav.PY = vp.X-40, vp.Y, vp.X-40, vp.Y
	lav.Facing, lav.Turret = 0, 0
	lav.Mode = "obey"
	id, fid := lav.ID, f.ID
	for i := 0; i < 400 && g.flagByID(fid) != nil; i++ {
		g.Step([2][]Action{
			{{Type: "attackFlag", ID: &id, FlagID: &fid}},
			nil,
		})
	}
	if g.flagByID(fid) != nil {
		t.Fatalf("旗未被击毁：hp=%.0f shots=%d", f.HP, g.Sides[0].Shots)
	}
	if g.Sides[1].VS.Flags != 0 {
		t.Fatalf("守方旗分 %d 应为 0", g.Sides[1].VS.Flags)
	}
	if g.Sides[0].VS.Flags != int(math.Floor(FlagValue*FlagStolenShare)) {
		t.Fatalf("攻方旗分 %d 应为 %d", g.Sides[0].VS.Flags, int(math.Floor(FlagValue*FlagStolenShare)))
	}
}

func TestV15Smoke(t *testing.T) {
	g := newV15Game(t, fairnessSeeds[1])
	for i := 0; i < 200; i++ {
		g.Step([2][]Action{nil, nil})
	}
	f := g.BuildFrame()
	if len(f.Relics) == 0 || len(f.VS) != 2 {
		t.Fatalf("v15 帧缺遗产/胜利分：relics=%d vs=%d", len(f.Relics), len(f.VS))
	}
}

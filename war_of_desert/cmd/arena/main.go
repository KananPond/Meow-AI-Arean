// Command arena 是 Desert Arena 正式赛的唯一入口。
//
// 版本：v1.2（Desert Arena 正式赛执行链）。
// 功能：build / plan / run / rate / check / selftest / season 全流程；
// 引擎双轨（oracle 快照 | Go 原生 enginehost）。
//
//	arena build  编译选手包并封存源码哈希
//	arena plan   冻结赛程
//	arena run    按冻结赛程跑完所有对局
//	arena rate   出 BT 实力值、95% 区间、计分卡加权总分
//	arena check  单场自检（选手包默认用它）
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"desertarena/internal/contest"
	"desertarena/internal/engine"
	"desertarena/internal/judge"
	"desertarena/internal/protocol"
	"desertarena/internal/rating"
	"desertarena/internal/schedule"
	"desertarena/internal/sim"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "build":
		err = cmdBuild(args)
	case "plan":
		err = cmdPlan(args)
	case "run":
		err = cmdRun(args)
	case "rate":
		err = cmdRate(args)
	case "check":
		err = cmdCheck(args)
	case "selftest":
		err = cmdSelftest(args)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "错误："+err.Error())
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Desert Arena 正式赛

  arena build  [-config tournament/formal.json]
  arena plan   [-config tournament/formal.json]
	arena run    [-config tournament/formal.json] [-resume] [-limit N]
	arena rate   [-config tournament/formal.json]
  arena check  [-a <bot.exe>] [-b <bot.exe>] [-seed N] [-config ...]
  arena selftest [-config ...]   验证超时 kill / 崩溃重启 / 非法结构红牌

所有相对路径都以 war_of_desert/ 为基准。请在 war_of_desert/ 下执行。
`)
	os.Exit(2)
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

/* ------------------------------------------------------------------ */
/* build                                                               */
/* ------------------------------------------------------------------ */

type sealRecord struct {
	GeneratedAt string            `json:"generatedAt"`
	Shared      map[string]string `json:"shared"`
	Entries     []sealEntry       `json:"entries"`
	Note        string            `json:"note"`
}

type sealEntry struct {
	Name   string            `json:"name"`
	Source string            `json:"source"`
	Bin    string            `json:"bin"`
	Files  map[string]string `json:"files"`
}

func cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	cfgPath := fs.String("config", "tournament/formal.json", "赛事配置")
	_ = fs.Parse(args)

	cfg, root, err := contest.Load(*cfgPath)
	if err != nil {
		return err
	}
	rec := sealRecord{GeneratedAt: time.Now().Format(time.RFC3339), Note: "封存源码哈希；bin 为编译产物，可随时由 source 重建；shared 为参考 Bot 共用的策略库"}
	rec.Shared = map[string]string{}
	for _, shared := range []string{"workspace/botkit", "workspace/refbot"} {
		files, err := hashTree(filepath.Join(root, filepath.FromSlash(shared)))
		if err != nil {
			return err
		}
		for f, h := range files {
			rec.Shared[shared+"/"+f] = h
		}
	}
	for _, e := range cfg.Entries {
		src := filepath.Join(root, filepath.FromSlash(e.Source))
		binRel := e.Bin
		if binRel == "" {
			binRel = filepath.ToSlash(filepath.Join("bots", "bin", e.Name+exeSuffix()))
		}
		bin := filepath.Join(root, filepath.FromSlash(binRel))
		if err := os.MkdirAll(filepath.Dir(bin), 0o755); err != nil {
			return err
		}
		cmd := exec.Command("go", "build", "-o", bin, "./"+filepath.ToSlash(e.Source))
		cmd.Dir = root
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("build %-12s %s → %s\n", e.Name, e.Source, binRel)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("编译 %s 失败: %w", e.Name, err)
		}
		files, err := hashTree(src)
		if err != nil {
			return err
		}
		rec.Entries = append(rec.Entries, sealEntry{Name: e.Name, Source: e.Source, Bin: binRel, Files: files})
	}
	raw, _ := json.MarshalIndent(rec, "", "  ")
	sealPath := filepath.Join(root, "bots", "封存记录.json")
	if err := os.MkdirAll(filepath.Dir(sealPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(sealPath, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Println("封存记录 → bots/封存记录.json（" + fmt.Sprint(len(rec.Entries)) + " 名选手）")
	return nil
}

func hashTree(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(raw)
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = hex.EncodeToString(sum[:])
		return nil
	})
	return out, err
}

/* ------------------------------------------------------------------ */
/* plan                                                                */
/* ------------------------------------------------------------------ */

func cmdPlan(args []string) error {
	fs := flag.NewFlagSet("plan", flag.ExitOnError)
	cfgPath := fs.String("config", "tournament/formal.json", "赛事配置")
	_ = fs.Parse(args)

	cfg, root, err := contest.Load(*cfgPath)
	if err != nil {
		return err
	}
	outDir := cfg.OutDir(root)
	plan := schedule.Build(cfg.Entries, cfg.Schedule.Seeds, cfg.Schedule.Mirror, cfg.ScheduleConfig())
	plan.RulesVersion = sim.RulesVersion
	plan.RulesHash = sim.RulesHash(cfg.Rules.TickLimit, sim.NormalizeVision(sim.Vision{Enabled: cfg.Rules.Vision, Fog: cfg.Rules.Vision, LOS: cfg.Rules.Vision}))
	plan.Engine = cfg.Engine.Kind
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	if err := schedule.Save(filepath.Join(outDir, "赛程.json"), plan); err != nil {
		return err
	}
	fmt.Printf("plan → %s（%d 场对阵 × %d 局，%d 名选手，seed %v）\n",
		filepath.Join(outDir, "赛程.json"), len(plan.Matches), gamesPerMatch(cfg.Schedule.Mirror), len(plan.Entries), cfg.Schedule.Seeds)
	return nil
}

func gamesPerMatch(mirror bool) int {
	if mirror {
		return 2
	}
	return 1
}

/* ------------------------------------------------------------------ */
/* run                                                                 */
/* ------------------------------------------------------------------ */

type gameLine struct {
	Match      int         `json:"match"`
	Seed       int         `json:"seed"`
	Game       int         `json:"game"`
	Swap       bool        `json:"swap"`
	Seats      []string    `json:"seats"`
	Winner     string      `json:"winner"`
	WinnerSeat int         `json:"winnerSeat"`
	Level      int         `json:"level"`
	Reason     string      `json:"reason"`
	Turn       int         `json:"turn"`
	VP         []int       `json:"vp"`
	Totals     []float64   `json:"totals"`
	Fields     [][]float64 `json:"fields"`
	RulesHash  string      `json:"rulesHash"`
	Faults     []int       `json:"faults"`
	VerifyOK   *bool       `json:"verifyOk,omitempty"`
	DurationMS int64       `json:"durationMs"`
}

// resolveOutDir 解析结果目录：默认取配置 output，-dir 可覆盖；强制留在仓库根内，
// 防止 `-dir ../…` 把赛果写到仓库之外。
func resolveOutDir(root, dirFlag, def string) (string, error) {
	outDir := filepath.Join(root, filepath.FromSlash(def))
	if dirFlag != "" {
		outDir = filepath.Join(root, filepath.FromSlash(dirFlag))
	}
	absDir, err := filepath.Abs(outDir)
	if err != nil {
		return "", err
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if absDir != absRoot && !strings.HasPrefix(absDir, absRoot+string(os.PathSeparator)) {
		return "", fmt.Errorf("结果目录必须在仓库目录内: %s", dirFlag)
	}
	return outDir, nil
}

func cmdRun(args []string) error {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfgPath := fs.String("config", "tournament/formal.json", "赛事配置")
	resume := fs.Bool("resume", false, "跳过 结果.jsonl 中已完成的对阵")
	limit := fs.Int("limit", 0, "最多再跑 N 场（0 = 不限）")
	skipSeal := fs.Bool("skip-seal", false, "跳过封存校验（仅限演练，正式赛禁用）")
	_ = fs.Parse(args)

	cfg, root, err := contest.Load(*cfgPath)
	if err != nil {
		return err
	}
	// 开赛硬门槛：源码哈希必须与封存记录一致，否则赛后无法自证跑的是收卷那份代码。
	if !*skipSeal {
		seal, err := contest.LoadSeal(root)
		if err != nil {
			return err
		}
		names := make([]string, 0, len(cfg.Entries))
		for _, e := range cfg.Entries {
			names = append(names, e.Name)
		}
		bad := append(seal.Verify(root), seal.CheckEntryCoverage(names)...)
		if len(bad) > 0 {
			return fmt.Errorf("封存校验未通过（先跑 arena build）:\n  %s", strings.Join(bad, "\n  "))
		}
	}
	outDir, err := resolveOutDir(root, "", cfg.Output)
	if err != nil {
		return err
	}
	plan, err := schedule.Load(filepath.Join(outDir, "赛程.json"))
	if err != nil {
		return fmt.Errorf("读不到冻结赛程（先跑 arena plan）: %w", err)
	}
	byName := map[string]schedule.Entry{}
	for _, e := range plan.Entries {
		byName[e.Name] = e
	}
	resultsPath := filepath.Join(outDir, "结果.jsonl")
	done := map[int]bool{}
	if *resume {
		if f, err := os.Open(resultsPath); err == nil {
			sc := bufio.NewScanner(f)
			sc.Buffer(make([]byte, 1<<20), 1<<20)
			for sc.Scan() {
				var l gameLine
				if json.Unmarshal(sc.Bytes(), &l) == nil {
					done[l.Match] = true
				}
			}
			f.Close()
		}
	}
	var out *os.File
	if *resume {
		out, err = os.OpenFile(resultsPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	} else {
		out, err = os.Create(resultsPath)
	}
	if err != nil {
		return err
	}
	defer out.Close()
	w := bufio.NewWriter(out)
	defer w.Flush()

	replayDir := filepath.Join(outDir, "回放")
	if err := os.MkdirAll(replayDir, 0o755); err != nil {
		return err
	}

	hostScript, err := cfg.EngineHostPath(root)
	if err != nil {
		return err
	}
	judgeCfg := judge.Config{
		HostScript: hostScript,
		Rules: engine.HostConfig{
			Generator: cfg.Rules.Generator,
			TickLimit: cfg.Rules.TickLimit,
			TickMs:    cfg.Rules.TickMs,
			DecideMs:  cfg.Rules.DecideMs,
			InitMs:    cfg.Rules.InitMs,
			Vision:    cfg.Rules.Vision,
		},
		Mirror:     plan.Mirror,
		MaxStrikes: plan.Config.MaxStrikes,
		StrikeMode: plan.Config.StrikeMode,
		Verify:     cfg.Verify,
	}

	ran := 0
	written := 0
	started := time.Now()
	for _, m := range plan.Matches {
		if *resume && done[m.ID] {
			continue
		}
		if *limit > 0 && ran >= *limit {
			break
		}
		ea, ok := byName[m.A]
		if !ok {
			return fmt.Errorf("赛程里的选手 %s 不在 entries 中", m.A)
		}
		eb, ok := byName[m.B]
		if !ok {
			return fmt.Errorf("赛程里的选手 %s 不在 entries 中", m.B)
		}
		specA := judge.BotSpec{Name: ea.Name, Path: filepath.Join(root, filepath.FromSlash(ea.Bin))}
		specB := judge.BotSpec{Name: eb.Name, Path: filepath.Join(root, filepath.FromSlash(eb.Bin))}
		rec, err := judge.RunMatch(m.ID, m.Seed, specA, specB, judgeCfg)
		if err != nil {
			return fmt.Errorf("第 %d 场（%s vs %s）失败: %w", m.ID, m.A, m.B, err)
		}
		writeReplay(replayDir, rec)
		for _, g := range rec.Games {
			line := gameLine{
				Match: m.ID, Seed: g.Seed, Game: g.Index, Swap: g.Swap, Seats: g.Seats,
				Winner: protocol.WinnerName(g.Result.Winner, g.Seats), WinnerSeat: g.Result.Winner,
				Level: g.Result.Level, Reason: g.Result.Reason, Turn: g.Result.Turn,
				VP: g.Result.VP, RulesHash: rec.RulesHash, Faults: []int{len(g.Faults[0]), len(g.Faults[1])},
				DurationMS: g.DurationMS,
			}
			for _, c := range g.Result.Cards {
				line.Totals = append(line.Totals, round2(c.Total))
				line.Fields = append(line.Fields, c.Fields)
			}
			if len(g.Verify) > 0 {
				var v struct {
					OK bool `json:"ok"`
				}
				if json.Unmarshal(g.Verify, &v) == nil {
					line.VerifyOK = &v.OK
				}
			}
			raw, _ := json.Marshal(line)
			w.Write(append(raw, '\n'))
			written++
		}
		ran++
		fmt.Printf("[%d/%d] %-4d %-12s vs %-12s  seed=%d  %s  %s\n",
			len(done)+ran, len(plan.Matches), m.ID, m.A, m.B, rec.Seed,
			scoreText(rec), durationText(rec.DurationMS))
	}
	fmt.Printf("run 完成：%d 场 / %d 局，用时 %s，结果 → %s\n",
		ran, written, time.Since(started).Round(time.Second), resultsPath)
	return nil
}

func writeReplay(dir string, rec *judge.MatchRecord) {
	payload := map[string]any{
		"version": 1,
		"match":   rec.Index,
		"seed":    rec.Seed,
		"header":  rec.Header,
		"games":   rec.Games,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(dir, fmt.Sprintf("match-%03d.json", rec.Index)), raw, 0o644)
}

func scoreText(rec *judge.MatchRecord) string {
	parts := make([]string, 0, len(rec.Games))
	for _, g := range rec.Games {
		w := g.Result.Winner
		mark := "平"
		if w == 0 {
			mark = "左胜"
		} else if w == 1 {
			mark = "右胜"
		}
		parts = append(parts, fmt.Sprintf("%s(L%d)", mark, g.Result.Level))
	}
	return strings.Join(parts, " ")
}

func durationText(ms int64) string {
	return (time.Duration(ms) * time.Millisecond).Round(100 * time.Millisecond).String()
}

func round2(v float64) float64 { return float64(int64(v*100+0.5)) / 100 }

/* ------------------------------------------------------------------ */
/* rate                                                                */
/* ------------------------------------------------------------------ */

type standing struct {
	Name   string
	Played int
	Wins   int
	Draws  int
	Losses int
	Points float64
	Total  float64
	Faults int
}

func cmdRate(args []string) error {
	fs := flag.NewFlagSet("rate", flag.ExitOnError)
	cfgPath := fs.String("config", "tournament/formal.json", "赛事配置")
	_ = fs.Parse(args)

	cfg, root, err := contest.Load(*cfgPath)
	if err != nil {
		return err
	}
	outDir, err := resolveOutDir(root, "", cfg.Output)
	if err != nil {
		return err
	}
	plan, err := schedule.Load(filepath.Join(outDir, "赛程.json"))
	if err != nil {
		return err
	}
	f, err := os.Open(filepath.Join(outDir, "结果.jsonl"))
	if err != nil {
		return err
	}
	defer f.Close()
	var lines []gameLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var l gameLine
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			return fmt.Errorf("结果.jsonl 某行非法: %w", err)
		}
		lines = append(lines, l)
	}
	if len(lines) == 0 {
		return fmt.Errorf("结果.jsonl 为空，先跑 arena run")
	}

	// 计分域数从结果行推导（v1.0 八领域 / v1.1 七领域），标签不再硬编码。
	domains := 0
	for _, l := range lines {
		if len(l.Fields) > 0 {
			domains = len(l.Fields[0])
			break
		}
	}
	if domains == 0 {
		domains = 7
	}
	domainLabel := fmt.Sprintf("%d领域", domains)

	table := map[string]*standing{}
	get := func(n string) *standing {
		if _, ok := table[n]; !ok {
			table[n] = &standing{Name: n}
		}
		return table[n]
	}
	for _, e := range plan.Entries {
		get(e.Name)
	}
	var btGames []rating.Game
	for _, l := range lines {
		a, b := l.Seats[0], l.Seats[1]
		score := 0.5
		if l.WinnerSeat == 0 {
			score = 1
		} else if l.WinnerSeat == 1 {
			score = 0
		}
		btGames = append(btGames, rating.Game{A: a, B: b, Score: score})
		sa, sb := get(a), get(b)
		sa.Played++
		sb.Played++
		sa.Points += score
		sb.Points += 1 - score
		switch l.WinnerSeat {
		case 0:
			sa.Wins++
			sb.Losses++
		case 1:
			sb.Wins++
			sa.Losses++
		default:
			sa.Draws++
			sb.Draws++
		}
		sa.Faults += l.Faults[0]
		sb.Faults += l.Faults[1]
		if len(l.Totals) == 2 {
			sa.Total += l.Totals[0]
			sb.Total += l.Totals[1]
		}
	}

	anchor := plan.Entries[0].Name
	bt := rating.Fit(btGames, anchor, cfg.Bootstrap(), 20261109)
	rawBT, _ := json.MarshalIndent(bt, "", "  ")
	if err := os.WriteFile(filepath.Join(outDir, "成绩.json"), append(rawBT, '\n'), 0o644); err != nil {
		return err
	}

	type row struct {
		Name      string
		R         float64
		Lo        float64
		Hi        float64
		Intervals bool
		St        *standing
	}
	var rows []row
	for name := range table {
		r := row{Name: name, R: bt.Ratings[name], St: table[name]}
		if iv, ok := bt.Intervals[name]; ok {
			r.Lo, r.Hi, r.Intervals = iv[0], iv[1], true
		}
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].R != rows[j].R {
			return rows[i].R > rows[j].R
		}
		return rows[i].St.Points > rows[j].St.Points
	})

	var b strings.Builder
	fmt.Fprintf(&b, "# %s · 正式赛成绩\n\n", cfg.Name)
	fmt.Fprintf(&b, "> 赛季：%s ｜ 规则 %s ｜ 引擎 %s ｜ 规则指纹：%s ｜ 局数：%d ｜ 基准 seed：%v（逐场按序递增）\n",
		cfg.Season, versionOr(plan.RulesVersion), valueOr(plan.Engine, cfg.Engine.Kind), lines[0].RulesHash, len(lines), cfg.Schedule.Seeds)
	fmt.Fprintf(&b, "> 评分口径：%s；%s。\n\n", bt.Method, bt.Scale)
	fmt.Fprintf(&b, "| 名次 | 选手 | 实力值 | 95%% 区间 | 胜/平/负 | 局数 | %s总分 |\n|---:|---|---:|---|---:|---:|---:|\n", domainLabel)
	for i, r := range rows {
		iv := "—"
		if r.Intervals {
			iv = fmt.Sprintf("%.1f ~ %.1f", r.Lo, r.Hi)
		}
		fmt.Fprintf(&b, "| %d | %s | %.1f | %s | %d/%d/%d | %d | %.1f |\n",
			i+1, r.Name, r.R, iv, r.St.Wins, r.St.Draws, r.St.Losses, r.St.Played, r.St.Total)
	}
	fmt.Fprintf(&b, "\n基准锚：%s = 0；共 %d 局；bootstrap %d 次。\n", bt.Anchor, bt.Games, bt.Bootstrap)
	fmt.Fprintf(&b, "\n> 胜负标签与%s加权总分是两条独立信号：本表按胜负拟合实力值，加权总分单列。\n", domainLabel)
	fmt.Fprintf(&b, "> 区间按对局整块重抽样；全胜/全败时区间会退化，属统计口径而非成绩异常。\n")
	if err := os.WriteFile(filepath.Join(outDir, "成绩.md"), []byte(b.String()), 0o644); err != nil {
		return err
	}

	var t strings.Builder
	fmt.Fprintf(&t, "# %s加权总分\n\n| 名次 | 选手 | 合计 | 场均 | 故障 |\n|---:|---|---:|---:|---:|\n", domainLabel)
	byTotal := make([]*standing, 0, len(table))
	for _, s := range table {
		byTotal = append(byTotal, s)
	}
	sort.Slice(byTotal, func(i, j int) bool { return byTotal[i].Total > byTotal[j].Total })
	for i, s := range byTotal {
		avg := 0.0
		if s.Played > 0 {
			avg = s.Total / float64(s.Played)
		}
		fmt.Fprintf(&t, "| %d | %s | %.1f | %.2f | %d |\n", i+1, s.Name, s.Total, avg, s.Faults)
	}
	if err := os.WriteFile(filepath.Join(outDir, "成绩_加权总分.md"), []byte(t.String()), 0o644); err != nil {
		return err
	}

	fmt.Printf("rate → %s\n", filepath.Join(outDir, "成绩.md"))
	fmt.Printf("       %s\n", filepath.Join(outDir, "成绩_加权总分.md"))
	fmt.Printf("       %s\n", filepath.Join(outDir, "成绩.json"))
	return nil
}

/* ------------------------------------------------------------------ */
/* check                                                               */
/* ------------------------------------------------------------------ */

func cmdCheck(args []string) error {
	fs := flag.NewFlagSet("check", flag.ExitOnError)
	cfgPath := fs.String("config", "tournament/formal.json", "赛事配置")
	aPath := fs.String("a", "", "左方程序（默认 workspace/submission 编译产物）")
	bPath := fs.String("b", "", "右方程序（默认第一个参考 Bot）")
	seed := fs.Int("seed", 20261109, "seed")
	_ = fs.Parse(args)

	cfg, root, err := contest.Load(*cfgPath)
	if err != nil {
		return err
	}
	if *aPath == "" {
		*aPath = filepath.Join(root, "bots", "bin", "submission"+exeSuffix())
	}
	if *bPath == "" {
		first := cfg.Entries[0]
		*bPath = filepath.Join(root, filepath.FromSlash(first.Bin))
	}
	for _, p := range []string{*aPath, *bPath} {
		if _, err := os.Stat(p); err != nil {
			return fmt.Errorf("找不到程序 %s（先跑 arena build 或 go build）", p)
		}
	}
	hostScript, err := cfg.EngineHostPath(root)
	if err != nil {
		return err
	}
	jcfg := judge.Config{
		HostScript: hostScript,
		Rules: engine.HostConfig{
			Generator: cfg.Rules.Generator,
			TickLimit: cfg.Rules.TickLimit,
			TickMs:    cfg.Rules.TickMs,
			DecideMs:  cfg.Rules.DecideMs,
			InitMs:    cfg.Rules.InitMs,
			Vision:    cfg.Rules.Vision,
		},
		Mirror:     true,
		MaxStrikes: cfg.Rules.MaxStrikes,
		StrikeMode: cfg.Rules.StrikeMode,
		Verify:     true,
	}
	rec, err := judge.RunMatch(0, *seed, judge.BotSpec{Name: "A", Path: *aPath}, judge.BotSpec{Name: "B", Path: *bPath}, jcfg)
	if err != nil {
		return err
	}
	fmt.Println("CHECK_OK")
	fmt.Printf("  引擎：%s ｜ 地图：%d×%d ｜ 规则指纹：%s ｜ seed=%d ｜ 生成器：%s\n",
		protocol.ProtoName, protocol.Grid, protocol.Grid, rec.RulesHash, rec.Seed, rec.Generator)
	for _, g := range rec.Games {
		fmt.Printf("  第 %d 局  座位=%s / %s  tick=%d  winner=%s  层级=%d  VP=%v  总分=%.2f : %.2f  故障=%d/%d\n",
			g.Index+1, g.Seats[0], g.Seats[1], g.Result.Turn,
			protocol.WinnerName(g.Result.Winner, g.Seats), g.Result.Level, g.Result.VP,
			g.Result.Cards[0].Total, g.Result.Cards[1].Total, len(g.Faults[0]), len(g.Faults[1]))
		fmt.Printf("    判定：%s ｜ 帧=%d ｜ 平均决策=%.3fms / %.3fms\n",
			g.Result.Reason, len(g.Frames), g.Result.Decide[0].AvgMS, g.Result.Decide[1].AvgMS)
		for seat, list := range g.Faults {
			for i, f := range list {
				if i >= 3 {
					fmt.Printf("      座位%d 故障（仅列前 3 条）…\n", seat)
					break
				}
				fmt.Printf("      座位%d 故障 t=%d %s: %s\n", seat, f.Turn, f.Kind, f.Err)
			}
		}
		if len(g.Verify) > 0 {
			var v struct {
				OK bool `json:"ok"`
			}
			_ = json.Unmarshal(g.Verify, &v)
			fmt.Printf("    确定性复核：%v\n", v.OK)
		}
	}
	return nil
}

/* ------------------------------------------------------------------ */
/* selftest                                                            */
/* ------------------------------------------------------------------ */

func cmdSelftest(args []string) error {
	fs := flag.NewFlagSet("selftest", flag.ExitOnError)
	cfgPath := fs.String("config", "tournament/formal.json", "赛事配置")
	_ = fs.Parse(args)

	cfg, root, err := contest.Load(*cfgPath)
	if err != nil {
		return err
	}
	normal := filepath.Join(root, "bots", "bin", "submission"+exeSuffix())
	if _, err := os.Stat(normal); err != nil {
		return fmt.Errorf("找不到基准选手 %s（先跑 arena build）", normal)
	}
	tmp, err := os.MkdirTemp("", "arena-selftest")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	faulty := filepath.Join(tmp, "faultbot"+exeSuffix())
	build := exec.Command("go", "build", "-o", faulty, "./cmd/faultbot")
	build.Dir = root
	build.Stderr = os.Stderr
	fmt.Println("自检：编译故障注入 Bot …")
	if err := build.Run(); err != nil {
		return fmt.Errorf("编译 faultbot 失败: %w", err)
	}

	hostScript, err := cfg.EngineHostPath(root)
	if err != nil {
		return err
	}
	base := judge.Config{
		HostScript: hostScript,
		Rules: engine.HostConfig{
			Generator: cfg.Rules.Generator,
			TickLimit: cfg.Rules.TickLimit,
			TickMs:    cfg.Rules.TickMs,
			DecideMs:  cfg.Rules.DecideMs,
			InitMs:    cfg.Rules.InitMs,
			Vision:    cfg.Rules.Vision,
		},
		Mirror:     false,
		MaxStrikes: cfg.Rules.MaxStrikes,
		StrikeMode: cfg.Rules.StrikeMode,
		Verify:     false,
	}
	fail := 0
	for _, mode := range []string{"timeout", "crash", "badjson"} {
		jc := base
		jc.Env = []string{"FAULT_MODE=" + mode}
		rec, err := judge.RunMatch(0, 20261109, judge.BotSpec{Name: "Faulty", Path: faulty}, judge.BotSpec{Name: "Normal", Path: normal}, jc)
		if err != nil {
			fmt.Printf("  %-8s ERROR %v\n", mode, err)
			fail++
			continue
		}
		g := rec.Games[0]
		ok := g.Result.Winner == 1 && g.Result.Level == 0 && len(g.Faults[0]) >= cfg.Rules.MaxStrikes
		status := "PASS"
		if !ok {
			status = "FAIL"
			fail++
		}
		fmt.Printf("  %-8s %s  胜者=座位%d 层级=%d 故障=%d 判定=%s\n",
			mode, status, g.Result.Winner, g.Result.Level, len(g.Faults[0]), g.Result.Reason)
	}
	if fail > 0 {
		return fmt.Errorf("自检失败 %d 项", fail)
	}
	fmt.Println("SELFTEST_OK：超时 kill、崩溃重启、非法结构三种路径都按连续 3 次红牌收局。")
	return nil
}

func versionOr(v string) string {
	if v == "" {
		return "（未记录）"
	}
	return v
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

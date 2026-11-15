// Package judge 是正式赛的唯一执行者。
//
// 版本：v1.2-beta（Desert Arena 正式赛执行链）。
// 功能：拉起引擎宿主、为每个座位拉起独立 Bot 进程、按 hello→ready→tick→actions→end
// 收发真实 UTF-8 行分隔 JSON、执行 120ms 决策死线、连续 3 次故障红牌、
// 超时 kill 与重启、换边重开、回放抽帧归档、逐局确定性复核。
//
// 它不负责：规则语义（引擎宿主）、计分公式（引擎宿主）、赛程与评分（上层）。
package judge

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"desertarena/internal/engine"
	"desertarena/internal/protocol"
)

// ErrTimeout 表示 Bot 在预算内没有交出完整一行。
var ErrTimeout = errors.New("decision timeout")

// BotSpec 是一个已编译的选手程序。
type BotSpec struct {
	Name string
	Path string
}

// Config 是一场比赛的裁判配置。
type Config struct {
	HostScript string
	RulesPath  string
	Rules      engine.HostConfig
	Mirror     bool
	MaxStrikes int
	StrikeMode string
	Verify     bool
	// 附加环境变量（自检用的故障注入）
	Env []string
}

// Fault 是一条故障记录。
type Fault struct {
	Turn int     `json:"turn"`
	Kind string  `json:"kind"`
	Err  string  `json:"err"`
	Ms   float64 `json:"ms"`
}

// GameRecord 是一局（换边前后各一局）的完整归档。
type GameRecord struct {
	Index       int               `json:"index"`
	Swap        bool              `json:"swap"`
	Seed        int               `json:"seed"`
	RequestSeed int               `json:"requestSeed"`
	Generator   string            `json:"generator"`
	MapAttempt  int               `json:"mapAttempt"`
	RulesHash   string            `json:"rulesHash"`
	Seats       []string          `json:"seats"`
	Result      protocol.Result   `json:"result"`
	Faults      [][]Fault         `json:"faults"`
	Frames      []json.RawMessage `json:"frames"`
	Verify      json.RawMessage   `json:"verify,omitempty"`
	Header      json.RawMessage   `json:"-"`
	DurationMS  int64             `json:"durationMs"`
	Timing      map[string]int    `json:"timing"`
}

// MatchRecord 是一场对阵（默认换边两局）。
type MatchRecord struct {
	Index       int             `json:"index"`
	RequestSeed int             `json:"requestSeed"`
	Seed        int             `json:"seed"`
	Generator   string          `json:"generator"`
	MapAttempt  int             `json:"mapAttempt"`
	RulesHash   string          `json:"rulesHash"`
	Seats       []string        `json:"seats"`
	Header      json.RawMessage `json:"header,omitempty"`
	Games       []GameRecord    `json:"games"`
	DurationMS  int64           `json:"durationMs"`
}

/* ------------------------------------------------------------------ */
/* Bot 进程                                                            */
/* ------------------------------------------------------------------ */

type botProc struct {
	spec  BotSpec
	cmd   *exec.Cmd
	stdin io.WriteCloser
	lines chan string
	done  chan struct{}
	stop  sync.Once
	errMu sync.Mutex
	err   bytes.Buffer
}

func startBot(spec BotSpec, hello string, initMs int, env []string) (*botProc, error) {
	cmd := exec.Command(spec.Path)
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	p := &botProc{spec: spec, cmd: cmd, stdin: stdin, lines: make(chan string, 4), done: make(chan struct{})}
	cmd.Stderr = &limitedWriter{buf: &p.err, max: 4096}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("启动 %s 失败: %w", spec.Name, err)
	}
	go p.readLoop(stdout)
	if err := p.writeLine(hello); err != nil {
		p.Kill()
		return nil, fmt.Errorf("%s 无法接收 hello: %w", spec.Name, err)
	}
	line, err := p.readLine(time.Duration(initMs) * time.Millisecond)
	if err != nil {
		p.Kill()
		return nil, fmt.Errorf("%s 初始化失败(%v): %s", spec.Name, err, p.stderrTail())
	}
	if !strings.Contains(line, "\"ready\"") {
		p.Kill()
		return nil, fmt.Errorf("%s 未按协议回报 ready: %s", spec.Name, truncate(line, 120))
	}
	return p, nil
}

func (p *botProc) readLoop(r io.Reader) {
	defer close(p.lines)
	br := bufio.NewReaderSize(r, 1<<20)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			select {
			case p.lines <- line:
			case <-p.done:
				return
			}
		}
		if err != nil {
			return
		}
	}
}

func (p *botProc) writeLine(line string) error {
	_, err := io.WriteString(p.stdin, line+"\n")
	return err
}

func (p *botProc) readLine(d time.Duration) (string, error) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case line, ok := <-p.lines:
		if !ok {
			return "", io.EOF
		}
		return line, nil
	case <-timer.C:
		return "", ErrTimeout
	}
}

func (p *botProc) sendEnd(line string) {
	_ = p.writeLine(line)
}

func (p *botProc) Kill() {
	p.stop.Do(func() {
		close(p.done)
		_ = p.stdin.Close()
		if p.cmd.Process != nil {
			_ = p.cmd.Process.Kill()
		}
		_ = p.cmd.Wait()
	})
}

func (p *botProc) stderrTail() string {
	p.errMu.Lock()
	defer p.errMu.Unlock()
	return truncate(strings.TrimSpace(p.err.String()), 240)
}

type limitedWriter struct {
	buf *bytes.Buffer
	max int
	mu  sync.Mutex
}

func (w *limitedWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf.Len() < w.max {
		room := w.max - w.buf.Len()
		if len(b) > room {
			b = b[:room]
		}
		w.buf.Write(b)
	}
	return len(b), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

/* ------------------------------------------------------------------ */
/* 对局                                                                */
/* ------------------------------------------------------------------ */

// RunMatch 跑完一场对阵（mirror=true 时换边两局，每局重启 Bot）。
func RunMatch(index, seed int, a, b BotSpec, cfg Config) (*MatchRecord, error) {
	if cfg.MaxStrikes <= 0 {
		cfg.MaxStrikes = 3
	}
	host, err := engine.Start(cfg.HostScript, cfg.RulesPath)
	if err != nil {
		return nil, err
	}
	defer host.Close()

	rec := &MatchRecord{Index: index, RequestSeed: seed, Seats: []string{a.Name, b.Name}}
	started := time.Now()

	games := 1
	if cfg.Mirror {
		games = 2
	}
	for gi := 0; gi < games; gi++ {
		swap := gi == 1
		specs := [2]BotSpec{a, b}
		names := []string{a.Name, b.Name}
		if swap {
			specs = [2]BotSpec{b, a}
			names = []string{b.Name, a.Name}
		}
		gameSeed := seed
		game, err := runGame(host, gi, swap, gameSeed, names, specs, cfg)
		if err != nil {
			return nil, fmt.Errorf("第 %d 局失败: %w", gi+1, err)
		}
		if rec.Header == nil && game.Header != nil {
			rec.Header = game.Header
		}
		rec.Seed = game.Seed
		rec.RulesHash = game.RulesHash
		rec.Generator = game.Generator
		rec.MapAttempt = game.MapAttempt
		rec.Games = append(rec.Games, *game)
	}
	rec.DurationMS = time.Since(started).Milliseconds()
	return rec, nil
}

func runGame(host *engine.Host, gi int, swap bool, seed int, seats []string, specs [2]BotSpec, cfg Config) (*GameRecord, error) {
	started := time.Now()
	cfg.Rules.Seed = seed
	nr, err := host.New(cfg.Rules)
	if err != nil {
		return nil, err
	}
	if nr.ProtoVersion != protocol.ProtoVersion {
		return nil, fmt.Errorf("协议版本不匹配：裁判 v%d，引擎 v%d", protocol.ProtoVersion, nr.ProtoVersion)
	}

	rec := &GameRecord{
		Index: gi, Swap: swap, Seed: nr.Seed, RequestSeed: seed,
		Generator: nr.Generator, MapAttempt: nr.MapAttempt, RulesHash: nr.RulesHash,
		Seats: seats, Faults: make([][]Fault, 2),
		Header: json.RawMessage(nr.Hello),
		Timing: map[string]int{
			"tickMs": cfg.Rules.TickMs, "decideMs": cfg.Rules.DecideMs,
			"initMs": cfg.Rules.InitMs, "orderLatency": protocol.OrderLatency,
		},
	}

	/* 起进程。初始化失败直接红牌，与冻结 oracle 的 init 语义一致。 */
	var procs [2]*botProc
	for s := 0; s < 2; s++ {
		p, err := startBot(specs[s], nr.Hello, cfg.Rules.InitMs, cfg.Env)
		if err != nil {
			rec.Faults[s] = append(rec.Faults[s], Fault{Turn: 0, Kind: "init", Err: err.Error()})
			if perr := host.Note(s, 0, true); perr != nil {
				return nil, perr
			}
		}
		procs[s] = p
	}
	if procs[0] == nil && procs[1] == nil {
		if err := host.EndGame(-1, 0, "双方初始化故障，判为平局"); err != nil {
			return nil, err
		}
	} else if procs[0] == nil {
		if err := host.Forfeit(0, "故障出局：初始化失败"); err != nil {
			return nil, err
		}
	} else if procs[1] == nil {
		if err := host.Forfeit(1, "故障出局：初始化失败"); err != nil {
			return nil, err
		}
	}

	strikes := [2]int{}
	forfeited := [2]bool{procs[0] == nil, procs[1] == nil}
	reason := "故障出局：连续失败达到上限（红牌）"
	decideLimit := time.Duration(cfg.Rules.DecideMs) * time.Millisecond
	curTurn := 0

	for !forfeited[0] || !forfeited[1] {
		done, turn, frame, err := host.Step()
		if err != nil {
			return nil, err
		}
		curTurn = turn
		if frame != nil {
			rec.Frames = append(rec.Frames, frame)
		}
		if done {
			break
		}
		for s := 0; s < 2; s++ {
			if forfeited[s] || procs[s] == nil {
				continue
			}
			line, err := host.Encode(s)
			if err != nil {
				return nil, err
			}
			t0 := time.Now()
			werr := procs[s].writeLine(line)
			var reply string
			var rerr error
			if werr != nil {
				rerr = werr
			} else {
				reply, rerr = procs[s].readLine(decideLimit)
			}
			ms := float64(time.Since(t0).Microseconds()) / 1000.0

			if rerr != nil {
				rec.Faults[s] = append(rec.Faults[s], Fault{Turn: curTurn, Kind: faultKind(rerr), Err: errText(rerr, procs[s]), Ms: round3(ms)})
				if err := host.Note(s, ms, true); err != nil {
					return nil, err
				}
				strikes[s]++
				procs[s].Kill()
				procs[s] = nil
				/* 超时/崩溃后换一条新进程继续打；重建失败才判负。 */
				if np, rerr := startBot(specs[s], nr.Hello, cfg.Rules.InitMs, cfg.Env); rerr == nil {
					procs[s] = np
				} else {
					forfeited[s] = true
					rec.Faults[s] = append(rec.Faults[s], Fault{Turn: curTurn, Kind: "restart", Err: rerr.Error()})
				}
				if strikes[s] >= cfg.MaxStrikes {
					forfeited[s] = true
				}
				continue
			}

			if _, aerr := host.Apply(s, reply); aerr != nil {
				/* 结构非法：这一拍作废、记故障；进程还活着就不重启。 */
				rec.Faults[s] = append(rec.Faults[s], Fault{Turn: curTurn, Kind: "bad_reply", Err: aerr.Error(), Ms: round3(ms)})
				if err := host.Note(s, ms, true); err != nil {
					return nil, err
				}
				strikes[s]++
				if strikes[s] >= cfg.MaxStrikes {
					forfeited[s] = true
				}
				continue
			}
			if err := host.Note(s, ms, false); err != nil {
				return nil, err
			}
			strikes[s] = 0
		}
		if forfeited[0] && forfeited[1] {
			if err := host.EndGame(-1, 0, "双方故障出局，判为平局"); err != nil {
				return nil, err
			}
			break
		}
		if forfeited[0] {
			if err := host.Forfeit(0, reason); err != nil {
				return nil, err
			}
			break
		}
		if forfeited[1] {
			if err := host.Forfeit(1, reason); err != nil {
				return nil, err
			}
			break
		}
	}

	raw, err := host.Result()
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &rec.Result); err != nil {
		return nil, fmt.Errorf("解析终局结果失败: %w", err)
	}
	if len(rec.Result.Cards) != 2 {
		return nil, fmt.Errorf("终局结果缺计分卡（cards=%d）", len(rec.Result.Cards))
	}

	if cfg.Verify {
		v, verr := host.Verify()
		if verr == nil {
			rec.Verify = v
		} else {
			rec.Verify = json.RawMessage(fmt.Sprintf(`{"ok":false,"error":%q}`, verr.Error()))
		}
	}

	endLine, _ := json.Marshal(map[string]any{"type": "end", "turn": rec.Result.Turn, "result": rec.Result})
	for s := 0; s < 2; s++ {
		if procs[s] != nil {
			procs[s].sendEnd(string(endLine))
			procs[s].Kill()
		}
	}
	_ = curTurn
	rec.DurationMS = time.Since(started).Milliseconds()
	return rec, nil
}

func faultKind(err error) string {
	if errors.Is(err, ErrTimeout) {
		return "timeout"
	}
	if errors.Is(err, io.EOF) {
		return "crash"
	}
	return "io"
}

func errText(err error, p *botProc) string {
	tail := ""
	if p != nil {
		tail = p.stderrTail()
	}
	if tail == "" {
		return err.Error()
	}
	return err.Error() + " | stderr: " + tail
}

func round3(v float64) float64 {
	return float64(int64(v*1000+0.5)) / 1000
}

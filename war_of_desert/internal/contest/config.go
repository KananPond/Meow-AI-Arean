// Package contest 读取正式赛配置并解析相对路径。
//
// 版本：v1.2-beta（Desert Arena 正式赛执行链）。
// 功能：tournament/*.json 的结构定义与校验、引擎宿主路径解析
// （kind=native 时构建 enginehost）、输出目录约束在仓库根内。
package contest

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"desertarena/internal/schedule"
)

// Config 是 tournament/formal.json 的结构。
type Config struct {
	Name    string `json:"name"`
	Season  string `json:"season"`
	Version string `json:"version"`
	Engine  struct {
		Kind         string `json:"kind"`
		Host         string `json:"host"`
		Snapshot     string `json:"snapshot"`
		OracleCommit string `json:"oracleCommit"`
	} `json:"engine"`
	Rules struct {
		TickLimit  int    `json:"tickLimit"`
		TickMs     int    `json:"tickMs"`
		DecideMs   int    `json:"decideMs"`
		InitMs     int    `json:"initMs"`
		MaxStrikes int    `json:"maxStrikes"`
		StrikeMode string `json:"strikeMode"`
		Generator  string `json:"generator"`
		Vision     bool   `json:"vision"`
	} `json:"rules"`
	Schedule struct {
		Seeds     []int `json:"seeds"`
		Mirror    bool  `json:"mirror"`
		Bootstrap int   `json:"bootstrap"`
	} `json:"schedule"`
	Entries []schedule.Entry `json:"entries"`
	Output  string           `json:"output"`
	Verify  bool             `json:"verify"`
}

// Load 读取配置。root 是 war_of_desert 目录（配置文件的上一级）。
func Load(path string) (*Config, string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil, "", err
	}
	var c Config
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, "", fmt.Errorf("配置不是合法 JSON: %w", err)
	}
	if c.Rules.TickLimit <= 0 || c.Rules.DecideMs <= 0 {
		return nil, "", fmt.Errorf("配置缺 rules.tickLimit / rules.decideMs")
	}
	if len(c.Entries) < 2 {
		return nil, "", fmt.Errorf("正式赛至少需要 2 名选手，当前 %d", len(c.Entries))
	}
	root := filepath.Dir(filepath.Dir(abs))
	return &c, root, nil
}

// ScheduleConfig 转成赛程包的口径。
func (c *Config) ScheduleConfig() schedule.Config {
	return schedule.Config{
		TickLimit:  c.Rules.TickLimit,
		TickMs:     c.Rules.TickMs,
		DecideMs:   c.Rules.DecideMs,
		InitMs:     c.Rules.InitMs,
		MaxStrikes: c.Rules.MaxStrikes,
		StrikeMode: c.Rules.StrikeMode,
		Generator:  c.Rules.Generator,
		Vision:     c.Rules.Vision,
	}
}

// HostPath 返回引擎宿主脚本的绝对路径。
func (c *Config) HostPath(root string) string {
	return filepath.Join(root, filepath.FromSlash(c.Engine.Host))
}

// EngineHostPath 返回引擎宿主：kind=native 时构建 Go 原生 enginehost
// 二进制（go build 有缓存，重复调用开销小），否则走 oracle_host.js。
func (c *Config) EngineHostPath(root string) (string, error) {
	if c.Engine.Kind != "native" {
		return c.HostPath(root), nil
	}
	bin := filepath.Join(root, "bots", "bin", "enginehost"+exeSuffix())
	build := exec.Command("go", "build", "-o", bin, "./cmd/enginehost")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("构建 enginehost 失败: %s", strings.TrimSpace(string(out)))
	}
	return bin, nil
}

func exeSuffix() string {
	if runtime.GOOS == "windows" {
		return ".exe"
	}
	return ""
}

// OutDir 返回赛事目录的绝对路径。
func (c *Config) OutDir(root string) string {
	if c.Output == "" {
		return filepath.Join(root, "results", "formal")
	}
	return filepath.Join(root, filepath.FromSlash(c.Output))
}

// Bootstrap 缺省 200 次，与 UP 主原版一致。
func (c *Config) Bootstrap() int {
	if c.Schedule.Bootstrap <= 0 {
		return 200
	}
	return c.Schedule.Bootstrap
}

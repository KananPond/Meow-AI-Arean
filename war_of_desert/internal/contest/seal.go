// 文件：internal/contest/seal.go
// 版本：v1.3
// 功能：封存记录的读取与开赛校验。
//
// 为什么要有这层：v1.2 的 bots/封存记录.json 是 11-15 生成的，源码在 11-16
// 的 v1.2 改动里全部重写过，7 个文件哈希一个都对不上——但 arena run 照样
// 开跑，赛后审计第 1 条（源码哈希与封存记录一致）直接过不去。
// 现在把校验做成开赛硬门槛：对不上就中止，并指出是哪个文件。
package contest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SealEntry 是一名选手的封存项。
type SealEntry struct {
	Name   string            `json:"name"`
	Source string            `json:"source"`
	Bin    string            `json:"bin"`
	Files  map[string]string `json:"files"`
}

// Seal 是 bots/封存记录.json 的结构。
type Seal struct {
	GeneratedAt string            `json:"generatedAt"`
	Shared      map[string]string `json:"shared"`
	Entries     []SealEntry       `json:"entries"`
	Note        string            `json:"note,omitempty"`
}

// LoadSeal 读封存记录。
func LoadSeal(root string) (*Seal, error) {
	raw, err := os.ReadFile(filepath.Join(root, "bots", "封存记录.json"))
	if err != nil {
		return nil, fmt.Errorf("读不到封存记录（先跑 arena build）: %w", err)
	}
	var s Seal
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("封存记录不是合法 JSON: %w", err)
	}
	return &s, nil
}

// Verify 逐文件比对当前工作区的哈希，返回不一致清单。
//
// 空清单表示封存链完整。清单非空时调用方必须中止开赛：
// 带着失配的封存开跑，赛后无法证明「跑的就是收卷的那份代码」。
func (s *Seal) Verify(root string) []string {
	var bad []string
	for rel, want := range s.Shared {
		got, err := hashFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s：%v", rel, err))
			continue
		}
		if got != want {
			bad = append(bad, fmt.Sprintf("%s：封存 %s，实际 %s", rel, short(want), short(got)))
		}
	}
	for _, e := range s.Entries {
		dir := filepath.Join(root, filepath.FromSlash(e.Source))
		have, err := hashTree(dir)
		if err != nil {
			bad = append(bad, fmt.Sprintf("%s/%s：%v", e.Name, e.Source, err))
			continue
		}
		for name, want := range e.Files {
			got, ok := have[name]
			if !ok {
				bad = append(bad, fmt.Sprintf("%s/%s：源码已删除", e.Name, name))
				continue
			}
			if got != want {
				bad = append(bad, fmt.Sprintf("%s/%s：封存 %s，实际 %s", e.Name, name, short(want), short(got)))
			}
		}
		for name := range have {
			if _, ok := e.Files[name]; !ok {
				bad = append(bad, fmt.Sprintf("%s/%s：封存后新增的源码文件", e.Name, name))
			}
		}
	}
	sort.Strings(bad)
	return bad
}

// CheckEntryCoverage 确认配置里的选手与封存记录一一对应。
func (s *Seal) CheckEntryCoverage(names []string) []string {
	sealed := map[string]bool{}
	for _, e := range s.Entries {
		sealed[e.Name] = true
	}
	var bad []string
	for _, n := range names {
		if !sealed[n] {
			bad = append(bad, n+"：不在封存记录里（需要重新 arena build）")
		}
	}
	for _, e := range s.Entries {
		found := false
		for _, n := range names {
			if n == e.Name {
				found = true
				break
			}
		}
		if !found {
			bad = append(bad, e.Name+"：封存了但不在本赛季名单里")
		}
	}
	sort.Strings(bad)
	return bad
}

func hashFile(p string) (string, error) {
	raw, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func hashTree(dir string) (map[string]string, error) {
	out := map[string]string{}
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(p, ".go") {
			return nil
		}
		h, err := hashFile(p)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = h
		return nil
	})
	return out, err
}

func short(h string) string {
	if len(h) <= 12 {
		return h
	}
	return h[:12]
}

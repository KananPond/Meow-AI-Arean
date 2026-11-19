// 文件：internal/judge/sandbox_other.go
// 版本：v1.3
// 功能：非 Windows 平台的沙箱占位。
//
// 正式赛执行链目前在 Windows 上跑（Job Object 那条路）。其他平台只能靠
// 外部约束（容器 / 独立低权账户 / cgroup），这里显式返回「未加固」，
// 让调用方能把这条写进赛务记录，而不是误以为已经隔离。
//go:build !windows

package judge

import (
	"errors"
	"os"
)

type sandbox struct{}

func newSandbox() *sandbox { return &sandbox{} }

func (s *sandbox) Apply(p *os.Process) error {
	return errors.New("当前平台没有内置沙箱：需要容器或独立低权账户")
}

func (s *sandbox) Close() {}

// 文件：internal/judge/sandbox_windows.go
// 版本：v1.3
// 功能：用 Windows Job Object 给选手进程加硬约束。
//
// 为什么必须有：v1.2 及其以前，裁判只是一句 exec.Command 拉起 Bot，
// 继承完整的环境变量与文件权限，无工作目录隔离、无内存/CPU 配额、无禁网。
// 手册 §3.2 的「禁止读写文件、访问网络、启动子进程」全写在纸上，
// 一个不友善的选手可以直接改 results/*/结果.jsonl 或读别人的源码。
//
// Job Object 能挡住的部分：
//   - 子进程派生（ActiveProcessLimit=1）——禁止 Bot 拉起外部程序；
//   - 内存爆炸（ProcessMemoryLimit / JobMemoryLimit）；
//   - 裁判退出时连带杀死（KILL_ON_JOB_CLOSE）——不留孤儿进程。
//
// 挡不住的部分：文件读写与网络。那需要独立低权账户或容器。
// 因此配套措施是 startBot 把工作目录切到一次性临时目录，
// 以及 arena run 开赛前后的封存哈希校验（见 contest.VerifySeal）。
//go:build windows

package judge

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	jobObjectExtendedLimitInformation = 9

	limProcessMemory     = 0x00000100
	limJobMemory         = 0x00000200
	limActiveProcess     = 0x00000008
	limKillOnJobClose    = 0x00002000
	botProcessMemoryCap  = 512 << 20
	botJobMemoryCap      = 1024 << 20
	botActiveProcessCap  = 1

	procSetQuota   = 0x0100
	procTerminate  = 0x0001
)

var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW         = modkernel32.NewProc("CreateJobObjectW")
	procSetInformationJobObject  = modkernel32.NewProc("SetInformationJobObject")
	procAssignProcessToJobObject = modkernel32.NewProc("AssignProcessToJobObject")
	procCloseHandle              = modkernel32.NewProc("CloseHandle")
)

type ioCounters struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type basicLimitInformation struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	_                       uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	_                       uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type extendedLimitInformation struct {
	BasicLimitInformation basicLimitInformation
	IoInfo                ioCounters
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

// sandbox 是一个 Job Object 句柄。
type sandbox struct {
	handle syscall.Handle
}

// newSandbox 建一个空 Job（限制在 Apply 时写入）。
func newSandbox() *sandbox {
	h, _, _ := procCreateJobObjectW.Call(0, 0)
	if h == 0 {
		return &sandbox{}
	}
	return &sandbox{handle: syscall.Handle(h)}
}

// Apply 把沙箱限制写进 Job，并把进程绑进去。
func (s *sandbox) Apply(p *os.Process) error {
	if s == nil || s.handle == 0 {
		return fmt.Errorf("创建 Job Object 失败（沙箱未生效）")
	}
	info := extendedLimitInformation{
		BasicLimitInformation: basicLimitInformation{
			LimitFlags:          limProcessMemory | limJobMemory | limActiveProcess | limKillOnJobClose,
			ActiveProcessLimit:  botActiveProcessCap,
		},
		ProcessMemoryLimit: botProcessMemoryCap,
		JobMemoryLimit:     botJobMemoryCap,
	}
	r, _, err := procSetInformationJobObject.Call(
		uintptr(s.handle),
		uintptr(jobObjectExtendedLimitInformation),
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	if r == 0 {
		return fmt.Errorf("SetInformationJobObject 失败: %v", err)
	}
	ph, err := syscall.OpenProcess(procSetQuota|procTerminate, false, uint32(p.Pid))
	if err != nil {
		return fmt.Errorf("打开选手进程句柄失败: %w", err)
	}
	defer syscall.CloseHandle(ph)
	r, _, err = procAssignProcessToJobObject.Call(uintptr(s.handle), uintptr(ph))
	if r == 0 {
		return fmt.Errorf("AssignProcessToJobObject 失败: %v", err)
	}
	return nil
}

// Close 释放 Job 句柄；KILL_ON_JOB_CLOSE 会连带杀掉残留进程。
func (s *sandbox) Close() {
	if s != nil && s.handle != 0 {
		procCloseHandle.Call(uintptr(s.handle))
		s.handle = 0
	}
}

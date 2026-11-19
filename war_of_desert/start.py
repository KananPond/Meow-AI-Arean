#!/usr/bin/env python3
"""Desert Arena 正式赛 · 组织方菜单。

等价于在 war_of_desert/ 目录逐条执行 go run ./cmd/arena ...；
给不熟悉命令行的组织者一个入口，不改变任何执行逻辑。
"""
from __future__ import annotations

import json
import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent
ARENA = ["go", "run", "./cmd/arena"]
CONFIG = "tournament/formal_v12.json"
OUT_DIR = "results/formal-v12"

MENU = [
    ("1", "build", "编译选手并封存源码哈希"),
    ("2", "plan", "冻结赛程"),
    ("3", "run", "跑完所有对局（可用 --resume 续跑）"),
    ("4", "rate", "出 BT 实力值与加权总分"),
    ("5", "check", "单场自检（模板 vs 首位参考 Bot）"),
    ("6", "selftest", "验证超时 kill / 崩溃重启 / 非法结构红牌"),
    ("7", "season", "整季流水线：build → plan → run → rate"),
]


def run(args: list[str]) -> int:
    full = ARENA + [args[0], "-config", CONFIG] + args[1:]
    print("$", " ".join(full), flush=True)
    return subprocess.call(full, cwd=ROOT)


def season() -> int:
    for cmd in ("build", "plan", "run", "rate"):
        code = run([cmd])
        if code != 0:
            print(f"流水线在 {cmd} 阶段中止（退出码 {code}）")
            return code
    print(f"整季完成：结果在 {OUT_DIR}/", flush=True)
    return 0


def status() -> None:
    record = ROOT / "赛务记录.json"
    if not record.exists():
        print("赛务：尚未开赛（没有 赛务记录.json）")
    else:
        data = json.loads(record.read_text(encoding="utf-8"))
        print("赛务记录：", json.dumps(data, ensure_ascii=False, indent=2)[:800])
    result = ROOT / "results" / "formal-v12" / "结果.jsonl"
    if result.exists():
        lines = result.read_text(encoding="utf-8").strip().splitlines()
        print(f"对局：{len(lines)} 局已记录")


def main() -> int:
    args = sys.argv[1:]
    if args and args[0] in {"-h", "--help"}:
        print("用法：python start.py [build|plan|run|rate|check|selftest|season|status]")
        return 0
    if args:
        cmd = args[0]
        if cmd == "status":
            status()
            return 0
        if cmd == "season":
            return season()
        if cmd in {name for _, name, _ in MENU}:
            return run([cmd] + args[1:])
        print("未知命令：", cmd)
        return 2

    while True:
        print("\nDesert Arena 正式赛")
        for key, name, desc in MENU:
            print(f"  {key}. {name:<9} {desc}")
        print("  s. status")
        print("  0. 退出")
        choice = input("选择：").strip().lower()
        if choice in {"0", "q", ""}:
            return 0
        if choice == "s":
            status()
            continue
        for key, name, _ in MENU:
            if choice == key:
                season() if name == "season" else run([name])
                break
        else:
            print("无效选择")


if __name__ == "__main__":
    raise SystemExit(main())

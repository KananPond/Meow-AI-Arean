#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""Desert Arena · 赛务（发卷 / 开赛 / 暂停 / 收卷）。

版本：v1.6
功能：以「一个选手目录」为单位管理编码期。目录可以是空的——开赛时先把
      选手包铺进去，再开始计时；到点把 submission/decide.go 冻结到 交卷/。

    python -X utf8 赛务.py --dir "算法赛/RTS算法赛@X" --issue     # 发卷
    python -X utf8 赛务.py --dir "算法赛/RTS算法赛@X" --start     # 开赛计时
    python -X utf8 赛务.py --dir "算法赛/RTS算法赛@X" --status
    python -X utf8 赛务.py --dir "算法赛/RTS算法赛@X" --pause --note="模型限额"
    python -X utf8 赛务.py --dir "算法赛/RTS算法赛@X" --resume
    python -X utf8 赛务.py --dir "算法赛/RTS算法赛@X" --freeze

目录约定（与 开赛.py 一致）：

    算法赛/RTS算法赛@X/
    ├── workspace/            选手包（由模板 RTS算法赛@Model_name 复制而来）
    │   ├── RULES.md …
    │   ├── bots/             骨架 Bot + 两个预编译基线
    │   ├── submission/X_bot.go    ← AI 在这里写（文件名 = 选手名 + _bot.go）
    │   └── clock.py               ← 选手自己查看剩余时间
    ├── 交卷/X_bot.go         收卷后的冻结副本
    ├── 交卷/交卷记录.json     SHA-256 / 字节数 / 用时 / accepted
    └── .赛务.json            计时与暂停的权威记录

计时口径：90 分钟目标、120 分钟强制收卷。超过 120 分钟仍会冻结留档，
但 accepted=false，不计正式成绩。
"""

from __future__ import annotations

import argparse
import ctypes
import hashlib
import json
import os
import shutil
import sys
import time
from datetime import datetime
from pathlib import Path

HERE = Path(__file__).resolve().parent
ENGINE = HERE / "war_of_desert"
# 选手包的唯一真源：模板工作区。其他选手目录都由它复制而来。
TEMPLATE = HERE / "算法赛" / "RTS算法赛@Model_name"
PACKAGE = TEMPLATE / "workspace"

TARGET_MINUTES = 90
LIMIT_MINUTES = 120

WS = "workspace"
CLOCK = Path(WS) / "clock.py"
FROZEN_DIR = "交卷"
RECORD = ".赛务.json"
WIN_EPOCH_OFFSET = 11644473600

# 发给选手的东西 = 模板 workspace/ 下除赛务产物以外的一切。
PACKAGE_ITEMS = ["RULES.md", "README.md", "prompt.md", "clock.py", "go.mod",
                 "botkit", "bots", "submission", "sandbox"]


# ---------------------------------------------------------------- 记录
def contestant_name(ws: Path) -> str:
    """选手名 = 目录名 @ 之后的部分；没有 @ 就用目录名。"""
    return ws.name.split("@")[-1] if "@" in ws.name else ws.name


def bot_filename(ws: Path) -> str:
    """交付物文件名：<选手名>_bot.go（规则里写死的约定）。"""
    return contestant_name(ws) + "_bot.go"


def submission_path(ws: Path) -> Path:
    """当前的交付文件。优先新命名，兼容早期发卷留下的 decide.go。"""
    new = ws / WS / "submission" / bot_filename(ws)
    if new.is_file():
        return new
    legacy = ws / WS / "submission" / "decide.go"
    return legacy if legacy.is_file() else new


def record_path(ws: Path) -> Path:
    return ws / RECORD


def load(ws: Path) -> dict:
    try:
        return json.loads(record_path(ws).read_text(encoding="utf-8"))
    except (OSError, ValueError):
        return {}


def save(ws: Path, data: dict) -> None:
    p = record_path(ws)
    tmp = p.with_name(p.name + ".tmp")
    tmp.write_text(json.dumps(data, ensure_ascii=False, indent=1) + "\n", encoding="utf-8")
    os.replace(tmp, p)


def fmt(ts: float) -> str:
    return datetime.fromtimestamp(ts).strftime("%Y-%m-%d %H:%M:%S")


def born(path: Path) -> float:
    st = path.stat()
    return float(getattr(st, "st_birthtime", None) or st.st_ctime)


def set_birth_windows(path: Path, ts: float) -> None:
    """把文件创建时间改成 ts（Windows）。

    选手侧的 clock.py 靠「文件创建时刻」算已用时间，pause/resume 必须
    把这条时间线同步过去，否则选手看到的剩余时间会跟赛务记录不一致。
    """
    file_write_attributes, open_existing, file_attribute_normal = 0x0100, 3, 0x80
    invalid = ctypes.c_void_p(-1).value
    k32 = ctypes.WinDLL("kernel32", use_last_error=True)
    k32.CreateFileW.restype = ctypes.c_void_p
    k32.CreateFileW.argtypes = [ctypes.c_wchar_p, ctypes.c_uint32, ctypes.c_uint32,
                                ctypes.c_void_p, ctypes.c_uint32, ctypes.c_uint32, ctypes.c_void_p]
    k32.SetFileTime.argtypes = [ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p, ctypes.c_void_p]
    k32.CloseHandle.argtypes = [ctypes.c_void_p]
    h = k32.CreateFileW(str(path), file_write_attributes, 0, None,
                        open_existing, file_attribute_normal, None)
    if h in (None, invalid):
        raise OSError(ctypes.get_last_error(), "打开 clock.py 失败")
    try:
        ft = ctypes.c_ulonglong(int((ts + WIN_EPOCH_OFFSET) * 10_000_000))
        if not k32.SetFileTime(h, ctypes.byref(ft), None, None):
            raise OSError(ctypes.get_last_error(), "设置创建时间失败")
    finally:
        k32.CloseHandle(h)


def sync_clock(ws: Path, logical_start: float) -> None:
    clock = ws / CLOCK
    if not clock.is_file():
        return
    if os.name == "nt":
        set_birth_windows(clock, logical_start)
    else:
        data = clock.read_bytes()
        tmp = clock.with_name(".clock.py.tmp")
        tmp.write_bytes(data)
        os.replace(tmp, clock)


# ---------------------------------------------------------------- 计时
def elapsed_seconds(ws: Path) -> float:
    rec = load(ws)
    started = rec.get("started_at")
    if not isinstance(started, (int, float)):
        return 0.0
    end = rec.get("paused_at") if isinstance(rec.get("paused_at"), (int, float)) else time.time()
    return max(0.0, end - float(started) - float(rec.get("paused_seconds", 0.0)))


def status(ws: Path) -> dict:
    rec = load(ws)
    started = rec.get("started_at")
    used = elapsed_seconds(ws) / 60.0
    paused = isinstance(rec.get("paused_at"), (int, float))
    frozen = rec.get("frozen") if isinstance(rec.get("frozen"), dict) else None
    if not isinstance(started, (int, float)):
        state = "未开赛"
    elif frozen:
        state = "已收卷"
    elif paused:
        state = "已暂停"
    elif used >= LIMIT_MINUTES:
        state = "已超时（待收卷）"
    else:
        state = "进行中"
    return {
        "state": state,
        "used_min": used,
        "left_min": max(0.0, LIMIT_MINUTES - used),
        "target_left": max(0.0, TARGET_MINUTES - used),
        "paused": paused,
        "frozen": frozen,
        "has_submission": submission_path(ws).is_file(),
        "submission": submission_path(ws).name,
        "issued": (ws / CLOCK).is_file(),
    }


# ---------------------------------------------------------------- 动作
def issue(ws: Path) -> list:
    """把选手包铺进 <ws>/workspace。已存在的文件不覆盖（避免冲掉选手的代码）。"""
    dest = ws / WS
    dest.mkdir(parents=True, exist_ok=True)
    copied = []
    for name in PACKAGE_ITEMS:
        src = PACKAGE / name
        if not src.exists():
            continue
        dst = dest / name
        if src.is_dir():
            if dst.exists():
                continue
            # 基线 exe 必须一起发（它们本身就是预编译产物，不能按 bin 过滤掉）
            shutil.copytree(src, dst, ignore=shutil.ignore_patterns("__pycache__"))
            copied.append(name + "/")
        else:
            if dst.exists():
                continue
            shutil.copy2(src, dst)
            copied.append(name)

    # 交付文件按「<选手名>_bot.go」命名：模板里叫 Model_name_bot.go，发卷时改名。
    sub = dest / "submission"
    if sub.is_dir():
        for stale in ("Model_name_bot.go", "decide.go"):
            p = sub / stale
            if p.is_file() and p.name != bot_filename(ws):
                p.rename(sub / bot_filename(ws))
                break
    return copied


def start(ws: Path) -> None:
    rec = load(ws)
    now = time.time()
    rec.update({
        "target": ws.name,
        "started_at": now,
        "paused_at": None,
        "paused_seconds": 0.0,
        "recorded_at": now,
    })
    rec.setdefault("pauses", [])
    rec["starts"] = int(rec.get("starts", 0)) + 1
    save(ws, rec)
    sync_clock(ws, now)


def pause(ws: Path, note: str = "") -> None:
    rec = load(ws)
    if not isinstance(rec.get("started_at"), (int, float)) or isinstance(rec.get("paused_at"), (int, float)):
        return
    now = time.time()
    rec["paused_at"] = now
    rec.setdefault("pauses", []).append({"at": now, "note": note})
    save(ws, rec)


def resume(ws: Path) -> None:
    rec = load(ws)
    paused_at = rec.get("paused_at")
    if not isinstance(paused_at, (int, float)):
        return
    now = time.time()
    rec["paused_seconds"] = float(rec.get("paused_seconds", 0.0)) + (now - float(paused_at))
    rec["paused_at"] = None
    save(ws, rec)
    sync_clock(ws, float(rec["started_at"]) + float(rec["paused_seconds"]))


def freeze(ws: Path) -> dict:
    """冻结 submission/decide.go 到 交卷/，记录哈希与用时。"""
    src = submission_path(ws)
    out = ws / FROZEN_DIR
    out.mkdir(parents=True, exist_ok=True)
    used = elapsed_seconds(ws) / 60.0
    info = {
        "frozen_at": time.time(),
        "used_minutes": round(used, 1),
        "limit_minutes": LIMIT_MINUTES,
        "accepted": src.is_file() and used <= LIMIT_MINUTES,
        "missing_submission": not src.is_file(),
    }
    if src.is_file():
        dst = out / src.name
        shutil.copy2(src, dst)
        raw = dst.read_bytes()
        info.update({
            "sha256": hashlib.sha256(raw).hexdigest(),
            "bytes": len(raw),
            "file": src.name,
        })
    (out / "交卷记录.json").write_text(
        json.dumps(info, ensure_ascii=False, indent=2), encoding="utf-8")
    rec = load(ws)
    rec["frozen"] = info
    save(ws, rec)
    return info


def describe(ws: Path) -> str:
    st = status(ws)
    if st["state"] == "未开赛":
        return "未开赛" + ("" if st["issued"] else "（未发卷）")
    if st["frozen"]:
        f = st["frozen"]
        return "已收卷 · 用 %.1f 分钟%s" % (
            f.get("used_minutes", 0), "" if f.get("accepted") else "（不计成绩）")
    return "%.1f / %d 分钟%s" % (st["used_min"], LIMIT_MINUTES,
                                 "（暂停中）" if st["paused"] else "")


def main() -> int:
    ap = argparse.ArgumentParser(description="Desert Arena 赛务：发卷 / 开赛 / 暂停 / 收卷")
    ap.add_argument("--dir", required=True, help="选手目录（可以为空）")
    g = ap.add_mutually_exclusive_group(required=True)
    g.add_argument("--issue", action="store_true", help="发放选手包")
    g.add_argument("--start", action="store_true", help="开始/重置计时")
    g.add_argument("--status", action="store_true", help="查看状态")
    g.add_argument("--pause", action="store_true", help="暂停计时")
    g.add_argument("--resume", action="store_true", help="恢复计时")
    g.add_argument("--freeze", action="store_true", help="收卷")
    ap.add_argument("--note", default="", help="暂停原因")
    a = ap.parse_args()

    ws = Path(a.dir)
    if not ws.is_absolute():
        ws = (Path.cwd() / ws).resolve()
    ws.mkdir(parents=True, exist_ok=True)

    if a.issue:
        got = issue(ws)
        print("发卷 %s：" % ws.name + ("、".join(got) if got else "已存在，跳过"))
    elif a.start:
        if not (ws / CLOCK).is_file():
            issue(ws)
            print("目录为空，已先发卷")
        start(ws)
        print("开赛 %s：计时起点 %s，%d 分钟目标 / %d 分钟收卷"
              % (ws.name, fmt(load(ws)["started_at"]), TARGET_MINUTES, LIMIT_MINUTES))
    elif a.pause:
        pause(ws, a.note)
        print("暂停 %s：已用 %.1f 分钟%s" % (ws.name, elapsed_seconds(ws) / 60,
                                            ("（" + a.note + "）") if a.note else ""))
    elif a.resume:
        resume(ws)
        print("恢复 %s：已用 %.1f 分钟" % (ws.name, elapsed_seconds(ws) / 60))
    elif a.freeze:
        info = freeze(ws)
        if info["missing_submission"]:
            print("收卷 %s：没有 decide.go，记为未提交" % ws.name)
        else:
            print("收卷 %s：%d 字节，用 %.1f 分钟，sha256=%s%s"
                  % (ws.name, info["bytes"], info["used_minutes"],
                     info["sha256"][:16], "" if info["accepted"] else "，超时不计成绩"))
    else:
        st = status(ws)
        print("选手：%s" % ws.name)
        print("状态：%s" % describe(ws))
        print("交付：%s" % ("有" if st["has_submission"] else "没有 decide.go"))
        rec = load(ws)
        if isinstance(rec.get("started_at"), (int, float)):
            print("计时起点：%s" % fmt(rec["started_at"]))
            if rec.get("pauses"):
                print("暂停次数：%d" % len(rec["pauses"]))
            print("目标 %d 分钟 / 强制收卷 %d 分钟" % (TARGET_MINUTES, LIMIT_MINUTES))
    return 0


if __name__ == "__main__":
    sys.exit(main())

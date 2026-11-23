#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""Desert Arena · 裁判台（GUI）。

版本：v1.10
功能：三个页签，同一个窗口——

  ① 编码（默认页签）：一个选手、一条流水线——
       选工作区 → 自检 → 发卷 → 复制/查看提示词 → 开始编码（可暂停/继续）
       → 手动收卷封存（到 120 分钟自动收卷）。
     **这一段只管给参赛 AI 开一段写代码的时间窗，不跑任何对局。**
     提示词由人手动粘贴发送，裁判台不代发。
  ② 比赛：选择参赛工作区的 Bot、比赛规则版本，调用 war_of_desert 的官方
     arena（build/plan/run/rate）与 baseline@1/2 对局，成绩写入
     `算法赛/比赛记录/<时间戳>/`。
  ③ 测试：编码 + 比赛的全部功能，成绩只写入 `算法赛/测试/测试成绩/<时间戳>/`。

    python -X utf8 referee_desk.py        # 或双击 裁判台.bat

为什么用 tkinter：裁判机常常就是比赛机，开赛时不该再抢几百 MB。
标准库单窗口，实测常驻约 41 MB，零第三方依赖。
"""

import hashlib
import json
import os
import queue
import re
import shutil
import subprocess
import sys
import tempfile
import threading
import time
from pathlib import Path

try:
    import tkinter as tk
    from tkinter import filedialog, font as tkfont, messagebox, ttk
    from tkinter.scrolledtext import ScrolledText
except ModuleNotFoundError:  # 精简版 Python 常常不带 tk
    sys.stderr.write(
        "这个 Python 没有 tkinter。\n"
        "  · Windows 官方安装包自带；如果是精简发行版，请改用 `py -3 referee_desk.py`\n"
        "  · 或者直接双击 裁判台.bat（会自动挑一个带 tk 的解释器）\n"
    )
    raise SystemExit(2)

import importlib.util

APP_DIR = Path(__file__).resolve().parent
ENGINE = APP_DIR / "war_of_desert"
DEFAULT_CONTEST_ROOT = APP_DIR / "算法赛"
STAGE_ROOT = ENGINE / "workspace" / "_contestants"
REF_BOTS = ["Vanguard", "Bulwark", "Swarm", "Nomad", "Sapper", "Hunter", "Warden"]

# 赛务（发卷 / 开赛 / 暂停 / 收卷）与裁判台同目录。用路径加载，
# 避免模块名含中文时在某些 Python 上 import 失败。
_spec = importlib.util.spec_from_file_location("wod_service", APP_DIR / "赛务.py")
service = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(service)

PROGRESS_RE = re.compile(r"\[(\d+)\s*/\s*(\d+)\]")
ANSI_RE = re.compile(r"\x1b\[[0-9;]*m")
SLUG_RE = re.compile(r"[^0-9A-Za-z_-]+")

BG = "#1e1f22"
PANEL = "#2b2d31"
FG = "#e6e6e6"
MUTED = "#9aa0a6"
OK = "#5cb85c"
BAD = "#e05252"

# 版本号写进窗口标题：改完代码没重启，一眼就能看出来跑的是旧的。
DESK_VERSION = "1.10"
# 成绩归档：正式比赛与测试分开，互不覆盖。
MATCH_RECORD_DIR = "比赛记录"
TEST_RECORD_DIR = Path("测试") / "测试成绩"
# 比赛根目录下这些子目录是归档区，不是参赛工作区。
NON_CONTESTANT_DIRS = {"比赛记录", "测试"}


class _StopCheck(Exception):
    """自检撞上「再往下跑也没意义」的环境问题，直接收尾。"""


def find_go():
    """确定性地找到 go.exe，返回 (路径, 来源)。

    不能只信 PATH：双击 bat 起来的进程，PATH 常常和终端里不一样；PATH 里
    也可能先命中一个残留的 shim。找到之后一律用**绝对路径**调用，别让
    CreateProcess 再自己猜一遍。
    """
    cands = [
        (r"C:\Program Files\Go\bin\go.exe", "默认安装路径"),
        (r"C:\Program Files (x86)\Go\bin\go.exe", "默认安装路径(x86)"),
        (os.path.expandvars(r"%LOCALAPPDATA%\Programs\Go\bin\go.exe"), "用户安装路径"),
        (os.path.expandvars(r"%USERPROFILE%\scoop\apps\go\current\bin\go.exe"), "scoop"),
    ]
    for path, why in cands:
        try:
            if path and Path(path).is_file():
                return path, why
        except OSError:
            continue
    hit = shutil.which("go")
    if hit:
        return hit, "PATH"
    return None, "未找到"


# ---------------------------------------------------------------- 工作区发现
def go_files(d: Path, depth: int = 1) -> list:
    """列出目录下的 .go 文件（含一层子目录）。"""
    out = []
    if not d.is_dir():
        return out
    out.extend(sorted(d.glob("*.go")))
    if depth > 0:
        for sub in sorted(d.iterdir()):
            if sub.is_dir() and sub.name not in ("bin", "vendor", "testdata"):
                out.extend(sorted(sub.glob("*.go")))
    return out


def detect_source(ws: Path):
    """判断一个工作区里可编译的 main 包在哪。返回 (源子目录, 说明)。

    判定看整个 .go 文件而不是开头若干行：选手包模板的许可证/说明注释很长，
    按前缀截断会把合法的 decide.go 判成「没有 package main」。
    """
    for c in (ws, ws / "submission", ws / "workspace" / "submission", ws / "src", ws / "workspace"):
        if not c.is_dir():
            continue
        for f in sorted(c.glob("*.go")):
            try:
                # 整文件扫：选手包模板开头的注释比 600 字符还长，截断会漏判
                src_text = f.read_text(encoding="utf-8", errors="replace")[:1 << 16]
            except OSError:
                continue
            if "package main" in src_text and "func main(" in src_text:
                # 必须返回相对工作区的完整路径：workspace/submission 只取
                # c.name 会得到 "submission"，暂存后按这个子路径找源码就找错了。
                rel = "." if c == ws else c.relative_to(ws).as_posix()
                return rel, "入口 %s" % f.name
    if not go_files(ws):
        return None, "没有 .go 文件（空工作区？）"
    return None, "没找到 package main"


def pick_frozen(stage: Path):
    """找收卷副本：优先 交卷记录.json 里登记的 file，其次扫 *_bot.go / decide.go。"""
    d = stage / "交卷"
    if not d.is_dir():
        return None
    rec = d / "交卷记录.json"
    if rec.is_file():
        try:
            name = json.loads(rec.read_text(encoding="utf-8")).get("file")
            if name and (d / name).is_file():
                return d / name
        except Exception:  # noqa: BLE001
            pass
    for p in sorted(d.glob("*_bot.go")) + sorted(d.glob("decide.go")):
        return p
    return None


def slugify(name: str) -> str:
    """工作区名 → 暂存目录名。

    必须是纯 ASCII：暂存目录在 Go 模块里，导入路径含中文会直接
    “malformed import path ... invalid char”。全中文的名字退回短哈希。
    """
    s = SLUG_RE.sub("_", name).strip("_")
    if not s:
        return "bot_" + hashlib.sha1(name.encode("utf-8")).hexdigest()[:6]
    if re.search(r"[^0-9A-Za-z_-]", name):
        # 原名含中文等非 ASCII：补个短哈希，避免 模板Bot / 测试Bot 撞成同一个 Bot
        s = "%s_%s" % (s, hashlib.sha1(name.encode("utf-8")).hexdigest()[:4])
    return s


def tree_digest(root: Path) -> str:
    h = hashlib.sha256()
    for p in sorted(root.rglob("*.go")):
        h.update(p.relative_to(root).as_posix().encode("utf-8"))
        h.update(b"\x00")
        h.update(p.read_bytes())
        h.update(b"\x00")
    return h.hexdigest()


# ---------------------------------------------------------------- 赛程
def seed_list(n: int) -> list:
    return [20261109 + i * 7 for i in range(max(1, n))]


def discover_players(root: Path) -> list:
    """参赛名单：预编译基线（root 下的 baseline@*.exe）+ 选手目录。"""
    players = []
    for exe in sorted(root.glob("baseline@*.exe")):
        players.append({"name": exe.stem, "kind": "baseline", "path": exe.name})
    for d in sorted(root.iterdir()):
        if d.is_dir() and not d.name.startswith("_") and d.name not in NON_CONTESTANT_DIRS:
            players.append({"name": d.name, "kind": "contestant", "path": d.name})
    return players


def build_schedule(root: Path, seeds: int) -> dict:
    players = discover_players(root)
    ms = seed_list(seeds)
    matches = []
    for seed in ms:
        for i in range(len(players)):
            for j in range(i + 1, len(players)):
                matches.append({"id": len(matches), "seed": seed,
                                "a": players[i]["name"], "b": players[j]["name"],
                                "mirror": True})
    return {
        "version": 1,
        "createdBy": "referee_desk.py · 初始化赛程",
        "createdAt": time.strftime("%Y-%m-%dT%H:%M:%S"),
        "rulesVersion": "1.4",
        "engine": "native",
        "seeds": ms,
        "mirror": True,
        "players": players,
        "matches": matches,
        "note": "对阵表按「所有报名者两两相遇 × 每个 seed 换边两局」冻结；"
                "实际开跑时只有已交付且被勾选的选手上场。",
    }


# ---------------------------------------------------------------- 主窗口
class RefereeDesk(tk.Tk):
    def __init__(self):
        super().__init__()
        self.title("Desert Arena · 裁判台 v%s" % DESK_VERSION)
        self.geometry("1120x820")
        self.minsize(980, 720)
        self.configure(bg=BG)

        self.status = tk.StringVar(value="就绪")
        self.contest_root = tk.StringVar(value=str(DEFAULT_CONTEST_ROOT))

        self.proc = None
        self.log_q = queue.Queue()
        self.running = False
        self.workspaces = []  # [(path, label, src, note)]
        self.last_out = None
        # 每个工作区本次会话是否自检通过：str(路径) -> bool。
        # 只用来在「开始编码」时提醒一句，不作为硬门槛——硬门槛是选手包真的齐备。
        self.checked = {}
        self._auto_frozen = set()  # 已自动封存过的路径，避免重复写盘
        self._emit = self.log_q.put
        # 页签各自一套控件与规则选择：编码 / 比赛 / 测试互不干扰。
        self.code_uis = {}    # scope -> 编码流水线控件
        self.battle_uis = {}  # scope -> 比赛控件
        self.rules_vars = {}  # scope -> 规则版本 StringVar
        self.last_record = {}  # scope -> 最近一次成绩目录
        # 一开始就把 go 定下来：双击 bat 启动的进程 PATH 常常和终端不一样，
        # 等真去编译时才发现就没法解释「为什么终端能过、界面不能过」。
        self.go_exe, self.go_from = find_go()
        # 无头模式（--scan / --init / --selfcheck / --smoke-contest）不往 Tk 里投回调。
        # 这些模式没有 mainloop，任何 self.after() 都会报 "main thread is not in
        # main loop"，所以必须在建界面前就认出来。
        self.headless = any(a in sys.argv for a in
                            ("--smoke-contest", "--scan", "--init", "--selfcheck"))

        self._build_styles()
        self._build_ui()
        self.scan_workspaces()
        if not self.headless:
            # 开局就把版本和 go 的位置写进日志：以后看到「编译失败但没这行」，
            # 就知道点的是旧窗口，不用再猜。
            self._log("— 裁判台 v%s · %s —\n" % (DESK_VERSION, time.strftime("%H:%M:%S")), "dim")
            if self.go_exe:
                self._log("   go：%s（%s）\n" % (self.go_exe, self.go_from), "dim")
            else:
                self._log("   go：未找到（自检会因此失败）\n", "err")
        self.after(80, self._drain_log)
        self.after(1000, self._tick)

    # ---------------- 样式 ----------------
    def _build_styles(self):
        mono = "Consolas" if sys.platform == "win32" else "Menlo"
        self.log_font = tkfont.Font(family=mono, size=10)
        style = ttk.Style(self)
        try:
            style.theme_use("clam")
        except tk.TclError:
            pass
        style.configure("TCombobox", fieldbackground=PANEL, background=PANEL, foreground=FG)
        style.configure("TButton", padding=(10, 5))
        style.configure("Go.TButton", padding=(12, 6), background="#2f5d3a", foreground="#ffffff")
        style.map("Go.TButton", background=[("active", "#3b7a4a")])
        style.configure("TNotebook", background=BG, borderwidth=0)
        style.configure("TNotebook.Tab", padding=(14, 6))

    # ---------------- 界面 ----------------
    def _build_ui(self):
        nb = ttk.Notebook(self)
        nb.pack(fill="x", padx=14, pady=(12, 4))
        self.nb = nb
        # 三个页签：编码（只开时间窗）/ 比赛（正式对局）/ 测试（两者都有，成绩单独归档）。
        nb.add(self._build_code_tab(nb), text="  编码  ")
        nb.add(self._build_match_tab(nb), text="  比赛  ")
        nb.add(self._build_test_tab(nb), text="  测试  ")

        prog = tk.Frame(self, bg=BG)
        prog.pack(fill="x", padx=14, pady=(8, 0))
        self.pbar = ttk.Progressbar(prog, mode="determinate", maximum=100)
        self.pbar.pack(side="left", fill="x", expand=True)
        self.stop_btn = ttk.Button(prog, text="中止", command=self.stop, state="disabled")
        self.stop_btn.pack(side="right", padx=(8, 0))
        tk.Label(prog, textvariable=self.status, bg=BG, fg=MUTED, width=30,
                 anchor="e").pack(side="right")

        box = tk.Frame(self, bg=BG)
        box.pack(fill="both", expand=True, padx=14, pady=(6, 12))
        self.log = ScrolledText(box, bg="#141517", fg=FG, insertbackground=FG,
                                font=self.log_font, relief="flat", wrap="none", height=16)
        self.log.pack(fill="both", expand=True)
        self.log.tag_config("err", foreground=BAD)
        self.log.tag_config("ok", foreground=OK)
        self.log.tag_config("dim", foreground=MUTED)
        self.log.configure(state="disabled")

    def _build_code_tab(self, parent):
        tab = tk.Frame(parent, bg=BG)
        self._build_code_section(tab, "code")
        return tab

    def _build_match_tab(self, parent):
        tab = tk.Frame(parent, bg=BG)
        self._build_battle_section(tab, "official")
        return tab

    def _build_test_tab(self, parent):
        """测试 = 编码 + 比赛。两段共用同一批工作区，成绩只写测试目录。"""
        tab = tk.Frame(parent, bg=BG)
        nb = ttk.Notebook(tab)
        nb.pack(fill="both", expand=True, padx=2, pady=2)
        code = tk.Frame(nb, bg=BG)
        battle = tk.Frame(nb, bg=BG)
        nb.add(code, text="  测试 · 编码  ")
        nb.add(battle, text="  测试 · 比赛  ")
        self._build_code_section(code, "test")
        self._build_battle_section(battle, "test")
        return tab

    def _build_code_section(self, parent, scope):
        """选工作区 → 自检 / 发卷 / 提示词 / 开始编码 / 收卷封存。"""
        ui = {"scope": scope, "steps": {}, "btns": {},
              "summary_var": tk.StringVar()}
        self.code_uis[scope] = ui
        self.rules_vars[scope] = tk.StringVar(value="v1.9")

        row = tk.Frame(parent, bg=BG)
        row.pack(fill="x", padx=12, pady=(10, 4))
        tk.Label(row, text="比赛根目录", bg=BG, fg=MUTED).pack(side="left")
        tk.Entry(row, textvariable=self.contest_root, bg=PANEL, fg=FG,
                 insertbackground=FG, relief="flat", width=40).pack(side="left", padx=8, ipady=3)
        ttk.Button(row, text="浏览", command=self.browse_root).pack(side="left")
        ttk.Button(row, text="扫描", command=self.scan_workspaces).pack(side="left", padx=8)
        tk.Label(row, text="比赛规则版本", bg=BG, fg=MUTED).pack(side="left", padx=(8, 4))
        rules_cb = ttk.Combobox(row, textvariable=self.rules_vars[scope],
                                values=("v1.9", "v1.4"), width=6, state="readonly")
        rules_cb.pack(side="left")
        rules_cb.bind("<<ComboboxSelected>>", lambda _e, s=scope: self._on_rules_change(s))

        rules_bar = tk.Frame(parent, bg=BG)
        rules_bar.pack(fill="x", padx=12, pady=(0, 4))
        tk.Label(rules_bar, text="当前规则：", bg=BG, fg=MUTED).pack(side="left", anchor="n")
        tk.Label(rules_bar, textvariable=ui["summary_var"], bg=BG, fg=FG,
                 font=(None, 9), justify="left", wraplength=760).pack(
            side="left", anchor="w")

        body = tk.Frame(parent, bg=BG)
        body.pack(fill="both", expand=True, padx=12, pady=(4, 10))
        body.grid_rowconfigure(0, weight=1)
        body.grid_columnconfigure(0, weight=1)
        # 右列固定最小宽度：状态行初始只有「—」，按自然宽度会让按钮挤到窗口外。
        body.grid_columnconfigure(1, weight=0, minsize=370)

        left = tk.Frame(body, bg=BG)
        left.grid(row=0, column=0, sticky="nsew", padx=(0, 12))
        tk.Label(left, text="选择工作区（目录名形如 RTS算法赛@<模型名>）",
                 bg=BG, fg=MUTED, font=(None, 9)).pack(anchor="w")
        wrap = tk.Frame(left, bg=BG)
        wrap.pack(fill="both", expand=True, pady=(4, 0))
        lst = tk.Listbox(wrap, selectmode="browse", height=6, bg=PANEL, fg=FG,
                         selectbackground="#3a5f8a", relief="flat",
                         font=("Consolas", 10), activestyle="none",
                         exportselection=False)
        lst.pack(side="left", fill="both", expand=True)
        sb = tk.Scrollbar(wrap, command=lst.yview, bg=PANEL)
        sb.pack(side="left", fill="y")
        lst.configure(yscrollcommand=sb.set)
        lst.bind("<<ListboxSelect>>", lambda _e, s=scope: self._on_code_select(s))
        ui["lst"] = lst

        right = tk.Frame(body, bg=BG)
        right.grid(row=0, column=1, sticky="nsew")
        self._build_code_steps(right, ui)

        tk.Label(parent, text="这一段只负责「给参赛 AI 开一段写代码的时间窗」：发卷把包铺好，"
                              "开始编码只是计时，都不跑对局。提示词要你自己复制、自己发送。",
                 bg=BG, fg=MUTED, font=(None, 9)).pack(anchor="w", padx=12, pady=(0, 8))
        self._on_rules_change(scope)

    def _step_block(self, parent, ui, key, title):
        """一步 = 标题与按钮同排 + 一行状态。状态行由 _refresh_code_ui 统一刷新。"""
        box = tk.Frame(parent, bg=PANEL)
        box.pack(fill="x", pady=(0, 4))
        head = tk.Frame(box, bg=PANEL)
        head.pack(fill="x", padx=9, pady=(6, 2))
        tk.Label(head, text=title, bg=PANEL, fg=FG, font=(None, 9, "bold"),
                 anchor="w").pack(side="left")
        btns = tk.Frame(head, bg=PANEL)
        btns.pack(side="right")
        lbl = tk.Label(box, text="—", bg=PANEL, fg=MUTED, font=(None, 8),
                       anchor="w", justify="left", wraplength=330)
        lbl.pack(fill="x", padx=9, pady=(0, 6))
        ui["steps"][key] = {"btns": btns, "lbl": lbl}
        return btns

    def _build_code_steps(self, parent, ui):
        scope = ui["scope"]
        b = self._step_block(parent, ui, "chk", "① 工作区自检")
        ui["btns"]["chk"] = ttk.Button(b, text="自检", width=8,
                                       command=lambda: self.selfcheck(scope))
        ui["btns"]["chk"].pack(side="left")

        b = self._step_block(parent, ui, "issue", "② 发卷")
        ui["btns"]["issue"] = ttk.Button(b, text="发卷", width=8,
                                         command=lambda: self.issue_package(scope))
        ui["btns"]["issue"].pack(side="left")

        b = self._step_block(parent, ui, "prompt", "③ 提示词")
        ui["btns"]["copy"] = ttk.Button(b, text="复制", width=5,
                                        command=lambda: self.copy_prompt(scope))
        ui["btns"]["copy"].pack(side="left")
        ui["btns"]["view"] = ttk.Button(b, text="查看", width=5,
                                        command=lambda: self.view_prompt(scope))
        ui["btns"]["view"].pack(side="left", padx=(4, 0))

        b = self._step_block(parent, ui, "start", "④ 开始编码")
        ui["btns"]["start"] = ttk.Button(b, text="开始编码", width=9, style="Go.TButton",
                                         command=lambda: self.start_contest_clock(scope))
        ui["btns"]["start"].pack(side="left")
        ui["btns"]["pause"] = ttk.Button(b, text="暂停", width=5,
                                         command=lambda: self.pause_timer(scope))
        ui["btns"]["pause"].pack(side="left", padx=(4, 0))
        ui["btns"]["resume"] = ttk.Button(b, text="继续", width=5,
                                          command=lambda: self.resume_timer(scope))
        ui["btns"]["resume"].pack(side="left", padx=(4, 0))

        b = self._step_block(parent, ui, "freeze", "⑤ 收卷封存")
        ui["btns"]["freeze"] = ttk.Button(b, text="立即收卷", width=9,
                                          command=lambda: self.freeze_timer(scope))
        ui["btns"]["freeze"].pack(side="left")

    def _build_battle_section(self, parent, scope):
        """选择参赛 Bot、规则版本，调用官方 arena 对局并归档成绩。"""
        ui = {"scope": scope, "summary_var": tk.StringVar(),
              "seed_var": tk.StringVar(value="1")}
        self.battle_uis[scope] = ui
        if scope not in self.rules_vars:
            self.rules_vars[scope] = tk.StringVar(value="v1.9")

        dest = "算法赛/比赛记录" if scope == "official" else "算法赛/测试/测试成绩"
        row = tk.Frame(parent, bg=BG)
        row.pack(fill="x", padx=12, pady=(10, 0))
        tk.Label(row, text="比赛根目录", bg=BG, fg=MUTED).pack(side="left")
        tk.Entry(row, textvariable=self.contest_root, bg=PANEL, fg=FG,
                 insertbackground=FG, relief="flat", width=40).pack(side="left", padx=8, ipady=3)
        ttk.Button(row, text="浏览", command=self.browse_root).pack(side="left")
        ttk.Button(row, text="扫描", command=self.scan_workspaces).pack(side="left", padx=8)

        tk.Label(parent, text="选择参赛工作区的 Bot：与 baseline@1 / baseline@2 对局，"
                              "按 seed 换边，跑完出 BT 成绩。成绩写入 %s/。" % dest,
                 bg=BG, fg=MUTED, font=(None, 9), justify="left").pack(
            anchor="w", padx=12, pady=(12, 6))

        wrap = tk.Frame(parent, bg=BG)
        wrap.pack(fill="both", expand=True, padx=12)
        lst = tk.Listbox(wrap, selectmode="extended", height=8, bg=PANEL, fg=FG,
                         selectbackground="#3a5f8a", relief="flat",
                         font=("Consolas", 10), activestyle="none",
                         exportselection=False)
        lst.pack(side="left", fill="both", expand=True)
        sb = tk.Scrollbar(wrap, command=lst.yview, bg=PANEL)
        sb.pack(side="left", fill="y")
        lst.configure(yscrollcommand=sb.set)
        lst.bind("<<ListboxSelect>>", lambda _e, s=scope: self._on_battle_select(s))
        lst.bind("<Double-Button-1>", lambda _e: "break")
        ui["lst"] = lst

        bar = tk.Frame(parent, bg=BG)
        bar.pack(fill="x", padx=12, pady=(8, 10))
        ttk.Button(bar, text="全选", command=lambda: self.select_all(scope)).pack(side="left")
        ttk.Button(bar, text="清空", command=lambda: self.select_none(scope)).pack(
            side="left", padx=6)
        tk.Label(bar, text="seed 数", bg=BG, fg=MUTED).pack(side="left", padx=(16, 4))
        ttk.Combobox(bar, textvariable=ui["seed_var"], values=("1", "2", "3"),
                     width=4, state="readonly").pack(side="left")
        tk.Label(bar, text="比赛规则版本", bg=BG, fg=MUTED).pack(side="left", padx=(16, 4))
        rules_cb = ttk.Combobox(bar, textvariable=self.rules_vars[scope],
                                values=("v1.9", "v1.4"), width=6, state="readonly")
        rules_cb.pack(side="left")
        rules_cb.bind("<<ComboboxSelected>>", lambda _e, s=scope: self._on_rules_change(s))
        ui["run_btn"] = ttk.Button(bar, text="开始对战（0）", style="Go.TButton",
                                   command=lambda: self.run_contest(scope))
        ui["run_btn"].pack(side="left", padx=(16, 0))
        ui["view_btn"] = ttk.Button(bar, text="查看成绩",
                                    command=lambda: self.open_contest_results(scope))
        ui["view_btn"].pack(side="left", padx=8)

        rules_bar = tk.Frame(parent, bg=BG)
        rules_bar.pack(fill="x", padx=12, pady=(0, 6))
        tk.Label(rules_bar, text="当前规则：", bg=BG, fg=MUTED).pack(side="left", anchor="n")
        tk.Label(rules_bar, textvariable=ui["summary_var"], bg=BG, fg=FG,
                 font=(None, 9), justify="left", wraplength=980).pack(
            side="left", anchor="w")
        self._on_rules_change(scope)

    # ---------------- 工作区 ----------------
    def browse_root(self):
        d = filedialog.askdirectory(initialdir=self.contest_root.get() or str(APP_DIR),
                                    title="选择比赛根目录")
        if d:
            self.contest_root.set(d)
            self.scan_workspaces()

    def _row_text(self, d: Path):
        """列表一行的文案。返回 (文本, 是否可编译)。

        不用 %-34s 对齐：中文是双宽，按字符数补空格对不齐。直接用分隔符切段，
        「N go」放最后——它变，说明智能体真的在写东西。
        """
        src, note = detect_source(d)
        try:
            srv = service.describe(d)
        except Exception:  # noqa: BLE001
            srv = "—"
        note = note.replace("入口 ", "")
        if src is None:
            return "%s  ×  %s  ·  %s" % (d.name, note, srv), False
        return "%s  ✓  %s  ·  %s  ·  %d go" % (
            d.name, note, srv, len(go_files(d))), True

    def scan_workspaces(self, keep=None):
        """扫描比赛根目录，刷新所有页签的工作区列表。

        keep 可以是目录名（所有页签沿用）或 {scope: 目录名}。这样自动封存
        触发的刷新不会把用户正在看的那一行弄丢。
        """
        if keep is None:
            keep = {}
            for scope in self.code_uis:
                cur = self.selected_one(scope)
                if cur:
                    keep[scope] = cur[1]
        elif isinstance(keep, str):
            keep = {scope: keep for scope in self.code_uis}
        root = Path(self.contest_root.get().strip() or DEFAULT_CONTEST_ROOT)
        for ui in self.code_uis.values():
            ui["lst"].delete(0, "end")
        for ui in self.battle_uis.values():
            ui["lst"].delete(0, "end")
        self.workspaces = []
        if not root.is_dir():
            self._log("比赛根目录不存在：%s\n" % root, "err")
            self.status.set("目录不存在")
            return
        self.root = root
        dirs = [root] if detect_source(root)[0] else [
            d for d in sorted(root.iterdir())
            if d.is_dir() and not d.name.startswith("_")
            and d.name not in NON_CONTESTANT_DIRS]
        for d in dirs:
            text, ok = self._row_text(d)
            for ui in self.code_uis.values():
                ui["lst"].insert("end", text)
                if not ok:
                    ui["lst"].itemconfig("end", foreground="#8a8f98")
            for ui in self.battle_uis.values():
                ui["lst"].insert("end", text)
                if not ok:
                    ui["lst"].itemconfig("end", foreground="#8a8f98")
            src, note = detect_source(d)
            self.workspaces.append((d, d.name, src, note))
        ok_n = sum(1 for w in self.workspaces if w[2])
        self.status.set("扫描到 %d 个，可用 %d 个" % (len(self.workspaces), ok_n))

        for scope, ui in self.code_uis.items():
            name = keep.get(scope)
            if name:
                for i, w in enumerate(self.workspaces):
                    if w[1] == name:
                        ui["lst"].selection_set(i)
                        ui["lst"].see(i)
                        break
            self._refresh_code_ui(scope)
        for scope in self.battle_uis:
            self._on_battle_select(scope)

    def _on_code_select(self, scope, _evt=None):
        """工作区列表是单选：选谁就对着谁走 ①~⑤。"""
        self._refresh_code_ui(scope)

    def _on_battle_select(self, scope, _evt=None):
        n = len(self.picked(scope))
        ui = self.battle_uis.get(scope)
        if ui and ui.get("run_btn"):
            ui["run_btn"].configure(text="开始对战（%d）" % n)

    def select_all(self, scope):
        ui = self.battle_uis.get(scope)
        if not ui:
            return
        for i, w in enumerate(self.workspaces):
            if w[2]:
                ui["lst"].selection_set(i)

    def select_none(self, scope):
        ui = self.battle_uis.get(scope)
        if not ui:
            return
        ui["lst"].selection_clear(0, "end")
        self._on_battle_select(scope)

    def selected_one(self, scope):
        """该页签当前选中的单个工作区（允许还是空的——发卷就是给空目录用的）。"""
        ui = self.code_uis.get(scope)
        if not ui:
            return None
        idx = ui["lst"].curselection()
        if len(idx) != 1:
            return None
        i = idx[0]
        return self.workspaces[i] if i < len(self.workspaces) else None

    def _need_one(self, scope, action: str):
        w = self.selected_one(scope)
        if w is None:
            messagebox.showwarning("裁判台", "%s：先在左边列表里选中一个工作区" % action)
            return None
        return w

    # ---------------- ① 工作区自检 ----------------
    def _dump_diag(self, ws: Path, shots: list) -> Path:
        """把这次自检的原始证据落到文件里。

        只写一句「编译失败」帮不上忙，尤其当子进程一声不吭地退出时。这里把
        实际用的 go 路径、关键环境变量、每条命令的 argv/cwd/退出码/原始文本
        全留下来，出问题直接看这个文件。
        """
        env_keys = ("PATH", "GOROOT", "GOPATH", "GOCACHE", "GOFLAGS", "GOWORK",
                    "GOTOOLCHAIN", "GOOS", "GOARCH", "TEMP", "USERPROFILE")
        out = ["Desert Arena 裁判台 · 工作区自检诊断",
               "生成时间：%s" % time.strftime("%Y-%m-%d %H:%M:%S"),
               "裁判台版本：v%s" % DESK_VERSION,
               "Python：%s" % sys.version.replace("\n", " "),
               "工作区：%s" % ws,
               "go 解析：%s（%s）" % (self.go_exe or "未找到", self.go_from),
               "", "—— 环境变量 ——"]
        for k in env_keys:
            out.append("%-12s = %s" % (k, os.environ.get(k, "（未设置）")))
        out += ["", "—— 命令 ——"]
        for s in shots:
            out.append("")
            out.append("[%s]" % s["label"])
            out.append("  argv : %s" % s["argv"])
            out.append("  cwd  : %s" % s["cwd"])
            out.append("  code : %s (0x%08X)" % (s["code"], s["code"] & 0xFFFFFFFF))
            out.append("  bytes: %d" % s["out_len"])
            body = s["out"].strip()
            if body:
                for ln in body.splitlines():
                    out.append("  | " + ln)
            else:
                out.append("  | （子进程没有任何输出）")
        p = APP_DIR / "自检诊断.txt"
        try:
            p.write_text("\n".join(out) + "\n", encoding="utf-8")
        except OSError:
            return APP_DIR
        return p

    def _check_workspace(self, ws: Path, mode: str = "v1.9"):
        """自检的实际动作：发卷 + 查结构 + 验 go + 两处编译。

        抽成独立方法，是因为 GUI 和 `--selfcheck` 命令行要跑同一套判定。
        返回 (行列表, 是否通过)，每行是 (文本, log 标签)。
        """
        lines = []
        shots = []  # 失败时写进 自检诊断.txt 的原始记录
        ok = True
        go = self.go_exe or "go"

        def run(argv, cwd, label):
            code, out = self._exec(argv, cwd=cwd)
            shots.append({"label": label, "argv": argv, "cwd": str(cwd),
                          "code": code, "out": out or "", "out_len": len(out or "")})
            return code, out

        try:
            # 每次都走一遍发卷：它只补缺件、不覆盖已有文件，顺带把 arena/ 刷新成
            # 当前引擎版本。老工作区缺新文件（例如后来才加的 RULES_V15.md）时，
            # 自检就是补发点，否则会一直卡在「缺件」上。
            got = [x for x in service.issue(ws) if not x.startswith("arena/")]
            if got:
                lines.append(("发卷：补发 %s（引擎副本已刷新）" % "、".join(got), "dim"))
            else:
                lines.append(("发卷：选手包已齐备，不覆盖已有文件", "dim"))
            # 复制来的目录不会走 issue()，交付文件可能还叫 Model_name_bot.go。
            # 这里顺手改名：否则自检显示「交付文件 None」，AI 再写一份
            # <选手名>_bot.go 就是同包重复定义，编译直接失败。
            renamed = service.normalize_submission(ws)
            if renamed:
                lines.append(("交付文件：%s → %s"
                              % (renamed, service.bot_filename(ws)), "dim"))

            chk = service.check_package(ws)
            if chk["missing"]:
                ok = False
                lines.append(("结构：缺 %s" % "、".join(chk["missing"]), "err"))
            elif not chk["submission"]:
                ok = False
                lines.append(("结构：submission/ 里没有 %s（智能体无处写代码）"
                              % service.bot_filename(ws), "err"))
            else:
                lines.append(("结构：%d 项齐备，交付文件 %s"
                              % (len(service.KEY_ITEMS), chk["submission"]), "ok"))
            if chk["is_template"]:
                lines.append(("提示：交付文件还是模板内容，等智能体来改", "dim"))

            # 规则版本：以引擎源码为准，再比对工作区 arena 副本。
            # 副本是刚发卷刷新过的，若仍不一致，说明本地调试环境和评测不是同一套规则。
            want = self._sim_rule_version(ENGINE / "internal" / "sim", mode)
            have = self._sim_rule_version(
                ws / "workspace" / "arena" / "internal" / "sim", mode)
            head = "v1.9.1 当前" if mode.startswith("v1.9") else "v1.4 历史"
            if want and have == want:
                lines.append(("规则：%s · 引擎与 arena 副本都是 %s" % (head, want), "dim"))
            elif want and have:
                ok = False
                lines.append(("规则：%s · 引擎 %s，但 arena 副本是 %s（本地调试与评测不一致）"
                              % (head, want, have), "err"))
            else:
                lines.append(("规则：%s · 版本号读取失败（%s）" % (head, want or have or "无"), "err"))
                ok = False

            # 用绝对路径调 go：CreateProcess 每次自己猜 PATH，猜错就只会
            # 留一个空白的「编译失败：」。这里先探一次，探不通就停。
            lines.append(("go：%s（%s）" % (self.go_exe or "未找到", self.go_from), "dim"))
            code, out = run([go, "version"], ws / "workspace", "go version")
            if code != 0:
                ok = False
                lines.append(("环境：go 不可用。%s" % self._cmd_fail(code, out), "err"))
                raise _StopCheck()
            first = (out.strip().splitlines() or ["go"])[0]
            lines.append(("环境：%s" % first, "dim"))

            # 编译选手代码。产物丢临时目录，别在工作区里留 bot.exe 骗人。
            tmp = Path(tempfile.mkdtemp(prefix="wod-selfcheck-"))
            try:
                code, out = run([go, "build", "-o", str(tmp / "sub.exe"), "./submission"],
                                ws / "workspace", "go build ./submission")
            finally:
                shutil.rmtree(tmp, ignore_errors=True)
            if code == 0:
                lines.append(("编译：go build ./submission 通过", "ok"))
            else:
                ok = False
                lines.append(("编译失败：%s" % self._cmd_fail(code, out), "err"))

            # 引擎副本能不能编：AI 要双击 本地对局.bat，这份坏了就是白等两小时。
            arena_out = Path(tempfile.gettempdir()) / "wod-arena-selfcheck.exe"
            code, out = run([go, "build", "-o", str(arena_out), "./cmd/arena"],
                            ws / "workspace" / "arena", "go build ./cmd/arena")
            if code == 0:
                lines.append(("引擎：arena 副本可编译", "ok"))
            else:
                ok = False
                lines.append(("引擎副本编译失败：%s" % self._cmd_fail(code, out), "err"))
        except _StopCheck:
            ok = False
        except Exception as e:  # noqa: BLE001
            ok = False
            lines.append(("自检出错：%s" % e, "err"))

        if not ok and shots:
            p = self._dump_diag(ws, shots)
            lines.append(("诊断：完整证据已写入 %s" % p, "dim"))
        return lines, ok

    def selfcheck(self, scope):
        """发卷 + 查结构 + 编译。

        这是「开始编码」之前唯一的硬门槛。选手包缺件的话，AI 那边一上来就是断的：
        没 prompt.md 不知道要干什么，没 arena/ 本地跑不起来，没 submission/ 无处可写。
        只做检查，不跑任何对局。
        """
        if self.running:
            return
        w = self._need_one(scope, "工作区自检")
        if not w:
            return
        ws, label = w[0], w[1]
        mode = self.rules_vars[scope].get()
        self._set_running(True)
        self.status.set("自检中…")
        self._log("— 工作区自检：%s（规则 %s）—\n" % (label, mode), "dim")

        def work():
            lines, ok = self._check_workspace(ws, mode)

            def apply():
                for text, tag in lines:
                    self._log(text + "\n", tag)
                self.checked[str(ws)] = ok
                self.status.set("自检通过" if ok else "自检未通过")
                self._log("— 自检%s —\n" % ("通过" if ok else "未通过"),
                          "ok" if ok else "err")
                self._set_running(False)
                self.scan_workspaces(keep=label)

            self.after(0, apply)

        threading.Thread(target=work, daemon=True).start()

    # ---------------- ② 发卷 ----------------
    def issue_package(self, scope):
        """只发卷：把模板工作区铺进选手目录，不检查、不计时、不跑对局。"""
        if self.running:
            return
        w = self._need_one(scope, "发卷")
        if not w:
            return
        ws, label = w[0], w[1]
        self.status.set("发卷中…")
        try:
            got = [x for x in service.issue(ws) if not x.startswith("arena/")]
        except Exception as e:  # noqa: BLE001
            messagebox.showerror("裁判台", "发卷失败：%s" % e)
            return
        self._log("发卷 %s：%s（引擎副本已刷新）\n"
                  % (label, "、".join(got) if got else "已齐备，不覆盖已有文件"),
                  "ok" if got else "dim")
        self.status.set("发卷完成")
        self.scan_workspaces(keep=label)

    # ---------------- ③ 提示词（人手动发，裁判台不代发） ----------------
    def _prompt_text(self, ws: Path):
        p = service.prompt_path(ws)
        if not p.is_file():
            messagebox.showwarning("裁判台", "还没有 prompt.md。先做 ① 工作区自检。")
            return None
        try:
            return p.read_text(encoding="utf-8")
        except OSError as e:
            messagebox.showerror("裁判台", "读 prompt.md 失败：%s" % e)
            return None

    def copy_prompt(self, scope):
        w = self._need_one(scope, "复制提示词")
        if not w:
            return
        text = self._prompt_text(w[0])
        if text is None:
            return
        self.clipboard_clear()
        self.clipboard_append(text)
        self.update()
        self._log("提示词已复制（%d 字）：去智能体那边粘贴发送，裁判台不代发\n"
                  % len(text), "ok")

    def view_prompt(self, scope):
        w = self._need_one(scope, "查看提示词")
        if not w:
            return
        text = self._prompt_text(w[0])
        if text is None:
            return
        win = tk.Toplevel(self)
        win.title("提示词 · %s" % w[1])
        win.configure(bg=BG)
        win.geometry("780x640")
        bar = tk.Frame(win, bg=BG)
        bar.pack(fill="x", padx=10, pady=(8, 4))
        tk.Label(bar, text="只读。要发出去点「复制」，然后自己去粘贴。",
                 bg=BG, fg=MUTED, font=(None, 9)).pack(side="left")
        box = tk.Text(win, bg=PANEL, fg=FG, wrap="word", relief="flat",
                      font=self.log_font, padx=10, pady=8)
        box.pack(fill="both", expand=True, padx=10, pady=(0, 10))
        box.insert("1.0", text)
        box.configure(state="disabled")

        def do_copy():
            win.clipboard_clear()
            win.clipboard_append(text)
            self._log("提示词已复制（%d 字）\n" % len(text), "ok")

        ttk.Button(bar, text="复制", command=do_copy).pack(side="right")

    # ---------------- ④ 开始编码 / 暂停 / 继续 ----------------
    def start_contest_clock(self, scope):
        """开始编码计时。**不跑对局**——对局在「比赛」页签，必须另外手动触发。"""
        if self.running:
            return
        w = self._need_one(scope, "开始编码")
        if not w:
            return
        ws, label = w[0], w[1]
        chk = service.check_package(ws)
        if not chk["ready"]:
            messagebox.showwarning(
                "裁判台",
                "%s 的选手包还不完整：\n  %s\n\n先点 ① 工作区自检把包铺好。"
                % (label, "、".join(chk["missing"]) or "没有交付文件"))
            return
        st = service.status(ws)
        lines = ["为 %s 开始编码计时。" % label, "",
                 "  · 只开始计时，不会跑任何对局",
                 "  · 90 分钟目标，120 分钟到点自动收卷",
                 "  · 提示词要你自己复制、自己发送"]
        if st["state"] != "未开赛":
            lines.insert(1, "它已经在计时（%s），这次会把起点重置为现在。"
                         % service.describe(ws))
            lines.insert(2, "")
        if not self.checked.get(str(ws)):
            lines += ["", "这次会话还没做过 ① 工作区自检。仍然开始？"]
        if not messagebox.askokcancel("开始编码（开始计时）", "\n".join(lines),
                                      default=messagebox.CANCEL):
            self._log("已取消：没有开始计时\n", "dim")
            return
        try:
            service.start(ws)
        except Exception as e:  # noqa: BLE001
            messagebox.showerror("裁判台", "启动失败：%s" % e)
            return
        self._auto_frozen.discard(str(ws))
        self._log("开始编码 %s：已开始计时（90 目标 / 120 强制收卷），本次不跑对局\n"
                  % label, "ok")
        self.scan_workspaces(keep=label)

    # ---------------- ④ 暂停 / 继续 ----------------
    def pause_timer(self, scope):
        w = self._need_one(scope, "暂停")
        if not w:
            return
        service.pause(w[0], "裁判台手动暂停")
        self._log("暂停 %s：已用 %.1f 分钟\n"
                  % (w[1], service.elapsed_seconds(w[0]) / 60), "dim")
        self.scan_workspaces(keep=w[1])

    def resume_timer(self, scope):
        w = self._need_one(scope, "继续")
        if not w:
            return
        service.resume(w[0])
        self._log("继续 %s：已用 %.1f 分钟\n"
                  % (w[1], service.elapsed_seconds(w[0]) / 60), "dim")
        self.scan_workspaces(keep=w[1])

    # ---------------- ⑤ 收卷封存 ----------------
    def _seal_one(self, ws: Path, label: str, auto: bool = False):
        """冻结交付文件到 交卷/。返回 info 或 None。"""
        try:
            info = service.freeze(ws)
        except Exception as e:  # noqa: BLE001
            self._log("%s封存 %s 失败：%s\n" % ("自动" if auto else "", label, e), "err")
            return None
        head = "自动封存" if auto else "收卷"
        if info["missing_submission"]:
            self._log("%s %s：没有交付文件，记为未提交\n" % (head, label), "err")
        else:
            self._log("%s %s：%d 字节，用 %.1f 分钟，sha256=%s\n"
                      % (head, label, info["bytes"], info["used_minutes"],
                         info["sha256"][:16]), "ok")
            if info.get("is_template"):
                self._log("           注意：交付物与模板逐字节相同，参赛 AI 可能没写出东西\n",
                          "err")
        return info

    def freeze_timer(self, scope):
        w = self._need_one(scope, "立即收卷")
        if not w:
            return
        ws, label = w[0], w[1]
        st = service.status(ws)
        if st["state"] == "已收卷":
            messagebox.showinfo("裁判台", "%s 已经封存过了。" % label)
            return
        if not st["has_submission"]:
            if not messagebox.askokcancel("收卷", "%s 没有交付文件，会记为未提交。继续？" % label):
                return
        elif not messagebox.askokcancel(
                "收卷", "%s 已用 %.1f 分钟。\n封存后 submission 里的改动不再进入成绩，确认？"
                        % (label, st["used_min"]), default=messagebox.CANCEL):
            return
        self._seal_one(ws, label)
        self.scan_workspaces(keep=label)

    def picked(self, scope):
        ui = self.battle_uis.get(scope)
        if not ui:
            return []
        return [self.workspaces[i] for i in ui["lst"].curselection()
                if i < len(self.workspaces) and self.workspaces[i][2]]

    RULE_SUMMARIES = {
        "v1.9": (
            "v1.9.1 当前正式（1500 tick · ruleset v15，引擎 1.9.1）：遗产第 1/2 件各 +50、每方上限 100；"
            "第 3 件起 0 VS 但推进 R1–R4 遗产科技（60/85/110/135 tick）；"
            "研究舰 100 CU / 25 tick / 速度 5.0，拔旗/舰毁中断研究；"
            "遗产补给 70 tick、场上最多 3 件无主；插旗 0 分；被拔旗守方 −25、攻方 +25（同一 VP 一次）；"
            "库存遗产分随旗毁撤回；击杀 = min(造价×2/3,200)；人口上限 36；"
            "RAILGUN 240/40、ARTILLERY 550/180、两者对母舰伤害 ×1.5；"
            "7 名参考 Bot（含 Warden 攻城）。时间到依次比 VS → KD → 母舰 HP → "
            "旗控制（领先 >5%） → 平。"
        ),
        "v1.4": (
            "v1.4 历史基线（1200 tick · ruleset v14）：VP 累计分、七领域计分、"
            "母舰削弱、对称经济；不使用遗产/研究舰/旗分转移。"
        ),
    }

    def _on_rules_change(self, scope=None):
        scopes = [scope] if scope else list(self.rules_vars)
        for s in scopes:
            if s not in self.rules_vars:
                continue
            mode = str(self.rules_vars[s].get())
            text = self.RULE_SUMMARIES.get(mode, self.RULE_SUMMARIES["v1.4"])
            ui = self.code_uis.get(s)
            if ui:
                ui["summary_var"].set(text)
            bu = self.battle_uis.get(s)
            if bu:
                bu["summary_var"].set(text)
            self._refresh_code_ui(s)

    @staticmethod
    def _sim_rule_version(sim_dir: Path, mode: str) -> str:
        """从 internal/sim 的源码读该规则版本号；读不到返回空串。

        裁判台自己写死版本号迟早会和引擎对不上，所以一律拿源码当基准：
        v1.9 读 rules15.go 的 RulesVersionV15，v1.4 读 rules.go 的 RulesVersion。
        """
        name = "rules15.go" if str(mode).startswith("v1.9") else "rules.go"
        key = "RulesVersionV15" if str(mode).startswith("v1.9") else r"\bRulesVersion\b"
        try:
            text = (sim_dir / name).read_text(encoding="utf-8", errors="replace")
        except OSError:
            return ""
        m = re.search(r"%s\s*=\s*\"([^\"]+)\"" % key, text)
        return m.group(1) if m else ""

    def run_contest(self, scope):
        if self.running:
            return
        picked = self.picked(scope)
        if not picked:
            messagebox.showwarning("裁判台", "先勾选至少一个可编译的工作区")
            return
        who = "只跑这一个" if len(picked) == 1 else "比赛 %d 个选手" % len(picked)
        root = Path(self.contest_root.get().strip() or DEFAULT_CONTEST_ROOT)
        baselines = sorted(root.glob("baseline@*.exe"))
        n_opp = len(baselines) or len(REF_BOTS)
        k = len(picked) + n_opp
        matches = k * (k - 1) // 2
        rules_mode = self.rules_vars[scope].get()
        is_exp = str(rules_mode).startswith("v1.9")
        if is_exp:
            rule_line = "规则 v1.9.1 当前正式（1500 tick · ruleset v15，引擎 1.9.1）"
        else:
            rule_line = "规则 v1.4 历史基线（1200 tick）"
        record_dir = self._record_dir(scope)
        # default=CANCEL：Enter / 空格默认落在「取消」上，双击列表后弹出
        # 立刻回车也不会开跑。
        if not messagebox.askokcancel(
                "真的要开跑？",
                "%s\n%s\n%s\n\n对手：%s%s\n\n"
                "将进行 %d 场对阵、%d 局真实对局（每场换边打 2 局）。\n"
                "成绩写入：%s\n"
                "对局期间会另写一本评测封存（bots/封存记录/_desk_*.json），跑完删掉。"
                % (who, "\n".join("  · " + w[1] for w in picked), rule_line,
                   "、".join(e.stem for e in baselines) if baselines
                   else "、".join(REF_BOTS),
                   "" if len(picked) == 1 else " + 彼此", matches, matches * 2,
                   record_dir),
                default=messagebox.CANCEL):
            self._log("已取消：没有发起对局\n", "dim")
            return
        seeds = int(self.battle_uis[scope]["seed_var"].get() or "1")

        def work():
            try:
                self._contest_pipeline(picked, seeds, rules_mode, scope, record_dir)
            except Exception as e:  # noqa: BLE001
                self.log_q.put(("line", "编排失败：%s" % e))
                self.log_q.put(("exit", 1, None))

        self._set_running(True)
        self.pbar["value"] = 0
        self.status.set("比赛准备中…")
        threading.Thread(target=work, daemon=True).start()

    def _record_dir(self, scope) -> Path:
        """正式成绩与测试成绩分开归档，互不覆盖。"""
        root = Path(self.contest_root.get().strip() or DEFAULT_CONTEST_ROOT)
        return root / (MATCH_RECORD_DIR if scope == "official" else TEST_RECORD_DIR)

    def _contest_pipeline(self, picked, seeds, rules_mode="v1.9", scope="official",
                          record_base=None):
        mode = str(rules_mode)
        is_exp = mode.startswith("v1.9")
        if is_exp:
            version = "1.9.1"
        else:
            version = "1.4"
        mode_label = "v%s 当前" % version if is_exp else "v1.4 历史"
        stamp = time.strftime("%m%d-%H%M%S")
        self._emit(("line", "规则 %s ｜ %d tick ｜ ruleset %s"
                    % (mode_label, 1500 if is_exp else 1200,
                       "v15" if is_exp else "v14"), "dim"))
        out_rel = "results/desk/%s" % stamp
        names, manifest, sources = [], [], {}
        STAGE_ROOT.mkdir(parents=True, exist_ok=True)
        for idx, (path, label, src, _note) in enumerate(picked):
            base = label.split("@")[-1] if "@" in label else label
            slug = slugify(base)
            if slug in names:
                slug = "%s_%d" % (slug, idx + 1)
            names.append(slug)
            sources[slug] = src
            stage = STAGE_ROOT / slug
            if stage.exists():
                shutil.rmtree(stage)
            # go.mod/go.sum 必须剥掉：工作区是独立模块（选手本地能直接 build），
            # 带着它进引擎目录会变成嵌套模块，arena build 就编不动了。
            # arena/ 是发给选手的引擎源码副本（他自己调试用），评测时用赛场这份，
            # 不随卷走；对局记录与构建产物同理。
            shutil.copytree(path, stage, ignore=shutil.ignore_patterns(
                "bin", "*.exe", ".git", "__pycache__", "go.mod", "go.sum",
                "arena", "对局记录", ".bin"))
            # 有收卷副本就以收卷副本为准：评的必须是冻结那一刻的代码。
            # 交付文件名是「<选手名>_bot.go」，不再固定叫 decide.go。
            frozen = pick_frozen(stage)
            if frozen is not None and src:
                dst = stage if src == "." else stage / src
                dst.mkdir(parents=True, exist_ok=True)
                shutil.copy2(frozen, dst / frozen.name)
                self._emit(("line", "     使用收卷副本 交卷/%s" % frozen.name, "dim"))
            digest = tree_digest(stage)
            self._emit(("line", "暂存 %-24s ← %s" % (slug, path)))
            self._emit(("line", "     源码摘要 %s" % digest[:16]))
            manifest.append({"name": slug, "workdir": str(path), "staged": str(stage),
                             "source": src, "sha256": digest})

        rules = {"tickLimit": 1500 if is_exp else 1200, "tickMs": 50, "decideMs": 120,
                 "initMs": 2000, "maxStrikes": 3, "strikeMode": "consecutive",
                 "generator": "minecraft", "vision": False}
        if is_exp:
            rules["ruleset"] = "v15"
            rules["rulesVersion"] = version
        if is_exp:
            # 把裁判台选中的 v1.9 规则写进评测配置，便于赛后审计。
            # 数值必须与 internal/sim/rules15.go（1.9.1）保持一致。
            rules.update({
                "popMax": 36,
                "relicValue": 50, "relicVSCap": 100,
                "relicSpawnTicks": 70, "relicMaxGround": 3,
                "relicLoadTicks": 30, "relicDropTicks": 30,
                "flagPlaceVP": 0, "flagPenalty": 25, "flagBounty": 25, "flagHP": 160,
                "killVPCap": 200,
                "relicResearch": True, "researchUnit": "RESEARCH", "researchRadius": 252,
                "relicTechTicks": {"R1": 60, "R2": 85, "R3": 110, "R4": 135},
                "units": {
                    "RESEARCH": {"cost": 100, "build": 25, "speed": 5.0},
                    "RAILGUN": {"cost": 240, "ru": 40, "speed": 2.0},
                    "ARTILLERY": {"cost": 550, "ru": 180, "build": 75, "speed": 1.8},
                },
                "siegeDamageVsMothership": 1.5,
                "tieBreak": ["flagControl"],
            })
        cfg = {
            "name": "Desert Arena · 裁判台比赛",
            "season": "裁判台比赛 %s（%s）" % (stamp, mode_label),
            "version": version,
            "engine": {"kind": "native", "host": "bots/bin/enginehost.exe",
                       "snapshot": "engine/golden"},
            "rules": rules,
            "schedule": {"seeds": [20261109 + i * 7 for i in range(max(1, seeds))],
                         "mirror": True, "bootstrap": 200},
            "entries": [],
            "output": out_rel,
            "verify": True,
        }
        # 对手必须排在前面：BT 以「名册第一名」为锚（=0），
        # 锚点放成选手的话，每个选手都会是 0 分，跨场无法比较。
        root = Path(self.contest_root.get().strip() or DEFAULT_CONTEST_ROOT)
        baselines = sorted(root.glob("baseline@*.exe"))
        if baselines:
            for exe in baselines:
                cfg["entries"].append({
                    "name": exe.stem, "author": "基线（预编译）", "source": "",
                    "bin": str(exe), "prebuilt": True,
                })
            self._emit(("line", "对手：%s（预编译基线）"
                        % "、".join(e.stem for e in baselines), "dim"))
        else:
            for bot in REF_BOTS:
                cfg["entries"].append({"name": bot, "author": "官方参考 Bot",
                                       "source": "workspace/bots/%s" % bot.lower(),
                                       "bin": "bots/bin/%s.exe" % bot.lower()})
            self._emit(("line", "未找到 baseline@*.exe，改用 %d 个参考 Bot"
                        % len(REF_BOTS), "dim"))
        stage_rel = STAGE_ROOT.relative_to(ENGINE).as_posix()
        for slug in names:
            sub = sources.get(slug) or "."
            src_rel = "%s/%s" % (stage_rel, slug) if sub == "." else "%s/%s/%s" % (stage_rel, slug, sub)
            cfg["entries"].append({"name": slug, "author": "选手工作区",
                                   "source": src_rel,
                                   "bin": "bots/bin/%s.exe" % slug})

        cfg_rel = "tournament/_desk_%s.json" % stamp
        (ENGINE / cfg_rel).write_text(json.dumps(cfg, ensure_ascii=False, indent=2),
                                      encoding="utf-8")
        out_abs = ENGINE / out_rel
        out_abs.mkdir(parents=True, exist_ok=True)
        (out_abs / "工作区清单.json").write_text(
            json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
        n_opp = len(baselines) if baselines else len(REF_BOTS)
        self._emit(("line", "编排完成：%d 名选手 + %d 个对手 → %s"
                        % (len(names), n_opp, out_rel), "ok"))

        for argv in (["go", "run", "./cmd/arena", "build", "-config", cfg_rel],
                     ["go", "run", "./cmd/arena", "plan", "-config", cfg_rel],
                     ["go", "run", "./cmd/arena", "run", "-config", cfg_rel],
                     ["go", "run", "./cmd/arena", "rate", "-config", cfg_rel]):
            if not self._stream(argv):
                return
        self.last_out = out_rel
        self._emit(("line", "对局完成，整理成绩…", "ok"))
        dest = out_abs
        if record_base is not None:
            # 引擎只允许把结果写在 war_of_desert 内，所以先落到 results/desk/<stamp>，
            # 跑完再整体搬到 算法赛/比赛记录 或 算法赛/测试/测试成绩。
            dest = Path(record_base) / stamp
            dest.parent.mkdir(parents=True, exist_ok=True)
            if dest.exists():
                shutil.rmtree(dest, ignore_errors=True)
            shutil.move(str(out_abs), str(dest))
            self.last_record[scope] = dest
            self.last_out = str(dest)
        self._emit(("line", "成绩 → %s" % dest, "ok"))
        # 临时编排的封存与配置：跑完就删，封存目录里只留赛季的账。
        for p in (ENGINE / "bots" / "封存记录" / (Path(cfg_rel).stem + ".json"),
                  ENGINE / cfg_rel):
            try:
                p.unlink()
            except OSError:
                pass
        self._emit(("exit", 0, None))

    # ---------------- 通用执行 ----------------
    def _go_argv(self, argv) -> list:
        """把 argv 里的 "go" 换成开机时解析出来的绝对路径。

        CreateProcess 每次都要自己按 PATH 找 go，而双击 bat 起来的进程 PATH
        未必和终端一致；固定成绝对路径，行为才和 `--selfcheck` 一致。
        """
        argv = list(argv)
        if argv and argv[0] == "go" and self.go_exe:
            argv[0] = self.go_exe
        return argv

    def _stream(self, argv) -> bool:
        argv = self._go_argv(argv)
        self._emit(("line", "$ " + " ".join(argv)))
        try:
            proc = subprocess.Popen(argv, cwd=str(ENGINE), stdout=subprocess.PIPE,
                                    stderr=subprocess.STDOUT, text=True, encoding="utf-8",
                                    errors="replace", bufsize=1,
                                    creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
        except Exception as e:  # noqa: BLE001
            self._emit(("line", "启动失败：%s" % e))
            self._emit(("exit", 127, None))
            return False
        self.proc = proc
        assert proc.stdout is not None
        seen = 0
        for line in proc.stdout:
            seen += 1
            self._emit(("line", line.rstrip("\n")))
        code = proc.wait()
        if code != 0:
            if seen == 0:
                self._emit(("line", "（这条命令没有任何输出就退出了，退出码 %s / 0x%08X）"
                            % (code, code & 0xFFFFFFFF)))
                p = self._dump_stream_fail(argv)
                if p:
                    self._emit(("line", "诊断已写入 %s" % p))
            self._emit(("exit", code, None))
            return False
        return True

    def _dump_stream_fail(self, argv) -> Path | None:
        """`go run ...` 静默失败时留证据，免得又只能看一个空白。"""
        env_keys = ("PATH", "GOROOT", "GOPATH", "GOCACHE", "GOFLAGS", "GOWORK",
                    "GOTOOLCHAIN", "TEMP")
        out = ["Desert Arena 裁判台 · 命令静默失败诊断",
               "生成时间：%s" % time.strftime("%Y-%m-%d %H:%M:%S"),
               "裁判台版本：v%s" % DESK_VERSION,
               "argv：%s" % argv,
               "cwd ：%s" % ENGINE,
               "go  ：%s（%s）" % (self.go_exe or "未找到", self.go_from),
               "", "—— 环境变量 ——"]
        for k in env_keys:
            out.append("%-12s = %s" % (k, os.environ.get(k, "（未设置）")))
        p = APP_DIR / "自检诊断.txt"
        try:
            p.write_text("\n".join(out) + "\n", encoding="utf-8")
            return p
        except OSError:
            return None

    def _cmd_fail(self, code, out) -> str:
        """把「非零退出但没有任何输出」翻译成人能看懂的话。

        Windows 上进程被外部干掉（杀软、OOM、作业对象限制）时，退出码是
        0xC0000005 这类大负数，而 stdout/stderr 是空的。只留一个空白的
        「编译失败：」等于什么都没说——上次就是这样，看不出到底是代码写错
        还是工具链根本没跑起来。
        """
        text = (out or "").strip()
        if text:
            return "\n" + text[-1200:]
        if code is not None and code < 0:
            return ("\n  （退出码 %d / 0x%08X：进程被系统或安全软件终止，不是编译错误）"
                    "\n  先把工作区加入杀毒软件白名单，再重试。"
                    % (code, code & 0xFFFFFFFF))
        return ("\n  （退出码 %s，命令没有任何输出）"
                "\n  在 cmd 里进到该工作区手动跑一遍同样的命令，那里能看到真实报错。"
                % code)

    def _exec(self, argv, cwd=None):
        argv = self._go_argv(argv)
        try:
            p = subprocess.run(argv, cwd=str(cwd or ENGINE), capture_output=True, text=True,
                               encoding="utf-8", errors="replace", timeout=300,
                               creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
            return p.returncode, (p.stdout or "") + (p.stderr or "")
        except Exception as e:  # noqa: BLE001
            return 1, str(e)

    def stop(self):
        if self.proc and self.proc.poll() is None:
            self.proc.terminate()
            self.status.set("已中止")
            self._log("\n— 已中止（已完成的对局保留在 结果.jsonl）—\n", "err")

    # ---------------- 流水线状态 ----------------
    @staticmethod
    def _set_step_label(ui, key, text, color):
        """只在文字/颜色真的变了才写，避免每秒刷新把控件抖一遍。"""
        lbl = ui["steps"].get(key, {}).get("lbl")
        if lbl is None:
            return
        if lbl.cget("text") != text:
            lbl.config(text=text)
        if lbl.cget("fg") != color:
            lbl.config(fg=color)

    def _refresh_code_ui(self, scope):
        """按磁盘上的真实状态开合该页签的 ①~⑤。

        不记「上次点了什么」，只看事实：包齐不齐、上次发卷没有、计时到哪、封没封。
        所以关掉裁判台再打开，界面和 .赛务.json 仍然对得上。
        """
        ui = self.code_uis.get(scope)
        if not ui:
            return
        btns = ui["btns"]
        w = self.selected_one(scope)
        if w is None:
            self._set_step_label(ui, "chk", "先在左边选一个工作区", MUTED)
            for key in ("issue", "prompt", "start", "freeze"):
                self._set_step_label(ui, key, "", MUTED)
            for b in btns.values():
                b.configure(state="disabled")
            return

        ws = w[0]
        busy = self.running
        mode = self.rules_vars[scope].get()
        try:
            chk = service.check_package(ws)
            st = service.status(ws)
        except Exception as e:  # noqa: BLE001
            self._set_step_label(ui, "chk", "读状态失败：%s" % e, BAD)
            return
        state = st["state"]

        # ① 自检
        btns["chk"].configure(state="disabled" if busy else "normal")
        if chk["ready"]:
            txt = "包齐备（%d 项）· 交付 %s · 规则 %s" % (
                len(service.KEY_ITEMS), chk["submission"], mode)
            if chk["is_template"]:
                txt += " · 仍是模板"
            self._set_step_label(ui, "chk", txt, OK)
        else:
            miss = chk["missing"]
            shown = "、".join(miss[:3]) + ("等 %d 项" % len(miss) if len(miss) > 3 else "")
            self._set_step_label(ui, "chk", "缺：%s%s" % (shown or "交付文件", "（点自检补全）"), BAD)

        # ② 发卷
        btns["issue"].configure(state="disabled" if busy else "normal")
        if chk["issued"]:
            self._set_step_label(ui, "issue", "已发卷 · 再发只补缺件，不覆盖已有文件", MUTED)
        else:
            self._set_step_label(ui, "issue", "未发卷 · 点发卷把选手包铺进 workspace", BAD)

        # ③ 提示词
        p = service.prompt_path(ws)
        has_p = p.is_file()
        btns["copy"].configure(state="normal" if (has_p and not busy) else "disabled")
        btns["view"].configure(state="normal" if (has_p and not busy) else "disabled")
        if has_p:
            self._set_step_label(ui, "prompt", "prompt.md · %d 字节 · 复制后自己去粘"
                                 % p.stat().st_size, MUTED)
        else:
            self._set_step_label(ui, "prompt", "还没有 prompt.md（先自检或发卷）", BAD)

        # ④ 开始编码 + 暂停/继续
        can_start = chk["ready"] and not busy and state != "已收卷"
        btns["start"].configure(state="normal" if can_start else "disabled")
        btns["pause"].configure(state="normal" if (state == "进行中" and not busy) else "disabled")
        btns["resume"].configure(state="normal" if (state == "已暂停" and not busy) else "disabled")
        if not chk["ready"]:
            self._set_step_label(ui, "start", "选手包不完整，先自检/发卷", BAD)
        elif state == "已收卷":
            self._set_step_label(ui, "start", "已收卷，不能再开始编码", MUTED)
        elif state == "未开赛":
            self._set_step_label(ui, "start", "未开赛 · 只开始计时，不跑对局 · 90 目标 / 120 强制",
                                 MUTED)
        elif st["frozen"]:
            f = st["frozen"]
            self._set_step_label(
                ui, "start",
                "已收卷 · 用时 %.1f 分钟%s" % (f.get("used_minutes", 0),
                                              "" if f.get("accepted") else "（超时，不计成绩）"),
                MUTED)
        else:
            self._set_step_label(
                ui, "start",
                "正在编码（%s）· 已用 %.1f 分钟 · 距 120 分钟还有 %.1f 分钟"
                % (state, st["used_min"], st["left_min"]),
                OK if st["used_min"] < 90 else BAD)

        # ⑤ 收卷封存
        can_seal = state in ("进行中", "已暂停") and not busy
        btns["freeze"].configure(state="normal" if can_seal else "disabled")
        if state == "已收卷":
            self._set_step_label(ui, "freeze", "已收卷，交卷/ 里有副本", MUTED)
        elif state == "未开赛":
            self._set_step_label(ui, "freeze", "开始编码后才能收卷", MUTED)
        elif state.startswith("已超时"):
            self._set_step_label(ui, "freeze", "已到点，正在自动收卷…", BAD)
        else:
            self._set_step_label(ui, "freeze", "到 120 分钟自动收卷，也可以现在手动收", MUTED)

    def _auto_freeze(self):
        """120 分钟到点自动收卷。

        判据是 .赛务.json 里的开始时间，不是内存里的状态：裁判台关着时错过的，
        下次打开第一秒就补上，算出来的用时也是真实用时。
        """
        fired = False
        for path, label, _src, _note in list(self.workspaces):
            key = str(path)
            if key in self._auto_frozen:
                continue
            try:
                st = service.status(path)
            except Exception:  # noqa: BLE001
                continue
            if st["state"] != "已超时（待收卷）":
                continue
            self._auto_frozen.add(key)
            self._log("⏰ %s 已到 120 分钟，自动收卷\n" % label, "dim")
            self._seal_one(path, label, auto=True)
            fired = True
        if fired:
            self.scan_workspaces()

    def _tick(self):
        self._auto_freeze()
        for scope in list(self.code_uis):
            self._refresh_code_ui(scope)
        self.after(1000, self._tick)

    def _drain_log(self):
        dirty = False
        while True:
            try:
                item = self.log_q.get_nowait()
            except queue.Empty:
                break
            if item[0] == "line":
                line = ANSI_RE.sub("", item[1])
                tag = ""
                if line.startswith("错误") or "✗" in line or "FAIL" in line or "失败" in line:
                    tag = "err"
                elif "完成" in line or "OK" in line or "PASS" in line or "✓" in line:
                    tag = "ok"
                m = PROGRESS_RE.search(line)
                if m and int(m.group(2)) > 0:
                    done, total = int(m.group(1)), int(m.group(2))
                    self.pbar["value"] = 100.0 * done / total
                    self.status.set("对局 %d / %d" % (done, total))
                self._log(line + "\n", tag)
                dirty = True
            else:
                code = item[1]
                if code == 0:
                    self.status.set("完成")
                    self.pbar["value"] = 100
                else:
                    self.status.set("失败（退出码 %s）" % code)
                    self._log("— 失败，退出码 %s —\n" % code, "err")
                if item[2]:
                    self._log(str(item[2]) + "\n", "err")
                self._set_running(False)
                dirty = True
        if dirty:
            self.log.see("end")
        self.after(80, self._drain_log)

    def _set_running(self, running):
        self.running = running
        state = "disabled" if running else "normal"
        for ui in self.battle_uis.values():
            for key in ("run_btn", "view_btn"):
                if ui.get(key):
                    ui[key].configure(state=state)
        self.stop_btn.configure(state="normal" if running else "disabled")
        for scope in self.code_uis:
            self._refresh_code_ui(scope)

    def _log(self, text, tag=""):
        self.log.configure(state="normal")
        self.log.insert("end", text, tag)
        self.log.configure(state="disabled")
        self.log.see("end")

    # ---------------- 快捷入口 ----------------
    def open_contest_results(self, scope):
        p = self.last_record.get(scope)
        if p and Path(p).is_dir():
            self._open_dir(Path(p))
            return
        base = self._record_dir(scope)
        dirs = sorted([d for d in base.iterdir() if d.is_dir()]) if base.is_dir() else []
        self._open_dir(dirs[-1] if dirs else base)

    def _open_dir(self, p: Path):
        if not p.is_dir():
            messagebox.showinfo("裁判台", "目录不存在：%s" % p)
            return
        os.startfile(str(p)) if sys.platform == "win32" else subprocess.Popen(["open", str(p)])


def main():
    app = RefereeDesk()
    argv = sys.argv
    if "--selftest" in argv:
        app.update_idletasks()
        app.update()
        app.destroy()
        print("GUI_OK")
        return
    if "--smoke" in argv:
        tabs = len(app.nb.tabs())
        print("TABS=%d" % tabs)
        print("SMOKE_OK" if tabs == 3 else "SMOKE_FAIL")
        app.destroy()
        return
    if "--init" in argv:
        # 无头初始化：--init [根目录] [seed 数]
        i = argv.index("--init")
        root = Path(argv[i + 1]) if len(argv) > i + 1 else DEFAULT_CONTEST_ROOT
        seeds = int(argv[i + 2]) if len(argv) > i + 2 else 1
        app.headless = True
        app.contest_root.set(str(root))
        for d in sorted(root.iterdir()):
            if (not d.is_dir() or d.name.startswith("_")
                    or d.name in NON_CONTESTANT_DIRS):
                continue
            if (d / "workspace" / "clock.py").is_file():
                print("已发卷  %s" % d.name)
                continue
            got = service.issue(d)
            print("发卷    %s  ← %s" % (d.name, "、".join(got)))
        plan = build_schedule(root, seeds)
        (root / "赛程.json").write_text(
            json.dumps(plan, ensure_ascii=False, indent=2), encoding="utf-8")
        print("赛程    %d 名报名者 / %d 场对阵（%d 个 seed）→ %s"
              % (len(plan["players"]), len(plan["matches"]), len(plan["seeds"]),
                 root / "赛程.json"))
        for p_ in plan["players"]:
            print("        %-44s %s" % (p_["name"], p_["kind"]))
        app.destroy()
        return
    if "--selfcheck" in argv:
        # 命令行自检：--selfcheck <工作区目录> [v1.9|v1.4]
        # 和界面上的 ① 完全同一套判定，但在 cmd 里跑，能看到子进程的原始输出。
        i = argv.index("--selfcheck")
        if len(argv) <= i + 1:
            print("用法：py -3 -X utf8 referee_desk.py --selfcheck <工作区目录> [v1.9|v1.4]")
            app.destroy()
            return 2
        ws = Path(argv[i + 1]).resolve()
        mode = argv[i + 2] if len(argv) > i + 2 else "v1.9"
        app.headless = True
        print("工作区自检：%s（规则 %s）\n" % (ws, mode))
        lines, ok = app._check_workspace(ws, mode)
        for text, tag in lines:
            mark = {"err": "  × ", "ok": "  ✓ "}.get(tag, "  · ")
            for j, ln in enumerate(text.splitlines()):
                print((mark if j == 0 else "    ") + ln)
        print("\n— 自检%s —" % ("通过" if ok else "未通过"))
        app.destroy()
        return 0 if ok else 1
    if "--smoke-contest" in argv:
        # 无头跑一次对局管线：--smoke-contest <工作区目录> [seed 数] [v1.9|v1.4] [test|official]
        i = argv.index("--smoke-contest")
        if len(argv) <= i + 1:
            print("用法：py -3 -X utf8 referee_desk.py --smoke-contest "
                  "<工作区目录> [seed 数] [v1.9|v1.4] [test|official]")
            app.destroy()
            return 2
        ws = Path(argv[i + 1]).resolve()
        seeds = int(argv[i + 2]) if len(argv) > i + 2 else 1
        rules_mode = argv[i + 3] if len(argv) > i + 3 else "v1.9"
        scope = argv[i + 4] if len(argv) > i + 4 else "test"
        app.headless = True
        app._emit = lambda item: print(item[1] if item[0] == "line" else "EXIT=%s" % item[1])
        src, note = detect_source(ws)
        app._contest_pipeline([(ws, ws.name, src, note)], seeds, rules_mode,
                              scope, app._record_dir(scope))
        app.destroy()
        return
    if "--scan" in argv:
        i = argv.index("--scan")
        if len(argv) > i + 1:
            app.contest_root.set(argv[i + 1])
            app.scan_workspaces()
        for path, label, src, note in app.workspaces:
            print("%-44s src=%-10s %s" % (label, src or "-", note))
        app.destroy()
        return
    app.mainloop()


if __name__ == "__main__":
    main()

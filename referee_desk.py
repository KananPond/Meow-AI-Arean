#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""Desert Arena · 裁判台（GUI）。

版本：v1.5
功能：两个工作模式，同一个窗口——

  ① 赛季：对 war_of_desert/ 里冻结好的正式赛季跑 自检/编译/冻结/开赛/评分。
  ② 工作区：扫描 算法赛/RTS算法赛@<模型名>/ 这类选手工作区，
     勾选其中若干个（不必全选），临时编排成一场评测跑完出成绩。

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
REF_BOTS = ["Vanguard", "Bulwark", "Swarm", "Nomad", "Sapper"]

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
        if d.is_dir() and not d.name.startswith("_"):
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
        self.title("Desert Arena · 裁判台")
        self.geometry("1000x720")
        self.minsize(880, 620)
        self.configure(bg=BG)

        self.status = tk.StringVar(value="就绪")
        self.cfg_path = tk.StringVar()
        self.contest_root = tk.StringVar(value=str(DEFAULT_CONTEST_ROOT))
        self.seed_count = tk.StringVar(value="1")

        self.info = {}
        self.proc = None
        self.log_q = queue.Queue()
        self.running = False
        self.total_matches = 0
        self.workspaces = []  # [(path, label, src, note)]
        self.last_out = None
        self._emit = self.log_q.put
        # 无头模式（--scan / --smoke-contest）不往 Tk 里投回调
        self.headless = "--smoke-contest" in sys.argv or "--scan" in sys.argv

        self._build_styles()
        self._build_ui()
        self.reload_configs()
        self.scan_workspaces()
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
        nb.add(self._build_season_tab(nb), text="  赛季  ")
        nb.add(self._build_contest_tab(nb), text="  工作区  ")

        prog = tk.Frame(self, bg=BG)
        prog.pack(fill="x", padx=14, pady=(8, 0))
        self.pbar = ttk.Progressbar(prog, mode="determinate", maximum=100)
        self.pbar.pack(side="left", fill="x", expand=True)
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

    def _build_season_tab(self, parent):
        tab = tk.Frame(parent, bg=BG)
        head = tk.Frame(tab, bg=BG)
        head.pack(fill="x", padx=12, pady=(10, 6))
        tk.Label(head, text="赛季配置", bg=BG, fg=MUTED).pack(side="left")
        self.cmb = ttk.Combobox(head, textvariable=self.cfg_path, width=32, state="readonly")
        self.cmb.pack(side="left", padx=8)
        self.cmb.bind("<<ComboboxSelected>>", lambda _e: self.on_config_change())
        ttk.Button(head, text="刷新", command=self.reload_configs).pack(side="left")
        self.seal_lbl = tk.Label(head, text="封存 —", bg=BG, fg=MUTED)
        self.seal_lbl.pack(side="right")

        info = tk.Frame(tab, bg=PANEL)
        info.pack(fill="x", padx=12, pady=(0, 8))
        self.info_labels = {}
        for col, key in enumerate(("赛季", "规则", "引擎", "选手", "场次")):
            tk.Label(info, text=key, bg=PANEL, fg=MUTED, font=(None, 9)).grid(
                row=0, column=col * 2, sticky="w", padx=(12, 4), pady=(8, 0))
            v = tk.Label(info, text="—", bg=PANEL, fg=FG, font=(None, 10, "bold"))
            v.grid(row=1, column=col * 2, sticky="w", padx=(12, 4), pady=(0, 8))
            self.info_labels[key] = v
            info.grid_columnconfigure(col * 2, weight=1)

        bar = tk.Frame(tab, bg=BG)
        bar.pack(fill="x", padx=12, pady=(0, 10))
        self.season_btns = {}
        for text, sub, style in (("环境自检", "selftest", "TButton"),
                                 ("编译封存", "build", "TButton"),
                                 ("冻结赛程", "plan", "TButton"),
                                 ("开赛", "run", "Go.TButton"),
                                 ("评分", "rate", "TButton")):
            b = ttk.Button(bar, text=text, command=lambda s=sub: self.run_season(s), style=style)
            b.pack(side="left", padx=(0, 8))
            self.season_btns[text] = b
        self.stop_btn = ttk.Button(bar, text="中止", command=self.stop, state="disabled")
        self.stop_btn.pack(side="left", padx=(16, 0))
        r = tk.Frame(bar, bg=BG)
        r.pack(side="right")
        ttk.Button(r, text="成绩目录", command=self.open_results).pack(side="right")
        ttk.Button(r, text="回放查看器", command=self.open_viewer).pack(side="right", padx=(0, 8))
        return tab

    def _build_contest_tab(self, parent):
        tab = tk.Frame(parent, bg=BG)

        row = tk.Frame(tab, bg=BG)
        row.pack(fill="x", padx=12, pady=(10, 4))
        tk.Label(row, text="比赛根目录", bg=BG, fg=MUTED).pack(side="left")
        tk.Entry(row, textvariable=self.contest_root, bg=PANEL, fg=FG,
                 insertbackground=FG, relief="flat", width=56).pack(side="left", padx=8, ipady=3)
        ttk.Button(row, text="浏览", command=self.browse_root).pack(side="left")
        ttk.Button(row, text="扫描", command=self.scan_workspaces).pack(side="left", padx=8)

        tk.Label(tab, text="勾选要评测的工作区（可多选）；只想跑一个就双击它。目录名建议 RTS算法赛@<模型名>",
                 bg=BG, fg=MUTED, font=(None, 9)).pack(anchor="w", padx=12)

        wrap = tk.Frame(tab, bg=BG)
        wrap.pack(fill="x", padx=12, pady=(4, 0))
        self.lst = tk.Listbox(wrap, selectmode="extended", height=6, bg=PANEL, fg=FG,
                              selectbackground="#3a5f8a", relief="flat",
                              font=("Consolas", 10), activestyle="none")
        self.lst.pack(side="left", fill="x", expand=True)
        sb = tk.Scrollbar(wrap, command=self.lst.yview, bg=PANEL)
        sb.pack(side="left", fill="y")
        self.lst.configure(yscrollcommand=sb.set)
        self.lst.bind("<<ListboxSelect>>", self._on_select)
        self.lst.bind("<Double-Button-1>", self.run_single)

        bar = tk.Frame(tab, bg=BG)
        bar.pack(fill="x", padx=12, pady=(8, 10))
        ttk.Button(bar, text="全选", command=self.select_all).pack(side="left")
        ttk.Button(bar, text="清空", command=self.select_none).pack(side="left", padx=6)
        tk.Label(bar, text="seed 数", bg=BG, fg=MUTED).pack(side="left", padx=(16, 4))
        ttk.Combobox(bar, textvariable=self.seed_count, values=("1", "2", "3"),
                     width=4, state="readonly").pack(side="left")
        self.eval_btn = ttk.Button(bar, text="评测选中（0）", style="Go.TButton",
                                   command=self.run_contest)
        self.eval_btn.pack(side="left", padx=(16, 0))
        ttk.Button(bar, text="只跑双击的那一个", command=self.run_single).pack(
            side="left", padx=6)
        ttk.Button(bar, text="打开评测结果", command=self.open_contest_results).pack(
            side="left", padx=8)

        srv = tk.Frame(tab, bg=BG)
        srv.pack(fill="x", padx=12, pady=(0, 4))
        tk.Label(srv, text="赛务", bg=BG, fg=MUTED).pack(side="left")
        for text, fn in (("初始化工作区与赛程", self.init_contest),
                         ("发卷并开赛", self.issue_start),
                         ("暂停", self.pause_timer),
                         ("恢复", self.resume_timer),
                         ("收卷", self.freeze_timer),
                         ("刷新状态", self.scan_workspaces)):
            ttk.Button(srv, text=text, command=fn).pack(side="left", padx=(8, 0))
        self.clock_lbl = tk.Label(srv, text="选中一个工作区查看计时", bg=BG, fg=MUTED)
        self.clock_lbl.pack(side="right")

        tk.Label(tab, text="「发卷并开赛」会把选手包铺进 <工作区>/workspace 并开始计时"
                           "（90 分钟目标 / 120 分钟收卷）；目录为空也可以直接开赛。",
                 bg=BG, fg=MUTED, font=(None, 9)).pack(anchor="w", padx=12, pady=(0, 8))
        return tab

    # ---------------- 赛季 ----------------
    def reload_configs(self):
        items = []
        tdir = ENGINE / "tournament"
        if tdir.is_dir():
            items = sorted(f.name for f in tdir.glob("*.json") if not f.name.startswith("_"))
        self.cmb["values"] = items
        if items:
            cur = self.cfg_path.get()
            self.cfg_path.set(cur if cur in items else items[-1])
            self.on_config_change()

    def on_config_change(self):
        path = ENGINE / "tournament" / self.cfg_path.get()
        try:
            cfg = json.loads(path.read_text(encoding="utf-8"))
        except Exception as e:  # noqa: BLE001
            self._log("读配置失败：%s\n" % e, "err")
            return
        self.info["cfg"] = cfg
        self.info_labels["赛季"].config(text=cfg.get("season", "—"))
        self.info_labels["规则"].config(text=cfg.get("version", "—"))
        self.info_labels["引擎"].config(text=cfg.get("engine", {}).get("kind", "—"))
        self.info_labels["选手"].config(text=str(len(cfg.get("entries", []))))
        out = ENGINE / cfg.get("output", "").replace("/", os.sep)
        self.total_matches = 0
        plan = out / "赛程.json"
        if plan.is_file():
            try:
                self.total_matches = len(json.loads(plan.read_text(encoding="utf-8")).get("matches", []))
            except Exception:  # noqa: BLE001
                pass
        self.info_labels["场次"].config(text=str(self.total_matches) if self.total_matches else "未冻结")
        self.pbar["value"] = 0
        self.check_seal()

    def check_seal(self):
        if getattr(self, "headless", False):
            return
        self.seal_lbl.config(text="封存 检查中…", fg=MUTED)
        cfg = self.cfg_path.get()
        if not cfg:
            return

        def work():
            code, out = self._exec(["go", "run", "./cmd/arena", "seal",
                                    "-config", "tournament/" + cfg])

            def apply():
                if code == 0:
                    self.seal_lbl.config(text="封存 ✓", fg=OK)
                else:
                    self.seal_lbl.config(text="封存 ✗", fg=BAD)
                    self._log(out.strip() + "\n", "err")

            self.after(0, apply)

        threading.Thread(target=work, daemon=True).start()

    def run_season(self, sub):
        if self.running:
            return
        cfg = self.cfg_path.get()
        if not cfg:
            messagebox.showwarning("裁判台", "先选一个赛季配置")
            return
        if sub == "run":
            if self.total_matches == 0:
                if messagebox.askokcancel("未冻结赛程", "还没有 赛程.json。\n先跑「冻结赛程」？"):
                    self.run_season("plan")
                return
            if not messagebox.askokcancel("开赛", "按冻结赛程跑完所有对局，确认开赛？"):
                return
        self._start([["go", "run", "./cmd/arena", sub, "-config", "tournament/" + cfg]])

    # ---------------- 工作区 ----------------
    def browse_root(self):
        d = filedialog.askdirectory(initialdir=self.contest_root.get() or str(APP_DIR),
                                    title="选择比赛根目录")
        if d:
            self.contest_root.set(d)
            self.scan_workspaces()

    def scan_workspaces(self):
        root = Path(self.contest_root.get().strip() or DEFAULT_CONTEST_ROOT)
        self.lst.delete(0, "end")
        self.workspaces = []
        if not root.is_dir():
            self._log("比赛根目录不存在：%s\n" % root, "err")
            self.status.set("目录不存在")
            return
        self.root = root
        dirs = [root] if detect_source(root)[0] else [
            d for d in sorted(root.iterdir()) if d.is_dir() and not d.name.startswith("_")]
        for d in dirs:
            src, note = detect_source(d)
            label = d.name
            try:
                srv = service.describe(d)
            except Exception:  # noqa: BLE001
                srv = "—"
            if src is None:
                self.lst.insert("end", "%-38s  × %-16s  %s" % (label, note, srv))
                self.lst.itemconfig("end", foreground="#8a8f98")
                self.workspaces.append((d, label, None, note))
            else:
                n = len(go_files(d))
                self.lst.insert("end", "%-38s  ✓ %-16s  %s  （%d 个 .go）"
                                % (label, note, srv, n))
                self.workspaces.append((d, label, src, note))
        ok = sum(1 for w in self.workspaces if w[2])
        self.status.set("扫描到 %d 个，可用 %d 个" % (len(self.workspaces), ok))
        self._on_select()
        self._log("扫描 %s：%d 个目录，%d 个可编译\n" % (root, len(self.workspaces), ok), "dim")

    def _on_select(self, _evt=None):
        n = len(self.picked())
        if hasattr(self, "eval_btn"):
            self.eval_btn.configure(text="评测选中（%d）" % n)

    def run_single(self, evt=None):
        """只跑一个：双击哪一行就只评测哪一行。"""
        if self.running:
            return
        if evt is not None:
            idx = self.lst.nearest(evt.y)
            self.lst.selection_clear(0, "end")
            self.lst.selection_set(idx)
        self._on_select()
        picked = self.picked()
        if len(picked) != 1:
            messagebox.showwarning("裁判台", "请双击（或只勾选）一个可编译的工作区")
            return
        self.run_contest()

    def select_all(self):
        for i, w in enumerate(self.workspaces):
            if w[2]:
                self.lst.selection_set(i)

    def select_none(self):
        self.lst.selection_clear(0, "end")

    def selected_one(self):
        """当前选中的单个工作区（允许还是空的——发卷就是给空目录用的）。"""
        idx = self.lst.curselection()
        if len(idx) != 1:
            return None
        i = idx[0]
        return self.workspaces[i] if i < len(self.workspaces) else None

    def _need_one(self, action: str):
        w = self.selected_one()
        if w is None:
            messagebox.showwarning("裁判台", "%s：先在列表里选中一个工作区" % action)
            return None
        return w

    def init_contest(self):
        """初始化工作区（发卷，不开表）+ 冻结赛程。"""
        if self.running:
            return
        root = Path(self.contest_root.get().strip() or DEFAULT_CONTEST_ROOT)
        if not root.is_dir():
            messagebox.showwarning("裁判台", "比赛根目录不存在：%s" % root)
            return
        seeds = int(self.seed_count.get() or "1")
        if not messagebox.askokcancel(
                "初始化", "对 %s 执行：\n"
                "  · 给还没有选手包的工作区发卷（不开表）\n"
                "  · 冻结赛程（%d 个 seed）\n确认？" % (root, seeds)):
            return
        done = []
        for d in sorted(root.iterdir()):
            if not d.is_dir() or d.name.startswith("_"):
                continue
            if (d / "workspace" / "clock.py").is_file():
                continue
            try:
                service.issue(d)
                done.append(d.name)
            except Exception as e:  # noqa: BLE001
                self._log("发卷失败 %s：%s\n" % (d.name, e), "err")
        plan = build_schedule(root, seeds)
        (root / "赛程.json").write_text(
            json.dumps(plan, ensure_ascii=False, indent=2), encoding="utf-8")
        self._log("发卷：%s\n" % ("、".join(done) if done else "全部已有选手包"), "dim")
        self._log("赛程已冻结：%d 名报名者，%d 场对阵 → %s\n"
                  % (len(plan["players"]), len(plan["matches"]), root / "赛程.json"), "ok")
        self.scan_workspaces()

    def issue_start(self):
        if self.running:
            return
        w = self._need_one("发卷并开赛")
        if not w:
            return
        ws, label = w[0], w[1]
        st = service.status(ws)
        if st["state"] != "未开赛":
            if not messagebox.askokcancel(
                    "重置计时", "%s 已经开赛（%s）。\n重新开赛会把计时起点重置为现在，确认？"
                    % (label, service.describe(ws))):
                return
        try:
            if not st["issued"]:
                got = service.issue(ws)
                self._log("发卷 %s：%s\n" % (label, "、".join(got) if got else "已存在"), "dim")
            service.start(ws)
        except Exception as e:  # noqa: BLE001
            messagebox.showerror("裁判台", "开赛失败：%s" % e)
            return
        self._log("开赛 %s：90 分钟目标 / 120 分钟收卷\n" % label, "ok")
        self.scan_workspaces()

    def pause_timer(self):
        w = self._need_one("暂停")
        if not w:
            return
        service.pause(w[0], "裁判台手动暂停")
        self._log("暂停 %s：已用 %.1f 分钟\n" % (w[1], service.elapsed_seconds(w[0]) / 60), "dim")
        self.scan_workspaces()

    def resume_timer(self):
        w = self._need_one("恢复")
        if not w:
            return
        service.resume(w[0])
        self._log("恢复 %s：已用 %.1f 分钟\n" % (w[1], service.elapsed_seconds(w[0]) / 60), "dim")
        self.scan_workspaces()

    def freeze_timer(self):
        w = self._need_one("收卷")
        if not w:
            return
        st = service.status(w[0])
        if not st["has_submission"]:
            if not messagebox.askokcancel("收卷", "%s 没有 decide.go，会记为未提交。继续？" % w[1]):
                return
        elif not messagebox.askokcancel("收卷", "%s 已用 %.1f 分钟。\n收卷后 submission 里的改动不再进入成绩，确认？"
                                        % (w[1], st["used_min"])):
            return
        info = service.freeze(w[0])
        if info["missing_submission"]:
            self._log("收卷 %s：没有 decide.go，记为未提交\n" % w[1], "err")
        else:
            self._log("收卷 %s：%d 字节，用 %.1f 分钟，sha256=%s\n"
                      % (w[1], info["bytes"], info["used_minutes"], info["sha256"][:16]), "ok")
        self.scan_workspaces()

    def picked(self):
        return [self.workspaces[i] for i in self.lst.curselection()
                if i < len(self.workspaces) and self.workspaces[i][2]]

    def run_contest(self):
        if self.running:
            return
        picked = self.picked()
        if not picked:
            messagebox.showwarning("裁判台", "先勾选至少一个可编译的工作区")
            return
        who = "只跑这一个" if len(picked) == 1 else "评测 %d 个选手" % len(picked)
        if not messagebox.askokcancel(
                who, "%s\n%s\n\n对手：%s%s\n确认开跑？"
                % (who, "\n".join("  · " + w[1] for w in picked), "、".join(REF_BOTS),
                   "" if len(picked) == 1 else " + 彼此")):
            return
        seeds = int(self.seed_count.get() or "1")

        def work():
            try:
                self._contest_pipeline(picked, seeds)
            except Exception as e:  # noqa: BLE001
                self.log_q.put(("line", "编排失败：%s" % e))
                self.log_q.put(("exit", 1, None))

        self._set_running(True)
        self.pbar["value"] = 0
        self.status.set("评测准备中…")
        threading.Thread(target=work, daemon=True).start()

    def _contest_pipeline(self, picked, seeds):
        stamp = time.strftime("%m%d-%H%M%S")
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
            shutil.copytree(path, stage,
                            ignore=shutil.ignore_patterns("bin", "*.exe", ".git", "__pycache__"))
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

        cfg = {
            "name": "Desert Arena · 工作区评测",
            "season": "工作区评测 %s" % stamp,
            "version": "1.5",
            "engine": {"kind": "native", "host": "bots/bin/enginehost.exe",
                       "snapshot": "engine/golden"},
            "rules": {"tickLimit": 1200, "tickMs": 50, "decideMs": 120, "initMs": 2000,
                      "maxStrikes": 3, "strikeMode": "consecutive",
                      "generator": "minecraft", "vision": False},
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
        self._emit(("line", "评测完成 → %s" % out_rel, "ok"))
        # 评测用的是临时编排，build 会把 bots/封存记录.json 改成这场评测的名册。
        # 跑完把正式赛季的封存重建回来，否则赛季页的封存灯会一直红着。
        season = self.cfg_path.get()
        if season:
            self._emit(("line", "恢复正式赛季封存：%s" % season, "dim"))
            self._stream(["go", "run", "./cmd/arena", "build",
                          "-config", "tournament/" + season])
        self._emit(("exit", 0, None))

    # ---------------- 通用执行 ----------------
    def _start(self, steps):
        self._set_running(True)
        self.pbar["value"] = 0
        self.status.set("执行中…")

        def work():
            for argv in steps:
                if not self._stream(argv):
                    return
            self.log_q.put(("exit", 0, None))

        threading.Thread(target=work, daemon=True).start()

    def _stream(self, argv) -> bool:
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
        for line in proc.stdout:
            self._emit(("line", line.rstrip("\n")))
        code = proc.wait()
        if code != 0:
            self._emit(("exit", code, None))
            return False
        return True

    def _exec(self, argv):
        try:
            p = subprocess.run(argv, cwd=str(ENGINE), capture_output=True, text=True,
                               encoding="utf-8", errors="replace", timeout=180,
                               creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
            return p.returncode, (p.stdout or "") + (p.stderr or "")
        except Exception as e:  # noqa: BLE001
            return 1, str(e)

    def stop(self):
        if self.proc and self.proc.poll() is None:
            self.proc.terminate()
            self.status.set("已中止")
            self._log("\n— 已中止（已完成的对局保留在 结果.jsonl）—\n", "err")

    def _tick(self):
        w = self.selected_one()
        if w is None:
            self.clock_lbl.config(text="选中一个工作区查看计时", fg=MUTED)
        else:
            try:
                st = service.status(w[0])
                color = MUTED
                if st["state"] == "进行中":
                    color = OK if st["used_min"] < 90 else BAD
                elif st["state"].startswith("已超时"):
                    color = BAD
                self.clock_lbl.config(text="%s：%s" % (w[1], service.describe(w[0])), fg=color)
            except Exception:  # noqa: BLE001
                pass
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
        for b in self.season_btns.values():
            b.configure(state=state)
        self.stop_btn.configure(state="normal" if running else "disabled")

    def _log(self, text, tag=""):
        self.log.configure(state="normal")
        self.log.insert("end", text, tag)
        self.log.configure(state="disabled")
        self.log.see("end")

    # ---------------- 快捷入口 ----------------
    def open_results(self):
        cfg = self.info.get("cfg") or {}
        self._open_dir(ENGINE / cfg.get("output", "results").replace("/", os.sep))

    def open_contest_results(self):
        if self.last_out:
            self._open_dir(ENGINE / self.last_out)
            return
        base = ENGINE / "results" / "desk"
        dirs = sorted([d for d in base.iterdir() if d.is_dir()]) if base.is_dir() else []
        self._open_dir(dirs[-1] if dirs else base)

    def open_viewer(self):
        p = ENGINE / "回放查看器.html"
        if not p.exists():
            messagebox.showinfo("裁判台", "找不到 回放查看器.html")
            return
        os.startfile(str(p)) if sys.platform == "win32" else subprocess.Popen(["open", str(p)])

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
        state = {"text": None}

        def poll():
            t = app.seal_lbl.cget("text")
            if "检查中" in t:
                app.after(100, poll)
                return
            state["text"] = t
            app.quit()

        app.after(100, poll)
        app.after(30000, app.quit)
        app.mainloop()
        text = state["text"] or app.seal_lbl.cget("text")
        print("SEAL_LABEL=%s" % text)
        print("SMOKE_OK" if ("✓" in text or "✗" in text) else "SMOKE_FAIL")
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
            if d.is_dir() and not d.name.startswith("_"):
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
    if "--smoke-contest" in argv:
        # 无头跑一次评测管线：--smoke-contest <工作区目录> [seed 数]
        i = argv.index("--smoke-contest")
        ws = Path(argv[i + 1]).resolve()
        seeds = int(argv[i + 2]) if len(argv) > i + 2 else 1
        app.headless = True
        app._emit = lambda item: print(item[1] if item[0] == "line" else "EXIT=%s" % item[1])
        src, note = detect_source(ws)
        app._contest_pipeline([(ws, ws.name, src, note)], seeds)
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

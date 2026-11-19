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
标准库单窗口，常驻约 30 MB，零第三方依赖。
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

APP_DIR = Path(__file__).resolve().parent
ENGINE = APP_DIR / "war_of_desert"
DEFAULT_CONTEST_ROOT = APP_DIR / "算法赛"
STAGE_ROOT = ENGINE / "workspace" / "contestants"
REF_BOTS = ["Vanguard", "Bulwark", "Swarm", "Nomad", "Sapper"]

PROGRESS_RE = re.compile(r"\[(\d+)\s*/\s*(\d+)\]")
ANSI_RE = re.compile(r"\x1b\[[0-9;]*m")
SLUG_RE = re.compile(r"[^0-9A-Za-z_\-\u4e00-\u9fa5]+")

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
    """判断一个工作区里可编译的 main 包在哪。返回 (源子目录, 说明)。"""
    for c in (ws, ws / "submission", ws / "src"):
        if not c.is_dir():
            continue
        for f in sorted(c.glob("*.go")):
            try:
                head = f.read_text(encoding="utf-8", errors="replace")[:600]
            except OSError:
                continue
            if "package main" in head and "func main(" in head:
                return ("." if c == ws else c.name), "入口 %s" % f.name
    if not go_files(ws):
        return None, "没有 .go 文件（空工作区？）"
    return None, "没找到 package main"


def slugify(name: str) -> str:
    return SLUG_RE.sub("_", name).strip("_") or "contestant"


def tree_digest(root: Path) -> str:
    h = hashlib.sha256()
    for p in sorted(root.rglob("*.go")):
        h.update(p.relative_to(root).as_posix().encode("utf-8"))
        h.update(b"\x00")
        h.update(p.read_bytes())
        h.update(b"\x00")
    return h.hexdigest()


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

        tk.Label(tab, text="勾选要评测的工作区（可多选，不必全选）；目录名建议 RTS算法赛@<模型名>",
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

        bar = tk.Frame(tab, bg=BG)
        bar.pack(fill="x", padx=12, pady=(8, 10))
        ttk.Button(bar, text="全选", command=self.select_all).pack(side="left")
        ttk.Button(bar, text="清空", command=self.select_none).pack(side="left", padx=6)
        tk.Label(bar, text="seed 数", bg=BG, fg=MUTED).pack(side="left", padx=(16, 4))
        ttk.Combobox(bar, textvariable=self.seed_count, values=("1", "2", "3"),
                     width=4, state="readonly").pack(side="left")
        ttk.Button(bar, text="评测选中", style="Go.TButton",
                   command=self.run_contest).pack(side="left", padx=(16, 0))
        ttk.Button(bar, text="打开评测结果", command=self.open_contest_results).pack(
            side="left", padx=8)
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
        dirs = [root] if detect_source(root)[0] else [
            d for d in sorted(root.iterdir()) if d.is_dir() and not d.name.startswith("_")]
        for d in dirs:
            src, note = detect_source(d)
            label = d.name
            if src is None:
                self.lst.insert("end", "%-42s  × %s" % (label, note))
                self.lst.itemconfig("end", foreground="#8a8f98")
                self.workspaces.append((d, label, None, note))
            else:
                n = len(go_files(d))
                self.lst.insert("end", "%-42s  ✓ %s，%d 个 .go" % (label, note, n))
                self.workspaces.append((d, label, src, note))
        ok = sum(1 for w in self.workspaces if w[2])
        self.status.set("扫描到 %d 个，可用 %d 个" % (len(self.workspaces), ok))
        self._log("扫描 %s：%d 个目录，%d 个可编译\n" % (root, len(self.workspaces), ok), "dim")

    def select_all(self):
        for i, w in enumerate(self.workspaces):
            if w[2]:
                self.lst.selection_set(i)

    def select_none(self):
        self.lst.selection_clear(0, "end")

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
        if not messagebox.askokcancel(
                "评测", "评测 %d 个选手：\n%s\n\n对手：%s + 彼此\n确认开跑？"
                % (len(picked), "\n".join("  · " + w[1] for w in picked), "、".join(REF_BOTS))):
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
        names, manifest = [], []
        STAGE_ROOT.mkdir(parents=True, exist_ok=True)
        for idx, (path, label, src, _note) in enumerate(picked):
            base = label.split("@")[-1] if "@" in label else label
            slug = slugify(base)
            if slug in names:
                slug = "%s_%d" % (slug, idx + 1)
            names.append(slug)
            stage = STAGE_ROOT / slug
            if stage.exists():
                shutil.rmtree(stage)
            shutil.copytree(path, stage,
                            ignore=shutil.ignore_patterns("bin", "*.exe", ".git", "__pycache__"))
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
        # 参考 Bot 必须排在前面：BT 以「名册第一名」为锚（=0），
        # 锚点放成选手的话，每个选手都会是 0 分，跨场无法比较。
        for bot in REF_BOTS:
            cfg["entries"].append({"name": bot, "author": "官方参考 Bot",
                                   "source": "workspace/bots/%s" % bot.lower(),
                                   "bin": "bots/bin/%s.exe" % bot.lower()})
        for slug in names:
            cfg["entries"].append({"name": slug, "author": "选手工作区",
                                   "source": "workspace/contestants/%s" % slug,
                                   "bin": "bots/bin/%s.exe" % slug})

        cfg_rel = "tournament/_desk_%s.json" % stamp
        (ENGINE / cfg_rel).write_text(json.dumps(cfg, ensure_ascii=False, indent=2),
                                      encoding="utf-8")
        out_abs = ENGINE / out_rel
        out_abs.mkdir(parents=True, exist_ok=True)
        (out_abs / "工作区清单.json").write_text(
            json.dumps(manifest, ensure_ascii=False, indent=2), encoding="utf-8")
        self._emit(("line", "编排完成：%d 名选手 + %d 个参考 Bot → %s"
                        % (len(names), len(REF_BOTS), out_rel), "ok"))

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
        self.log_q.put(("line", "$ " + " ".join(argv)))
        try:
            proc = subprocess.Popen(argv, cwd=str(ENGINE), stdout=subprocess.PIPE,
                                    stderr=subprocess.STDOUT, text=True, encoding="utf-8",
                                    errors="replace", bufsize=1,
                                    creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0))
        except Exception as e:  # noqa: BLE001
            self.log_q.put(("line", "启动失败：%s" % e))
            self.log_q.put(("exit", 127, None))
            return False
        self.proc = proc
        assert proc.stdout is not None
        for line in proc.stdout:
            self.log_q.put(("line", line.rstrip("\n")))
        code = proc.wait()
        if code != 0:
            self.log_q.put(("exit", code, None))
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
    if "--smoke-contest" in argv:
        # 无头跑一次评测管线：--smoke-contest <工作区目录> [seed 数]
        i = argv.index("--smoke-contest")
        ws = Path(argv[i + 1]).resolve()
        seeds = int(argv[i + 2]) if len(argv) > i + 2 else 1
        app.headless = True
        app._emit = lambda item: print(item[1] if item[0] == "line" else "EXIT=%s" % item[1])
        app._contest_pipeline([(ws, ws.name, ".", "smoke")], seeds)
        app.destroy()
        return
    if "--scan" in argv:
        for path, label, src, note in app.workspaces:
            print("%-44s src=%-10s %s" % (label, src or "-", note))
        app.destroy()
        return
    app.mainloop()


if __name__ == "__main__":
    main()

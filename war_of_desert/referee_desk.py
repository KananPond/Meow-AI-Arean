#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""Desert Arena · 裁判台（GUI）。

版本：v1.4
功能：把 arena 的执行链包成一个窗口——选赛季、自检、编译封存、冻结赛程、
      开赛、评分、看成绩。所有重活都在子进程里跑，界面只负责显示，
      所以常驻内存只有 tkinter 那点开销（约 30 MB），不加载任何第三方库。

    python -X utf8 裁判台.py

设计取舍：不用 Web/Electron，因为裁判机器往往和比赛机是同一台，
开赛时不该再抢几百 MB 内存。tkinter 是标准库，双击就能跑。
"""

import json
import os
import queue
import re
import subprocess
import sys
import threading
try:
    import tkinter as tk
    from tkinter import font as tkfont
    from tkinter import messagebox, ttk
    from tkinter.scrolledtext import ScrolledText
except ModuleNotFoundError:  # 精简版 Python 常常不带 tk
    sys.stderr.write(
        "这个 Python 没有 tkinter。\n"
        "  · Windows 官方安装包自带；如果是精简发行版，请改用 `py -3 裁判台.py`\n"
        "  · 或者直接双击 裁判台.bat（会自动挑一个带 tk 的解释器）\n"
    )
    raise SystemExit(2)

ROOT = os.path.dirname(os.path.abspath(__file__))
TOURNAMENT_DIR = os.path.join(ROOT, "tournament")
PROGRESS_RE = re.compile(r"\[(\d+)\s*/\s*(\d+)\]")
ANSI_RE = re.compile(r"\x1b\[[0-9;]*m")

BG = "#1e1f22"
PANEL = "#2b2d31"
FG = "#e6e6e6"
MUTED = "#9aa0a6"
ACCENT = "#4a9eff"
OK = "#4caf50"
BAD = "#e05252"


class RefereeDesk(tk.Tk):
    def __init__(self):
        super().__init__()
        self.title("Desert Arena · 裁判台")
        self.geometry("940x660")
        self.minsize(820, 560)
        self.configure(bg=BG)

        self.cfg_path = tk.StringVar()
        self.status = tk.StringVar(value="就绪")
        self.info = {}
        self.proc = None
        self.log_q = queue.Queue()
        self.running = False
        self.total_matches = 0

        self._build_styles()
        self._build_ui()
        self.reload_configs()
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
        style.configure(
            "Go.TButton", padding=(12, 6),
            background="#2f5d3a", foreground="#ffffff",
        )
        style.map("Go.TButton", background=[("active", "#3b7a4a")])
        style.configure("TProgressbar", background=ACCENT)

    # ---------------- 界面 ----------------
    def _build_ui(self):
        head = tk.Frame(self, bg=BG)
        head.pack(fill="x", padx=14, pady=(12, 6))

        tk.Label(head, text="赛季配置", bg=BG, fg=MUTED).pack(side="left")
        self.cmb = ttk.Combobox(head, textvariable=self.cfg_path, width=34, state="readonly")
        self.cmb.pack(side="left", padx=(8, 8))
        self.cmb.bind("<<ComboboxSelected>>", lambda _e: self.on_config_change())
        ttk.Button(head, text="刷新", command=self.reload_configs).pack(side="left")

        self.seal_lbl = tk.Label(head, text="封存 —", bg=BG, fg=MUTED)
        self.seal_lbl.pack(side="right")

        info = tk.Frame(self, bg=PANEL)
        info.pack(fill="x", padx=14, pady=(0, 8))
        self.info_labels = {}
        for col, key in enumerate(("赛季", "规则", "引擎", "选手", "场次")):
            tk.Label(info, text=key, bg=PANEL, fg=MUTED, font=(None, 9)).grid(
                row=0, column=col * 2, sticky="w", padx=(12, 4), pady=(8, 0))
            v = tk.Label(info, text="—", bg=PANEL, fg=FG, font=(None, 10, "bold"))
            v.grid(row=1, column=col * 2, sticky="w", padx=(12, 4), pady=(0, 8))
            self.info_labels[key] = v
        for col in range(5):
            info.grid_columnconfigure(col * 2, weight=1)

        bar = tk.Frame(self, bg=BG)
        bar.pack(fill="x", padx=14, pady=(0, 6))
        self.btns = {}
        for text, cmd, style in (
            ("环境自检", lambda: self.run_cmd("selftest"), "TButton"),
            ("编译封存", lambda: self.run_cmd("build"), "TButton"),
            ("冻结赛程", lambda: self.run_cmd("plan"), "TButton"),
            ("开赛", lambda: self.run_cmd("run"), "Go.TButton"),
            ("评分", lambda: self.run_cmd("rate"), "TButton"),
        ):
            b = ttk.Button(bar, text=text, command=cmd, style=style)
            b.pack(side="left", padx=(0, 8))
            self.btns[text] = b

        self.stop_btn = ttk.Button(bar, text="中止", command=self.stop, state="disabled")
        self.stop_btn.pack(side="left", padx=(8, 0))

        right = tk.Frame(bar, bg=BG)
        right.pack(side="right")
        ttk.Button(right, text="成绩目录", command=self.open_results).pack(side="right")
        ttk.Button(right, text="回放查看器", command=self.open_viewer).pack(side="right", padx=(0, 8))

        prog = tk.Frame(self, bg=BG)
        prog.pack(fill="x", padx=14)
        self.pbar = ttk.Progressbar(prog, mode="determinate", maximum=100)
        self.pbar.pack(side="left", fill="x", expand=True)
        tk.Label(prog, textvariable=self.status, bg=BG, fg=MUTED, width=28, anchor="e").pack(side="right")

        box = tk.Frame(self, bg=BG)
        box.pack(fill="both", expand=True, padx=14, pady=(6, 12))
        self.log = ScrolledText(
            box, bg="#141517", fg=FG, insertbackground=FG,
            font=self.log_font, relief="flat", wrap="none", height=18,
        )
        self.log.pack(fill="both", expand=True)
        self.log.tag_config("err", foreground=BAD)
        self.log.tag_config("ok", foreground=OK)
        self.log.tag_config("dim", foreground=MUTED)
        self.log.configure(state="disabled")

    # ---------------- 配置 ----------------
    def reload_configs(self):
        items = []
        if os.path.isdir(TOURNAMENT_DIR):
            items = sorted(f for f in os.listdir(TOURNAMENT_DIR)
                           if f.endswith(".json") and not f.startswith("_"))
        self.cmb["values"] = items
        if items:
            current = self.cfg_path.get()
            pick = current if current in items else items[-1]
            self.cfg_path.set(pick)
            self.on_config_change()

    def on_config_change(self):
        path = os.path.join(TOURNAMENT_DIR, self.cfg_path.get())
        try:
            with open(path, encoding="utf-8") as f:
                cfg = json.load(f)
        except Exception as e:  # noqa: BLE001
            self._log("读配置失败：%s\n" % e, "err")
            return
        self.info["cfg"] = cfg
        self.info_labels["赛季"].config(text=cfg.get("season", "—"))
        self.info_labels["规则"].config(text=cfg.get("version", "—"))
        self.info_labels["引擎"].config(text=cfg.get("engine", {}).get("kind", "—"))
        self.info_labels["选手"].config(text=str(len(cfg.get("entries", []))))

        out = os.path.join(ROOT, cfg.get("output", "").replace("/", os.sep))
        self.total_matches = 0
        plan = os.path.join(out, "赛程.json")
        if os.path.exists(plan):
            try:
                with open(plan, encoding="utf-8") as f:
                    self.total_matches = len(json.load(f).get("matches", []))
            except Exception:  # noqa: BLE001
                pass
        self.info_labels["场次"].config(text=str(self.total_matches) if self.total_matches else "未冻结")
        self.pbar["value"] = 0
        self.check_seal()

    # ---------------- 封存状态灯 ----------------
    def check_seal(self):
        self.seal_lbl.config(text="封存 检查中…", fg=MUTED)
        cfg = self.cfg_path.get()
        if not cfg:
            return

        def work():
            code, out = self._exec(["go", "run", "./cmd/arena", "seal",
                                    "-config", "tournament/" + cfg], capture=True)
            def apply():
                if code == 0:
                    self.seal_lbl.config(text="封存 ✓", fg=OK)
                else:
                    self.seal_lbl.config(text="封存 ✗", fg=BAD)
                    self._log(out.strip() + "\n", "err" if code else "dim")
            self.after(0, apply)

        threading.Thread(target=work, daemon=True).start()

    # ---------------- 执行 ----------------
    def run_cmd(self, sub):
        if self.running:
            return
        cfg = self.cfg_path.get()
        if not cfg:
            messagebox.showwarning("裁判台", "先选一个赛季配置")
            return
        if sub == "run" and self.total_matches == 0:
            if not messagebox.askokcancel("未冻结赛程", "还没有 赛程.json，开赛会失败。\n要先跑「冻结赛程」吗？"):
                return
            self.run_cmd("plan")
            return
        if sub == "run":
            if not messagebox.askokcancel("开赛", "按冻结赛程跑完所有对局，确认开赛？"):
                return
        self._start(["go", "run", "./cmd/arena", sub, "-config", "tournament/" + cfg])

    def _start(self, argv):
        self._set_running(True)
        self.pbar["value"] = 0
        self.status.set("执行中…")
        self._log("$ " + " ".join(argv) + "\n", "dim")

        def work():
            try:
                proc = subprocess.Popen(
                    argv, cwd=ROOT, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
                    text=True, encoding="utf-8", errors="replace", bufsize=1,
                    creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0),
                )
            except Exception as e:  # noqa: BLE001
                self.log_q.put(("exit", 127, str(e)))
                return
            self.proc = proc
            assert proc.stdout is not None
            for line in proc.stdout:
                self.log_q.put(("line", line.rstrip("\n")))
            code = proc.wait()
            self.log_q.put(("exit", code, None))

        threading.Thread(target=work, daemon=True).start()

    def _exec(self, argv, capture=False):
        try:
            p = subprocess.run(
                argv, cwd=ROOT, capture_output=capture, text=True,
                encoding="utf-8", errors="replace",
                creationflags=getattr(subprocess, "CREATE_NO_WINDOW", 0),
            )
            return p.returncode, (p.stdout or "") + (p.stderr or "")
        except Exception as e:  # noqa: BLE001
            return 1, str(e)

    def stop(self):
        if self.proc and self.proc.poll() is None:
            self.proc.terminate()
            self.status.set("已中止")
            self._log("\n— 已中止（已跑完的对局保留在 结果.jsonl）—\n", "err")

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
                if line.startswith("错误") or "✗" in line or "FAIL" in line:
                    tag = "err"
                elif "完成" in line or "OK" in line or "PASS" in line:
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
                    self.pbar["value"] = 100 if self.total_matches else self.pbar["value"]
                else:
                    self.status.set("失败（退出码 %s）" % code)
                    self._log("— 失败，退出码 %s —\n" % code, "err")
                if item[2]:
                    self._log(str(item[2]) + "\n", "err")
                self._set_running(False)
                self.on_config_change()
                dirty = True
        if dirty:
            self.log.see("end")
        self.after(80, self._drain_log)

    def _set_running(self, running):
        self.running = running
        state = "disabled" if running else "normal"
        for b in self.btns.values():
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
        out = os.path.join(ROOT, cfg.get("output", "results").replace("/", os.sep))
        if not os.path.isdir(out):
            messagebox.showinfo("裁判台", "还没有结果目录：%s" % out)
            return
        os.startfile(out) if sys.platform == "win32" else subprocess.Popen(["open", out])

    def open_viewer(self):
        p = os.path.join(ROOT, "回放查看器.html")
        if not os.path.exists(p):
            messagebox.showinfo("裁判台", "找不到 回放查看器.html")
            return
        os.startfile(p) if sys.platform == "win32" else subprocess.Popen(["open", p])


def main():
    app = RefereeDesk()
    if "--selftest" in sys.argv:
        app.update_idletasks()
        app.update()
        app.destroy()
        print("GUI_OK")
        return
    if "--smoke" in sys.argv:
        # 走真实事件循环：验证子进程 + 线程 + after 回调 + 状态灯整条链路
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
        ok = ("✓" in text) or ("✗" in text)
        print("SMOKE_OK" if ok else "SMOKE_FAIL")
        app.destroy()
        return
    if "--smoke-cmd" in sys.argv:
        # 驱动一条真实命令（默认 selftest），验证流式日志与进度条链路
        sub = sys.argv[sys.argv.index("--smoke-cmd") + 1]
        app.run_cmd(sub)
        state = {"done": False}

        def wait():
            if app.running:
                app.after(120, wait)
                return
            app.quit()

        app.after(200, wait)
        app.after(180000, app.quit)
        app.mainloop()
        body = app.log.get("1.0", "end")
        print("STATUS=%s" % app.status.get())
        print("LOG_LINES=%d" % body.count("\n"))
        tail = [l for l in body.strip().splitlines() if l.strip()][-1:]
        print("LAST=%s" % (tail[0] if tail else ""))
        app.destroy()
        return

    app.mainloop()


if __name__ == "__main__":
    main()

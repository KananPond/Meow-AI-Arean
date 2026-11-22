#!/usr/bin/env python
# -*- coding: utf-8 -*-
"""赛务与裁判台核心回归测试（纯标准库 unittest，零第三方依赖）。

覆盖：
  1. 120 分钟到点自动收卷不误杀截止前已保存的交付物，同时拦截截止后改动；
  2. 工作区暂存对已收卷选手严格校验 accepted / SHA-256，且只放入单文件交卷；
  3. 对战确认框与评测管线共用对手解析与局数计算；
  4. referee_desk.py 命令行（--selfcheck / --smoke-contest / --selftest）退出码透传。
"""

from __future__ import annotations

import importlib.util
import json
import os
import shutil
import subprocess
import sys
import tempfile
import time
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parent


def _load_module(name: str, path: Path):
    spec = importlib.util.spec_from_file_location(name, path)
    mod = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(mod)
    return mod


service = _load_module("wod_service_test", ROOT / "赛务.py")
desk = _load_module("wod_desk_test", ROOT / "referee_desk.py")


def _set_mtime(path: Path, ts: float) -> None:
    os.utime(path, (ts, ts))


class TestAutoFreezeEligibility(unittest.TestCase):
    def test_ontime_file_accepted_when_archived_after_deadline(self):
        with tempfile.TemporaryDirectory(prefix="wod-freeze-ok-") as td:
            ws = Path(td) / "RTS算法赛@OnTime"
            service.issue(ws)
            service.start(ws)

            now = time.time()
            # 模拟 120 分钟截止点已过去 0.5 秒（裁判台每秒轮询触发自动封存的典型场景）
            started = now - service.LIMIT_MINUTES * 60 - 0.5
            deadline = started + service.LIMIT_MINUTES * 60
            rec = service.load(ws)
            rec["started_at"] = started
            service.save(ws, rec)

            sub = service.submission_path(ws)
            _set_mtime(sub, deadline - 5.0)

            self.assertEqual(service.status(ws)["state"], "已超时（待收卷）")
            info = service.freeze(ws, auto=True)
            self.assertTrue(info["accepted"], f"截止前保存的文件不应被自动收卷判超时: {info}")
            self.assertEqual(info["used_minutes"], float(service.LIMIT_MINUTES))
            self.assertTrue(info["auto_frozen"])
            self.assertLessEqual(info["submitted_at"], info["deadline_at"])

    def test_post_deadline_edit_rejected(self):
        with tempfile.TemporaryDirectory(prefix="wod-freeze-late-") as td:
            ws = Path(td) / "RTS算法赛@LateEdit"
            service.issue(ws)
            service.start(ws)

            now = time.time()
            started = now - service.LIMIT_MINUTES * 60 - 10.0
            rec = service.load(ws)
            rec["started_at"] = started
            service.save(ws, rec)

            # 截止 10 秒后修改了交付文件
            sub = service.submission_path(ws)
            sub.write_text(sub.read_text(encoding="utf-8") + "\n// late edit\n", encoding="utf-8")
            _set_mtime(sub, now)

            info = service.freeze(ws, auto=True)
            self.assertFalse(info["accepted"], f"截止后修改的文件必须判超时: {info}")
            self.assertGreater(info["submitted_at"], info["deadline_at"])

    def test_pause_extends_deadline_fairly(self):
        with tempfile.TemporaryDirectory(prefix="wod-freeze-pause-") as td:
            ws = Path(td) / "RTS算法赛@Paused"
            service.issue(ws)
            service.start(ws)

            now = time.time()
            # 暂停了 10 分钟：实际墙钟走了 130 分 + 1 秒，逻辑截止时刻在 1 秒前
            started = now - (service.LIMIT_MINUTES + 10) * 60 - 1.0
            deadline = started + 600.0 + service.LIMIT_MINUTES * 60
            rec = service.load(ws)
            rec["started_at"] = started
            rec["paused_seconds"] = 600.0
            service.save(ws, rec)

            sub = service.submission_path(ws)
            _set_mtime(sub, deadline - 30.0)

            info = service.freeze(ws, auto=True)
            self.assertTrue(info["accepted"], f"计入暂停顺延后应判按时交付: {info}")


class TestStagingAndOpponents(unittest.TestCase):
    def test_frozen_staging_checks_eligibility_hash_and_isolates_extra_go(self):
        with tempfile.TemporaryDirectory(prefix="wod-stage-") as td:
            td_path = Path(td)
            ws = td_path / "RTS算法赛@CleanBot"
            service.issue(ws)
            service.start(ws)
            info = service.freeze(ws)
            self.assertTrue(info["accepted"])

            # 在工作区额外放入一个多余的 .go 文件，模拟收卷后篡改或附带其他源码
            extra = ws / "workspace" / "submission" / "extra_unused.go"
            extra.write_text("package main\nTHIS IS INVALID GO SYNTAX\n", encoding="utf-8")

            stage = td_path / "stage_ok"
            staged = desk.stage_contestant(ws, stage, "workspace/submission")
            self.assertTrue(staged["frozen"])
            staged_files = sorted(p.name for p in (stage / "workspace" / "submission").glob("*.go"))
            self.assertEqual(staged_files, ["CleanBot_bot.go"])

            # 篡改 交卷/ 里的代码但未更新交卷记录哈希：必须拒绝
            frozen_go = ws / "交卷" / "CleanBot_bot.go"
            orig_bytes = frozen_go.read_bytes()
            frozen_go.write_bytes(orig_bytes + b"\n// tampered\n")
            with self.assertRaises(ValueError) as cm:
                desk.stage_contestant(ws, td_path / "stage_tamper", "workspace/submission")
            self.assertIn("哈希", str(cm.exception))
            frozen_go.write_bytes(orig_bytes)

            # 交卷记录 accepted=False：必须拒绝
            rec_path = ws / "交卷" / "交卷记录.json"
            rec = json.loads(rec_path.read_text(encoding="utf-8"))
            rec["accepted"] = False
            rec_path.write_text(json.dumps(rec, ensure_ascii=False, indent=2), encoding="utf-8")
            with self.assertRaises(ValueError) as cm2:
                desk.stage_contestant(ws, td_path / "stage_rejected", "workspace/submission")
            self.assertIn("accepted", str(cm2.exception))

    def test_unfrozen_staging_marks_practice_mode(self):
        with tempfile.TemporaryDirectory(prefix="wod-unfrozen-") as td:
            td_path = Path(td)
            ws = td_path / "RTS算法赛@Practice"
            service.issue(ws)
            stage = td_path / "stage_practice"
            staged = desk.stage_contestant(ws, stage, "workspace/submission")
            self.assertFalse(staged["frozen"])
            self.assertTrue((stage / "workspace" / "submission" / "Practice_bot.go").is_file())
            self.assertFalse((stage / "workspace" / "arena").exists())

    def test_opponents_and_match_counts_match_actual_layout(self):
        # 仓库默认 算法赛/ 带 2 个预编译基线：1 选手 + 2 基线 = 3 人 → 每 seed 3 场 6 局
        opp_names, baselines = desk.resolve_opponents(ROOT / "算法赛")
        if baselines:
            self.assertEqual(len(opp_names), len(baselines))
            m1, g1 = desk.contest_match_counts(1, len(opp_names), 1)
            m3, g3 = desk.contest_match_counts(1, len(opp_names), 3)
            k = 1 + len(opp_names)
            self.assertEqual((m1, g1), (k * (k - 1) // 2, k * (k - 1)))
            self.assertEqual((m3, g3), (m1 * 3, g1 * 3))

        # 无 baseline@*.exe 的目录回退到 REF_BOTS（7 个参考 Bot）
        with tempfile.TemporaryDirectory(prefix="wod-empty-root-") as td:
            opp_names2, baselines2 = desk.resolve_opponents(Path(td))
            self.assertEqual(baselines2, [])
            self.assertEqual(opp_names2, desk.REF_BOTS)
            self.assertEqual(desk.contest_match_counts(1, len(opp_names2), 2), (56, 112))

    def test_run_contest_dialog_shows_real_opponents_and_seed_multiplied_counts(self):
        app = desk.RefereeDesk()
        captured = {}
        orig_ask = desk.messagebox.askokcancel
        try:
            app.headless = True
            app.contest_root.set(str(ROOT / "算法赛"))
            app.scan_workspaces()
            app.seed_count.set("2")
            picked_idx = next(i for i, w in enumerate(app.workspaces) if w[2])
            app.battle_lst.selection_clear(0, "end")
            app.battle_lst.selection_set(picked_idx)

            def fake_ask(title, msg, **kwargs):
                captured["title"] = title
                captured["msg"] = msg
                return False  # 取消，不真正开跑

            desk.messagebox.askokcancel = fake_ask
            app.run_contest()
        finally:
            desk.messagebox.askokcancel = orig_ask
            app.destroy()

        opp_names, _ = desk.resolve_opponents(ROOT / "算法赛")
        exp_matches, exp_games = desk.contest_match_counts(1, len(opp_names), 2)
        self.assertIn("、".join(opp_names), captured["msg"])
        self.assertIn(f"2 个 seed，将进行 {exp_matches} 场对阵、{exp_games} 局真实对局", captured["msg"])


class TestRefereeDeskCLI(unittest.TestCase):
    def _run_desk(self, *args: str) -> subprocess.CompletedProcess:
        cmd = [sys.executable, "-X", "utf8", "-B", str(ROOT / "referee_desk.py"), *args]
        return subprocess.run(cmd, cwd=str(ROOT), capture_output=True, text=True,
                              encoding="utf-8", errors="replace", timeout=180)

    def test_selftest_exits_zero(self):
        p = self._run_desk("--selftest")
        self.assertEqual(p.returncode, 0, p.stdout + p.stderr)
        self.assertIn("GUI_OK", p.stdout)

    def test_selfcheck_exit_code_reflects_compile_result(self):
        diag = ROOT / "自检诊断.txt"
        had_diag = diag.exists()
        try:
            with tempfile.TemporaryDirectory(prefix="wod-cli-check-") as td:
                td_path = Path(td)
                # 合法工作区：退出码应为 0
                ok_ws = td_path / "RTS算法赛@GoodCLI"
                service.issue(ok_ws)
                p_ok = self._run_desk("--selfcheck", str(ok_ws))
                self.assertEqual(p_ok.returncode, 0, p_ok.stdout + p_ok.stderr)
                self.assertIn("自检通过", p_ok.stdout)

                # 语法错误工作区：必须返回非零退出码（旧代码漏写 sys.exit 恒返回 0）
                bad_ws = td_path / "RTS算法赛@BadCLI"
                service.issue(bad_ws)
                service.submission_path(bad_ws).write_text("package main\nfunc main(\n", encoding="utf-8")
                p_bad = self._run_desk("--selfcheck", str(bad_ws))
                self.assertNotEqual(p_bad.returncode, 0, p_bad.stdout + p_bad.stderr)
                self.assertIn("自检未通过", p_bad.stdout)
        finally:
            if not had_diag and diag.exists():
                diag.unlink(missing_ok=True)

    def test_smoke_contest_rejects_unaccepted_and_isolates_frozen_submission(self):
        engine = ROOT / "war_of_desert"
        desk_results = engine / "results" / "desk"
        tourn = engine / "tournament"
        seals = engine / "bots" / "封存记录"
        stage_root = engine / "workspace" / "_contestants"

        before_results = set(desk_results.iterdir()) if desk_results.is_dir() else set()
        before_cfgs = set(tourn.glob("_desk_*.json"))
        before_stages = set(stage_root.iterdir()) if stage_root.is_dir() else set()
        before_seals = set(seals.glob("*.json")) if seals.is_dir() else set()

        try:
            with tempfile.TemporaryDirectory(prefix="wod-cli-smoke-") as td:
                td_path = Path(td)

                # 1) accepted=False 的已收卷选手：--smoke-contest 必须拒绝并返回非零退出码
                rej_ws = td_path / "RTS算法赛@RejectedBot"
                service.issue(rej_ws)
                service.start(rej_ws)
                service.freeze(rej_ws)
                rec_path = rej_ws / "交卷" / "交卷记录.json"
                rec = json.loads(rec_path.read_text(encoding="utf-8"))
                rec["accepted"] = False
                rec_path.write_text(json.dumps(rec, ensure_ascii=False, indent=2), encoding="utf-8")

                p_rej = self._run_desk("--smoke-contest", str(rej_ws), "1", "v1.4")
                self.assertNotEqual(p_rej.returncode, 0, p_rej.stdout + p_rej.stderr)
                self.assertIn("拒绝进入评测", p_rej.stdout)

                # 2) 合法收卷 + 工作区混入语法损坏的 extra_poison.go：
                #    只暂存单文件交卷，应无视多余文件、正常编跑完 3 场 6 局并返回 0
                ok_ws = td_path / "RTS算法赛@FrozenClean"
                service.issue(ok_ws)
                service.start(ok_ws)
                info = service.freeze(ok_ws)
                self.assertTrue(info["accepted"])
                (ok_ws / "workspace" / "submission" / "extra_poison.go").write_text(
                    "package main\nBROKEN SYNTAX THAT MUST NOT BE STAGED\n", encoding="utf-8"
                )

                p_ok = self._run_desk("--smoke-contest", str(ok_ws), "1", "v1.4")
                self.assertEqual(p_ok.returncode, 0, p_ok.stdout + p_ok.stderr)
                self.assertIn("使用已校验收卷副本", p_ok.stdout)
                self.assertIn("评测完成", p_ok.stdout)
        finally:
            # 清理测试产生的临时评测产物，保持仓库工作树干净
            if desk_results.is_dir():
                for item in set(desk_results.iterdir()) - before_results:
                    shutil.rmtree(item, ignore_errors=True)
            for cfg in set(tourn.glob("_desk_*.json")) - before_cfgs:
                cfg.unlink(missing_ok=True)
            if stage_root.is_dir():
                for item in set(stage_root.iterdir()) - before_stages:
                    shutil.rmtree(item, ignore_errors=True)
            if seals.is_dir():
                for item in set(seals.glob("*.json")) - before_seals:
                    item.unlink(missing_ok=True)


if __name__ == "__main__":
    unittest.main(verbosity=2)

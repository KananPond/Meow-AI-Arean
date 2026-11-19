@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

rem 优先用 py 启动器（官方安装包一定带 tkinter），退回 PATH 上的 python。
where py >nul 2>nul
if not errorlevel 1 (
  py -3 -X utf8 "referee_desk.py" %*
  if not errorlevel 1 goto :end
)
python -X utf8 "referee_desk.py" %*
if errorlevel 1 (
  echo.
  echo 启动失败：当前 Python 可能缺少 tkinter。
  echo 请安装 Windows 官方 Python，或用 "py -3 -X utf8 referee_desk.py" 手动启动。
  pause
)
:end
endlocal

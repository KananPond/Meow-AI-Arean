@echo off
rem ASCII only: cmd parses .bat with the console codepage, so non-ASCII
rem characters in this file would be split into bogus commands on a GBK box.
chcp 65001 >nul
setlocal
cd /d "%~dp0"

echo ============================================================
echo   Desert Arena - local match
echo   engine + judge + match runner: arena\ (same source as the contest)
echo ============================================================
echo.

echo [1/3] Building submission ...
go build -o bot.exe .\submission
if errorlevel 1 (
  echo.
  echo [x] Build failed. Fix the errors above.
  pause
  exit /b 1
)

echo [2/3] Rules identity ...
pushd arena
go run ./cmd/arena version
popd

echo.
echo [3/3] Your bot vs baseline@1 / baseline@2 (2 seeds, side swap)
echo.
pushd arena
go run ./cmd/arena local -bot ..\bot.exe -vs ..\bots\baseline@1.exe,..\bots\baseline@2.exe -ruleset v15 -seeds 2 -out ..\matches\local
set RC=%errorlevel%
popd

echo.
if not "%RC%"=="0" (
  echo [x] Match run failed. See the errors above.
) else (
  echo Replays and per-game results: matches\local
)
pause

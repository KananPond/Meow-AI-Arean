@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

echo ============================================================
echo   Desert Arena 本地对局
echo   引擎 / 裁判 / 比赛程序 = arena\ 下的正式赛源码
echo ============================================================
echo.

echo [1/3] 编译 submission\*.go ...
go build -o bot.exe .\submission
if errorlevel 1 (
  echo.
  echo [x] 编译没通过，先修上面的报错。
  pause
  exit /b 1
)

echo [2/3] 规则版本 ...
pushd arena
go run ./cmd/arena version
popd

echo.
echo [3/3] 对局：你的 Bot vs baseline@1 / baseline@2（2 个 seed x 换边两局）
echo.
pushd arena
go run ./cmd/arena local -bot ..\bot.exe -vs ..\bots\baseline@1.exe,..\bots\baseline@2.exe -seeds 2 -out ..\对局记录\本地对局
set RC=%errorlevel%
popd

echo.
if not "%RC%"=="0" (
  echo [x] 对局失败，看上面的报错。
) else (
  echo 回放与逐局结果：对局记录\本地对局
)
pause

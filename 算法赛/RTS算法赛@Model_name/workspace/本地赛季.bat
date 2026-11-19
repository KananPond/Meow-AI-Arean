@echo off
chcp 65001 >nul
setlocal
cd /d "%~dp0"

echo ============================================================
echo   Desert Arena 本地赛季
echo   与正式赛同一套：冻结赛程 -^> 跑完 -^> BT 排名 + 七领域加权总分
echo ============================================================
echo.

go build -o bot.exe .\submission
if errorlevel 1 (
  echo [x] 编译没通过，先修上面的报错。
  pause
  exit /b 1
)

pushd arena

echo.
echo [1/3] 冻结赛程 ...
go run ./cmd/arena plan -config ..\tournament\本地赛.json
if errorlevel 1 goto :done

echo.
echo [2/3] 跑完所有对局 ...
go run ./cmd/arena run -config ..\tournament\本地赛.json -skip-seal
if errorlevel 1 goto :done

echo.
echo [3/3] 评分 ...
go run ./cmd/arena rate -config ..\tournament\本地赛.json

:done
popd
echo.
echo 成绩：对局记录\本地赛季\成绩.md
echo 回放：对局记录\本地赛季\回放\
pause
